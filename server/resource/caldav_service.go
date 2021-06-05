package resource

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"github.com/daptin/daptin/server/database"
	"github.com/jmoiron/sqlx"
	"github.com/samedi/caldav-go"
	"github.com/samedi/caldav-go/data"
	"github.com/samedi/caldav-go/errs"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var Schema = `
CREATE EXTENSION IF NOT EXISTS pgcrypto;

DROP TABLE IF EXISTS calendar;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS collection;
DROP TABLE IF EXISTS collection_role;


CREATE TABLE calendar (
	-- id         UUID DEFAULT uuid_generate_v4() PRIMARY KEY,
	id         SERIAL PRIMARY KEY,
	owner_id   INT, /* NOT NULL, */
	created    TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	modified   TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	collection INT,
	rpath      TEXT,
	content    TEXT
);

CREATE TABLE collection (
	-- id          UUID DEFAULT uuid_generate_v4() PRIMARY KEY,
	id          SERIAL PRIMARY KEY,
	owner_id    INT,
	name        VARCHAR(64),
	description TEXT,
);

DROP TYPE IF EXISTS perm;
CREATE TYPE perm AS ENUM ('admin', 'write', 'read', 'none');

CREATE TABLE collection_role (
	collection_id   INT,
	user_id         INT,
	permission perm DEFAULT 'none',
);

CREATE TABLE users (
	-- id         UUID DEFAULT uuid_generate_v4() PRIMARY KEY,
	id         SERIAL PRIMARY KEY,
	username   TEXT UNIQUE NOT NULL,
	email      TEXT UNIQUE NOT NULL,
	password   VARCHAR(64) NOT NULL, /* crypt('input', password) */
	firstname  TEXT,
	lastname   TEXT,
);

CREATE INDEX user_index ON users (username, password);
CREATE INDEX user_cal_index ON users (id, firstname, lastname);
CREATE INDEX cal_index ON calendar (rpath);
CREATE INDEX cal_user_index ON calendar (rpath, owner_id);`

type CalDavStorage struct {
	cruds database.DatabaseConnection
	UserID int64
	Mailer Mail
	User string
	Email string
}

type DummyResourceAdapter struct{
	resourcePath string
	resourceData string
}



func (cs *CalDavStorage) GetResourcesByList(rpaths []string) ([]data.Resource, error) {
	results := []data.Resource{}

	for _, rpath := range rpaths {
		resource, found, err := cs.GetShallowResource(rpath)

		if err != nil && err != errs.ResourceNotFoundError {
			return nil, err
		}

		if found {
			results = append(results, *resource)
		}
	}

	return results, nil
}

func (cs *CalDavStorage) GetShallowResource(rpath string) (*data.Resource, bool, error) {
	resources, err := cs.GetResources(rpath, false)

	if err != nil {
		return nil, false, err
	}

	if resources == nil || len(resources) == 0 {
		return nil, false, errs.ResourceNotFoundError
	}

	res := resources[0]
	return &res, true, nil
}

func (cs *CalDavStorage) GetResources(rpath string, withChildren bool) ([]data.Resource, error) {
	result := []data.Resource{}

	a, err := cs.haveAccess(rpath, "read")
	if err != nil {
		log.Error(err, "failed to get Access [" + rpath + "]")
		return nil, err
	}

	if ! a {
		log.Info("no access to collection [" + rpath + "]")
		return nil, nil
	}

	var rows *sql.Rows
	rows, err = cs.cruds.Query("SELECT rpath FROM calendar WHERE rpath = $1 AND owner_id = $2 ", rpath, cs.UserID)
	if err != nil {
		log.Error(err, "failed to fetch rpath")
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var rrpath string
		err := rows.Scan(&rrpath)
		if err != nil {
			log.Error(err, "failed to scan rows")
			return nil, err
		}
		res := data.NewResource(rrpath, &PGResourceAdapter{db: cs.cruds, resourcePath: rpath, UserID: cs.UserID})
		result = append(result, res)
	}
	if isCollection(rpath) {
		res := data.NewResource(rpath, &PGResourceAdapter{db: cs.cruds, resourcePath: rpath,  UserID: cs.UserID})
		result = append(result, res)
	}
	if withChildren && isCollection(rpath) {
		rows, err = cs.cruds.Query("SELECT rpath FROM calendar WHERE owner_id = $1", cs.UserID)
		if err != nil {
			log.Error(err, "failed to scan rows")
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var rrpath string
			err := rows.Scan(&rrpath)
			if err != nil {
				log.Error(err, "failed to scan rows")
				return nil, err
			}
			res := data.NewResource(rrpath, &PGResourceAdapter{db: cs.cruds, resourcePath: rrpath,  UserID: cs.UserID})
			result = append(result, res)
		}
	}

	return result, nil
}

