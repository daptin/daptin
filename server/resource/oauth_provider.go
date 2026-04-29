package resource

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const (
	OAuthAccessTokenLifetimeSeconds  = int64(3600)
	OAuthRefreshTokenLifetimeSeconds = int64(60 * 60 * 24 * 30)
	OAuthCodeLifetimeSeconds         = int64(600)
)

type OAuthProvider struct {
	cruds       map[string]*DbResource
	configStore *ConfigStore
}

type OAuthIssuedToken struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
	Scope        string
}

type OAuthSigningKey struct {
	KeyID      string
	PrivateKey *rsa.PrivateKey
	JWK        map[string]interface{}
}

func NewOAuthProvider(cruds map[string]*DbResource, configStore *ConfigStore) *OAuthProvider {
	return &OAuthProvider{
		cruds:       cruds,
		configStore: configStore,
	}
}

func (op *OAuthProvider) BeginTransaction() (*sqlx.Tx, error) {
	return op.cruds["oauth_app"].Connection().Beginx()
}

func OAuthHashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func OAuthRandomToken() (string, error) {
	data := make([]byte, 32)
	_, err := rand.Read(data)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func OAuthPKCES256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (op *OAuthProvider) Issuer(req *http.Request, transaction *sqlx.Tx) string {
	if op.configStore != nil {
		if issuer, err := op.configStore.GetConfigValueFor("oauth.issuer", "backend", transaction); err == nil && issuer != "" {
			return strings.TrimRight(issuer, "/")
		}
	}

	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	} else if forwardedProto := req.Header.Get("X-Forwarded-Proto"); forwardedProto != "" {
		scheme = forwardedProto
	}
	return fmt.Sprintf("%s://%s", scheme, req.Host)
}

func (op *OAuthProvider) GetAppByClientID(clientID string, transaction *sqlx.Tx) (map[string]interface{}, error) {
	rows, _, err := op.cruds["oauth_app"].GetRowsByWhereClauseWithTransaction("oauth_app", nil, transaction, goqu.Ex{"client_id": clientID})
	if err != nil {
		return nil, err
	}
	if len(rows) < 1 {
		return nil, fmt.Errorf("unknown client")
	}
	return rows[0], nil
}

func (op *OAuthProvider) AuthenticateClient(clientID string, clientSecret string, transaction *sqlx.Tx) (map[string]interface{}, error) {
	app, err := op.GetAppByClientID(clientID, transaction)
	if err != nil {
		return nil, err
	}
	if !oauthBool(app["is_enabled"]) {
		return nil, fmt.Errorf("disabled client")
	}
	if !oauthBool(app["is_confidential"]) {
		return app, nil
	}
	storedSecret, _ := app["client_secret"].(string)
	if storedSecret == "" || clientSecret == "" || !BcryptCheckStringHash(clientSecret, storedSecret) {
		return nil, fmt.Errorf("invalid client credentials")
	}
	return app, nil
}

func (op *OAuthProvider) ValidateRedirectURI(app map[string]interface{}, redirectURI string) bool {
	if redirectURI == "" {
		return false
	}
	for _, allowed := range splitOAuthList(fmt.Sprintf("%v", app["redirect_uris"])) {
		if allowed == redirectURI {
			return true
		}
	}
	return false
}

func (op *OAuthProvider) NormalizeScopes(app map[string]interface{}, requested string) (string, error) {
	allowed := map[string]bool{}
	for _, scope := range splitOAuthList(fmt.Sprintf("%v", app["scopes"])) {
		allowed[scope] = true
	}

	requestedScopes := splitOAuthList(requested)
	if len(requestedScopes) == 0 {
		requestedScopes = splitOAuthList(fmt.Sprintf("%v", app["scopes"]))
	}

	finalScopes := make([]string, 0, len(requestedScopes))
	for _, scope := range requestedScopes {
		if !allowed[scope] {
			return "", fmt.Errorf("invalid scope")
		}
		finalScopes = append(finalScopes, scope)
	}
	return strings.Join(finalScopes, " "), nil
}

