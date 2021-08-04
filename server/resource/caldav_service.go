package resource

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/artpar/api2go"
	"github.com/artpar/go-guerrilla"
	"github.com/daptin/daptin/server/auth"
	"github.com/doug-martin/goqu/v9"
	"github.com/samedi/caldav-go"
	"github.com/samedi/caldav-go/data"
	"github.com/samedi/caldav-go/errs"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type CalDavStorage struct {
	cruds       map[string]*DbResource
	Mailer      *mailSendActionPerformer
	Username    string
	SessionUser *auth.SessionUser
}

func NewCaldavStorage(cruds map[string]*DbResource, certificateManager *CertificateManager) (*CalDavStorage, error) {
	d := &guerrilla.Daemon{}
	return &CalDavStorage{
		cruds: cruds,
		Mailer: &mailSendActionPerformer{
			cruds:              cruds,
			mailDaemon:         d,
			certificateManager: certificateManager,
		},
	}, nil
}

func (cs *CalDavStorage) CalDavHandler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {

		stg := new(CalDavStorage)
		stg.cruds = cs.cruds
		stg.Mailer = cs.Mailer

		str := strings.SplitN(request.Header.Get("Authorization"), " ", 2)
		if len(str) != 2 {
			body, _ := ioutil.ReadAll(request.Body)
			fmt.Println(string(body))
			writer.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(writer, "Not authorized", 401)
			return
		} else {

			b, err := base64.StdEncoding.DecodeString(str[1])
			if err != nil {
				http.Error(writer, err.Error(), 401)
				return
			}

			pair := strings.SplitN(string(b), ":", 2)
			if len(pair) != 2 {
				http.Error(writer, "Not authorized", 401)
				return
			}

			pword, err := cs.cruds["user_account"].GetUserPassword(pair[0])
			if err != nil {
				http.Error(writer, "User Not Found!", http.StatusUnauthorized)
				return
			}

			if !BcryptCheckStringHash(pair[1], pword) {
				http.Error(writer, "Unauthorized access", http.StatusUnauthorized)
				return
			}

			retrievedUser, err := cs.cruds["user_account"].GetUserAccountRowByEmail(pair[0])
			if err != nil {
				http.Error(writer, "Unable To Retrieve User", 401)
				return
			}

			userReferenceId := retrievedUser["reference_id"].(string)
			userId := retrievedUser["id"].(int64)

			userGroup, err := cs.cruds[USER_ACCOUNT_TABLE_NAME].GetUserGroupById(USER_ACCOUNT_TABLE_NAME, userId, userReferenceId)
			if err != nil {
				http.Error(writer, "Unable To Retrieve User", 401)
				log.Error("Unable To Retrieve User Group", err)
			}
			stg.SessionUser = &auth.SessionUser{
				UserId:          userId,
				UserReferenceId: userReferenceId,
				Groups:          userGroup,
			}
		}

		response := caldav.HandleRequestWithStorage(request, stg)
		response.Write(writer)
	})
}

func (cs *CalDavStorage) GetResourcesByList(rpaths []string) ([]data.Resource, error) {
	var results []data.Resource

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
	var result []data.Resource

	log.Infof("Get resources: [%s] => %v", rpath, withChildren)

	obj, err := cs.cruds["calendar"].GetObjectByWhereClause("calendar", "rpath", rpath)

	if err != nil {
		return nil, errs.ResourceNotFoundError
	}

	objPermission := cs.cruds["calendar"].GetRowPermission(obj)

	if !objPermission.CanRead(cs.SessionUser.UserReferenceId, cs.SessionUser.Groups) {
		return nil, fmt.Errorf("unauthorized")
	}

	encodedContent := obj["content"].([]map[string]interface{})[0]["content"].(string)
	decodedContent, _ := base64.StdEncoding.DecodeString(encodedContent)
	res := data.NewResource(rpath, &PGResourceAdapter{
		db:                  cs.cruds,
		resourcePath:        rpath,
		sessionUser:         cs.SessionUser,
		data:                obj,
		decodedCalendarData: string(decodedContent),
	})
	result = append(result, res)

	return result, nil
}

func (cs *CalDavStorage) haveAccess(rpath string, perm string) (bool, error) {
	if perm == "admin" {
		return cs.cruds[USER_ACCOUNT_TABLE_NAME].IsAdmin(cs.SessionUser.UserReferenceId), nil

	}
	permission := cs.cruds["calendar"].GetObjectPermissionByWhereClause("calendar", "rpath", rpath)

	switch perm {
	case "read":
		return permission.CanRead(cs.SessionUser.UserReferenceId, cs.SessionUser.Groups), nil
	case "write":
		return permission.CanCreate(cs.SessionUser.UserReferenceId, cs.SessionUser.Groups), nil
	}
	return false, nil
}

