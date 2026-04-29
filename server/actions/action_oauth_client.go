package actions

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const (
	oauthDefaultScopes = "openid profile email"
	oauthDefaultGrants = "authorization_code,refresh_token"
)

type oauthClientActionPerformer struct {
	name  string
	cruds map[string]*resource.DbResource
}

func (d *oauthClientActionPerformer) Name() string {
	return d.name
}

func (d *oauthClientActionPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	switch d.name {
	case "oauth.client.register":
		return d.registerClient(inFields, transaction)
	case "oauth.client.update":
		return d.updateClient(inFields, transaction)
	case "oauth.client.rotate_secret":
		return d.rotateClientSecret(inFields, transaction)
	case "oauth.client.disable":
		return d.setClientEnabled(inFields, transaction, false)
	case "oauth.client.enable":
		return d.setClientEnabled(inFields, transaction, true)
	case "oauth.client.revoke_tokens":
		return d.revokeClientTokens(inFields, transaction)
	default:
		return nil, nil, []error{fmt.Errorf("unknown oauth client action: %s", d.name)}
	}
}

func (d *oauthClientActionPerformer) registerClient(inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	attrs := oauthActionAttrs(inFields)
	name := strings.TrimSpace(fmt.Sprintf("%v", attrs["name"]))
	if name == "" {
		return nil, nil, []error{fmt.Errorf("name is required")}
	}

	redirectURIs, err := normalizeRedirectURIs(fmt.Sprintf("%v", attrs["redirect_uris"]))
	if err != nil {
		return nil, nil, []error{err}
	}
	scopes, err := normalizeOAuthScopes(fmt.Sprintf("%v", attrs["scopes"]))
	if err != nil {
		return nil, nil, []error{err}
	}
	grants, err := normalizeOAuthGrants(fmt.Sprintf("%v", attrs["grants"]))
	if err != nil {
		return nil, nil, []error{err}
	}
	isConfidential := oauthActionBool(attrs["is_confidential"], true)

	clientID, err := oauthGeneratedValue("dapc")
	if err != nil {
		return nil, nil, []error{err}
	}

	var clientSecret string
	var clientSecretHash string
	if isConfidential {
		clientSecret, err = oauthGeneratedValue("daps")
		if err != nil {
			return nil, nil, []error{err}
		}
		clientSecretHash, err = resource.BcryptHashString(clientSecret)
		if err != nil {
			return nil, nil, []error{err}
		}
	}

	referenceID, err := d.createOAuthApp(map[string]interface{}{
		"name":            name,
		"client_id":       clientID,
		"client_secret":   clientSecretHash,
		"redirect_uris":   redirectURIs,
		"scopes":          scopes,
		"grants":          grants,
		"is_confidential": isConfidential,
		"is_enabled":      true,
	}, transaction)
	if err != nil {
		return nil, nil, []error{err}
	}

	response := map[string]interface{}{
		"reference_id":     referenceID.String(),
		"name":             name,
		"client_id":        clientID,
		"redirect_uris":    redirectURIs,
		"scopes":           scopes,
		"grants":           grants,
		"is_confidential":  isConfidential,
		"is_enabled":       true,
		"client_secret":    clientSecret,
		"secret_returned":  clientSecret != "",
		"secret_store":     "bcrypt",
		"management_route": "/action/oauth_app",
	}
	if !isConfidential {
		delete(response, "client_secret")
	}

	return nil, []actionresponse.ActionResponse{
		resource.NewActionResponse("oauth_app", response),
	}, nil
}

func (d *oauthClientActionPerformer) updateClient(inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	attrs := oauthActionAttrs(inFields)
	subject, err := oauthSubject(inFields)
	if err != nil {
		return nil, nil, []error{err}
	}

	updates := map[string]interface{}{}
	if value, ok := attrs["name"]; ok {
		name := strings.TrimSpace(fmt.Sprintf("%v", value))
		if name == "" {
			return nil, nil, []error{fmt.Errorf("name cannot be blank")}
		}
		updates["name"] = name
	}
	if value, ok := attrs["redirect_uris"]; ok {
		redirectURIs, err := normalizeRedirectURIs(fmt.Sprintf("%v", value))
		if err != nil {
			return nil, nil, []error{err}
		}
		updates["redirect_uris"] = redirectURIs
	}
	if value, ok := attrs["scopes"]; ok {
		scopes, err := normalizeOAuthScopes(fmt.Sprintf("%v", value))
		if err != nil {
			return nil, nil, []error{err}
		}
		updates["scopes"] = scopes
	}
	if value, ok := attrs["grants"]; ok {
		grants, err := normalizeOAuthGrants(fmt.Sprintf("%v", value))
		if err != nil {
			return nil, nil, []error{err}
		}
		updates["grants"] = grants
	}
	if value, ok := attrs["is_confidential"]; ok {
		updates["is_confidential"] = oauthActionBool(value, true)
	}
	if len(updates) == 0 {
		return nil, nil, []error{fmt.Errorf("no oauth client fields to update")}
	}
	if err := d.updateOAuthApp(oauthActionInt64(subject["id"]), updates, transaction); err != nil {
		return nil, nil, []error{err}
	}
	updates["reference_id"] = fmt.Sprintf("%v", subject["reference_id"])
	updates["client_id"] = fmt.Sprintf("%v", subject["client_id"])
	return nil, []actionresponse.ActionResponse{resource.NewActionResponse("oauth_app", updates)}, nil
}