func (op *OAuthProvider) HasGrant(app map[string]interface{}, grantType string) bool {
	for _, grant := range splitOAuthList(fmt.Sprintf("%v", app["grants"])) {
		if grant == grantType {
			return true
		}
	}
	return false
}

func (op *OAuthProvider) CreateCode(sessionUser *auth.SessionUser, app map[string]interface{}, redirectURI string, scope string, codeChallenge string, codeChallengeMethod string, nonce string, transaction *sqlx.Tx) (string, error) {
	code, err := OAuthRandomToken()
	if err != nil {
		return "", err
	}

	err = op.createInternalRow("oauth_code", map[string]interface{}{
		"code_hash":             OAuthHashToken(code),
		"redirect_uri":          redirectURI,
		"scope":                 scope,
		"expires_at":            time.Now().Add(time.Duration(OAuthCodeLifetimeSeconds) * time.Second).Unix(),
		"code_challenge":        codeChallenge,
		"code_challenge_method": codeChallengeMethod,
		"nonce":                 nonce,
		"oauth_app_id":          oauthInt64(app["id"]),
		"user_account_id":       sessionUser.UserId,
	}, transaction)
	if err != nil {
		return "", err
	}
	return code, nil
}

func (op *OAuthProvider) ExchangeCode(app map[string]interface{}, code string, redirectURI string, codeVerifier string, transaction *sqlx.Tx) (*OAuthIssuedToken, map[string]interface{}, map[string]interface{}, error) {
	rows, _, err := op.cruds["oauth_code"].GetRowsByWhereClauseWithTransaction("oauth_code", nil, transaction, goqu.Ex{"code_hash": OAuthHashToken(code)})
	if err != nil {
		return nil, nil, nil, err
	}
	if len(rows) < 1 {
		return nil, nil, nil, fmt.Errorf("invalid grant")
	}
	codeRow := rows[0]
	if oauthInt64(codeRow["expires_at"]) <= time.Now().Unix() || oauthInt64(codeRow["used_at"]) > 0 {
		return nil, nil, nil, fmt.Errorf("invalid grant")
	}
	if fmt.Sprintf("%v", codeRow["redirect_uri"]) != redirectURI {
		return nil, nil, nil, fmt.Errorf("invalid grant")
	}
	if !op.RowBelongsToApp(codeRow, app) {
		return nil, nil, nil, fmt.Errorf("invalid grant")
	}
	if err := validatePKCE(codeRow, codeVerifier); err != nil {
		return nil, nil, nil, err
	}

	sessionUser, err := op.sessionUserFromRow(codeRow, transaction)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := op.markUsed("oauth_code", oauthInt64(codeRow["id"]), transaction); err != nil {
		return nil, nil, nil, err
	}

	token, err := op.createTokenPair(sessionUser, app, fmt.Sprintf("%v", codeRow["scope"]), transaction)
	if err != nil {
		return nil, nil, nil, err
	}
	return token, codeRow, sessionUserRow(op, sessionUser, transaction), nil
}

func (op *OAuthProvider) Refresh(app map[string]interface{}, refreshToken string, transaction *sqlx.Tx) (*OAuthIssuedToken, map[string]interface{}, error) {
	rows, _, err := op.cruds["oauth_refresh"].GetRowsByWhereClauseWithTransaction("oauth_refresh", nil, transaction, goqu.Ex{"token_hash": OAuthHashToken(refreshToken)})
	if err != nil {
		return nil, nil, err
	}
	if len(rows) < 1 {
		return nil, nil, fmt.Errorf("invalid grant")
	}
	refreshRow := rows[0]
	if oauthInt64(refreshRow["expires_at"]) <= time.Now().Unix() || oauthInt64(refreshRow["revoked_at"]) > 0 {
		return nil, nil, fmt.Errorf("invalid grant")
	}
	if !op.RowBelongsToApp(refreshRow, app) {
		return nil, nil, fmt.Errorf("invalid grant")
	}

	sessionUser, err := op.sessionUserFromRow(refreshRow, transaction)
	if err != nil {
		return nil, nil, err
	}
	if err := op.revokeByID("oauth_refresh", oauthInt64(refreshRow["id"]), transaction); err != nil {
		return nil, nil, err
	}
	token, err := op.createTokenPair(sessionUser, app, fmt.Sprintf("%v", refreshRow["scope"]), transaction)
	if err != nil {
		return nil, nil, err
	}
	return token, sessionUserRow(op, sessionUser, transaction), nil
}

