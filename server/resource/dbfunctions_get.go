package resource

import (
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"gopkg.in/Masterminds/squirrel.v1"
	"fmt"
	"strings"
	"golang.org/x/oauth2"
	"time"
)

func GetObjectByWhereClause(objType string, db *sqlx.DB, queries ...squirrel.Eq) ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, 0)

	builder := squirrel.Select("*").From(objType).Where(squirrel.Eq{"deleted_at": nil})

	for _, q := range queries {
		builder = builder.Where(q)
	}
	q, v, err := builder.ToSql()

	if err != nil {
		return result, err
	}

	rows, err := db.Queryx(q, v...)

	if err != nil {
		return result, err
	}

	return RowsToMap(rows, objType)
}

func GetActionMapByTypeName(db *sqlx.DB) (map[string]map[string]interface{}, error) {

	allActions, err := GetObjectByWhereClause("action", db)
	if err != nil {
		return nil, err
	}

	typeActionMap := make(map[string]map[string]interface{})

	for _, action := range allActions {
		actioName := action["action_name"].(string)
		worldIdString := fmt.Sprintf("%v", action["world_id"])

		_, ok := typeActionMap[worldIdString]
		if !ok {
			typeActionMap[worldIdString] = make(map[string]interface{})
		}

		_, ok = typeActionMap[worldIdString][actioName]
		if ok {
			log.Infof("Action [%v][%v] already exisys", worldIdString, actioName)
		}
		typeActionMap[worldIdString][actioName] = action
	}

	return typeActionMap, err

}

func GetWorldTableMapBy(col string, db *sqlx.DB) (map[string]map[string]interface{}, error) {

	allWorlds, err := GetObjectByWhereClause("world", db)
	if err != nil {
		return nil, err
	}

	resMap := make(map[string]map[string]interface{})

	for _, world := range allWorlds {
		resMap[world[col].(string)] = world
	}
	return resMap, err

}

func GetAdminUserIdAndUserGroupId(db *sqlx.DB) (int64, int64) {
	var userCount int
	s, v, err := squirrel.Select("count(*)").From("user").Where(squirrel.Eq{"deleted_at": nil}).ToSql()
	err = db.QueryRowx(s, v...).Scan(&userCount)
	CheckErr(err, "Failed to get user count")

	var userId int64
	var userGroupId int64

	if userCount < 2 {
		s, v, err := squirrel.Select("id").From("user").Where(squirrel.Eq{"deleted_at": nil}).OrderBy("id").Limit(1).ToSql()
		CheckErr(err, "Failed to create select user sql")
		err = db.QueryRowx(s, v...).Scan(&userId)
		CheckErr(err, "Failed to select existing user")
		s, v, err = squirrel.Select("id").From("usergroup").Where(squirrel.Eq{"deleted_at": nil}).Limit(1).ToSql()
		CheckErr(err, "Failed to create user group sql")
		err = db.QueryRowx(s, v...).Scan(&userGroupId)
		CheckErr(err, "Failed to user group")
	} else {

		s, v, err := squirrel.Select("id").From("user").Where(squirrel.Eq{"deleted_at": nil}).Where(squirrel.NotEq{"email": "guest@cms.go"}).OrderBy("id").Limit(1).ToSql()
		CheckErr(err, "Failed to create select user sql")
		err = db.QueryRowx(s, v...).Scan(&userId)
		CheckErr(err, "Failed to select existing user")
		s, v, err = squirrel.Select("id").From("usergroup").Where(squirrel.Eq{"deleted_at": nil}).Limit(1).ToSql()
		CheckErr(err, "Failed to create user group sql")
		err = db.QueryRowx(s, v...).Scan(&userGroupId)
		CheckErr(err, "Failed to user group")
	}
	return userId, userGroupId

}

type SubSite struct {
}

func (resource *DbResource) GetSites() ([]SubSite, error) {

	sites := []SubSite{}

	return sites, nil

}