func (cs *CalDavStorage) haveAccess(rpath string, perm string) (bool, error) {
	var rows *sql.Rows
	var err error
	rows, err = cs.cruds.Query("SELECT permission FROM collection_role JOIN users ON collection_role.user_id = users.id JOIN collection ON collection_role.collection_id = collection.id  WHERE collection.name = $1 AND users.id = $2", getCollection(rpath), cs.UserID)
	if err != nil {
		log.Error(err, "failed to fetch permissions for " + rpath)
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var rowPerm string
		err := rows.Scan(&perm)
		if err != nil {
			log.Error(err, "failed to scan rows")
			return false, err
		}
		if rowPerm == "admin" {
			return true, nil
		}
		if rowPerm == "write" && perm == "admin" {
			return false, nil
		} else {
			return true, nil
		}
		if rowPerm == "read" && perm == "read" {
			return true, nil
		} else {
			return false, nil
		}
	}

	return false, nil
}

func isCollection(rpath string) bool {
	if rpath[len(rpath) - 1 : ] == "/" {
		return true
	}
	if rpath[len(rpath) - 3 : ] != "ics" {
		return true
	}
	return false
}




var regex = regexp.MustCompile(`/[A-Za-z0-9-%@\.]*\.ics`)
func getCollection(rpath string) string {
	replace := regex.ReplaceAll([]byte(rpath), []byte("/"))
	return string(replace)
}



func (cs *CalDavStorage) GetResourcesByFilters(rpath string, filters *data.ResourceFilter) ([]data.Resource, error) {
	result := []data.Resource{}

	res, err := cs.GetResources("/", true)
	if err != nil {
		return nil, err
	}
	for _, r := range res {
		if filters == nil || filters.Match(&r) {
			result = append(result, r)
		}
	}

	return result, nil
}

func (cs *CalDavStorage) GetResource(rpath string) (*data.Resource, bool, error) {
	return cs.GetShallowResource(rpath)
}

func parseMail(m string) string {
	s := strings.Split(m, ":")
	if s[0] == "mailto" {
		if len(s) > 1 {
			return s[1]
		}
	}
	return m
}

func (cs *CalDavStorage) CreateResource(rpath, content string) (*data.Resource, error) {
	a, err := cs.haveAccess(rpath, "write")
	if err != nil {
		log.Error(err, "failed to get Access [" + rpath + "]")
		return nil, err
	}
	if ! a {
		log.Info("no access to collection [" + rpath + "]")
		return nil, nil
	}
	stmt, err := cs.cruds.Prepare("INSERT INTO calendar (rpath, content, owner_id) VALUES ($1, $2, $3)")
	if err != nil {
		log.Error(err, "failed to prepare insert statement")
		return nil, err
	}
	defer stmt.Close()
	if _, err := stmt.Exec(rpath, base64.StdEncoding.EncodeToString([]byte(content)), cs.UserID); err != nil {
		log.Error(err, "failed to insert ", rpath)
		return nil, err
	}
	res := data.NewResource(rpath, &PGResourceAdapter{db: cs.cruds, resourcePath: rpath, UserID: cs.UserID})
	attending := res.GetPropertyValue("VEVENT", "ATTENDEE")
	title     := res.GetPropertyValue("VEVENT", "SUMMARY")
	start     := res.StartTimeUTC()
	end       := res.EndTimeUTC()
	subject   := "Invitation: " + title + " @ " + start.Weekday().String() + " " + start.Month().String() + " " + strconv.Itoa(start.Day()) + ", " + strconv.Itoa(start.Year()) + " " + strconv.Itoa(start.Hour()) + " - " + strconv.Itoa(end.Hour()) + " (" + start.Location().String() + ") (" + cs.Email + ")"
	u := parseMail(attending)
	cs.Mailer.Send(u, u, content, subject)
	cs.Mailer.Send(cs.User, cs.Email, content, subject)
	log.Info("resource created ", rpath)
	return &res, nil
}

