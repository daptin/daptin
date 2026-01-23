package resource

import (
	"context"
	"fmt"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/table_info"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/go-guerrilla/backends"
	"github.com/artpar/go-guerrilla/mail"
	"github.com/artpar/go-imap"
	"github.com/artpar/go-imap/backend/backendutil"
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type DbResource struct {
	model                api2go.Api2GoModel
	db                   sqlx.Ext
	connection           database.DatabaseConnection
	tableInfo            *table_info.TableInfo
	Cruds                map[string]*DbResource
	ms                   *MiddlewareSet
	ActionHandlerMap     map[string]actionresponse.ActionPerformerInterface
	ConfigStore          *ConfigStore
	EncryptionSecret     []byte
	contextCache         map[string]interface{}
	envMap               map[string]string
	defaultGroups        []int64
	AdministratorGroupId daptinid.DaptinReferenceId
	defaultRelations     map[string][]int64
	contextLock          sync.RWMutex
	OlricDb              *olric.EmbeddedClient
	AssetFolderCache     map[string]map[string]*assetcachepojo.AssetFolderCache
	subsiteFolderCache   map[daptinid.DaptinReferenceId]*assetcachepojo.AssetFolderCache
	MailSender           func(e *mail.Envelope, task backends.SelectTask) (backends.Result, error)
}

func (dbResource *DbResource) InitializeObject(value interface{}) {
	model := value.(*api2go.Api2GoModel)
	model.SetRelations(dbResource.model.GetRelations())
}

func (dbResource *DbResource) GetActionHandler(name string) actionresponse.ActionPerformerInterface {
	//TODO implement me
	return ActionHandlerMap[name]
}

func (dbResource *DbResource) Connection() database.DatabaseConnection {
	//TODO implement me
	return dbResource.connection
}

func (dbResource *DbResource) SubsiteFolderCache(id daptinid.DaptinReferenceId) (*assetcachepojo.AssetFolderCache, bool) {
	val, ok := dbResource.subsiteFolderCache[id]
	return val, ok
}

var CRUD_MAP = make(map[string]*DbResource)

func NewDbResource(model api2go.Api2GoModel, db database.DatabaseConnection,
	ms *MiddlewareSet, cruds map[string]*DbResource, configStore *ConfigStore,
	olricDb *olric.EmbeddedClient, tableInfo table_info.TableInfo) (*DbResource, error) {

	envLines := os.Environ()
	envMap := make(map[string]string)
	for _, env := range envLines {
		key := env[0:strings.Index(env, "=")]
		value := env[strings.Index(env, "=")+1:]
		envMap[key] = value
	}

	if OlricCache == nil {
		OlricCache, _ = olricDb.NewDMap("default-cache")
	}
	tx, err := db.Beginx()
	administratorGroupId, err := GetIdToReferenceIdWithTransaction("usergroup", 2, tx)
	if err != nil {
		return nil, err
	}

	err = tx.Rollback()

	if err != nil {
		return nil, err
	}

	defaultgroupIds, err := GroupNamesToIds(db, tableInfo.DefaultGroups)
	if err != nil {
		return nil, err
	}
	defaultRelationsIds, err := RelationNamesToIds(db, tableInfo)
	if err != nil {
		return nil, err
	}

	//log.Printf("Columns [%v]: %v\n", model.GetName(), model.GetColumnNames())
	tableCrud := &DbResource{
		model:                model,
		db:                   db,
		connection:           db,
		ms:                   ms,
		ConfigStore:          configStore,
		Cruds:                cruds,
		envMap:               envMap,
		tableInfo:            &tableInfo,
		OlricDb:              olricDb,
		defaultGroups:        defaultgroupIds,
		defaultRelations:     defaultRelationsIds,
		AdministratorGroupId: administratorGroupId,
		contextCache:         make(map[string]interface{}),
		contextLock:          sync.RWMutex{},
		AssetFolderCache:     make(map[string]map[string]*assetcachepojo.AssetFolderCache),
		subsiteFolderCache:   make(map[daptinid.DaptinReferenceId]*assetcachepojo.AssetFolderCache),
	}

	CRUD_MAP[model.GetTableName()] = tableCrud
	return tableCrud, nil
}

func RelationNamesToIds(db database.DatabaseConnection, tableInfo table_info.TableInfo) (map[string][]int64, error) {

	if len(tableInfo.DefaultRelations) == 0 {
		return map[string][]int64{}, nil
	}

	result := make(map[string][]int64)

	for relationName, values := range tableInfo.DefaultRelations {

		relation, found := tableInfo.GetRelationByName(relationName)
		if !found {
			log.Infof("Relation [%v] not found on table [%v] skipping default values", relationName, tableInfo.TableName)
			continue
		}

		typeName := relation.Subject

		if tableInfo.TableName == relation.Subject {
			typeName = relation.Object
		}

		query, args, err := statementbuilder.Squirrel.Select("id").Prepared(true).
			From(typeName).Where(goqu.Ex{"reference_id": goqu.Op{"in": values}}).ToSQL()
		CheckErr(err, fmt.Sprintf("[165] failed to convert %v names to ids", relationName))
		query = db.Rebind(query)

		stmt1, err := db.Preparex(query)
		if err != nil {
			log.Errorf("[170] failed to prepare statment: %v", err)
			return map[string][]int64{}, fmt.Errorf("failed to prepare statment to convert usergroup name to ids for default usergroup")
		}
		defer func(stmt1 *sqlx.Stmt) {
			err := stmt1.Close()
			if err != nil {
				log.Errorf("failed to close prepared statement: %v", err)
			}
		}(stmt1)

		rows, err := stmt1.Queryx(args...)
		CheckErr(err, "[176] failed to query user-group names to ids")
		if err != nil {
			return nil, err
		}

		retInt := make([]int64, 0)

		for rows.Next() {
			//iVal, _ := strconv.ParseInt(val, 10, 64)
			var id int64
			err := rows.Scan(&id)
			if err != nil {
				log.Errorf("[185] failed to scan value after query: %v", err)
				return nil, err
			}
			retInt = append(retInt, id)
		}
		err = rows.Close()
		stmt1.Close()
		CheckErr(err, "[206] Failed to close rows after default group name conversation")

		result[relationName] = retInt

	}

	return result, nil

}

func GroupNamesToIds(db database.DatabaseConnection, groupsName []string) ([]int64, error) {

	if len(groupsName) == 0 {
		return []int64{}, nil
	}

	query, args, err := statementbuilder.Squirrel.Select("id").Prepared(true).
		From("usergroup").Where(goqu.Ex{"name": goqu.Op{"in": groupsName}}).ToSQL()
	CheckErr(err, "[165] failed to convert usergroup names to ids")
	query = db.Rebind(query)

	stmt1, err := db.Preparex(query)
	if err != nil {
		log.Errorf("[170] failed to prepare statment: %v", err)
		return []int64{}, fmt.Errorf("failed to prepare statment to convert usergroup name to ids for default usergroup")
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	rows, err := stmt1.Queryx(args...)
	CheckErr(err, "[176] failed to query user-group names to ids")
	if err != nil {
		return nil, err
	}

	retInt := make([]int64, 0)

	for rows.Next() {
		//iVal, _ := strconv.ParseInt(val, 10, 64)
		var id int64
		err := rows.Scan(&id)
		if err != nil {
			log.Errorf("[185] failed to scan value after query: %v", err)
			return nil, err
		}
		retInt = append(retInt, id)
	}
	err = rows.Close()
	CheckErr(err, "[206] Failed to close rows after default group name conversation")

	return retInt, nil

}

func (dbResource *DbResource) PutContext(key string, val interface{}) {
	dbResource.contextLock.Lock()
	defer dbResource.contextLock.Unlock()

	dbResource.contextCache[key] = val
}

func (dbResource *DbResource) GetContext(key string) interface{} {

	dbResource.contextLock.RLock()
	defer dbResource.contextLock.RUnlock()

	return dbResource.contextCache[key]
}

type AdminMapType map[uuid.UUID]bool

func (a AdminMapType) MarshalBinary() (data []byte, err error) {
	for key, value := range a {
		// Append the UUID (16 bytes)
		data = append(data, key[:]...) // key[:] converts uuid.UUID to []byte

		// Append the boolean as a byte (1 byte)
		if value {
			data = append(data, 0x01)
		} else {
			data = append(data, 0x00)
		}
	}
	return data, nil
}

func (a AdminMapType) UnmarshalBinary(data []byte) error {
	const uuidSize = 16
	if len(data)%(uuidSize+1) != 0 { // Each entry should be exactly 17 bytes
		return errors.New("invalid data length")
	}

	if a == nil {
		a = make(AdminMapType)
	}

	for i := 0; i < len(data); i += uuidSize + 1 {
		key := uuid.UUID{}
		copy(key[:], data[i:i+uuidSize])  // Extract UUID from data
		value := data[i+uuidSize] == 0x01 // Extract boolean from data

		a[key] = value
	}
	return nil
}

func GetAdminReferenceIdWithTransaction(transaction *sqlx.Tx) map[uuid.UUID]bool {
	adminMap := make(AdminMapType)
	if OlricCache != nil {
		cacheValueGet, err := OlricCache.Get(context.Background(), "administrator_reference_id")

		if err == nil {
			cacheValueGet.Scan(&adminMap)
			return adminMap
		}
	}
	userRefId := GetUserMembersByGroupNameWithTransaction("administrators", transaction)
	for _, id := range userRefId {
		uuidVal, _ := uuid.FromBytes(id[:])
		adminMap[uuidVal] = true
	}

	if OlricCache != nil && userRefId != nil {
		err := OlricCache.Put(context.Background(), "administrator_reference_id", adminMap, olric.EX(60*time.Minute), olric.NX())
		CheckErr(err, "[257] Failed to cache admin reference ids")
	}
	return adminMap
}

func IsAdminWithTransaction(userReferenceId *auth.SessionUser, transaction *sqlx.Tx) bool {
	userUUid, _ := uuid.FromBytes(userReferenceId.UserReferenceId[:])
	key := "admin." + string(userReferenceId.UserReferenceId[:])
	adminGroupId := CRUD_MAP[USER_ACCOUNT_TABLE_NAME].AdministratorGroupId
	for _, ugid := range userReferenceId.Groups {
		if ugid.GroupReferenceId == adminGroupId {
			return true
		}
	}

	if OlricCache != nil {
		//fmt.Println("IsAdminWithTransaction [" + key + "]")
		value, err := OlricCache.Get(context.Background(), key)
		if err == nil && value != nil {
			if val, err := value.Bool(); val && err == nil {
				return true
			} else {
				return false
			}
		}
	}
	admins := GetAdminReferenceIdWithTransaction(transaction)
	_, ok := admins[userUUid]
	if ok {
		if OlricCache != nil {
			OlricCache.Put(context.Background(), key, true, olric.EX(5*time.Minute), olric.NX())
			//CheckErr(err, "[320] Failed to set admin id value in olric cache")
		}
		return true
	}
	if OlricCache != nil {
		OlricCache.Put(context.Background(), key, false, olric.EX(5*time.Minute), olric.NX())
	}
	//CheckErr(err, "[327] Failed to set admin id value in olric cache")
	return false

}

func (dbResource *DbResource) TableInfo() *table_info.TableInfo {
	return dbResource.tableInfo
}

func (dbResource *DbResource) GetAdminEmailId(transaction *sqlx.Tx) string {
	cacheVal := dbResource.GetContext("administrator_email_id")
	if cacheVal == nil {
		userRefId := dbResource.GetUserEmailIdByUsergroupId(2, transaction)
		dbResource.PutContext("administrator_email_id", userRefId)
		return userRefId
	} else {
		return cacheVal.(string)
	}
}

func (dbResource *DbResource) GetMailBoxMailsByOffset(mailBoxId int64, start uint32, stop uint32, transaction *sqlx.Tx) ([]map[string]interface{}, error) {

	q := statementbuilder.Squirrel.Select("*").Prepared(true).From("mail").Where(goqu.Ex{
		"mail_box_id": mailBoxId,
		"deleted":     false,
	}).Offset(uint(start - 1))

	if stop > 0 {
		q = q.Limit(uint(stop - start + 1))
	}

	query, args, err := q.ToSQL()

	if err != nil {
		return nil, err
	}

	stmt1, err := transaction.Preparex(query)
	if err != nil {
		log.Errorf("[275] failed to prepare statment: %v", err)
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	row, err := stmt1.Queryx(args...)

	if err != nil {
		return nil, err
	}
	responseArray, err := RowsToMap(row, dbResource.model.GetName())
	err = stmt1.Close()
	err = row.Close()

	m, _, err := dbResource.ResultToArrayOfMapWithTransaction(responseArray, dbResource.Cruds["mail"].model.GetColumnMap(), nil, transaction)

	return m, err

}

func (dbResource *DbResource) GetMailBoxMailsByUidSequence(mailBoxId int64, start uint32, stop uint32, transaction *sqlx.Tx) ([]map[string]interface{}, error) {

	q := statementbuilder.Squirrel.Select("*").Prepared(true).From("mail").Where(goqu.Ex{
		"mail_box_id": mailBoxId,
		"deleted":     false,
	}).Where(goqu.Ex{
		"id": goqu.Op{"gte": start},
	})

	if stop > 0 {
		q = q.Where(goqu.Ex{
			"id": goqu.Op{"lte": stop},
		})
	}

	q = q.Order(goqu.C("id").Asc())

	query, args, err := q.ToSQL()

	if err != nil {
		return nil, err
	}

	stmt1, err := transaction.Preparex(query)
	if err != nil {
		log.Errorf("[322] failed to prepare statment: %v", err)
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	row, err := stmt1.Queryx(args...)

	if err != nil {
		return nil, err
	}
	responseArray, err := RowsToMap(row, dbResource.model.GetName())
	err = stmt1.Close()
	err = row.Close()

	m, _, err := dbResource.ResultToArrayOfMapWithTransaction(responseArray, dbResource.Cruds["mail"].model.GetColumnMap(), nil, transaction)

	return m, err

}

func (dbResource *DbResource) GetMailBoxStatus(mailAccountId int64, mailBoxId int64, transaction *sqlx.Tx) (*imap.MailboxStatus, error) {

	var unseenCount uint32
	var recentCount uint32
	var uidValidity uint32
	var uidNext uint32
	var messgeCount uint32

	q4, v4, e4 := statementbuilder.Squirrel.Select(goqu.L("count(*)")).Prepared(true).From("mail").Where(goqu.Ex{
		"mail_box_id": mailBoxId,
	}).ToSQL()

	if e4 != nil {
		return nil, e4
	}

	stmt1, err := transaction.Preparex(q4)
	if err != nil {
		log.Errorf("[362] failed to prepare statment: %v", err)
	}
	defer stmt1.Close()

	r4 := stmt1.QueryRowx(v4...)
	r4.Scan(&messgeCount)

	q1, v1, e1 := statementbuilder.Squirrel.Select(goqu.L("count(*)")).Prepared(true).From("mail").Where(goqu.Ex{
		"mail_box_id": mailBoxId,
		"seen":        false,
	}).ToSQL()

	if e1 != nil {
		return nil, e1
	}

	stmt1, err = transaction.Preparex(q1)
	if err != nil {
		log.Errorf("[384] failed to prepare statment: %v", err)
	}
	defer stmt1.Close()

	r := stmt1.QueryRowx(v1...)
	r.Scan(&unseenCount)

	q2, v2, e2 := statementbuilder.Squirrel.Select(goqu.L("count(*)")).Prepared(true).From("mail").Where(goqu.Ex{
		"mail_box_id": mailBoxId,
		"recent":      true,
	}).ToSQL()

	if e2 != nil {
		return nil, e2
	}

	stmt1, err = transaction.Preparex(q2)
	if err != nil {
		log.Errorf("[405] failed to prepare statment: %v", err)
	}
	defer stmt1.Close()

	r2 := stmt1.QueryRowx(v2...)
	r2.Scan(&recentCount)

	q3, v3, e3 := statementbuilder.Squirrel.Select("uidvalidity").Prepared(true).From("mail_box").Where(goqu.Ex{
		"id": mailBoxId,
	}).ToSQL()

	if e3 != nil {
		return nil, e3
	}

	stmt1, err = transaction.Preparex(q3)
	if err != nil {
		log.Errorf("[425] failed to prepare statment: %v", err)
	}
	defer stmt1.Close()

	r3 := stmt1.QueryRowx(v3...)
	r3.Scan(&uidValidity)

	uidNext, _ = dbResource.GetMailboxNextUid(mailBoxId, transaction)

	st := imap.NewMailboxStatus("", []imap.StatusItem{imap.StatusUnseen, imap.StatusMessages, imap.StatusRecent, imap.StatusUidNext, imap.StatusUidValidity})

	err = st.Parse([]interface{}{
		string(imap.StatusMessages), messgeCount,
		string(imap.StatusUnseen), unseenCount,
		string(imap.StatusRecent), recentCount,
		string(imap.StatusUidValidity), uidValidity,
		string(imap.StatusUidNext), uidNext,
	})

	return st, err
}

func (dbResource *DbResource) GetFirstUnseenMailSequence(mailBoxId int64, transaction *sqlx.Tx) uint32 {

	query, args, err := statementbuilder.Squirrel.Select(goqu.L("min(id)")).Prepared(true).From("mail").Where(
		goqu.Ex{
			"mail_box_id": mailBoxId,
			"seen":        false,
		}).ToSQL()

	if err != nil {
		return 0
	}

	var id uint32
	stmt1, err := transaction.Preparex(query)
	if err != nil {
		log.Errorf("[465] failed to prepare statment: %v", err)
	}
	defer stmt1.Close()
	row := stmt1.QueryRowx(args...)
	if row.Err() != nil {
		return 0
	}
	row.Scan(&id)
	return id

}
func (dbResource *DbResource) UpdateMailFlags(mailBoxId int64, mailId int64, newFlags []string) error {

	//log.Printf("Update mail flags for [%v][%v]: %v", mailBoxId, mailId, newFlags)
	seen := false
	recent := false
	deleted := false

	if HasAnyFlag(newFlags, []string{imap.RecentFlag}) {
		recent = true
	} else {
		seen = true
	}

	if HasAnyFlag(newFlags, []string{"\\seen"}) {
		seen = true
		newFlags = backendutil.UpdateFlags(newFlags, imap.RemoveFlags, []string{imap.RecentFlag})
		log.Printf("New flags: [%v]", newFlags)
	}

	if HasAnyFlag(newFlags, []string{"\\expunge", "\\deleted"}) {
		newFlags = backendutil.UpdateFlags(newFlags, imap.RemoveFlags, []string{imap.RecentFlag})
		newFlags = backendutil.UpdateFlags(newFlags, imap.AddFlags, []string{"\\Seen"})
		log.Printf("New flags: [%v]", newFlags)
		deleted = true
		seen = true
	}

	query, args, err := statementbuilder.Squirrel.
		Update("mail").Prepared(true).
		Set(goqu.Record{
			"flags":   strings.Join(newFlags, ","),
			"seen":    seen,
			"recent":  recent,
			"deleted": deleted,
		}).
		Where(goqu.Ex{
			"mail_box_id": mailBoxId,
			"id":          mailId,
		}).ToSQL()
	if err != nil {
		return err
	}

	_, err = dbResource.db.Exec(query, args...)
	return err

}
func (dbResource *DbResource) ExpungeMailBox(mailBoxId int64) (int64, error) {

	selectQuery, args, err := statementbuilder.Squirrel.Select("id").Prepared(true).From("mail").Where(
		goqu.Ex{
			"mail_box_id": mailBoxId,
			"deleted":     true,
		},
	).ToSQL()

	if err != nil {
		return 0, err
	}

	stmt1, err := dbResource.Connection().Preparex(selectQuery)
	if err != nil {
		log.Errorf("[544] failed to prepare statment: %v", err)
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	rows, err := stmt1.Queryx(args...)
	if err != nil {
		return 0, err
	}

	ids := make([]interface{}, 0)

	for rows.Next() {
		var id int64
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	stmt1.Close()

	if len(ids) < 1 {
		return 0, nil
	}

	query, args, err := statementbuilder.Squirrel.Delete("mail_mail_id_has_usergroup_usergroup_id").Prepared(true).Where(goqu.Ex{
		"mail_id": ids,
	}).ToSQL()

	if err != nil {
		log.Printf("Query: %v", query)
		return 0, err
	}

	_, err = dbResource.db.Exec(query, args...)
	if err != nil {
		return 0, err
	}

	query, args, err = statementbuilder.Squirrel.Delete("mail").Prepared(true).Where(goqu.Ex{
		"id": ids,
	}).ToSQL()
	if err != nil {
		return 0, err
	}

	result, err := dbResource.db.Exec(query, args...)
	if err != nil {
		log.Printf("Query: %v", query)
		return 0, err
	}

	return result.RowsAffected()

}

func (dbResource *DbResource) GetMailboxNextUid(mailBoxId int64, transaction *sqlx.Tx) (uint32, error) {

	var uidNext int64
	q5, v5, e5 := statementbuilder.Squirrel.Select(goqu.L("COALESCE(MAX(id), 0)").As("max_id")).From("mail").Prepared(true).Where(goqu.Ex{
		"mail_box_id": mailBoxId,
	}).ToSQL()

	if e5 != nil {
		return 1, e5
	}

	stmt1, err := transaction.Preparex(q5)
	if err != nil {
		log.Errorf("[615] failed to prepare statment: %v", err)
		return 0, err
	}
	defer stmt1.Close()
	r5 := stmt1.QueryRowx(v5...)
	err = r5.Scan(&uidNext)
	return uint32(int32(uidNext) + 1), err

}

func (dbResource *DbResource) SetSubsitesFolderCache(cache map[daptinid.DaptinReferenceId]*assetcachepojo.AssetFolderCache) {
	dbResource.subsiteFolderCache = cache
}

func (dbResource *DbResource) StoreToken(token *oauth2.Token,
	token_type string, oauth_connect_reference_id daptinid.DaptinReferenceId,
	user_reference_id daptinid.DaptinReferenceId, transaction *sqlx.Tx) error {
	storeToken := make(map[string]interface{})

	storeToken["access_token"] = token.AccessToken
	storeToken["refresh_token"] = token.RefreshToken
	expiry := token.Expiry.Unix()
	if expiry < 0 {
		expiry = time.Now().Add(24 * 300 * time.Hour).Unix()
	}
	storeToken["expires_in"] = expiry
	storeToken["token_type"] = token_type
	storeToken["oauth_connect_id"] = oauth_connect_reference_id

	userId, err := dbResource.GetReferenceIdToId(USER_ACCOUNT_TABLE_NAME, user_reference_id, transaction)

	if err != nil {
		return err
	}

	sessionUser := &auth.SessionUser{
		UserId:          userId,
		UserReferenceId: user_reference_id,
		Groups:          nil,
	}

	ur, _ := url.Parse("/oauth_token")

	pr := &http.Request{
		Method: "POST",
		URL:    ur,
	}
	pr = pr.WithContext(context.WithValue(context.Background(), "user", sessionUser))

	req := api2go.Request{
		PlainRequest: pr,
	}

	model := api2go.NewApi2GoModelWithData("oauth_token", nil, int64(auth.DEFAULT_PERMISSION), nil, storeToken)

	_, err = dbResource.Cruds["oauth_token"].CreateWithoutFilter(model, req, transaction)
	return err
}

func (dbResource *DbResource) UpdateAssetColumnWithFile(columnName,
	fileName string, resourceUuid daptinid.DaptinReferenceId, fileSize int64, fileType string, transaction *sqlx.Tx) error {

	obj, _, err := dbResource.GetSingleRowByReferenceIdWithTransaction(dbResource.tableInfo.TableName, resourceUuid, nil, transaction)
	if err != nil {
		return err
	}

	colData := obj[columnName]

	var files []map[string]interface{}
	if colData != nil {
		files = colData.([]map[string]interface{})
	} else {
		files = make([]map[string]interface{}, 0)
	}

	// Check if file already exists and update or append
	found := false
	for i, file := range files {
		if file["name"] == fileName {
			// Update existing file
			files[i] = map[string]interface{}{
				"name":        fileName,
				"size":        fileSize,
				"type":        fileType,
				"status":      "completed",
				"uploaded_at": time.Now(),
			}
			found = true
			break
		}
	}

	if !found {
		// Add new file
		files = append(files, map[string]interface{}{
			"name":        fileName,
			"size":        fileSize,
			"type":        fileType,
			"status":      "completed",
			"uploaded_at": time.Now(),
		})
	}

	// Update column
	jsonData, _ := json.Marshal(files)

	newData := goqu.Record{
		"updated_at": time.Now(),
	}
	newData[columnName] = jsonData
	query, args, err := statementbuilder.Squirrel.Update(dbResource.tableInfo.TableName).Where(goqu.Ex{"reference_id": resourceUuid[:]}).Set(newData).Prepared(true).ToSQL()
	if err != nil {
		return err
	}

	_, err = transaction.Exec(query, args...)

	return err
}

func (dbResource *DbResource) UpdateAssetColumnStatus(resourceUuid daptinid.DaptinReferenceId, columnName,
	uploadId, status string, metadata map[string]interface{}, transaction *sqlx.Tx) error {

	// Get current resource - use cruds to access the method
	referenceId := resourceUuid
	resourceData, err := dbResource.GetReferenceIdToObjectWithTransaction(dbResource.tableInfo.TableName, referenceId, transaction)
	if err != nil {
		return err
	}

	colData := resourceData[columnName]
	var files []map[string]interface{}
	if colData != nil {
		// Handle both string (JSON) and direct array types
		switch v := colData.(type) {
		case string:
			json.Unmarshal([]byte(v), &files)
		case []map[string]interface{}:
			files = v
		default:
			files = make([]map[string]interface{}, 0)
		}
	}

	// Find and update the file with matching upload_id
	for i, file := range files {
		if file["upload_id"] == uploadId {
			file["status"] = status
			delete(file, "upload_id")

			// Add metadata if provided
			if metadata != nil {
				if size, ok := metadata["size"]; ok {
					file["size"] = size
				}
				if fileType, ok := metadata["type"]; ok {
					file["type"] = fileType
				}
			}

			file["uploaded_at"] = time.Now()
			files[i] = file
			break
		}
	}

	// Update column
	jsonData, _ := json.Marshal(files)

	newData := goqu.Record{
		"updated_at": time.Now(),
	}
	newData[columnName] = jsonData
	query, args, err := statementbuilder.Squirrel.Update(dbResource.tableInfo.TableName).
		Where(goqu.Ex{"reference_id": resourceUuid[:]}).Set(newData).Prepared(true).ToSQL()
	if err != nil {
		return err
	}

	_, err = transaction.Exec(query, args...)

	return err

}

func (dbResource *DbResource) UpdateAssetColumnWithPendingUpload(resourceUuid daptinid.DaptinReferenceId,
	columnName, fileName, uploadId string, fileSize int64, fileType string, transaction *sqlx.Tx) error {

	obj, _, err := dbResource.GetSingleRowByReferenceIdWithTransaction(dbResource.tableInfo.TableName, resourceUuid, nil, transaction)
	if err != nil {
		return err
	}

	colData := obj[columnName]

	var files []map[string]interface{}
	if colData != nil {
		files = colData.([]map[string]interface{})
	} else {
		files = make([]map[string]interface{}, 0)
	}

	// Add pending upload entry
	files = append(files, map[string]interface{}{
		"name":       fileName,
		"size":       fileSize,
		"type":       fileType,
		"upload_id":  uploadId,
		"status":     "pending",
		"created_at": time.Now(),
	})

	// Update column
	jsonData, _ := json.MarshalToString(files)

	newData := goqu.Record{
		"updated_at": time.Now(),
	}
	newData[columnName] = jsonData
	query, args, err := statementbuilder.Squirrel.Update(dbResource.tableInfo.TableName).Prepared(true).
		Where(goqu.Ex{"reference_id": resourceUuid[:]}).Set(newData).ToSQL()
	log.Debugf("[950] Query [%s] => %v", query, args)
	if err != nil {
		return err
	}

	_, err = transaction.Exec(query, args...)
	if err != nil {
		log.Errorf("Failed to execute query: %v", err)
		return err
	}

	return err

}
