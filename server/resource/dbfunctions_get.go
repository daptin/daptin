package resource

import (
	"context"
	"fmt"
	"github.com/Masterminds/squirrel"
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"strconv"
	"strings"
	"time"
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
	if stmt != nil {
		defer func() {
			err = stmt.Close()
			CheckErr(err, "Failed to close prepared query [%v]", objType)
		}()
	} else {
		return nil, err
	}
	rows, err := stmt.Queryx(v...)

	if err != nil {
		return result, err
	}
	if rows != nil {
		defer func() {
			err = rows.Close()
			CheckErr(err, "Failed to close rows after get object by where clause [%s]", objType)
		}()
	}

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

func GetAdminUserIdAndUserGroupId(db sqlx.Ext) (int64, int64) {
	var userCount int
	s, v, err := statementbuilder.Squirrel.Select("count(*)").From(USER_ACCOUNT_TABLE_NAME).ToSql()
	err = db.QueryRowx(s, v...).Scan(&userCount)
	CheckErr(err, "Failed to get user count")

	var userId int64
	var userGroupId int64

	if userCount < 2 {
		s, v, err := statementbuilder.Squirrel.Select("id").From(USER_ACCOUNT_TABLE_NAME).OrderBy("id").Limit(1).ToSql()
		CheckErr(err, "Failed to create select user sql")
		err = db.QueryRowx(s, v...).Scan(&userId)
		CheckErr(err, "Failed to select existing user")
		s, v, err = statementbuilder.Squirrel.Select("id").From("usergroup").Limit(1).ToSql()
		CheckErr(err, "Failed to create user group sql")
		err = db.QueryRowx(s, v...).Scan(&userGroupId)
		CheckErr(err, "Failed to user group")
	} else {
		s, v, err := statementbuilder.Squirrel.Select("id").From(USER_ACCOUNT_TABLE_NAME).Where(squirrel.NotEq{"email": "guest@cms.go"}).OrderBy("id").Limit(1).ToSql()
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
	SiteType     string `db:"site_type"`
	FtpEnabled   bool   `db:"ftp_enabled"`
	UserId       *int64 `db:"user_account_id"`
	ReferenceId  string `db:"reference_id"`
	Enable       bool   `db:"enable"`
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
	var cloudStores []CloudStore

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

		if storeMap[USER_ACCOUNT_ID_COLUMN] != nil {
			cloudStore.UserId = storeMap[USER_ACCOUNT_ID_COLUMN].(string)
		}

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

		if len(storeParameters) > 0 {
			err = json.Unmarshal([]byte(storeParameters), &storeParamMap)
			CheckErr(err, "Failed to unmarshal store parameters for store %v", storeMap["name"])
		}

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

type Integration struct {
	Name                        string
	SpecificationLanguage       string
	SpecificationFormat         string
	Specification               string
	AuthenticationType          string
	AuthenticationSpecification string
	Enable                      bool
}

func (resource *DbResource) GetActiveIntegrations() ([]Integration, error) {

	integrations := make([]Integration, 0)
	rows, _, err := resource.GetRowsByWhereClause("integration")
	if err == nil && len(rows) > 0 {

		for _, row := range rows {
			i, ok := row["enable"].(int64)
			if !ok {
				iI, ok := row["enable"].(int)

				if ok {
					i = int64(iI)
				} else {
					strI, ok := row["enable"].(string)
					if ok {
						i, err = strconv.ParseInt(strI, 10, 32)
						CheckErr(err, "Failed to convert column 'enable' value to int")
					}

				}

			}

			integration := Integration{
				Name:                        row["name"].(string),
				SpecificationLanguage:       row["specification_language"].(string),
				SpecificationFormat:         row["specification_format"].(string),
				Specification:               row["specification"].(string),
				AuthenticationType:          row["authentication_type"].(string),
				AuthenticationSpecification: row["authentication_specification"].(string),
				Enable:                      i == 1,
			}
			integrations = append(integrations, integration)
		}

	}

	return integrations, err

}

func (resource *DbResource) GetCloudStoreByName(name string) (CloudStore, error) {
	var cloudStore CloudStore

	rows, _, err := resource.GetRowsByWhereClause("cloud_store", squirrel.Eq{"name": name})

	if err == nil && len(rows) > 0 {
		row := rows[0]
		cloudStore.Name = row["name"].(string)
		cloudStore.StoreType = row["store_type"].(string)
		params := make(map[string]interface{})
		if row["store_parameters"] != nil && row["store_parameters"].(string) != "" {
			err = json.Unmarshal([]byte(row["store_parameters"].(string)), &params)
			CheckInfo(err, "Failed to unmarshal store provider parameters [%v]", cloudStore.Name)
		}
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
		if row["store_parameters"] != nil && row["store_parameters"].(string) != "" {
			err = json.Unmarshal([]byte(row["store_parameters"].(string)), &params)
			CheckInfo(err, "Failed to unmarshal store provider parameters [%v]", cloudStore.Name)
		}
		cloudStore.StoreParameters = params
		cloudStore.RootPath = row["root_path"].(string)
		cloudStore.StoreProvider = row["store_provider"].(string)
		if row["oauth_token_id"] != nil {
			cloudStore.OAutoTokenId = row["oauth_token_id"].(string)
		}
	}

	return cloudStore, nil

}

func (resource *DbResource) GetAllTasks() ([]Task, error) {

	var tasks []Task

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

func (resource *DbResource) GetAllSites() ([]SubSite, error) {

	var sites []SubSite

	s, v, err := statementbuilder.Squirrel.Select("s.name", "s.hostname", "s.cloud_store_id",
		"s."+USER_ACCOUNT_ID_COLUMN, "s.path", "s.reference_id", "s.id", "s.enable", "s.site_type", "s.ftp_enabled").
		From("site s").
		ToSql()
	if err != nil {
		return sites, err
	}

	rows, err := resource.db.Queryx(s, v...)
	if err != nil {
		return sites, err
	}
	defer func() {
		err = rows.Close()
		CheckErr(err, "Failed to close rows after getting all sites")
	}()

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
