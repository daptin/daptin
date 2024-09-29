package resource

import (
	"context"
	"fmt"
	"github.com/buraksezer/olric"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"strconv"
	"strings"
	"time"
)

func GetObjectByWhereClauseWithTransaction(objType string, transaction *sqlx.Tx, queries ...goqu.Ex) ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, 0)

	builder := statementbuilder.Squirrel.Select(goqu.L("*")).Prepared(true).From(objType)

	for _, q := range queries {
		builder = builder.Where(q)
	}
	q, v, err := builder.ToSQL()

	if err != nil {
		return result, err
	}

	stmt, err := transaction.Preparex(q)
	if stmt != nil {
		defer func() {
			err = stmt.Close()
			CheckErr(err, "Failed to close prepared query [%v]", objType)
		}()
	} else {
		return nil, err
	}

	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt)

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

func GetActionMapByTypeName(transaction *sqlx.Tx) (map[string]map[string]interface{}, error) {

	allActions, err := GetObjectByWhereClauseWithTransaction("action", transaction)
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
			log.Debugf("Action [%v][%v] already exists", worldIdString, actioName)
		}
		typeActionMap[worldIdString][actioName] = action
	}

	return typeActionMap, err

}

func GetWorldTableMapBy(col string, transaction *sqlx.Tx) (map[string]map[string]interface{}, error) {

	allWorlds, err := GetObjectByWhereClauseWithTransaction("world", transaction)
	if err != nil {
		return nil, err
	}

	resMap := make(map[string]map[string]interface{})

	for _, world := range allWorlds {
		resMap[world[col].(string)] = world
	}
	return resMap, err

}

func GetAdminUserIdAndUserGroupId(db *sqlx.Tx) (int64, int64) {
	var userCount int
	s, v, err := statementbuilder.Squirrel.Select(goqu.L("count(*)")).Prepared(true).From(USER_ACCOUNT_TABLE_NAME).ToSQL()

	err = db.QueryRowx(s, v...).Scan(&userCount)
	CheckErr(err, "Failed to get user count 104")

	var userId int64
	var userGroupId int64

	if userCount < 2 {
		s, v, err := statementbuilder.Squirrel.Select("id").Prepared(true).From(USER_ACCOUNT_TABLE_NAME).
			Order(goqu.C("id").Asc()).Limit(1).ToSQL()
		CheckErr(err, "Failed to create select user sql")
		err = db.QueryRowx(s, v...).Scan(&userId)
		CheckErr(err, "Failed to select existing user")
		s, v, err = statementbuilder.Squirrel.Select("id").Prepared(true).
			From("usergroup").Limit(1).ToSQL()
		CheckErr(err, "Failed to create user group sql")
		err = db.QueryRowx(s, v...).Scan(&userGroupId)
		CheckErr(err, "Failed to user group")
	} else {
		s, v, err := statementbuilder.Squirrel.Select("id").Prepared(true).
			From(USER_ACCOUNT_TABLE_NAME).
			Where(goqu.Ex{"email": goqu.Op{"neq": "guest@cms.go"}}).Order(goqu.C("id").Asc()).Limit(1).ToSQL()
		CheckErr(err, "Failed to create select user sql")
		err = db.QueryRowx(s, v...).Scan(&userId)
		CheckErr(err, "Failed to select existing user")
		s, v, err = statementbuilder.Squirrel.Select("id").Prepared(true).
			From("usergroup").Limit(1).ToSQL()
		CheckErr(err, "Failed to create user group sql")
		err = db.QueryRowx(s, v...).Scan(&userGroupId)
		CheckErr(err, "Failed to user group")
	}
	log.Tracef("GetAdminUserIdAndUserGroupId [%d], [%d]", userId, userGroupId)
	return userId, userGroupId

}

type SubSite struct {
	Id           int64
	Name         string
	Hostname     string
	Path         string
	CloudStoreId *int64 `db:"cloud_store_id"`
	Permission   PermissionInstance
	SiteType     string                     `db:"site_type"`
	FtpEnabled   bool                       `db:"ftp_enabled"`
	UserId       *int64                     `db:"user_account_id"`
	ReferenceId  daptinid.DaptinReferenceId `db:"reference_id"`
	Enable       bool                       `db:"enable"`
}

type CloudStore struct {
	Id              int64
	RootPath        string
	StoreParameters map[string]interface{}
	UserId          daptinid.DaptinReferenceId
	OAutoTokenId    daptinid.DaptinReferenceId
	Name            string
	StoreType       string
	StoreProvider   string
	Version         int
	CreatedAt       *time.Time
	UpdatedAt       *time.Time
	DeletedAt       *time.Time
	ReferenceId     daptinid.DaptinReferenceId
	Permission      PermissionInstance
}

