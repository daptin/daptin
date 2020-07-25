package resource

import (
	"encoding/base64"
	"github.com/Masterminds/squirrel"
	"github.com/artpar/api2go"
	"github.com/artpar/go-guerrilla/backends"
	"github.com/artpar/go-guerrilla/mail"
	"github.com/artpar/go-imap"
	"github.com/artpar/go-imap/backend/backendutil"
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
)

type DbResource struct {
	model              *api2go.Api2GoModel
	db                 sqlx.Ext
	connection         database.DatabaseConnection
	tableInfo          *TableInfo
	Cruds              map[string]*DbResource
	ms                 *MiddlewareSet
	ActionHandlerMap   map[string]ActionPerformerInterface
	configStore        *ConfigStore
	contextCache       map[string]interface{}
	defaultGroups      []int64
	contextLock        sync.RWMutex
	AssetFolderCache   map[string]map[string]*AssetFolderCache
	SubsiteFolderCache map[string]*AssetFolderCache
	MailSender         func(e *mail.Envelope, task backends.SelectTask) (backends.Result, error)
}

type AssetFolderCache struct {
	LocalSyncPath string
	Keyname       string
	CloudStore    CloudStore
}

func (afc *AssetFolderCache) GetFileByName(fileName string) (*os.File, error) {

	return os.Open(afc.LocalSyncPath + string(os.PathSeparator) + fileName)

}

func (afc *AssetFolderCache) UploadFiles(files []interface{}) error {

	for i := range files {
		file := files[i].(map[string]interface{})
		contents, ok := file["file"]
		if !ok {
			contents = file["contents"]
		}
		if contents != nil {

			contentString, ok := contents.(string)
			if ok {

				if contentString[0:4] == "data" {
					contentString = contentString[strings.Index(contentString, "base64,")+7:]
				}
				fileBytes, e := base64.StdEncoding.DecodeString(contentString)
				if e != nil {
					continue
				}
				localFilePath := afc.LocalSyncPath + "/" + file["name"].(string)
				err := ioutil.WriteFile(localFilePath, fileBytes, os.ModePerm)
				CheckErr(err, "Failed to write data to local file store")
				return errors.WithMessage(err, "Failed to write data to local file store ")
			}
		}
	}

	return nil

}

func NewDbResource(model *api2go.Api2GoModel, db database.DatabaseConnection,
	ms *MiddlewareSet, cruds map[string]*DbResource, configStore *ConfigStore,
	tableInfo TableInfo) *DbResource {
	//log.Infof("Columns [%v]: %v\n", model.GetName(), model.GetColumnNames())
	return &DbResource{
		model:              model,
		db:                 db,
		connection:         db,
		ms:                 ms,
		configStore:        configStore,
		Cruds:              cruds,
		tableInfo:          &tableInfo,
		defaultGroups:      GroupNamesToIds(db, tableInfo.DefaultGroups),
		contextCache:       make(map[string]interface{}),
		contextLock:        sync.RWMutex{},
		AssetFolderCache:   make(map[string]map[string]*AssetFolderCache),
		SubsiteFolderCache: make(map[string]*AssetFolderCache),
	}
}
func GroupNamesToIds(db database.DatabaseConnection, groupsName []string) []int64 {

	if len(groupsName) == 0 {
		return []int64{}
	}

	var retArray []int64

	query, args, err := sqlx.In("select id from usergroup where name in (?)", groupsName)
	CheckErr(err, "Failed to convert usergroup names to ids")
	query = db.Rebind(query)

	err = db.Select(&retArray, query, args...)
	CheckErr(err, "Failed to query user-group names to ids")

	//retInt := make([]int64, 0)

	//for _, val := range retArray {
	//	iVal, _ := strconv.ParseInt(val, 10, 64)
	//	retInt = append(retInt, iVal)
	//}

	return retArray

}

func (dr *DbResource) PutContext(key string, val interface{}) {
	dr.contextLock.Lock()
	defer dr.contextLock.Unlock()

	dr.contextCache[key] = val
}

