package server

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/doug-martin/goqu/v9"
	"github.com/gin-gonic/gin"
)

func initializeOAuthBrowserRoutes(cruds map[string]*resource.DbResource, defaultRouter *gin.Engine) {
	defaultRouter.GET("/auth/signin", oauthSigninPageHandler())
	defaultRouter.POST("/auth/signin", oauthSigninSubmitHandler(cruds))
	defaultRouter.GET("/oauth/login/:authenticator", oauthLoginBeginBrowserHandler(cruds))
	defaultRouter.GET("/oauth/response", oauthConsumerResponseHandler(cruds))
}

func oauthSigninPageHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		returnTo := safeSameOriginPath(c.Query("return_to"), "/")
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(renderOAuthSigninPage(returnTo, "")))
	}
}

func oauthSigninSubmitHandler(cruds map[string]*resource.DbResource) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := c.Request.ParseForm(); err != nil {
			c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(renderOAuthSigninPage("/", "Invalid form submission.")))
			return
		}

		returnTo := safeSameOriginPath(c.PostForm("return_to"), "/")
		email := strings.TrimSpace(c.PostForm("email"))
		password := c.PostForm("password")
		if email == "" || password == "" {
			c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(renderOAuthSigninPage(returnTo, "Email and password are required.")))
			return
		}

		responses, err := executeBrowserAction(c, cruds, resource.USER_ACCOUNT_TABLE_NAME, "signin", map[string]interface{}{
			"email":    email,
			"password": password,
		}, false)
		if err != nil {
			c.Data(http.StatusUnauthorized, "text/html; charset=utf-8", []byte(renderOAuthSigninPage(returnTo, "Invalid email or password.")))
			return
		}

		if !applyCookieResponses(c, responses) {
			c.Data(http.StatusInternalServerError, "text/html; charset=utf-8", []byte(renderOAuthSigninPage(returnTo, "Sign in did not create a browser session.")))
			return
		}
		c.Redirect(http.StatusFound, returnTo)
	}
}

func oauthLoginBeginBrowserHandler(cruds map[string]*resource.DbResource) gin.HandlerFunc {
	return func(c *gin.Context) {
		authenticator := strings.TrimSpace(c.Param("authenticator"))
		if authenticator == "" {
			c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(renderOAuthMessagePage("OAuth login failed", "Missing authenticator.")))
			return
		}

		oauthConnectId, err := oauthConnectReferenceForBrowserLogin(cruds, authenticator)
		if err != nil {
			c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(renderOAuthMessagePage("OAuth login failed", err.Error())))
			return
		}

		responses, err := executeBrowserAction(c, cruds, "oauth_connect", "oauth_login_begin", map[string]interface{}{
			"oauth_connect_id": oauthConnectId,
		}, true)
		if err != nil {
			c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(renderOAuthMessagePage("OAuth login failed", err.Error())))
			return
		}

		redirectTo := lastSafeRedirect(responses, "")
		if redirectTo == "" {
			redirectTo = lastRedirect(responses)
		}
		if redirectTo == "" {
			c.Data(http.StatusInternalServerError, "text/html; charset=utf-8", []byte(renderOAuthMessagePage("OAuth login failed", "OAuth begin action did not return a redirect.")))
			return
		}
		c.Redirect(http.StatusFound, redirectTo)
	}
}

func oauthConsumerResponseHandler(cruds map[string]*resource.DbResource) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := strings.TrimSpace(c.Query("code"))
		state := strings.TrimSpace(c.Query("state"))
		authenticator := strings.TrimSpace(c.Query("authenticator"))
		if code == "" || state == "" || authenticator == "" {
			c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(renderOAuthMessagePage("OAuth callback failed", "Missing code, state, or authenticator.")))
			return
		}

		responses, err := executeBrowserAction(c, cruds, "oauth_token", "oauth.login.response", map[string]interface{}{
			"code":          code,
			"state":         state,
			"authenticator": authenticator,
		}, true)
		if err != nil {
			c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(renderOAuthMessagePage("OAuth callback failed", err.Error())))
			return
		}

		applyCookieResponses(c, responses)
		redirectTo := lastSafeRedirect(responses, "/")
		c.Redirect(http.StatusFound, redirectTo)
	}
}