func (resource *DbResource) GetOauthDescriptionByTokenId(id *int64) (*oauth2.Config, error) {

	var clientId, clientSecret, redirectUri, authUrl, tokenUrl, scope string

	s, v, err := squirrel.
	Select("oc.client_id", "oc.client_secret", "oc.redirect_uri", "oc.auth_url", "oc.token_url", "oc.scope").
			From("oauth_token ot").Join("oauth_connect oc").
			JoinClause("on oc.id = ot.oauth_connect_id").
			Where(squirrel.Eq{"ot.deleted_at": nil}).Where(squirrel.Eq{"ot.id": id}).ToSql()

	if err != nil {
		return nil, err
	}

	err = resource.db.QueryRowx(s, v...).Scan(&clientId, &clientSecret, &redirectUri, &authUrl, &tokenUrl, &scope)

	if err != nil {
		return nil, err
	}

	encryptionSecret, err := resource.configStore.GetConfigValueFor("encryption.secret", "backend")
	if err != nil {
		return nil, err
	}

	clientSecret, err = Decrypt([]byte(encryptionSecret), clientSecret)
	if err != nil {
		return nil, err
	}

	conf := &oauth2.Config{
		ClientID:     clientId,
		ClientSecret: clientSecret,
		RedirectURL:  redirectUri,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authUrl,
			TokenURL: tokenUrl,
		},
		Scopes: strings.Split(scope, ","),
	}

	return conf, nil

}

func (resource *DbResource) GetOauthDescriptionByTokenReferenceId(referenceId string) (*oauth2.Config, error) {

	var clientId, clientSecret, redirectUri, authUrl, tokenUrl, scope string

	s, v, err := squirrel.
	Select("oc.client_id", "oc.client_secret", "oc.redirect_uri", "oc.auth_url", "oc.token_url", "oc.scope").
			From("oauth_token ot").Join("oauth_connect oc").
			JoinClause("on oc.id = ot.oauth_connect_id").
			Where(squirrel.Eq{"ot.deleted_at": nil}).Where(squirrel.Eq{"ot.reference_id": referenceId}).ToSql()

	if err != nil {
		return nil, err
	}

	err = resource.db.QueryRowx(s, v...).Scan(&clientId, &clientSecret, &redirectUri, &authUrl, &tokenUrl, &scope)

	if err != nil {
		return nil, err
	}

	encryptionSecret, err := resource.configStore.GetConfigValueFor("encryption.secret", "backend")
	if err != nil {
		return nil, err
	}

	clientSecret, err = Decrypt([]byte(encryptionSecret), clientSecret)
	if err != nil {
		return nil, err
	}

	conf := &oauth2.Config{
		ClientID:     clientId,
		ClientSecret: clientSecret,
		RedirectURL:  redirectUri,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authUrl,
			TokenURL: tokenUrl,
		},
		Scopes: strings.Split(scope, ","),
	}

	return conf, nil

}

func (resource *DbResource) GetTokenByTokenReferenceId(referenceId string) (*oauth2.Token, error) {

	var access_token, refresh_token, token_type string
	var expires_in int64
	var token oauth2.Token
	s, v, err := squirrel.Select("access_token", "refresh_token", "token_type", "expires_in").From("oauth_token").
			Where(squirrel.Eq{"deleted_at": nil}).Where(squirrel.Eq{"reference_id": referenceId}).ToSql()

	if err != nil {
		return nil, err
	}

	err = resource.db.QueryRowx(s, v...).Scan(&access_token, &refresh_token, &token_type, &expires_in)

	if err != nil {
		return nil, err
	}

	secret, err := resource.configStore.GetConfigValueFor("encryption.secret", "backend")
	CheckErr(err, "Failed to get encryption secret")

	dec, err := Decrypt([]byte(secret), access_token)
	CheckErr(err, "Failed to decrypt access token")

	ref, err := Decrypt([]byte(secret), refresh_token)
	CheckErr(err, "Failed to decrypt refresh token")

	token.AccessToken = dec
	token.RefreshToken = ref
	token.TokenType = "Bearer"
	token.Expiry = time.Unix(expires_in, 0)

	return &token, err

}

func (resource *DbResource) GetTokenByTokenId(id *int64) (*oauth2.Token, error) {

	var access_token, refresh_token, token_type string
	var expires_in int64
	var token oauth2.Token
	s, v, err := squirrel.Select("access_token", "refresh_token", "token_type", "expires_in").From("oauth_token").
			Where(squirrel.Eq{"deleted_at": nil}).Where(squirrel.Eq{"id": id}).ToSql()

	if err != nil {
		return nil, err
	}

	err = resource.db.QueryRowx(s, v...).Scan(&access_token, &refresh_token, &token_type, &expires_in)

	if err != nil {
		return nil, err
	}

	secret, err := resource.configStore.GetConfigValueFor("encryption.secret", "backend")
	CheckErr(err, "Failed to get encryption secret")

	dec, err := Decrypt([]byte(secret), access_token)
	CheckErr(err, "Failed to decrypt access token")

	ref, err := Decrypt([]byte(secret), refresh_token)
	CheckErr(err, "Failed to decrypt refresh token")

	token.AccessToken = dec
	token.RefreshToken = ref
	token.TokenType = token_type
	token.Expiry = time.Unix(expires_in, 0)

	return &token, err

}