func (cs *CalDavStorage) UpdateResource(rpath, content string) (*data.Resource, error) {
	a, err := cs.haveAccess(rpath, "write")
	if err != nil {
		log.Error(err, "failed to get Access [" + rpath + "]")
		return nil, err
	}
	if ! a {
		log.Info("no access to collection [" + rpath + "]")
		return nil, nil
	}
	stmt, err := cs.cruds.Prepare("UPDATE calendar SET content = $2, modified = $3 WHERE rpath = $1")
	if err != nil {
		log.Error(err, "failed to prepare update statement ", rpath)
		return nil, err
	}
	defer stmt.Close()
	if _, err := stmt.Exec(rpath, base64.StdEncoding.EncodeToString([]byte(content)), time.Now()); err != nil {
		log.Error(err, "failed to update ", rpath)
		return nil, err
	}
	res := data.NewResource(rpath, &PGResourceAdapter{db: cs.cruds, resourcePath: rpath,  UserID: cs.UserID})
	attending := res.GetPropertyValue("VEVENT", "ATTENDEE")
	title     := res.GetPropertyValue("VEVENT", "SUMMARY")
	start     := res.StartTimeUTC()
	end       := res.EndTimeUTC()
	subject   := "Updated invitation: " + title + " @ " + start.Weekday().String() + " " + start.Month().String() + " " + strconv.Itoa(start.Day()) + ", " + strconv.Itoa(start.Year()) + " " + strconv.Itoa(start.Hour()) + " - " + strconv.Itoa(end.Hour()) + " (" + start.Location().String() + ") (" + cs.Email + ")"

	u := parseMail(attending)
	cs.Mailer.Send(u, u, content, subject)
	cs.Mailer.Send(cs.User, cs.Email, content, subject)
	log.Info("resource updated ", rpath)
	return &res, nil
}

func (cs *CalDavStorage) DeleteResource(rpath string) error {
	a, err := cs.haveAccess(rpath, "admin")
	if err != nil {
		log.Error(err, "failed to get Access [" + rpath + "]")
		return  err
	}
	if ! a {
		log.Info("no access to collection [" + rpath + "]")
		return nil
	}
	_, err = cs.cruds.Exec("DELETE FROM calendar WHERE rpath = $1 AND owner_id = $2", rpath, cs.UserID)
	if err != nil {
		log.Info("failed to delete resource ", rpath, " ", err.Error())
		return err
	}
	return nil
}

func (cs *CalDavStorage) isResourcePresent(rpath string) bool {
	rows, err := cs.cruds.Query("SELECT rpath FROM calendar WHERE rpath = $1 AND owner_id = $2", rpath, cs.UserID)
	if err != nil {
		return false
	}
	defer rows.Close()
	var rrpath string
	for rows.Next() {
		err = rows.Scan(&rrpath)
		if err != nil {
			return false
		}
		if rrpath == rpath {
			return true
		}
	}
	return false
}

type PGResourceAdapter struct {
	db           sqlx.Ext
	resourcePath string
	UserID       int64
}

func (pa *PGResourceAdapter) CalculateEtag() string {
	if pa.IsCollection() {
		return ""
	}

	return fmt.Sprintf(`"%x%x"`, pa.GetContentSize(), pa.GetModTime().UnixNano())
}