func (dbResource *DbResource) GetAllCloudStores(transaction *sqlx.Tx) ([]CloudStore, error) {
	var cloudStores []CloudStore

	rows, err := dbResource.GetAllObjects("cloud_store", transaction)
	if err != nil {
		return cloudStores, err
	}

	for _, storeRowMap := range rows {
		var cloudStore CloudStore

		tokenId := daptinid.InterfaceToDIR(storeRowMap["oauth_token_id"])
		if tokenId == daptinid.NullReferenceId {
			log.Printf("Token id for store [%v] is empty", storeRowMap["name"])
		} else {
			cloudStore.OAutoTokenId = tokenId
		}
		cloudStore.Name = storeRowMap["name"].(string)

		id, ok := storeRowMap["id"].(int64)
		if !ok {
			id, err = strconv.ParseInt(storeRowMap["id"].(string), 10, 64)
			CheckErr(err, "Failed to parse id as int in loading stores")
		}

		cloudStore.Id = id
		cloudStore.ReferenceId = daptinid.InterfaceToDIR(storeRowMap["reference_id"])
		if cloudStore.ReferenceId == daptinid.NullReferenceId {
			CheckErr(err, "Failed to parse permission as int in loading stores")
		}
		cloudStore.Permission = dbResource.GetObjectPermissionByReferenceId("cloud_store", cloudStore.ReferenceId, transaction)

		if storeRowMap[USER_ACCOUNT_ID_COLUMN] != nil {
			cloudStore.UserId = daptinid.InterfaceToDIR(storeRowMap[USER_ACCOUNT_ID_COLUMN])
		}

		createdAt, ok := storeRowMap["created_at"].(time.Time)
		if !ok {
			createdAt, _ = time.Parse(storeRowMap["created_at"].(string), "2006-01-02 15:04:05")
		}

		cloudStore.CreatedAt = &createdAt
		if storeRowMap["updated_at"] != nil {
			updatedAt, ok := storeRowMap["updated_at"].(time.Time)
			if !ok {
				updatedAt, _ = time.Parse(storeRowMap["updated_at"].(string), "2006-01-02 15:04:05")
			}
			cloudStore.UpdatedAt = &updatedAt
		}
		storeParameters := storeRowMap["store_parameters"].(string)

		storeParamMap := make(map[string]interface{})

		if len(storeParameters) > 0 {
			err = json.Unmarshal([]byte(storeParameters), &storeParamMap)
			CheckErr(err, "Failed to unmarshal store parameters for store %v", storeRowMap["name"])
		}

		cloudStore.StoreParameters = storeParamMap
		cloudStore.StoreProvider = storeRowMap["store_provider"].(string)
		cloudStore.StoreType = storeRowMap["store_type"].(string)
		cloudStore.RootPath = storeRowMap["root_path"].(string)

		version, ok := storeRowMap["version"].(int64)
		if !ok {
			version, _ = strconv.ParseInt(storeRowMap["version"].(string), 10, 64)
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

func (dbResource *DbResource) GetActiveIntegrations(transaction *sqlx.Tx) ([]Integration, error) {

	integrations := make([]Integration, 0)
	rows, _, err := dbResource.GetRowsByWhereClauseWithTransaction("integration", nil, transaction)
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

func (dbResource *DbResource) GetCloudStoreByNameWithTransaction(name string, transaction *sqlx.Tx) (CloudStore, error) {
	var cloudStore CloudStore

	cacheKey := fmt.Sprintf("store-%v", name)
	if OlricCache != nil {
		cachedValue, err := OlricCache.Get(context.Background(), cacheKey)
		if err == nil {
			bytes, err := cachedValue.Byte()
			err = json.Unmarshal(bytes, cloudStore)
			if err == nil {
				return cloudStore, nil
			}
		}
	}

	rows, _, err := dbResource.GetRowsByWhereClauseWithTransaction("cloud_store", nil, transaction, goqu.Ex{"name": name})

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
		cloudStore.OAutoTokenId = daptinid.InterfaceToDIR(row["oauth_token_id"])
		cloudStore.Id = row["id"].(int64)
		cloudStore.ReferenceId = daptinid.InterfaceToDIR(row["reference_id"])
		cloudStore.Version = int(row["version"].(int64))

		if OlricCache != nil {
			asJson := toJson(cloudStore)
			OlricCache.Put(context.Background(), cacheKey, asJson, olric.EX(10*time.Minute), olric.NX())
			//CheckErr(cachePutErr, "[336] failed to store cloud store in cache")
		}
	} else {
		return cloudStore, fmt.Errorf("cloud store not found [%v]", name)
	}

	return cloudStore, nil

}

func (dbResource *DbResource) GetCloudStoreByReferenceId(referenceID daptinid.DaptinReferenceId, transaction *sqlx.Tx) (CloudStore, error) {
	var cloudStore CloudStore = CloudStore{}

	rows, _, err := dbResource.GetRowsByWhereClause("cloud_store", nil, transaction, goqu.Ex{
		"reference_id": referenceID[:],
	})
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
		cloudStore.Id = row["id"].(int64)
		cloudStore.Version = int(row["version"].(int64))
		cloudStore.ReferenceId = daptinid.InterfaceToDIR(row["reference_id"])
		cloudStore.OAutoTokenId = daptinid.InterfaceToDIR(row["oauth_token_id"])
	}

	return cloudStore, nil

}

func (dbResource *DbResource) GetAllTasks() ([]Task, error) {

	var tasks []Task

	s, v, err := statementbuilder.Squirrel.Select(goqu.I("t.name"),
		goqu.I("t.action_name"), goqu.I("t.entity_name"), goqu.I("t.schedule"),
		goqu.I("t.active"), goqu.I("t.attributes"), goqu.I("t.as_user_id")).Prepared(true).
		From(goqu.T("task").As("t")).ToSQL()
	if err != nil {
		return tasks, err
	}

	stmt1, err := dbResource.Connection.Preparex(s)
	if err != nil {
		log.Errorf("[359] failed to prepare statment: %v", err)
		return nil, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	rows, err := stmt1.Queryx(v...)
	if err != nil {
		return tasks, err
	}
	defer func(rows *sqlx.Rows) {
		err := rows.Close()
		if err != nil {
			log.Errorf("[371] failed to close result after value scan in defer")
		}
	}(rows)

	for rows.Next() {
		var task Task
		err = rows.Scan(&task.Name, &task.ActionName, &task.EntityName, &task.Schedule, &task.Active, &task.AttributesJson, &task.AsUserEmail)
		if err != nil {
			log.Errorf("failed to scan task from db to struct: %v", err)
			continue
		}
		err = json.Unmarshal([]byte(task.AttributesJson), &task.Attributes)
		if CheckErr(err, "failed to unmarshal attributes for task") {
			continue
		}
		tasks = append(tasks, task)
	}

	return tasks, nil

}

func (dbResource *DbResource) GetAllSites(transaction *sqlx.Tx) ([]SubSite, error) {

	var sites []SubSite

	s, v, err := statementbuilder.Squirrel.Select(
		goqu.I("s.name"), goqu.I("s.hostname"),
		goqu.I("s.cloud_store_id"),
		goqu.I("s."+USER_ACCOUNT_ID_COLUMN), goqu.I("s.path"),
		goqu.I("s.reference_id"), goqu.I("s.id"), goqu.I("s.enable"),
		goqu.I("s.site_type"), goqu.I("s.ftp_enabled")).Prepared(true).
		From(goqu.T("site").As("s")).ToSQL()
	if err != nil {
		return sites, err
	}

	stmt1, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[424] failed to prepare statment: %v", err)
		return nil, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	rows, err := stmt1.Queryx(v...)
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
		perm := dbResource.GetObjectPermissionByReferenceId("site", site.ReferenceId, transaction)
		site.Permission = perm
		sites = append(sites, site)
	}

	return sites, nil

}

func (dbResource *DbResource) GetOauthDescriptionByTokenId(id int64, transaction *sqlx.Tx) (*oauth2.Config, error) {

	var clientId, clientSecret, redirectUri, authUrl, tokenUrl, scope string

	s, v, err := statementbuilder.Squirrel.
		Select(goqu.I("oc.client_id"), goqu.I("oc.client_secret"),
			goqu.I("oc.redirect_uri"), goqu.I("oc.auth_url"),
			goqu.I("oc.token_url"), goqu.I("oc.scope")).Prepared(true).
		From(goqu.T("oauth_token").As("ot")).Join(goqu.T("oauth_connect").As("oc"), goqu.On(goqu.Ex{
		"oc.id": goqu.I("ot.oauth_connect_id"),
	})).
		Where(goqu.Ex{"ot.id": id}).ToSQL()

	if err != nil {
		return nil, err
	}

	stmt1, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[478] failed to prepare statment: %v", err)
		return nil, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	err = stmt1.QueryRowx(v...).Scan(&clientId, &clientSecret, &redirectUri, &authUrl, &tokenUrl, &scope)

	if err != nil {
		return nil, err
	}

	encryptionSecret, err := dbResource.configStore.GetConfigValueFor("encryption.secret", "backend", transaction)
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

func (dbResource *DbResource) GetOauthDescriptionByTokenReferenceId(referenceId daptinid.DaptinReferenceId, transaction *sqlx.Tx) (*oauth2.Config, error) {

	var clientId, clientSecret, redirectUri, authUrl, tokenUrl, scope string

	s, v, err := statementbuilder.Squirrel.
		Select(goqu.I("oc.client_id"), goqu.I("oc.client_secret"), goqu.I("oc.redirect_uri"),
			goqu.I("oc.auth_url"), goqu.I("oc.token_url"), goqu.I("oc.scope")).Prepared(true).
		From(goqu.T("oauth_token").As("ot")).Join(goqu.T("oauth_connect").As("oc"), goqu.On(goqu.Ex{
		"oc.id": goqu.I("ot.oauth_connect_id"),
	})).
		Where(goqu.Ex{"ot.reference_id": referenceId[:]}).ToSQL()

	if err != nil {
		return nil, err
	}

	stmt1, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[538] failed to prepare statment: %v", err)
		return nil, err
	}
	defer stmt1.Close()
	err = stmt1.QueryRowx(v...).Scan(&clientId, &clientSecret, &redirectUri, &authUrl, &tokenUrl, &scope)

	if err != nil {
		return nil, err
	}

	encryptionSecret, err := dbResource.configStore.GetConfigValueFor("encryption.secret", "backend", transaction)
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

func (dbResource *DbResource) GetTokenByTokenReferenceId(referenceId daptinid.DaptinReferenceId, transaction *sqlx.Tx) (*oauth2.Token, *oauth2.Config, error) {
	oauthConf := &oauth2.Config{}

	var access_token, refresh_token, token_type string
	var expires_in int64
	var token oauth2.Token
	s, v, err := statementbuilder.Squirrel.
		Select("access_token", "refresh_token", "token_type", "expires_in").From("oauth_token").Prepared(true).
		Where(goqu.Ex{"reference_id": referenceId[:]}).ToSQL()

	if err != nil {
		return nil, oauthConf, err
	}

	stmt1, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[594] failed to prepare statment: %v", err)
		return nil, nil, err
	}
	defer stmt1.Close()
	err = stmt1.QueryRowx(v...).Scan(&access_token, &refresh_token, &token_type, &expires_in)

	if err != nil {
		return nil, oauthConf, err
	}

	secret, err := dbResource.configStore.GetConfigValueFor("encryption.secret", "backend", transaction)
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
	oauthConf, err = dbResource.GetOauthDescriptionByTokenReferenceId(referenceId, transaction)
	if err != nil {
		log.Printf("Failed to get oauth token configuration for token refresh: %v", err)
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
				err = dbResource.UpdateAccessTokenByTokenReferenceId(referenceId, refreshedToken.AccessToken, refreshedToken.Expiry.Unix(), transaction)
				CheckErr(err, "failed to update access token")
			}
		}
	}

	return &token, oauthConf, err

}