func oauthConnectReferenceForBrowserLogin(cruds map[string]*resource.DbResource, authenticator string) (string, error) {
	if cruds["world"] == nil || cruds["oauth_connect"] == nil {
		return "", fmt.Errorf("oauth resources are not available")
	}
	transaction, err := cruds["world"].Connection().Beginx()
	if err != nil {
		return "", err
	}
	defer transaction.Rollback()

	rows, _, err := cruds["oauth_connect"].GetRowsByWhereClauseWithTransaction("oauth_connect", nil, transaction, goqu.Ex{
		"name": authenticator,
	})
	if err != nil {
		return "", err
	}
	if len(rows) < 1 {
		return "", fmt.Errorf("No such authenticator [%s]", authenticator)
	}
	if !oauthBrowserBool(rows[0]["allow_login"]) {
		return "", fmt.Errorf("OAuth login is not enabled for [%s]", authenticator)
	}
	referenceId := strings.TrimSpace(fmt.Sprintf("%v", rows[0]["reference_id"]))
	if referenceId == "" || referenceId == "<nil>" {
		return "", fmt.Errorf("OAuth connector [%s] has no reference id", authenticator)
	}
	return referenceId, nil
}

func executeBrowserAction(c *gin.Context, cruds map[string]*resource.DbResource, actionType string, actionName string, attrs map[string]interface{}, internal bool) ([]actionresponse.ActionResponse, error) {
	actionCrudResource, ok := cruds[actionType]
	if !ok {
		actionCrudResource = cruds["world"]
	}
	if actionCrudResource == nil || cruds["world"] == nil {
		return nil, fmt.Errorf("action resources are not available")
	}

	transaction, err := cruds["world"].Connection().Beginx()
	if err != nil {
		return nil, err
	}

	actionReq := actionresponse.ActionRequest{
		Type:       actionType,
		Action:     actionName,
		Attributes: attrs,
	}

	plainRequest := &http.Request{
		Method: "POST",
		URL:    c.Request.URL,
		Header: c.Request.Header,
	}
	requestContext := c.Request.Context()
	if internal {
		requestContext = context.WithValue(requestContext, "user", &auth.SessionUser{
			Groups: auth.GroupPermissionList{
				{GroupReferenceId: cruds["world"].AdministratorGroupId},
			},
		})
	}
	plainRequest = plainRequest.WithContext(requestContext)

	req := api2go.Request{PlainRequest: plainRequest}
	responses, err := actionCrudResource.HandleActionRequest(actionReq, req, transaction)
	if err != nil {
		_ = transaction.Rollback()
		return responses, err
	}
	if err := transaction.Commit(); err != nil {
		return responses, err
	}
	return responses, nil
}

func applyCookieResponses(c *gin.Context, responses []actionresponse.ActionResponse) bool {
	applied := false
	for _, response := range responses {
		if response.ResponseType != "client.cookie.set" {
			continue
		}
		attrs := responseAttributes(response)
		key := strings.TrimSpace(fmt.Sprintf("%v", attrs["key"]))
		value := strings.TrimSpace(fmt.Sprintf("%v", attrs["value"]))
		if key == "" || value == "" || value == "<nil>" {
			continue
		}
		if idx := strings.Index(value, ";"); idx >= 0 {
			value = strings.TrimSpace(value[:idx])
		}
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     key,
			Value:    value,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
			Secure:   c.Request.TLS != nil || strings.EqualFold(c.Request.Header.Get("X-Forwarded-Proto"), "https"),
		})
		applied = true
	}
	return applied
}

func lastSafeRedirect(responses []actionresponse.ActionResponse, fallback string) string {
	redirectTo := fallback
	for _, response := range responses {
		if response.ResponseType != "client.redirect" {
			continue
		}
		attrs := responseAttributes(response)
		if location, ok := attrs["location"]; ok {
			redirectTo = safeSameOriginPath(fmt.Sprintf("%v", location), redirectTo)
		}
	}
	return redirectTo
}