func (d *oauthClientActionPerformer) rotateClientSecret(inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	subject, err := oauthSubject(inFields)
	if err != nil {
		return nil, nil, []error{err}
	}
	if !oauthActionBool(subject["is_confidential"], true) {
		return nil, nil, []error{fmt.Errorf("public clients do not have a client secret")}
	}
	clientSecret, err := oauthGeneratedValue("daps")
	if err != nil {
		return nil, nil, []error{err}
	}
	clientSecretHash, err := resource.BcryptHashString(clientSecret)
	if err != nil {
		return nil, nil, []error{err}
	}
	if err := d.updateOAuthApp(oauthActionInt64(subject["id"]), map[string]interface{}{"client_secret": clientSecretHash}, transaction); err != nil {
		return nil, nil, []error{err}
	}
	return nil, []actionresponse.ActionResponse{resource.NewActionResponse("oauth_app", map[string]interface{}{
		"reference_id":     fmt.Sprintf("%v", subject["reference_id"]),
		"client_id":        fmt.Sprintf("%v", subject["client_id"]),
		"client_secret":    clientSecret,
		"secret_returned":  true,
		"secret_store":     "bcrypt",
		"management_route": "/action/oauth_app",
	})}, nil
}

func (d *oauthClientActionPerformer) setClientEnabled(inFields map[string]interface{}, transaction *sqlx.Tx, enabled bool) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	subject, err := oauthSubject(inFields)
	if err != nil {
		return nil, nil, []error{err}
	}
	if err := d.updateOAuthApp(oauthActionInt64(subject["id"]), map[string]interface{}{"is_enabled": enabled}, transaction); err != nil {
		return nil, nil, []error{err}
	}
	return nil, []actionresponse.ActionResponse{resource.NewActionResponse("oauth_app", map[string]interface{}{
		"reference_id": fmt.Sprintf("%v", subject["reference_id"]),
		"client_id":    fmt.Sprintf("%v", subject["client_id"]),
		"is_enabled":   enabled,
	})}, nil
}

func (d *oauthClientActionPerformer) revokeClientTokens(inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	subject, err := oauthSubject(inFields)
	if err != nil {
		return nil, nil, []error{err}
	}
	appID := oauthActionInt64(subject["id"])
	revokedAt := time.Now().Unix()
	for _, tableName := range []string{"oauth_access", "oauth_refresh"} {
		query, args, err := statementbuilder.Squirrel.Update(tableName).Prepared(true).
			Set(goqu.Record{"revoked_at": revokedAt}).
			Where(goqu.Ex{"oauth_app_id": appID}).
			Where(goqu.Ex{"revoked_at": nil}).
			ToSQL()
		if err != nil {
			return nil, nil, []error{err}
		}
		if _, err := transaction.Exec(query, args...); err != nil {
			return nil, nil, []error{err}
		}
	}
	return nil, []actionresponse.ActionResponse{resource.NewActionResponse("oauth_app", map[string]interface{}{
		"reference_id": fmt.Sprintf("%v", subject["reference_id"]),
		"client_id":    fmt.Sprintf("%v", subject["client_id"]),
		"revoked_at":   revokedAt,
	})}, nil
}

func (d *oauthClientActionPerformer) createOAuthApp(values map[string]interface{}, transaction *sqlx.Tx) (daptinid.DaptinReferenceId, error) {
	u, err := uuid.NewV7()
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	referenceID := daptinid.DaptinReferenceId(u)
	values["reference_id"] = u[:]
	values["permission"] = int64(d.cruds["oauth_app"].TableInfo().DefaultPermission)
	values["created_at"] = time.Now()
	values["updated_at"] = time.Now()

	cols := make([]interface{}, 0, len(values))
	vals := make([]interface{}, 0, len(values))
	for col, val := range values {
		if col == "client_secret" && val == "" {
			continue
		}
		cols = append(cols, col)
		vals = append(vals, val)
	}
	query, args, err := statementbuilder.Squirrel.Insert("oauth_app").Prepared(true).Cols(cols...).Vals(vals).ToSQL()
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	if _, err := transaction.Exec(query, args...); err != nil {
		return daptinid.NullReferenceId, err
	}
	return referenceID, nil
}

