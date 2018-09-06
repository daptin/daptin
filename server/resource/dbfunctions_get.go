package resource

import (
	"encoding/json"
	"fmt"
	"github.com/daptin/daptin/server/database"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"gopkg.in/Masterminds/squirrel.v1"
	"strconv"
	"strings"
	"time"
	"context"
	"github.com/daptin/daptin/server/statementbuilder"
)

func GetObjectByWhereClause(objType string, db database.DatabaseConnection, queries ...squirrel.Eq) ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, 0)

	builder := statementbuilder.Squirrel.Select("*").From(objType)

	for _, q := range queries {
		builder = builder.Where(q)
	}
	q, v, err := builder.ToSql()

	if err != nil {
		return result, err
	}

	stmt, err := db.Preparex(q)
	defer stmt.Close()
	rows, err := stmt.Queryx(v...)

	if err != nil {
		return result, err
	}
	defer rows.Close()

	return RowsToMap(rows, objType)
}

func GetActionMapByTypeName(db database.DatabaseConnection) (map[string]map[string]interface{}, error) {

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
			log.Infof("Action [%v][%v] already exists", worldIdString, actioName)
		}
		typeActionMap[worldIdString][actioName] = action
	}

	return typeActionMap, err

}

func GetWorldTableMapBy(col string, db database.DatabaseConnection) (map[string]map[string]interface{}, error) {

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

func GetAdminUserIdAndUserGroupId(db database.DatabaseConnection) (int64, int64) {
	var userCount int
	s, v, err := statementbuilder.Squirrel.Select("count(*)").From("user_account").ToSql()
	err = db.QueryRowx(s, v...).Scan(&userCount)
	CheckErr(err, "Failed to get user count")

	var userId int64
	var userGroupId int64

	if userCount < 2 {
		s, v, err := statementbuilder.Squirrel.Select("id").From("user_account").OrderBy("id").Limit(1).ToSql()
		CheckErr(err, "Failed to create select user sql")
		err = db.QueryRowx(s, v...).Scan(&userId)
		CheckErr(err, "Failed to select existing user")
		s, v, err = statementbuilder.Squirrel.Select("id").From("usergroup").Limit(1).ToSql()
		CheckErr(err, "Failed to create user group sql")
		err = db.QueryRowx(s, v...).Scan(&userGroupId)
		CheckErr(err, "Failed to user group")
	} else {
		s, v, err := statementbuilder.Squirrel.Select("id").From("user_account").Where(squirrel.NotEq{"email": "guest@cms.go"}).OrderBy("id").Limit(1).ToSql()
		CheckErr(err, "Failed to create select user sql")
		err = db.QueryRowx(s, v...).Scan(&userId)
		CheckErr(err, "Failed to select existing user")
		s, v, err = statementbuilder.Squirrel.Select("id").From("usergroup").Limit(1).ToSql()
		CheckErr(err, "Failed to create user group sql")
		err = db.QueryRowx(s, v...).Scan(&userGroupId)
		CheckErr(err, "Failed to user group")
	}
	return userId, userGroupId

}

type SubSite struct {
	Id           int64
	Name         string
	Hostname     string
	Path         string
	CloudStoreId *int64 `db:"cloud_store_id"`
	Permission   PermissionInstance
	UserId       *int64 `db:"user_account_id"`
	ReferenceId  string `db:"reference_id"`
}

type CloudStore struct {
	Id              int64
	RootPath        string
	StoreParameters map[string]interface{}
	UserId          string
	OAutoTokenId    string
	Name            string
	StoreType       string
	StoreProvider   string
	Version         int
	CreatedAt       *time.Time
	UpdatedAt       *time.Time
	DeletedAt       *time.Time
	ReferenceId     string
	Permission      PermissionInstance
}

func (resource *DbResource) GetAllCloudStores() ([]CloudStore, error) {
	cloudStores := []CloudStore{}

	rows, err := resource.GetAllObjects("cloud_store")
	if err != nil {
		return cloudStores, err
	}

	for _, storeMap := range rows {
		var cloudStore CloudStore

		tokenId := storeMap["oauth_token_id"]
		if tokenId == nil {
			log.Infof("Token id for store [%v] is empty", storeMap["name"])
		} else {
			cloudStore.OAutoTokenId = tokenId.(string)
		}
		cloudStore.Name = storeMap["name"].(string)

		id, ok := storeMap["id"].(int64)
		if !ok {
			id, err = strconv.ParseInt(storeMap["id"].(string), 10, 64)
			CheckErr(err, "Failed to parse id as int in loading stores")
		}

		cloudStore.Id = id
		cloudStore.ReferenceId = storeMap["reference_id"].(string)
		CheckErr(err, "Failed to parse permission as int in loading stores")
		cloudStore.Permission = resource.GetObjectPermissionByReferenceId("cloud_store", cloudStore.ReferenceId)

		cloudStore.UserId = storeMap["user_account_id"].(string)

		createdAt, ok := storeMap["created_at"].(time.Time)
		if !ok {
			createdAt, _ = time.Parse(storeMap["created_at"].(string), "2006-01-02 15:04:05")
		}

		cloudStore.CreatedAt = &createdAt
		if storeMap["updated_at"] != nil {
			updatedAt, ok := storeMap["updated_at"].(time.Time)
			if !ok {
				updatedAt, _ = time.Parse(storeMap["updated_at"].(string), "2006-01-02 15:04:05")
			}
			cloudStore.UpdatedAt = &updatedAt
		}
		storeParameters := storeMap["store_parameters"].(string)

		storeParamMap := make(map[string]interface{})

		json.Unmarshal([]byte(storeParameters), &storeParamMap)

		cloudStore.StoreParameters = storeParamMap
		cloudStore.StoreProvider = storeMap["store_provider"].(string)
		cloudStore.StoreType = storeMap["store_type"].(string)
		cloudStore.RootPath = storeMap["root_path"].(string)

		version, ok := storeMap["version"].(int64)
		if !ok {
			version, _ = strconv.ParseInt(storeMap["version"].(string), 10, 64)
		}

		cloudStore.Version = int(version)

		cloudStores = append(cloudStores, cloudStore)
	}

	return cloudStores, nil

}

func (resource *DbResource) GetCloudStoreByName(name string) (CloudStore, error) {
	var cloudStore CloudStore

	rows, _, err := resource.GetRowsByWhereClause("cloud_store", squirrel.Eq{"name": name})

	if err == nil && len(rows) > 0 {
		row := rows[0]
		cloudStore.Name = row["name"].(string)
		cloudStore.StoreType = row["store_type"].(string)
		params := make(map[string]interface{})
		err = json.Unmarshal([]byte(row["store_parameters"].(string)), params)
		CheckInfo(err, "Failed to unmarshal store provider parameters [%v]", cloudStore.Name)
		cloudStore.StoreParameters = params
		cloudStore.RootPath = row["root_path"].(string)
		cloudStore.StoreProvider = row["store_provider"].(string)
		if row["oauth_token_id"] != nil {
			cloudStore.OAutoTokenId = row["oauth_token_id"].(string)
		}
	}

	return cloudStore, nil

}

func (resource *DbResource) GetCloudStoreByReferenceId(referenceID string) (CloudStore, error) {
	var cloudStore CloudStore

	rows, _, err := resource.GetRowsByWhereClause("cloud_store", squirrel.Eq{"reference_id": referenceID})

	if err == nil && len(rows) > 0 {
		row := rows[0]
		cloudStore.Name = row["name"].(string)
		cloudStore.StoreType = row["store_type"].(string)
		params := make(map[string]interface{})
		err = json.Unmarshal([]byte(row["store_parameters"].(string)), params)
		CheckInfo(err, "Failed to unmarshal store provider parameters [%v]", cloudStore.Name)
		cloudStore.StoreParameters = params
		cloudStore.RootPath = row["root_path"].(string)
		cloudStore.StoreProvider = row["store_provider"].(string)
		if row["oauth_token_id"] != nil {
			cloudStore.OAutoTokenId = row["oauth_token_id"].(string)
		}
	}

	return cloudStore, nil

}

func (resource *DbResource) GetAllMarketplaces() ([]Marketplace, error) {

	marketPlaces := []Marketplace{}

	s, v, err := statementbuilder.Squirrel.Select("s.endpoint", "s.root_path", "s.permission", "s.user_account_id", "s.reference_id").
		From("marketplace s").
		ToSql()
	if err != nil {
		return marketPlaces, err
	}

	rows, err := resource.db.Queryx(s, v...)
	if err != nil {
		return marketPlaces, err
	}
	defer rows.Close()

	for rows.Next() {
		var marketplace Marketplace
		err = rows.StructScan(&marketplace)
		if err != nil {
			log.Errorf("Failed to scan marketplace from db to struct: %v", err)
			continue
		}
		marketPlaces = append(marketPlaces, marketplace)
	}

	return marketPlaces, nil

}

func (resource *DbResource) GetAllTasks() ([]Task, error) {

	tasks := []Task{}

	s, v, err := statementbuilder.Squirrel.Select("t.name", "t.action_name", "t.entity_name", "t.schedule", "t.active", "t.attributes", "t.as_user_id").
		From("task t").
		ToSql()
	if err != nil {
		return tasks, err
	}

	rows, err := resource.db.Queryx(s, v...)
	if err != nil {
		return tasks, err
	}
	defer rows.Close()

	for rows.Next() {
		var task Task
		err = rows.Scan(&task.Name, &task.ActionName, &task.EntityName, &task.Schedule, &task.Active, &task.AttributesJson, &task.AsUserEmail)
		if err != nil {
			log.Errorf("Failed to scan task from db to struct: %v", err)
			continue
		}
		err = json.Unmarshal([]byte(task.AttributesJson), &task.Attributes)
		if CheckErr(err, "Failed to unmarshal attributes for task") {
			continue
		}
		tasks = append(tasks, task)
	}

	return tasks, nil

}

func (resource *DbResource) GetMarketplaceByReferenceId(referenceId string) (Marketplace, error) {

	marketPlace := Marketplace{}

	s, v, err := statementbuilder.Squirrel.Select("s.endpoint", "s.root_path", "s.permission", "s.user_account_id", "s.reference_id").
		From("marketplace s").Where(squirrel.Eq{"reference_id": referenceId}).
		ToSql()
	if err != nil {
		return marketPlace, err
	}

	err = resource.db.QueryRowx(s, v...).StructScan(&marketPlace)
	return marketPlace, err

}

func (resource *DbResource) GetAllSites() ([]SubSite, error) {

	sites := []SubSite{}

	s, v, err := statementbuilder.Squirrel.Select("s.name", "s.hostname", "s.cloud_store_id", "s.user_account_id", "s.path", "s.reference_id", "s.id").
		From("site s").
		ToSql()
	if err != nil {
		return sites, err
	}

	rows, err := resource.db.Queryx(s, v...)
	if err != nil {
		return sites, err
	}
	defer rows.Close()

	for rows.Next() {
		var site SubSite
		err = rows.StructScan(&site)
		if err != nil {
			log.Errorf("Failed to scan site from db to struct: %v", err)
		}
		perm := resource.GetObjectPermissionByReferenceId("site", site.ReferenceId)
		site.Permission = perm
		sites = append(sites, site)
	}

	return sites, nil

}

func (resource *DbResource) GetOauthDescriptionByTokenId(id int64) (*oauth2.Config, error) {

	var clientId, clientSecret, redirectUri, authUrl, tokenUrl, scope string

	s, v, err := statementbuilder.Squirrel.
		Select("oc.client_id", "oc.client_secret", "oc.redirect_uri", "oc.auth_url", "oc.token_url", "oc.scope").
		From("oauth_token ot").Join("oauth_connect oc").
		JoinClause("on oc.id = ot.oauth_connect_id").
		Where(squirrel.Eq{"ot.id": id}).ToSql()

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

	s, v, err := statementbuilder.Squirrel.
		Select("oc.client_id", "oc.client_secret", "oc.redirect_uri", "oc.auth_url", "oc.token_url", "oc.scope").
		From("oauth_token ot").Join("oauth_connect oc").
		JoinClause("on oc.id = ot.oauth_connect_id").
		Where(squirrel.Eq{"ot.reference_id": referenceId}).ToSql()

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

func (resource *DbResource) GetTokenByTokenReferenceId(referenceId string) (*oauth2.Token, *oauth2.Config, error) {
	oauthConf := &oauth2.Config{}

	var access_token, refresh_token, token_type string
	var expires_in int64
	var token oauth2.Token
	s, v, err := statementbuilder.Squirrel.Select("access_token", "refresh_token", "token_type", "expires_in").From("oauth_token").
		Where(squirrel.Eq{"reference_id": referenceId}).ToSql()

	if err != nil {
		return nil, oauthConf, err
	}

	err = resource.db.QueryRowx(s, v...).Scan(&access_token, &refresh_token, &token_type, &expires_in)

	if err != nil {
		return nil, oauthConf, err
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

	// check validity and refresh if required
	oauthConf, err = resource.GetOauthDescriptionByTokenReferenceId(referenceId)
	if err != nil {
		log.Infof("Failed to get oauth token configuration for token refresh: %v", err)
	} else {
		if !token.Valid() {
			ctx := context.Background()
			tokenSource := oauthConf.TokenSource(ctx, &token)
			refreshedToken, err := tokenSource.Token()
			CheckErr(err, "Failed to get new oauth2 access token")
			if refreshedToken == nil {
				log.Errorf("Failed to obtain a valid oauth2 token: %v", referenceId)
				return nil, oauthConf, err
			} else {
				token = *refreshedToken
				err = resource.UpdateAccessTokenByTokenReferenceId(referenceId, refreshedToken.AccessToken, refreshedToken.Expiry.Unix())
				CheckErr(err, "failed to update access token")
			}
		}
	}

	return &token, oauthConf, err

}

func (resource *DbResource) GetTokenByTokenId(id int64) (*oauth2.Token, error) {

	var access_token, refresh_token, token_type string
	var expires_in int64
	var token oauth2.Token
	s, v, err := statementbuilder.Squirrel.Select("access_token", "refresh_token", "token_type", "expires_in").From("oauth_token").
		Where(squirrel.Eq{"id": id}).ToSql()

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

func (resource *DbResource) GetTokenByTokenName(name string) (*oauth2.Token, error) {

	var access_token, refresh_token, token_type string
	var expires_in int64
	var token oauth2.Token
	s, v, err := statementbuilder.Squirrel.Select("access_token", "refresh_token", "token_type", "expires_in").From("oauth_token").
		Where(squirrel.Eq{"token_type": name}).OrderBy("created_at desc").Limit(1).ToSql()

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
