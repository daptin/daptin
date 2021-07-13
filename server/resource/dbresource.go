package resource

import (
	"encoding/base64"
	"github.com/artpar/api2go"
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
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"
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
	OlricDb            *olric.Olric
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
func (afc *AssetFolderCache) DeleteFileByName(fileName string) error {

	return os.Remove(afc.LocalSyncPath + string(os.PathSeparator) + fileName)

}

func (afc *AssetFolderCache) GetPathContents(path string) ([]map[string]interface{}, error) {

	fileInfo, err := ioutil.ReadDir(afc.LocalSyncPath + string(os.PathSeparator) + path)
	if err != nil {
		return nil, err
	}

	//files, err := filepath.Glob(afc.LocalSyncPath + string(os.PathSeparator) + path + "*")
	//fmt.Println(files)
	var files []map[string]interface{}
	for _, file := range fileInfo {
		//files[i] = strings.Replace(file, afc.LocalSyncPath, "", 1)
		files = append(files, map[string]interface{}{
			"name":     file.Name(),
			"is_dir":   file.IsDir(),
			"mod_time": file.ModTime(),
			"size":     file.Size(),
		})
	}

	return files, err

}

func createDirIfNotExist(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			panic(err)
		}
	}
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
			if ok && len(contentString) > 4 {

				if strings.Index(contentString, ",") > -1 {
					contentString = strings.SplitN(contentString, ",", 2)[1]
				}
				fileBytes, e := base64.StdEncoding.DecodeString(contentString)
				if e != nil {
					continue
				}
				if file["name"] == nil {
					return errors.WithMessage(errors.New("file name cannot be null"), "File name is null")
				}
				filePath := string(os.PathSeparator)
				if file["path"] != nil {
					filePath = strings.Replace(file["path"].(string), "/", string(os.PathSeparator), -1) + string(os.PathSeparator)
				}
				localPath := afc.LocalSyncPath + string(os.PathSeparator) + filePath
				createDirIfNotExist(localPath)
				localFilePath := localPath + file["name"].(string)
				err := ioutil.WriteFile(localFilePath, fileBytes, os.ModePerm)
				CheckErr(err, "Failed to write data to local file store asset cache folder")
				if err != nil {
					return errors.WithMessage(err, "Failed to write data to local file store ")
				}
			}
		}
	}

	return nil

}