func (op *OAuthProvider) ValidateAccessToken(token string, transaction *sqlx.Tx) (map[string]interface{}, map[string]interface{}, error) {
	rows, _, err := op.cruds["oauth_access"].GetRowsByWhereClauseWithTransaction("oauth_access", nil, transaction, goqu.Ex{"token_hash": OAuthHashToken(token)})
	if err != nil {
		return nil, nil, err
	}
	if len(rows) < 1 {
		return nil, nil, fmt.Errorf("invalid token")
	}
	accessRow := rows[0]
	if oauthInt64(accessRow["expires_at"]) <= time.Now().Unix() || oauthInt64(accessRow["revoked_at"]) > 0 {
		return nil, nil, fmt.Errorf("invalid token")
	}
	sessionUser, err := op.sessionUserFromRow(accessRow, transaction)
	if err != nil {
		return nil, nil, err
	}
	return accessRow, sessionUserRow(op, sessionUser, transaction), nil
}

func (op *OAuthProvider) RevokeToken(token string, transaction *sqlx.Tx) error {
	tokenHash := OAuthHashToken(token)
	if err := op.revokeByHash("oauth_access", tokenHash, transaction); err != nil {
		return err
	}
	return op.revokeByHash("oauth_refresh", tokenHash, transaction)
}

func (op *OAuthProvider) ActiveSigningKey(transaction *sqlx.Tx) (*OAuthSigningKey, error) {
	rows, _, err := op.cruds["oauth_key"].GetRowsByWhereClauseWithTransaction("oauth_key", nil, transaction, goqu.Ex{"is_active": true})
	if err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		return op.signingKeyFromRow(rows[0], transaction)
	}
	return op.createSigningKey(transaction)
}

func (op *OAuthProvider) JWKS(transaction *sqlx.Tx) ([]map[string]interface{}, error) {
	key, err := op.ActiveSigningKey(transaction)
	if err != nil {
		return nil, err
	}
	return []map[string]interface{}{key.JWK}, nil
}

func (op *OAuthProvider) SignIDToken(claims map[string]interface{}, transaction *sqlx.Tx) (string, error) {
	key, err := op.ActiveSigningKey(transaction)
	if err != nil {
		return "", err
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims(claims))
	token.Header["kid"] = key.KeyID
	return token.SignedString(key.PrivateKey)
}

func (op *OAuthProvider) createTokenPair(sessionUser *auth.SessionUser, app map[string]interface{}, scope string, transaction *sqlx.Tx) (*OAuthIssuedToken, error) {
	accessToken, err := OAuthRandomToken()
	if err != nil {
		return nil, err
	}
	refreshToken, err := OAuthRandomToken()
	if err != nil {
		return nil, err
	}

	appID := oauthInt64(app["id"])
	err = op.createInternalRow("oauth_access", map[string]interface{}{
		"token_hash":      OAuthHashToken(accessToken),
		"token_type":      "Bearer",
		"scope":           scope,
		"expires_at":      time.Now().Add(time.Duration(OAuthAccessTokenLifetimeSeconds) * time.Second).Unix(),
		"oauth_app_id":    appID,
		"user_account_id": sessionUser.UserId,
	}, transaction)
	if err != nil {
		return nil, err
	}

	err = op.createInternalRow("oauth_refresh", map[string]interface{}{
		"token_hash":      OAuthHashToken(refreshToken),
		"scope":           scope,
		"expires_at":      time.Now().Add(time.Duration(OAuthRefreshTokenLifetimeSeconds) * time.Second).Unix(),
		"oauth_app_id":    appID,
		"user_account_id": sessionUser.UserId,
	}, transaction)
	if err != nil {
		return nil, err
	}

	return &OAuthIssuedToken{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    OAuthAccessTokenLifetimeSeconds,
		Scope:        scope,
	}, nil
}

