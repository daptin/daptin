package server

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
)

func InitializeOAuthResources(cruds map[string]*resource.DbResource, configStore *resource.ConfigStore, defaultRouter *gin.Engine) {
	provider := resource.NewOAuthProvider(cruds, configStore)

	defaultRouter.GET("/.well-known/oauth-authorization-server", oauthMetadataHandler(provider, false))
	defaultRouter.GET("/.well-known/openid-configuration", oauthMetadataHandler(provider, true))
	defaultRouter.GET("/oauth/jwks", func(c *gin.Context) {
		transaction, err := provider.BeginTransaction()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
			return
		}
		defer transaction.Rollback()
		keys, err := provider.JWKS(transaction)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
			return
		}
		if err := transaction.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"keys": keys})
	})
	defaultRouter.GET("/oauth/authorize", oauthAuthorizeHandler(provider, configStore))
	defaultRouter.POST("/oauth/token", oauthTokenHandler(provider))
	defaultRouter.POST("/oauth/revoke", oauthRevokeHandler(provider))
	defaultRouter.POST("/oauth/introspect", oauthIntrospectHandler(provider))
	defaultRouter.GET("/oauth/userinfo", oauthUserinfoHandler(provider))
	defaultRouter.POST("/oauth/userinfo", oauthUserinfoHandler(provider))
}

func oauthMetadataHandler(provider *resource.OAuthProvider, openid bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		transaction, err := provider.BeginTransaction()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
			return
		}
		defer transaction.Rollback()

		issuer := provider.Issuer(c.Request, transaction)
		response := gin.H{
			"issuer":                                issuer,
			"authorization_endpoint":                issuer + "/oauth/authorize",
			"token_endpoint":                        issuer + "/oauth/token",
			"revocation_endpoint":                   issuer + "/oauth/revoke",
			"introspection_endpoint":                issuer + "/oauth/introspect",
			"scopes_supported":                      []string{"openid", "profile", "email"},
			"response_types_supported":              []string{"code"},
			"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
			"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post", "none"},
			"code_challenge_methods_supported":      []string{"plain", "S256"},
		}
		if openid {
			response["userinfo_endpoint"] = issuer + "/oauth/userinfo"
			response["jwks_uri"] = issuer + "/oauth/jwks"
			response["subject_types_supported"] = []string{"public"}
			response["id_token_signing_alg_values_supported"] = []string{"RS256"}
		}
		c.JSON(http.StatusOK, response)
	}
}

func oauthAuthorizeHandler(provider *resource.OAuthProvider, configStore *resource.ConfigStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		transaction, err := provider.BeginTransaction()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
			return
		}
		defer transaction.Rollback()

		responseType := c.Query("response_type")
		clientID := c.Query("client_id")
		redirectURI := c.Query("redirect_uri")
		scope := c.Query("scope")
		state := c.Query("state")
		codeChallenge := c.Query("code_challenge")
		codeChallengeMethod := c.Query("code_challenge_method")
		nonce := c.Query("nonce")

		if responseType != "code" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported_response_type"})
			return
		}

		app, err := provider.GetAppByClientID(clientID, transaction)
		if err != nil || !oauthEndpointBool(app["is_enabled"]) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client"})
			return
		}
		if !provider.ValidateRedirectURI(app, redirectURI) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "error_description": "invalid redirect_uri"})
			return
		}
		if !provider.HasGrant(app, "authorization_code") {
			redirectOAuthError(c, redirectURI, "unauthorized_client", state)
			return
		}
		if codeChallenge == "" {
			redirectOAuthError(c, redirectURI, "invalid_request", state)
			return
		}
		normalizedScope, err := provider.NormalizeScopes(app, scope)
		if err != nil {
			redirectOAuthError(c, redirectURI, "invalid_scope", state)
			return
		}
		if codeChallengeMethod != "" && !strings.EqualFold(codeChallengeMethod, "plain") && !strings.EqualFold(codeChallengeMethod, "S256") {
			redirectOAuthError(c, redirectURI, "invalid_request", state)
			return
		}

		sessionUser, _ := c.Request.Context().Value("user").(*auth.SessionUser)
		if sessionUser == nil || sessionUser.UserId == 0 {
			loginURL := "/auth/signin"
			if configuredLoginURL, err := configStore.GetConfigValueFor("oauth.login_url", "backend", transaction); err == nil && configuredLoginURL != "" {
				loginURL = configuredLoginURL
			}
			location := appendQuery(loginURL, "return_to", c.Request.URL.RequestURI())
			c.Redirect(http.StatusFound, location)
			return
		}

		code, err := provider.CreateCode(sessionUser, app, redirectURI, normalizedScope, codeChallenge, codeChallengeMethod, nonce, transaction)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
			return
		}
		if err := transaction.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
			return
		}

		redirectURL := appendQuery(redirectURI, "code", code)
		if state != "" {
			redirectURL = appendQuery(redirectURL, "state", state)
		}
		c.Redirect(http.StatusFound, redirectURL)
	}
}