func (dbResource *DbResource) GetTokenByTokenId(id int64) (*oauth2.Token, error) {

	transaction, err := dbResource.Connection.Beginx()
	if err != nil {
		CheckErr(err, "Failed to begin transaction [656]")
		return nil, err
	}

	var access_token, refresh_token, token_type string
	var expires_in int64
	var token oauth2.Token
	s, v, err := statementbuilder.Squirrel.
		Select("access_token", "refresh_token", "token_type", "expires_in").From("oauth_token").Prepared(true).
		Where(goqu.Ex{"id": id}).ToSQL()

	if err != nil {
		return nil, err
	}

	stmt1, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[663] failed to prepare statment: %v", err)
		return nil, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	err = stmt1.QueryRowx(v...).Scan(&access_token, &refresh_token, &token_type, &expires_in)

	if err != nil {
		return nil, err
	}

	secret, err := dbResource.configStore.GetConfigValueFor("encryption.secret", "backend", transaction)
	transaction.Rollback()
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

func (dbResource *DbResource) GetTokenByTokenName(name string, transaction *sqlx.Tx) (*oauth2.Token, error) {
	var access_token, refresh_token, token_type string
	var expires_in int64
	var token oauth2.Token
	s, v, err := statementbuilder.Squirrel.
		Select("access_token", "refresh_token", "token_type", "expires_in").From("oauth_token").Prepared(true).
		Where(goqu.Ex{"token_type": name}).Order(goqu.C("created_at").Desc()).Limit(1).ToSQL()

	if err != nil {
		return nil, err
	}

	stmt1, err := dbResource.Connection.Preparex(s)
	if err != nil {
		log.Errorf("[711] failed to prepare statment: %v", err)
		return nil, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	err = stmt1.QueryRowx(v...).Scan(&access_token, &refresh_token, &token_type, &expires_in)

	if err != nil {
		return nil, err
	}

	secret, err := dbResource.configStore.GetConfigValueFor("encryption.secret", "backend", transaction)
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