func (op *OAuthProvider) RowBelongsToApp(row map[string]interface{}, app map[string]interface{}) bool {
	rowRef := daptinid.InterfaceToDIR(row["oauth_app_id"])
	appRef := daptinid.InterfaceToDIR(app["reference_id"])
	if rowRef != daptinid.NullReferenceId && appRef != daptinid.NullReferenceId {
		return rowRef == appRef
	}
	return oauthInt64(row["oauth_app_id"]) == oauthInt64(app["id"])
}

func (op *OAuthProvider) createInternalRow(tableName string, values map[string]interface{}, transaction *sqlx.Tx) error {
	now := time.Now()
	u, _ := uuid.NewV7()
	ref := daptinid.DaptinReferenceId(u)
	values["reference_id"] = ref[:]
	values["permission"] = int64(auth.DEFAULT_PERMISSION)
	values["created_at"] = now
	values["updated_at"] = now

	cols := make([]interface{}, 0, len(values))
	vals := make([]interface{}, 0, len(values))
	for col, val := range values {
		cols = append(cols, col)
		vals = append(vals, val)
	}

	query, args, err := statementbuilder.Squirrel.Insert(tableName).Prepared(true).Cols(cols...).Vals(vals).ToSQL()
	if err != nil {
		return err
	}
	_, err = transaction.Exec(query, args...)
	return err
}

func (op *OAuthProvider) signingKeyFromRow(row map[string]interface{}, transaction *sqlx.Tx) (*OAuthSigningKey, error) {
	secret, err := op.configStore.GetConfigValueForWithTransaction("encryption.secret", "backend", transaction)
	if err != nil {
		return nil, err
	}
	privatePEMEncrypted, _ := row["private_key"].(string)
	privatePEM, err := Decrypt([]byte(secret), privatePEMEncrypted)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode([]byte(privatePEM))
	if block == nil {
		return nil, fmt.Errorf("invalid oauth signing key")
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	keyID := fmt.Sprintf("%v", row["key_id"])
	return &OAuthSigningKey{
		KeyID:      keyID,
		PrivateKey: privateKey,
		JWK:        rsaPublicJWK(keyID, &privateKey.PublicKey),
	}, nil
}

func (op *OAuthProvider) createSigningKey(transaction *sqlx.Tx) (*OAuthSigningKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	u, _ := uuid.NewV7()
	keyID := u.String()

	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	publicBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicBytes})

	secret, err := op.configStore.GetConfigValueForWithTransaction("encryption.secret", "backend", transaction)
	if err != nil {
		return nil, err
	}
	encryptedPrivateKey, err := Encrypt([]byte(secret), string(privatePEM))
	if err != nil {
		return nil, err
	}

	err = op.createInternalRow("oauth_key", map[string]interface{}{
		"key_id":      keyID,
		"algorithm":   "RS256",
		"public_key":  string(publicPEM),
		"private_key": encryptedPrivateKey,
		"is_active":   true,
	}, transaction)
	if err != nil {
		return nil, err
	}

	return &OAuthSigningKey{
		KeyID:      keyID,
		PrivateKey: privateKey,
		JWK:        rsaPublicJWK(keyID, &privateKey.PublicKey),
	}, nil
}

func rsaPublicJWK(keyID string, publicKey *rsa.PublicKey) map[string]interface{} {
	exponentBytes := make([]byte, 0)
	e := publicKey.E
	for e > 0 {
		exponentBytes = append([]byte{byte(e & 0xff)}, exponentBytes...)
		e >>= 8
	}
	return map[string]interface{}{
		"kty": "RSA",
		"use": "sig",
		"kid": keyID,
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(exponentBytes),
	}
}