func oauthTokenHandler(provider *resource.OAuthProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		transaction, err := provider.BeginTransaction()
		if err != nil {
			oauthTokenError(c, http.StatusInternalServerError, "server_error")
			return
		}
		defer transaction.Rollback()

		clientID, clientSecret := oauthClientCredentials(c)
		if clientID == "" {
			clientID = c.PostForm("client_id")
			clientSecret = c.PostForm("client_secret")
		}

		app, err := provider.AuthenticateClient(clientID, clientSecret, transaction)
		if err != nil {
			oauthTokenError(c, http.StatusUnauthorized, "invalid_client")
			return
		}

		grantType := c.PostForm("grant_type")
		var issued *resource.OAuthIssuedToken
		var codeRow map[string]interface{}
		var userRow map[string]interface{}

		switch grantType {
		case "authorization_code":
			if !provider.HasGrant(app, "authorization_code") {
				oauthTokenError(c, http.StatusBadRequest, "unauthorized_client")
				return
			}
			issued, codeRow, userRow, err = provider.ExchangeCode(app, c.PostForm("code"), c.PostForm("redirect_uri"), c.PostForm("code_verifier"), transaction)
		case "refresh_token":
			if !provider.HasGrant(app, "refresh_token") {
				oauthTokenError(c, http.StatusBadRequest, "unauthorized_client")
				return
			}
			issued, userRow, err = provider.Refresh(app, c.PostForm("refresh_token"), transaction)
		default:
			oauthTokenError(c, http.StatusBadRequest, "unsupported_grant_type")
			return
		}
		if err != nil {
			oauthTokenError(c, http.StatusBadRequest, "invalid_grant")
			return
		}

		if err := transaction.Commit(); err != nil {
			oauthTokenError(c, http.StatusInternalServerError, "server_error")
			return
		}

		response := gin.H{
			"access_token":  issued.AccessToken,
			"token_type":    "Bearer",
			"expires_in":    issued.ExpiresIn,
			"refresh_token": issued.RefreshToken,
			"scope":         issued.Scope,
		}
		if scopeHas(issued.Scope, "openid") {
			if idToken, err := makeIDToken(provider, c.Request, app, userRow, codeRow); err == nil {
				response["id_token"] = idToken
			}
		}
		c.Header("Cache-Control", "no-store")
		c.Header("Pragma", "no-cache")
		c.JSON(http.StatusOK, response)
	}
}

func oauthRevokeHandler(provider *resource.OAuthProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		transaction, err := provider.BeginTransaction()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
			return
		}
		defer transaction.Rollback()

		clientID, clientSecret := oauthClientCredentials(c)
		if clientID == "" {
			clientID = c.PostForm("client_id")
			clientSecret = c.PostForm("client_secret")
		}
		if _, err := provider.AuthenticateClient(clientID, clientSecret, transaction); err != nil {
			c.Header("WWW-Authenticate", `Basic realm="oauth"`)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_client"})
			return
		}

		_ = provider.RevokeToken(c.PostForm("token"), transaction)
		if err := transaction.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
			return
		}
		c.Status(http.StatusOK)
	}
}

func oauthIntrospectHandler(provider *resource.OAuthProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		transaction, err := provider.BeginTransaction()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
			return
		}
		defer transaction.Rollback()

		clientID, clientSecret := oauthClientCredentials(c)
		if clientID == "" {
			clientID = c.PostForm("client_id")
			clientSecret = c.PostForm("client_secret")
		}
		app, err := provider.AuthenticateClient(clientID, clientSecret, transaction)
		if err != nil {
			c.Header("WWW-Authenticate", `Basic realm="oauth"`)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_client"})
			return
		}

		accessRow, userRow, err := provider.ValidateAccessToken(c.PostForm("token"), transaction)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"active": false})
			return
		}
		if !provider.RowBelongsToApp(accessRow, app) {
			c.JSON(http.StatusOK, gin.H{"active": false})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"active":     true,
			"scope":      accessRow["scope"],
			"client_id":  app["client_id"],
			"token_type": "Bearer",
			"exp":        accessRow["expires_at"],
			"sub":        daptinid.InterfaceToDIR(userRow["reference_id"]).String(),
			"username":   userRow["email"],
		})
	}
}