func (d *oauthClientActionPerformer) updateOAuthApp(id int64, updates map[string]interface{}, transaction *sqlx.Tx) error {
	if id == 0 {
		return fmt.Errorf("oauth app id missing")
	}
	updates["updated_at"] = time.Now()
	query, args, err := statementbuilder.Squirrel.Update("oauth_app").Prepared(true).
		Set(goqu.Record(updates)).
		Where(goqu.Ex{"id": id}).
		ToSQL()
	if err != nil {
		return err
	}
	_, err = transaction.Exec(query, args...)
	return err
}

func oauthActionAttrs(inFields map[string]interface{}) map[string]interface{} {
	attrs, _ := inFields["request_attributes"].(map[string]interface{})
	if attrs == nil {
		return map[string]interface{}{}
	}
	return attrs
}

func oauthSubject(inFields map[string]interface{}) (map[string]interface{}, error) {
	subject, _ := inFields["subject"].(map[string]interface{})
	if subject == nil {
		return nil, fmt.Errorf("oauth client subject missing")
	}
	return subject, nil
}

func oauthGeneratedValue(prefix string) (string, error) {
	token, err := resource.OAuthRandomToken()
	if err != nil {
		return "", err
	}
	return prefix + "_" + token, nil
}

func normalizeRedirectURIs(value string) (string, error) {
	parts := splitOAuthActionList(value)
	if len(parts) == 0 {
		return "", fmt.Errorf("at least one redirect_uri is required")
	}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.Contains(part, "*") {
			return "", fmt.Errorf("redirect_uri wildcards are not allowed")
		}
		parsed, err := url.Parse(part)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return "", fmt.Errorf("invalid redirect_uri: %s", part)
		}
		host := parsed.Hostname()
		isLocalhost := host == "localhost" || host == "127.0.0.1" || host == "::1"
		if parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLocalhost) {
			return "", fmt.Errorf("redirect_uri must use https except localhost: %s", part)
		}
		fragment := parsed.Fragment
		if fragment != "" {
			return "", fmt.Errorf("redirect_uri fragments are not allowed: %s", part)
		}
		out = append(out, part)
	}
	return strings.Join(out, " "), nil
}

func normalizeOAuthScopes(value string) (string, error) {
	if strings.TrimSpace(value) == "" || value == "<nil>" {
		value = oauthDefaultScopes
	}
	allowed := map[string]bool{"openid": true, "profile": true, "email": true}
	return normalizeOAuthListWithAllow(value, allowed, "scope")
}

func normalizeOAuthGrants(value string) (string, error) {
	if strings.TrimSpace(value) == "" || value == "<nil>" {
		value = oauthDefaultGrants
	}
	allowed := map[string]bool{"authorization_code": true, "refresh_token": true}
	grants, err := normalizeOAuthListWithAllow(value, allowed, "grant")
	if err != nil {
		return "", err
	}
	if !containsOAuthActionValue(grants, "authorization_code") {
		return "", fmt.Errorf("authorization_code grant is required")
	}
	return grants, nil
}

func normalizeOAuthListWithAllow(value string, allowed map[string]bool, label string) (string, error) {
	parts := splitOAuthActionList(value)
	if len(parts) == 0 {
		return "", fmt.Errorf("at least one %s is required", label)
	}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if !allowed[part] {
			return "", fmt.Errorf("unsupported oauth %s: %s", label, part)
		}
		out = append(out, part)
	}
	return strings.Join(out, " "), nil
}

func splitOAuthActionList(value string) []string {
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

func containsOAuthActionValue(list string, value string) bool {
	for _, part := range splitOAuthActionList(list) {
		if part == value {
			return true
		}
	}
	return false
}

func oauthActionBool(value interface{}, defaultValue bool) bool {
	if value == nil {
		return defaultValue
	}
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
		if v == "" || v == "<nil>" {
			return defaultValue
		}
		return v == "true" || v == "1" || v == "yes"
	default:
		return defaultValue
	}
}

func oauthActionInt64(value interface{}) int64 {
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

func NewOAuthClientRegisterPerformer(cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {
	return &oauthClientActionPerformer{name: "oauth.client.register", cruds: cruds}, nil
}

func NewOAuthClientUpdatePerformer(cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {
	return &oauthClientActionPerformer{name: "oauth.client.update", cruds: cruds}, nil
}

func NewOAuthClientRotateSecretPerformer(cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {
	return &oauthClientActionPerformer{name: "oauth.client.rotate_secret", cruds: cruds}, nil
}

func NewOAuthClientDisablePerformer(cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {
	return &oauthClientActionPerformer{name: "oauth.client.disable", cruds: cruds}, nil
}

func NewOAuthClientEnablePerformer(cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {
	return &oauthClientActionPerformer{name: "oauth.client.enable", cruds: cruds}, nil
}

func NewOAuthClientRevokeTokensPerformer(cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {
	return &oauthClientActionPerformer{name: "oauth.client.revoke_tokens", cruds: cruds}, nil
}