func (pa *PGResourceAdapter) haveAccess(perm string) (bool, error) {
	var rows *sql.Rows
	var err error
	rows, err = pa.db.Query("SELECT permission FROM collection_role JOIN users ON collection_role.user_id = users.id JOIN collection ON collection_role.collection_id = collection.id  WHERE collection.name = $1 AND users.id = $2", getCollection(pa.resourcePath), pa.UserID)
	if err != nil {
		log.Error(err, "failed to fetch permissions for " + pa.resourcePath)
		return false, err
	}

	defer rows.Close()

	for rows.Next() {
		var rowPerm string
		err := rows.Scan(&perm)
		if err != nil {
			log.Error(err, "failed to scan rows")
			return false, err
		}
		if rowPerm == "admin" {
			return true, nil
		}
		if rowPerm == "write" && perm == "admin" {
			return false, nil
		} else {
			return true, nil
		}
		if rowPerm == "read" && perm == "read" {
			return true, nil
		} else {
			return false, nil
		}
	}

	return false, nil
}

func (pa *PGResourceAdapter) GetContent() string {
	if pa.IsCollection() {
		return ""
	}
	a, err := pa.haveAccess("read")
	if err != nil {
		log.Error(err, "failed to get Access ", pa.resourcePath)
		return ""
	}
	if ! a {
		return ""
	}
	rows, err := pa.db.Query("SELECT content FROM calendar WHERE rpath = $1 AND owner_id = $2", pa.resourcePath, pa.UserID)
	if err != nil {
		log.Error(err, "failed to fetch content ", pa.resourcePath)
		return ""
	}
	defer rows.Close()
	var content string
	for rows.Next() {
		err = rows.Scan(&content)
		if err != nil {
			log.Error(err, "failed to scan ", pa.resourcePath)
			return ""
		}
	}
	ret, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		log.Error(err, "decode error ", pa.resourcePath)
		return ""
	}
	return string(ret)
}

func (pa *PGResourceAdapter) GetContentSize() int64 {
	return int64(len(pa.GetContent()))
}

func (pa *PGResourceAdapter) IsCollection() bool {
	return isCollection(pa.resourcePath)
}

func (pa *PGResourceAdapter) GetModTime() time.Time {
	a, err := pa.haveAccess("read")
	if err != nil {
		log.Error(err, "failed to get Access ", pa.resourcePath)
		return time.Unix(0, 0)
	}
	if ! a {
		log.Info("failed to get Access ", pa.resourcePath)
		return time.Unix(0, 0)
	}
	rows, err := pa.db.Query("SELECT modified FROM calendar WHERE rpath = $1 AND owner_id = $2", pa.resourcePath, pa.UserID)
	if err != nil {
		log.Error(err, "failed to fetch modTime ", pa.resourcePath)
		return time.Unix(0, 0)
	}
	defer rows.Close()
	var mod time.Time
	for rows.Next() {
		err = rows.Scan(&mod)
		if err != nil {
			log.Error(err, "failed to scan modTime ", pa.resourcePath)
			return time.Unix(0, 0)
		}
	}
	return mod
}


func NewCaldavServer(CaldavListenInterface string,  cruds database.DatabaseConnection) *http.Server {

	if err := setupDB(cruds); err != nil {
		log.Error(err)
		os.Exit(1)
	}

	var username, email string
	var id int64
	rows, err := cruds.Query("SELECT id, username, email FROM users")
	if err != nil {
		log.Error(err)
		return nil
	}

	for rows.Next() {
		err = rows.Scan(&id, &username, &email)
		if err != nil {
			log.Error(err)
			return nil
		}
	}

	stg := new(CalDavStorage)
	stg.cruds    = cruds
	stg.User   = username
	stg.UserID = id
	stg.Email  = email
	caldav.SetupStorage(stg)

	servermux := http.NewServeMux()
	servermux.HandleFunc("/caldav", caldav.RequestHandler)


	s := &http.Server{
		Addr: CaldavListenInterface,
		Handler: servermux,
	}

	return s
}

func setupDB(db sqlx.Ext) error {
	_, err := db.Query("SELECT * FROM calendar LIMIT 1")
	if err == nil {
		return nil
	}

	_, err = db.Exec(Schema)
	if err != nil {
		log.Error(err, "failed to configure calendar Schema")
		return err
	}

	return nil
}