func (dr *DbResource) GetContext(key string) interface{} {

	dr.contextLock.RLock()
	defer dr.contextLock.RUnlock()

	return dr.contextCache[key]
}

func (dr *DbResource) GetAdminReferenceId() string {
	cacheVal := dr.GetContext("administrator_reference_id")
	if cacheVal == nil || cacheVal == "" {

		userRefId := dr.GetUserIdByUsergroupId(2)
		dr.PutContext("administrator_reference_id", userRefId)
		return userRefId
	} else {
		return cacheVal.(string)
	}
}

func (dr *DbResource) TableInfo() *TableInfo {
	return dr.tableInfo
}

func (dr *DbResource) GetAdminEmailId() string {
	cacheVal := dr.GetContext("administrator_email_id")
	if cacheVal == nil {
		userRefId := dr.GetUserEmailIdByUsergroupId(2)
		dr.PutContext("administrator_email_id", userRefId)
		return userRefId
	} else {
		return cacheVal.(string)
	}
}

func (dr *DbResource) GetMailBoxMailsByOffset(mailBoxId int64, start uint32, stop uint32) ([]map[string]interface{}, error) {

	q := statementbuilder.Squirrel.Select("*").From("mail").Where(squirrel.Eq{
		"mail_box_id": mailBoxId,
		"deleted":     false,
	}).Offset(uint64(start - 1))

	if stop > 0 {
		q = q.Limit(uint64(stop - start + 1))
	}

	query, args, err := q.ToSql()

	if err != nil {
		return nil, err
	}

	row, err := dr.db.Queryx(query, args...)

	if err != nil {
		return nil, err
	}
	defer row.Close()

	m, _, err := dr.ResultToArrayOfMap(row, dr.Cruds["mail"].model.GetColumnMap(), nil)

	return m, err

}

func (dr *DbResource) GetMailBoxMailsByUidSequence(mailBoxId int64, start uint32, stop uint32) ([]map[string]interface{}, error) {

	q := statementbuilder.Squirrel.Select("*").From("mail").Where(squirrel.Eq{
		"mail_box_id": mailBoxId,
		"deleted":     false,
	}).Where(squirrel.GtOrEq{
		"id": start,
	})

	if stop > 0 {
		q = q.Where(squirrel.LtOrEq{
			"id": stop,
		})
	}

	q = q.OrderBy("id asc")

	query, args, err := q.ToSql()

	if err != nil {
		return nil, err
	}

	row, err := dr.db.Queryx(query, args...)

	if err != nil {
		return nil, err
	}
	defer row.Close()

	m, _, err := dr.ResultToArrayOfMap(row, dr.Cruds["mail"].model.GetColumnMap(), nil)

	return m, err

}

func (dr *DbResource) GetMailBoxStatus(mailAccountId int64, mailBoxId int64) (*imap.MailboxStatus, error) {

	var unseenCount uint32
	var recentCount uint32
	var uidValidity uint32
	var uidNext uint32
	var messgeCount uint32

	q4, v4, e4 := statementbuilder.Squirrel.Select("count(*)").From("mail").Where(squirrel.Eq{
		"mail_box_id": mailBoxId,
	}).ToSql()

	if e4 != nil {
		return nil, e4
	}

	r4 := dr.db.QueryRowx(q4, v4...)
	r4.Scan(&messgeCount)

	q1, v1, e1 := statementbuilder.Squirrel.Select("count(*)").From("mail").Where(squirrel.Eq{
		"mail_box_id": mailBoxId,
		"seen":        false,
	}).ToSql()

	if e1 != nil {
		return nil, e1
	}

	r := dr.db.QueryRowx(q1, v1...)
	r.Scan(&unseenCount)

	q2, v2, e2 := statementbuilder.Squirrel.Select("count(*)").From("mail").Where(squirrel.Eq{
		"mail_box_id": mailBoxId,
		"recent":      true,
	}).ToSql()

	if e2 != nil {
		return nil, e2
	}

	r2 := dr.db.QueryRowx(q2, v2...)
	r2.Scan(&recentCount)

	q3, v3, e3 := statementbuilder.Squirrel.Select("uidvalidity").From("mail_box").Where(squirrel.Eq{
		"id": mailBoxId,
	}).ToSql()

	if e3 != nil {
		return nil, e3
	}

	r3 := dr.db.QueryRowx(q3, v3...)
	r3.Scan(&uidValidity)

	uidNext, _ = dr.GetMailboxNextUid(mailBoxId)

	st := imap.NewMailboxStatus("", []imap.StatusItem{imap.StatusUnseen, imap.StatusMessages, imap.StatusRecent, imap.StatusUidNext, imap.StatusUidValidity})

	err := st.Parse([]interface{}{
		string(imap.StatusMessages), messgeCount,
		string(imap.StatusUnseen), unseenCount,
		string(imap.StatusRecent), recentCount,
		string(imap.StatusUidValidity), uidValidity,
		string(imap.StatusUidNext), uidNext,
	})

	return st, err
}

