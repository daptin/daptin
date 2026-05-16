package actions

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
	"golang.org/x/oauth2"
)

const oauthStateLifetimeSeconds = int64(600)

type oauthStateRecord struct {
	Row                  map[string]interface{}
	ReferenceID          daptinid.DaptinReferenceId
	CodeVerifier         string
	OwnerUserID          int64
	OwnerUserReferenceID daptinid.DaptinReferenceId
}

func oauthBool(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case int:
		return val != 0
	case int64:
		return val != 0
	case int32:
		return val != 0
	case uint:
		return val != 0
	case uint64:
		return val != 0
	case []byte:
		return oauthBool(string(val))
	case string:
		switch strings.ToLower(strings.TrimSpace(val)) {
		case "1", "true", "t", "yes", "y", "on":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func oauthConnectorPKCEEnabled(row map[string]interface{}) bool {
	return oauthBool(row["pkce_enabled"])
}

func oauthConnectorPKCEChallengeMethod(row map[string]interface{}) string {
	method := strings.TrimSpace(fmt.Sprintf("%v", row["pkce_challenge_method"]))
	if method == "" || method == "<nil>" {
		return "S256"
	}
	return strings.ToUpper(method)
}

func oauthVerifierForConnector(row map[string]interface{}) (string, error) {
	if !oauthConnectorPKCEEnabled(row) {
		return "", nil
	}
	if method := oauthConnectorPKCEChallengeMethod(row); method != "S256" {
		return "", fmt.Errorf("unsupported PKCE challenge method [%s]", method)
	}
	return oauth2.GenerateVerifier(), nil
}

func oauthAuthorizationOptions(row map[string]interface{}, verifier string) ([]oauth2.AuthCodeOption, error) {
	opts := []oauth2.AuthCodeOption{}
	if oauthBool(row["access_type_offline"]) {
		opts = append(opts, oauth2.AccessTypeOffline)
		opts = append(opts, oauth2.SetAuthURLParam("prompt", "consent"))
	}
	if oauthConnectorPKCEEnabled(row) {
		if verifier == "" {
			return nil, fmt.Errorf("missing PKCE verifier")
		}
		if method := oauthConnectorPKCEChallengeMethod(row); method != "S256" {
			return nil, fmt.Errorf("unsupported PKCE challenge method [%s]", method)
		}
		opts = append(opts, oauth2.S256ChallengeOption(verifier))
	}
	return opts, nil
}

func oauthStateHash(state string) string {
	return resource.OAuthHashToken(state)
}

func oauthActionRequest(method string, tableName string, sessionUser *auth.SessionUser) api2go.Request {
	ur, _ := url.Parse("/" + tableName)
	req := &http.Request{
		Method: method,
		URL:    ur,
	}
	req = req.WithContext(context.WithValue(context.Background(), "user", sessionUser))
	return api2go.Request{PlainRequest: req}
}

func storeOAuthState(cruds map[string]*resource.DbResource, oauthConnectReferenceId daptinid.DaptinReferenceId, state string, verifier string, now time.Time, sessionUser *auth.SessionUser, transaction *sqlx.Tx) error {
	if cruds["oauth_state"] == nil {
		return fmt.Errorf("oauth_state resource is not available")
	}
	attrs := map[string]interface{}{
		"state_hash":       oauthStateHash(state),
		"expires_at":       now.Unix() + oauthStateLifetimeSeconds,
		"oauth_connect_id": oauthConnectReferenceId,
	}
	if strings.TrimSpace(verifier) != "" {
		attrs["code_verifier"] = verifier
	}
	model := api2go.NewApi2GoModelWithData("oauth_state", nil, int64(auth.DEFAULT_PERMISSION), nil, attrs)
	_, err := cruds["oauth_state"].CreateWithoutFilter(model, oauthActionRequest("POST", "oauth_state", sessionUser), transaction)
	return err
}

func loadOAuthState(cruds map[string]*resource.DbResource, configStore *resource.ConfigStore, oauthConnectReferenceId daptinid.DaptinReferenceId, state string, now time.Time, requireVerifier bool, transaction *sqlx.Tx) (*oauthStateRecord, error) {
	if cruds["oauth_state"] == nil {
		return nil, fmt.Errorf("oauth_state resource is not available")
	}
	rows, _, err := cruds["oauth_state"].GetRowsByWhereClauseWithTransaction("oauth_state", nil, transaction, goqu.Ex{"state_hash": oauthStateHash(state)})
	if err != nil {
		return nil, err
	}
	if len(rows) < 1 {
		return nil, fmt.Errorf("no ongoing authentication")
	}

	row := rows[0]
	if daptinid.InterfaceToDIR(row["oauth_connect_id"]) != oauthConnectReferenceId {
		return nil, fmt.Errorf("oauth state does not belong to authenticator")
	}
	if oauthActionInt64(row["expires_at"]) <= now.Unix() {
		return nil, fmt.Errorf("oauth state expired")
	}
	if oauthActionInt64(row["used_at"]) > 0 {
		return nil, fmt.Errorf("oauth state already used")
	}

	var verifier string
	if requireVerifier {
		encryptedVerifier := strings.TrimSpace(fmt.Sprintf("%v", row["code_verifier"]))
		if encryptedVerifier == "" || encryptedVerifier == "<nil>" {
			return nil, fmt.Errorf("missing PKCE verifier")
		}
		secret, err := configStore.GetConfigValueFor("encryption.secret", "backend", transaction)
		if err != nil {
			return nil, err
		}
		verifier, err = resource.Decrypt([]byte(secret), encryptedVerifier)
		if err != nil {
			return nil, err
		}
	}

	ownerUserReferenceID := daptinid.InterfaceToDIR(row[resource.USER_ACCOUNT_ID_COLUMN])
	var ownerUserID int64
	if ownerUserReferenceID != daptinid.NullReferenceId {
		ownerUserID, err = resource.GetReferenceIdToIdWithTransaction(resource.USER_ACCOUNT_TABLE_NAME, ownerUserReferenceID, transaction)
		if err != nil {
			return nil, err
		}
	}

	return &oauthStateRecord{
		Row:                  row,
		ReferenceID:          daptinid.InterfaceToDIR(row["reference_id"]),
		CodeVerifier:         verifier,
		OwnerUserID:          ownerUserID,
		OwnerUserReferenceID: ownerUserReferenceID,
	}, nil
}

func oauthStateOwnerSession(record *oauthStateRecord, fallback *auth.SessionUser) *auth.SessionUser {
	if record == nil || record.OwnerUserID == 0 || record.OwnerUserReferenceID == daptinid.NullReferenceId {
		return fallback
	}
	owner := &auth.SessionUser{
		UserId:          record.OwnerUserID,
		UserReferenceId: record.OwnerUserReferenceID,
	}
	if fallback != nil {
		owner.Groups = fallback.Groups
		owner.AuthVersion = fallback.AuthVersion
	}
	return owner
}

func markOAuthStateUsed(cruds map[string]*resource.DbResource, record *oauthStateRecord, now time.Time, sessionUser *auth.SessionUser, transaction *sqlx.Tx) error {
	if cruds["oauth_state"] == nil {
		return fmt.Errorf("oauth_state resource is not available")
	}
	if record == nil || record.ReferenceID == daptinid.NullReferenceId {
		return fmt.Errorf("oauth state reference id missing")
	}
	model := api2go.NewApi2GoModelWithData("oauth_state", nil, int64(auth.DEFAULT_PERMISSION), nil, record.Row)
	model.SetID(record.ReferenceID.String())
	model.SetAttributes(map[string]interface{}{
		"used_at": now.Unix(),
	})
	_, err := cruds["oauth_state"].UpdateWithoutFilters(model, oauthActionRequest("PATCH", "oauth_state", sessionUser), transaction)
	return err
}

func exchangeOAuthCode(ctx context.Context, conf *oauth2.Config, code string, row map[string]interface{}, state *oauthStateRecord) (*oauth2.Token, error) {
	if oauthConnectorPKCEEnabled(row) {
		if state == nil || state.CodeVerifier == "" {
			return nil, fmt.Errorf("missing PKCE verifier")
		}
		return conf.Exchange(ctx, code, oauth2.VerifierOption(state.CodeVerifier))
	}
	return conf.Exchange(ctx, code)
}