func (op *OAuthProvider) sessionUserFromRow(row map[string]interface{}, transaction *sqlx.Tx) (*auth.SessionUser, error) {
	userRef := daptinid.InterfaceToDIR(row["user_account_id"])
	userRow, _, err := op.cruds[USER_ACCOUNT_TABLE_NAME].GetSingleRowByReferenceIdWithTransaction(USER_ACCOUNT_TABLE_NAME, userRef, nil, transaction)
	if err != nil {
		return nil, err
	}
	userID := oauthInt64(userRow["id"])
	groups := op.cruds[USER_ACCOUNT_TABLE_NAME].GetObjectUserGroupsByWhereWithTransaction(USER_ACCOUNT_TABLE_NAME, transaction, "id", userID)
	return &auth.SessionUser{
		UserId:          userID,
		UserReferenceId: daptinid.InterfaceToDIR(userRow["reference_id"]),
		Groups:          groups,
	}, nil
}

func (op *OAuthProvider) markUsed(tableName string, id int64, transaction *sqlx.Tx) error {
	query, args, err := statementbuilder.Squirrel.Update(tableName).Prepared(true).Set(goqu.Record{"used_at": time.Now().Unix()}).Where(goqu.Ex{"id": id}).ToSQL()
	if err != nil {
		return err
	}
	_, err = transaction.Exec(query, args...)
	return err
}

func (op *OAuthProvider) revokeByID(tableName string, id int64, transaction *sqlx.Tx) error {
	query, args, err := statementbuilder.Squirrel.Update(tableName).Prepared(true).Set(goqu.Record{"revoked_at": time.Now().Unix()}).Where(goqu.Ex{"id": id}).ToSQL()
	if err != nil {
		return err
	}
	_, err = transaction.Exec(query, args...)
	return err
}

func (op *OAuthProvider) revokeByHash(tableName string, tokenHash string, transaction *sqlx.Tx) error {
	query, args, err := statementbuilder.Squirrel.Update(tableName).Prepared(true).Set(goqu.Record{"revoked_at": time.Now().Unix()}).Where(goqu.Ex{"token_hash": tokenHash}).ToSQL()
	if err != nil {
		return err
	}
	_, err = transaction.Exec(query, args...)
	return err
}

func validatePKCE(codeRow map[string]interface{}, verifier string) error {
	challenge := fmt.Sprintf("%v", codeRow["code_challenge"])
	if challenge == "" || challenge == "<nil>" {
		return nil
	}
	method := strings.ToUpper(fmt.Sprintf("%v", codeRow["code_challenge_method"]))
	if method == "" || method == "<NIL>" {
		method = "PLAIN"
	}
	if verifier == "" {
		return fmt.Errorf("missing code verifier")
	}
	if method == "S256" && OAuthPKCES256(verifier) == challenge {
		return nil
	}
	if method == "PLAIN" && verifier == challenge {
		return nil
	}
	return fmt.Errorf("invalid code verifier")
}

func sessionUserRow(op *OAuthProvider, sessionUser *auth.SessionUser, transaction *sqlx.Tx) map[string]interface{} {
	userRow, _, err := op.cruds[USER_ACCOUNT_TABLE_NAME].GetSingleRowByReferenceIdWithTransaction(USER_ACCOUNT_TABLE_NAME, sessionUser.UserReferenceId, nil, transaction)
	if err != nil {
		return nil
	}
	return userRow
}

func splitOAuthList(value string) []string {
	value = strings.ReplaceAll(value, ",", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\t", " ")
	parts := strings.Fields(value)
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || seen[part] {
			continue
		}
		seen[part] = true
		out = append(out, part)
	}
	return out
}

func oauthBool(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case int:
		return v != 0
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

func oauthInt64(value interface{}) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case int32:
		return int64(v)
	case uint64:
		if v > math.MaxInt64 {
			return 0
		}
		return int64(v)
	case float64:
		if v > float64(math.MaxInt64) || v < float64(math.MinInt64) {
			return 0
		}
		return int64(v)
	case string:
		i, _ := strconv.ParseInt(v, 10, 64)
		return i
	default:
		return 0
	}
}