func (dr *DbResource) GetFirstUnseenMailSequence(mailBoxId int64) uint32 {

	query, args, err := statementbuilder.Squirrel.Select("min(id)").From("mail").Where(
		squirrel.Eq{
			"mail_box_id": mailBoxId,
			"seen":        false,
		}).ToSql()

	if err != nil {
		return 0
	}

	var id uint32
	row := dr.db.QueryRowx(query, args...)
	if row.Err() != nil {
		return 0
	}
	row.Scan(&id)
	return id

}
func (dr *DbResource) UpdateMailFlags(mailBoxId int64, mailId int64, newFlags []string) error {

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
		Update("mail").
		Set("flags", strings.Join(newFlags, ",")).
		Set("seen", seen).
		Set("recent", recent).
		Set("deleted", deleted).
		Where(squirrel.Eq{
			"mail_box_id": mailBoxId,
			"id":          mailId,
		}).ToSql()
	if err != nil {
		return err
	}

	_, err = dr.db.Exec(query, args...)
	return err

}
func (dr *DbResource) ExpungeMailBox(mailBoxId int64) (int64, error) {

	selectQuery, args, err := statementbuilder.Squirrel.Select("id").From("mail").Where(
		squirrel.Eq{
			"mail_box_id": mailBoxId,
			"deleted":     true,
		},
	).ToSql()

	if err != nil {
		return 0, err
	}

	rows, err := dr.db.Queryx(selectQuery, args...)
	if err != nil {
		return 0, err
	}

	ids := make([]interface{}, 0)

	for rows.Next() {
		var id int64
		rows.Scan(&id)
		ids = append(ids, id)
	}

	if len(ids) < 1 {
		return 0, nil
	}

	query, args, err := statementbuilder.Squirrel.Delete("mail_mail_id_has_usergroup_usergroup_id").Where(squirrel.Eq{
		"mail_id": ids,
	}).ToSql()

	if err != nil {
		log.Printf("Query: %v", query)
		return 0, err
	}

	_, err = dr.db.Exec(query, args...)
	if err != nil {
		return 0, err
	}

	query, args, err = statementbuilder.Squirrel.Delete("mail").Where(squirrel.Eq{
		"id": ids,
	}).ToSql()
	if err != nil {
		return 0, err
	}

	result, err := dr.db.Exec(query, args...)
	if err != nil {
		log.Printf("Query: %v", query)
		return 0, err
	}

	return result.RowsAffected()

}

func (dr *DbResource) GetMailboxNextUid(mailBoxId int64) (uint32, error) {

	var uidNext int64
	q5, v5, e5 := statementbuilder.Squirrel.Select("max(id)").From("mail").Where(squirrel.Eq{
		"mail_box_id": mailBoxId,
	}).ToSql()

	if e5 != nil {
		return 1, e5
	}

	r5 := dr.db.QueryRowx(q5, v5...)
	err := r5.Scan(&uidNext)
	return uint32(int32(uidNext) + 1), err

}