func isCollection(rpath string) bool {
	if rpath[len(rpath)-1:] == "/" {
		return true
	}
	if rpath[len(rpath)-3:] != "ics" {
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
	var result []data.Resource

	res, err := cs.GetResources(rpath, true)
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

func (cs *CalDavStorage) CreateResource(rpath, content string) (*data.Resource, error) {

	transaction, err := cs.cruds["calendar"].Connection.Beginx()
	if err != nil {
		return nil, err
	}
	calendarTablePermission := cs.cruds["world"].GetObjectPermissionByWhereClause("world", "table_name", "calendar")

	if !calendarTablePermission.CanCreate(cs.SessionUser.UserReferenceId, cs.SessionUser.Groups) {
		return nil, errs.ForbiddenError
	}

	base64EncodedContent := base64.StdEncoding.EncodeToString([]byte(content))

	calendarName := strings.Split(rpath, "/")
	createObj := &api2go.Api2GoModel{
		DeleteIncludes: nil,
		Data: map[string]interface{}{
			"rpath": rpath,
			"content": []interface{}{
				map[string]interface{}{
					"name":    calendarName[len(calendarName)-1],
					"content": base64EncodedContent,
				},
			}},
		Includes: nil,
	}
	httpRequest := &http.Request{
		Method: "POST",
	}
	httpRequest = httpRequest.WithContext(context.WithValue(context.Background(), "user", cs.SessionUser))
	apiRequest := api2go.Request{
		PlainRequest: httpRequest,
	}
	createdObj, err := cs.cruds["calendar"].CreateWithTransaction(createObj, apiRequest, transaction)
	if err != nil {
		rollbackErr := transaction.Rollback()
		CheckErr(rollbackErr, "failed to rollback")
		log.Errorf("failed to insert: %v", rpath)
		return nil, err
	}

	res := data.NewResource(rpath, &PGResourceAdapter{
		db:                  cs.cruds,
		resourcePath:        rpath,
		sessionUser:         cs.SessionUser,
		data:                createdObj.Result().(*api2go.Api2GoModel).Data,
		decodedCalendarData: content,
	})

	attendees := res.GetPropertyValue("VEVENT", "ATTENDEE")
	title := res.GetPropertyValue("VEVENT", "SUMMARY")
	start := res.StartTimeUTC()
	end := res.EndTimeUTC()
	subject := "Invitation: " + title + " @ " +
		start.Weekday().String() + " " + start.Month().String() + " " +
		strconv.Itoa(start.Day()) + ", " + strconv.Itoa(start.Year()) + " " +
		strconv.Itoa(start.Hour()) + " - " + strconv.Itoa(end.Hour()) +
		" (" + start.Location().String() + ")"

	actionRequestParameters := make(map[string]interface{})
	actionRequestParameters["to"] = strings.Split(attendees, ",")
	actionRequestParameters["subject"] = subject
	actionRequestParameters["from"] = "daptin.no-reply@localhost"
	actionRequestParameters["body"] = "You are invited to a new event: " + subject

	_, _, mailerError := cs.Mailer.DoAction(Outcome{}, actionRequestParameters, transaction)
	if mailerError != nil && len(mailerError) > 0 {
		rollbackErr := transaction.Rollback()
		CheckErr(rollbackErr, "Failed to rollback")
		log.Error("Unable To Send mail", mailerError)
		return nil, mailerError[0]
	}

	log.Info("resource created ", rpath)
	return &res, nil
}

func (cs *CalDavStorage) UpdateResource(rpath, content string) (*data.Resource, error) {

	transaction, err := cs.cruds["calendar"].Connection.Beginx()
	if err != nil {
		return nil, err
	}
	obj, err := GetObjectByWhereClauseWithTransaction("calendar", transaction, goqu.Ex{
		"rpath": rpath,
	})
	if err != nil {
		rollbackerr := transaction.Rollback()
		CheckErr(rollbackerr, "Failed to rollback")
		return nil, errs.ResourceNotFoundError
	}

	objPermission := cs.cruds["calendar"].GetRowPermission(obj[0])

	if !objPermission.CanUpdate(cs.SessionUser.UserReferenceId, cs.SessionUser.Groups) {
		return nil, errs.ForbiddenError
	}

	base64EncodedContent := base64.StdEncoding.EncodeToString([]byte(content))
	calendarName := strings.Split(rpath, "/")
	objectUpdate := &api2go.Api2GoModel{
		DeleteIncludes: nil,
		Data: map[string]interface{}{
			"rpath": rpath,
			"content": []interface{}{
				map[string]interface{}{
					"file":    calendarName[len(calendarName)-1],
					"content": base64EncodedContent,
				},
			}},
		Includes: nil,
	}
	httpRequest := &http.Request{
		Method: "PATCH",
	}
	httpRequest = httpRequest.WithContext(context.WithValue(context.Background(), "user", cs.SessionUser))
	apiRequest := api2go.Request{
		PlainRequest: httpRequest,
	}

	updatedObj, err := cs.cruds["calendar"].UpdateWithTransaction(objectUpdate, apiRequest, transaction)
	if err != nil {
		rollbackErr := transaction.Rollback()
		CheckErr(rollbackErr, "failed to rollback")
		log.Errorf("failed to update: %v", rpath)
		return nil, err
	}

	res := data.NewResource(rpath, &PGResourceAdapter{
		db:                  cs.cruds,
		resourcePath:        rpath,
		sessionUser:         cs.SessionUser,
		data:                updatedObj.Result().(*api2go.Api2GoModel).Data,
		decodedCalendarData: content,
	})

	attendees := res.GetPropertyValue("VEVENT", "ATTENDEE")
	title := res.GetPropertyValue("VEVENT", "SUMMARY")
	start := res.StartTimeUTC()
	end := res.EndTimeUTC()
	subject := "Updated invitation: " + title + " @ " + start.Weekday().String() + " " +
		start.Month().String() + " " + strconv.Itoa(start.Day()) + ", " +
		strconv.Itoa(start.Year()) + " " + strconv.Itoa(start.Hour()) +
		" - " + strconv.Itoa(end.Hour()) + " (" + start.Location().String() + ")"

	actionRequestParameters := make(map[string]interface{})
	actionRequestParameters["to"] = strings.Split(attendees, ",")
	actionRequestParameters["subject"] = subject
	actionRequestParameters["from"] = "daptin.no-reply@localhost"
	actionRequestParameters["body"] = content

	_, _, mailerError := cs.Mailer.DoAction(Outcome{}, actionRequestParameters, transaction)
	if mailerError != nil && len(mailerError) > 0 {
		rollbackErr := transaction.Rollback()
		CheckErr(rollbackErr, "failed to rollback")

		log.Error("Unable To Send mail", mailerError)
		return nil, mailerError[0]
	}
	commitErr := transaction.Commit()
	CheckErr(commitErr, "Failed to commit")

	log.Info("resource updated ", rpath)
	return &res, commitErr
}

func (cs *CalDavStorage) DeleteResource(rpath string) error {
	a, err := cs.haveAccess(rpath, "admin")
	if err != nil {
		log.Error(err, "failed to get Access ["+rpath+"]")
		return err
	}
	if !a {
		log.Info("no access to collection [" + rpath + "]")
		return nil
	}

	err = cs.cruds["calendar"].DeleteCalendarEvent(cs.SessionUser.UserId, rpath)
	if err != nil {
		log.Info("failed to delete resource ", rpath, " ", err.Error())
		return err
	}

	return nil
}

func (cs *CalDavStorage) isResourcePresent(rpath string) bool {
	_, err := cs.cruds["calendar"].GetObjectByWhereClause("calendar", "rpath", rpath)

	if err != nil {
		return false
	}

	return true
}

type PGResourceAdapter struct {
	db                  map[string]*DbResource
	resourcePath        string
	data                map[string]interface{}
	sessionUser         *auth.SessionUser
	decodedCalendarData string
}

func (pa *PGResourceAdapter) CalculateEtag() string {
	//if pa.IsCollection() {
	//	return ""
	//}

	return fmt.Sprintf(`"%x%x"`, pa.GetContentSize(), pa.GetModTime().UnixNano())
}

func (pa *PGResourceAdapter) haveAccess(perm string) (bool, error) {
	calId, err := pa.db["calendar"].GetCalendarIdByAccountId("calendar", pa.sessionUser.UserId)
	if err != nil {
		return false, err
	}

	permInst := pa.db["calendar"].GetObjectPermissionById("calendar", calId)
	fmt.Println("permInst", permInst)

	uGroupId := permInst.UserGroupId

	if perm == "read" {
		return permInst.CanRead(pa.sessionUser.UserReferenceId, uGroupId), nil

	}

	if perm == "write" {
		return permInst.CanCreate(pa.sessionUser.UserReferenceId, uGroupId), nil
	}

	if perm == "admin" {
		return pa.db[USER_ACCOUNT_TABLE_NAME].IsAdmin(pa.sessionUser.UserReferenceId), nil
	}

	return false, nil
}

func (pa *PGResourceAdapter) GetContent() string {
	return pa.decodedCalendarData
}

func (pa *PGResourceAdapter) GetContentSize() int64 {
	return int64(len(pa.decodedCalendarData))

}

func (pa *PGResourceAdapter) IsCollection() bool {
	return isCollection(pa.resourcePath)
}

func (pa *PGResourceAdapter) GetModTime() time.Time {
	return pa.data["updated_at"].(time.Time)
}