func NewDbResource(model *api2go.Api2GoModel, db database.DatabaseConnection,
	ms *MiddlewareSet, cruds map[string]*DbResource, configStore *ConfigStore,
	olricDb *olric.Olric, tableInfo TableInfo) *DbResource {
	if OlricCache == nil {
		OlricCache, _ = olricDb.NewDMap("default-cache")
	}

	//log.Printf("Columns [%v]: %v\n", model.GetName(), model.GetColumnNames())
	return &DbResource{
		model:              model,
		db:                 db,
		connection:         db,
		ms:                 ms,
		configStore:        configStore,
		Cruds:              cruds,
		tableInfo:          &tableInfo,
		OlricDb:            olricDb,
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

	query, args, err := statementbuilder.Squirrel.Select("id").From("usergroup").Where(goqu.Ex{"name": goqu.Op{"in": groupsName}}).ToSQL()
	CheckErr(err, "[165] failed to convert usergroup names to ids")
	query = db.Rebind(query)

	stmt1, err := db.Preparex(query)
	if err != nil {
		log.Errorf("[170] failed to prepare statment: %v", err)
		return []int64{}
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	rows, err := stmt1.Queryx(args...)
	CheckErr(err, "[176] failed to query user-group names to ids")

	retInt := make([]int64, 0)

	for rows.Next() {
		//iVal, _ := strconv.ParseInt(val, 10, 64)
		var id int64
		err := rows.Scan(&id)
		if err != nil {
			log.Errorf("[185] failed to scan value after query: %v", err)
			return nil
		}
		retInt = append(retInt, id)
	}
	rows.Close()

	return retInt

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

func (dr *DbResource) GetAdminReferenceId() []string {
	var err error
	var cacheValue interface{}
	if OlricCache != nil {
		cacheValue, err = OlricCache.Get("administrator_reference_id")
		if err != nil &&  cacheValue != nil && len(cacheValue.([]string)) > 0 {
			return cacheValue.([]string)
		}
	}
	userRefId := dr.GetUserMembersByGroupName("administrators")
	if OlricCache != nil && userRefId != nil {
		err = OlricCache.PutEx("administrator_reference_id", userRefId, 1*time.Minute)
		CheckErr(err, "Failed to cache admin reference ids")
	}
	return userRefId
}

func (dr *DbResource) IsAdmin(userReferenceId string) bool {
	admins := dr.GetAdminReferenceId()
	//if len(admins) < 1 {
	//	return true
	//}
	for _, id := range admins {
		if id == userReferenceId {
			return true
		}
	}
	return false

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

	q := statementbuilder.Squirrel.Select("*").From("mail").Where(goqu.Ex{
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

	stmt1, err := dr.connection.Preparex(query)
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

	m, _, err := dr.ResultToArrayOfMap(row, dr.Cruds["mail"].model.GetColumnMap(), nil)
	row.Close()

	return m, err

}

func (dr *DbResource) GetMailBoxMailsByUidSequence(mailBoxId int64, start uint32, stop uint32) ([]map[string]interface{}, error) {

	q := statementbuilder.Squirrel.Select("*").From("mail").Where(goqu.Ex{
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

	stmt1, err := dr.connection.Preparex(query)
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

	m, _, err := dr.ResultToArrayOfMap(row, dr.Cruds["mail"].model.GetColumnMap(), nil)
	row.Close()

	return m, err

}

func (dr *DbResource) GetMailBoxStatus(mailAccountId int64, mailBoxId int64) (*imap.MailboxStatus, error) {

	var unseenCount uint32
	var recentCount uint32
	var uidValidity uint32
	var uidNext uint32
	var messgeCount uint32

	q4, v4, e4 := statementbuilder.Squirrel.Select(goqu.L("count(*)")).From("mail").Where(goqu.Ex{
		"mail_box_id": mailBoxId,
	}).ToSQL()

	if e4 != nil {
		return nil, e4
	}

	stmt1, err := dr.connection.Preparex(q4)
	if err != nil {
		log.Errorf("[362] failed to prepare statment: %v", err)
	}

	r4 := stmt1.QueryRowx(v4...)
	r4.Scan(&messgeCount)
	err = stmt1.Close()
	if err != nil {
		log.Errorf("failed to close prepared statement: %v", err)
	}

	q1, v1, e1 := statementbuilder.Squirrel.Select(goqu.L("count(*)")).From("mail").Where(goqu.Ex{
		"mail_box_id": mailBoxId,
		"seen":        false,
	}).ToSQL()

	if e1 != nil {
		return nil, e1
	}

	stmt1, err = dr.connection.Preparex(q1)
	if err != nil {
		log.Errorf("[384] failed to prepare statment: %v", err)
	}

	r := stmt1.QueryRowx(v1...)
	r.Scan(&unseenCount)
	err = stmt1.Close()
	if err != nil {
		log.Errorf("failed to close prepared statement: %v", err)
	}

	q2, v2, e2 := statementbuilder.Squirrel.Select(goqu.L("count(*)")).From("mail").Where(goqu.Ex{
		"mail_box_id": mailBoxId,
		"recent":      true,
	}).ToSQL()

	if e2 != nil {
		return nil, e2
	}

	stmt1, err = dr.connection.Preparex(q2)
	if err != nil {
		log.Errorf("[405] failed to prepare statment: %v", err)
	}

	r2 := stmt1.QueryRowx(v2...)
	r2.Scan(&recentCount)
	err = stmt1.Close()
	if err != nil {
		log.Errorf("failed to close prepared statement: %v", err)
	}

	q3, v3, e3 := statementbuilder.Squirrel.Select("uidvalidity").From("mail_box").Where(goqu.Ex{
		"id": mailBoxId,
	}).ToSQL()

	if e3 != nil {
		return nil, e3
	}

	stmt1, err = dr.connection.Preparex(q3)
	if err != nil {
		log.Errorf("[425] failed to prepare statment: %v", err)
	}

	r3 := stmt1.QueryRowx(v3...)
	r3.Scan(&uidValidity)
	err = stmt1.Close()
	if err != nil {
		log.Errorf("failed to close prepared statement: %v", err)
	}

	uidNext, _ = dr.GetMailboxNextUid(mailBoxId)

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

func (dr *DbResource) GetFirstUnseenMailSequence(mailBoxId int64) uint32 {

	query, args, err := statementbuilder.Squirrel.Select(goqu.L("min(id)")).From("mail").Where(
		goqu.Ex{
			"mail_box_id": mailBoxId,
			"seen":        false,
		}).ToSQL()

	if err != nil {
		return 0
	}

	var id uint32
	stmt1, err := dr.connection.Preparex(query)
	if err != nil {
		log.Errorf("[465] failed to prepare statment: %v", err)
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	row := stmt1.QueryRowx(args...)
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

	_, err = dr.db.Exec(query, args...)
	return err

}
func (dr *DbResource) ExpungeMailBox(mailBoxId int64) (int64, error) {

	selectQuery, args, err := statementbuilder.Squirrel.Select("id").From("mail").Where(
		goqu.Ex{
			"mail_box_id": mailBoxId,
			"deleted":     true,
		},
	).ToSQL()

	if err != nil {
		return 0, err
	}

	stmt1, err := dr.connection.Preparex(selectQuery)
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

	if len(ids) < 1 {
		return 0, nil
	}

	query, args, err := statementbuilder.Squirrel.Delete("mail_mail_id_has_usergroup_usergroup_id").Where(goqu.Ex{
		"mail_id": ids,
	}).ToSQL()

	if err != nil {
		log.Printf("Query: %v", query)
		return 0, err
	}

	_, err = dr.db.Exec(query, args...)
	if err != nil {
		return 0, err
	}

	query, args, err = statementbuilder.Squirrel.Delete("mail").Where(goqu.Ex{
		"id": ids,
	}).ToSQL()
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
	q5, v5, e5 := statementbuilder.Squirrel.Select("max(id)").From("mail").Where(goqu.Ex{
		"mail_box_id": mailBoxId,
	}).ToSQL()

	if e5 != nil {
		return 1, e5
	}

	stmt1, err := dr.connection.Preparex(q5)
	if err != nil {
		log.Errorf("[615] failed to prepare statment: %v", err)
		return 0, err
	}
	defer func(stmt1 *sqlx.Stmt) {
		err := stmt1.Close()
		if err != nil {
			log.Errorf("failed to close prepared statement: %v", err)
		}
	}(stmt1)

	r5 := stmt1.QueryRowx(v5...)
	err = r5.Scan(&uidNext)
	return uint32(int32(uidNext) + 1), err

}