func lastRedirect(responses []actionresponse.ActionResponse) string {
	redirectTo := ""
	for _, response := range responses {
		if response.ResponseType != "client.redirect" {
			continue
		}
		attrs := responseAttributes(response)
		if location, ok := attrs["location"]; ok {
			redirectTo = strings.TrimSpace(fmt.Sprintf("%v", location))
		}
	}
	return redirectTo
}

func responseAttributes(response actionresponse.ActionResponse) map[string]interface{} {
	switch attrs := response.Attributes.(type) {
	case map[string]interface{}:
		return attrs
	case map[string]string:
		out := make(map[string]interface{}, len(attrs))
		for key, value := range attrs {
			out[key] = value
		}
		return out
	default:
		return map[string]interface{}{}
	}
}

func safeSameOriginPath(raw string, fallback string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return fallback
	}
	if !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(raw, "//") || strings.Contains(raw, "\\") {
		return fallback
	}
	return parsed.RequestURI()
}

func oauthBrowserBool(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case int32:
		return typed != 0
	case uint:
		return typed != 0
	case uint64:
		return typed != 0
	case []byte:
		return oauthBrowserBool(string(typed))
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "t", "yes", "y", "on":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func renderOAuthSigninPage(returnTo string, message string) string {
	messageHTML := ""
	if message != "" {
		messageHTML = `<div class="error">` + html.EscapeString(message) + `</div>`
	}
	return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Daptin Sign In</title>
  <style>
    :root { color-scheme: light; font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    body { margin: 0; min-height: 100vh; display: grid; place-items: center; background: #f7f8fb; color: #121826; }
    main { width: min(92vw, 380px); background: #fff; border: 1px solid #d9dee8; border-radius: 8px; padding: 28px; box-shadow: 0 18px 45px rgba(18, 24, 38, 0.08); }
    h1 { margin: 0 0 6px; font-size: 24px; line-height: 1.2; letter-spacing: 0; }
    p { margin: 0 0 22px; color: #667085; font-size: 14px; line-height: 1.5; }
    label { display: block; margin: 14px 0 6px; font-size: 13px; font-weight: 600; }
    input { box-sizing: border-box; width: 100%; height: 40px; border: 1px solid #cfd6e3; border-radius: 6px; padding: 8px 10px; font: inherit; }
    input:focus { outline: 2px solid #8ab4f8; border-color: #477ed8; }
    button { width: 100%; height: 42px; margin-top: 18px; border: 0; border-radius: 6px; background: #1f5fbf; color: white; font: inherit; font-weight: 700; cursor: pointer; }
    button:hover { background: #184d9c; }
    .error { margin: 0 0 16px; padding: 10px 12px; border: 1px solid #f0b4ad; border-radius: 6px; color: #9f1d13; background: #fff1ef; font-size: 14px; }
  </style>
</head>
<body>
  <main>
    <h1>Sign in to Daptin</h1>
    <p>Continue to the requested application.</p>
    ` + messageHTML + `
    <form method="post" action="/auth/signin">
      <input type="hidden" name="return_to" value="` + html.EscapeString(returnTo) + `">
      <label for="email">Email</label>
      <input id="email" name="email" type="email" autocomplete="username" required autofocus>
      <label for="password">Password</label>
      <input id="password" name="password" type="password" autocomplete="current-password" required>
      <button type="submit">Sign in</button>
    </form>
  </main>
</body>
</html>`
}

func renderOAuthMessagePage(title string, message string) string {
	return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>` + html.EscapeString(title) + `</title>
  <style>
    :root { font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    body { margin: 0; min-height: 100vh; display: grid; place-items: center; background: #f7f8fb; color: #121826; }
    main { width: min(92vw, 420px); background: #fff; border: 1px solid #d9dee8; border-radius: 8px; padding: 28px; }
    h1 { margin: 0 0 10px; font-size: 22px; letter-spacing: 0; }
    p { margin: 0; color: #667085; line-height: 1.5; }
  </style>
</head>
<body>
  <main>
    <h1>` + html.EscapeString(title) + `</h1>
    <p>` + html.EscapeString(message) + `</p>
  </main>
</body>
</html>`
}