func oauthUserinfoHandler(provider *resource.OAuthProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		transaction, err := provider.BeginTransaction()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
			return
		}
		defer transaction.Rollback()

		token := bearerToken(c.Request)
		if token == "" {
			c.Header("WWW-Authenticate", `Bearer realm="oauth"`)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
			return
		}
		accessRow, userRow, err := provider.ValidateAccessToken(token, transaction)
		if err != nil {
			c.Header("WWW-Authenticate", `Bearer realm="oauth", error="invalid_token"`)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
			return
		}

		response := gin.H{"sub": daptinid.InterfaceToDIR(userRow["reference_id"]).String()}
		scope := fmt.Sprintf("%v", accessRow["scope"])
		if scopeHas(scope, "profile") {
			response["name"] = userRow["name"]
		}
		if scopeHas(scope, "email") {
			response["email"] = userRow["email"]
			response["email_verified"] = true
		}
		c.JSON(http.StatusOK, response)
	}
}

func oauthClientCredentials(c *gin.Context) (string, string) {
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Basic ") {
		return "", ""
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(header, "Basic "))
	if err != nil {
		return "", ""
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return "", ""
	}
	clientID, _ := url.QueryUnescape(parts[0])
	clientSecret, _ := url.QueryUnescape(parts[1])
	return clientID, clientSecret
}

func bearerToken(req *http.Request) string {
	header := req.Header.Get("Authorization")
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return ""
	}
	return strings.TrimSpace(header[7:])
}

func redirectOAuthError(c *gin.Context, redirectURI string, code string, state string) {
	if redirectURI == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": code})
		return
	}
	location := appendQuery(redirectURI, "error", code)
	if state != "" {
		location = appendQuery(location, "state", state)
	}
	c.Redirect(http.StatusFound, location)
}

func oauthTokenError(c *gin.Context, status int, code string) {
	if code == "invalid_client" {
		c.Header("WWW-Authenticate", `Basic realm="oauth"`)
	}
	c.JSON(status, gin.H{"error": code})
}

func appendQuery(rawURL string, key string, value string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	q.Set(key, value)
	u.RawQuery = q.Encode()
	return u.String()
}

func scopeHas(scope string, needle string) bool {
	for _, part := range strings.Fields(scope) {
		if part == needle {
			return true
		}
	}
	return false
}

func oauthEndpointBool(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case int64:
		return v != 0
	case float64:
		return v != 0
	case string:
		v = strings.ToLower(strings.TrimSpace(v))
		return v == "true" || v == "1"
	default:
		return false
	}
}

func makeIDToken(provider *resource.OAuthProvider, req *http.Request, app map[string]interface{}, userRow map[string]interface{}, codeRow map[string]interface{}) (string, error) {
	if userRow == nil {
		return "", fmt.Errorf("missing user")
	}
	transaction, err := provider.BeginTransaction()
	if err != nil {
		return "", err
	}
	defer transaction.Rollback()

	now := time.Now().UTC()
	claims := map[string]interface{}{
		"iss":   provider.Issuer(req, transaction),
		"sub":   daptinid.InterfaceToDIR(userRow["reference_id"]).String(),
		"aud":   app["client_id"],
		"iat":   now.Unix(),
		"exp":   now.Add(time.Duration(resource.OAuthAccessTokenLifetimeSeconds) * time.Second).Unix(),
		"email": userRow["email"],
		"name":  userRow["name"],
	}
	if codeRow != nil && fmt.Sprintf("%v", codeRow["nonce"]) != "" && fmt.Sprintf("%v", codeRow["nonce"]) != "<nil>" {
		claims["nonce"] = codeRow["nonce"]
	}
	tokenString, err := provider.SignIDToken(claims, transaction)
	if err != nil {
		return "", err
	}
	if err := transaction.Commit(); err != nil {
		return "", err
	}
	return tokenString, nil
}
