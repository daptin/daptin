package resource

import (
	"encoding/base64"
	"fmt"
	"github.com/artpar/go-guerrilla"
	"github.com/daptin/daptin/server/auth"
	"github.com/samedi/caldav-go"
	"github.com/samedi/caldav-go/data"
	"github.com/samedi/caldav-go/errs"
	log "github.com/sirupsen/logrus"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)


type CalDavStorage struct {
	cruds map[string]*DbResource
	Mailer *mailSendActionPerformer
	Username string
	Email string
	UserID  int64
	UserReferenceID string
	GroupID  []auth.GroupPermission
}


func NewCaldavStorage(cruds map[string]*DbResource,certificateManager *CertificateManager)(*CalDavStorage, error){
	d := &guerrilla.Daemon{}
	return &CalDavStorage{
		cruds: cruds,
		Mailer: &mailSendActionPerformer{
			cruds:cruds,
			mailDaemon:d,
			certificateManager:certificateManager,
		},
	}, nil
}

func (c *CalDavStorage) CalDavHandler() http.Handler{
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var username, email string
		user := request.Context().Value("user")
		sessionUser := user.(auth.SessionUser)

		if user == nil {
			http.Error(writer, "Unauthorised User", 401)
		}

		if sessionUser.UserId == 0 {
			http.Error(writer, "Unauthorised User", 401)
		}

		retrievedUser,err:= c.cruds["user_account"].GetUserById(sessionUser.UserId)
		if err != nil {
			http.Error(writer, "Unable To Retrieve User", 401)
		}

		username = retrievedUser["name"].(string)
		email = retrievedUser["email"].(string)


		stg := new(CalDavStorage)
		stg.cruds    = c.cruds
		stg.Mailer   = c.Mailer
		stg.UserID = sessionUser.UserId
		stg.Email  = email
		stg.Username = username
		stg.UserReferenceID = sessionUser.UserReferenceId
		stg.GroupID = sessionUser.Groups

		caldav.SetupStorage(stg)

		response := caldav.HandleRequest(request)
		response.Body = "response"
		response.Write(writer)

	})
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
	var result []data.Resource

	a, err := cs.haveAccess(rpath, "read")
	if err != nil {
		log.Error(err, "failed to get Access [" + rpath + "]")
		return nil, err
	}

	if ! a {
		log.Info("no access to collection [" + rpath + "]")
		return nil, nil
	}

	if !cs.isResourcePresent(rpath){
		log.Info("no resource found for [" + rpath + "]")
		return nil, nil
	}

	res := data.NewResource(rpath, &PGResourceAdapter{db: cs.cruds, resourcePath: rpath, UserID: cs.UserID})
	result = append(result, res)


	return result, nil
}

func (cs *CalDavStorage) haveAccess(rpath string, perm string) (bool, error) {

	rowPermission := make(map[string]interface{})

	calRefId, err := cs.cruds["calendar"].GetReferenceIdByAccountId("calendar", cs.UserID)
	if err != nil {
		return false, err
	}

	rowPermission["__type"] = "calendar"
	rowPermission["reference_id"] = calRefId
	rowPermission["id"] = cs.UserID

	permInst := cs.cruds["calendar"].GetRowPermission(rowPermission)

	if perm == "read"{
		return permInst.CanRead(strconv.FormatInt(cs.UserID, 10), cs.GroupID), nil
	}

	if perm == "write"{
		return permInst.CanCreate(strconv.FormatInt(cs.UserID, 10), cs.GroupID), nil
	}

	if perm == "admin"{
		return cs.cruds[USER_ACCOUNT_TABLE_NAME].IsAdmin(cs.UserReferenceID), nil
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

	c := base64.StdEncoding.EncodeToString([]byte(content))
	err = cs.cruds["calendar"].InsertResource(rpath, c, cs.UserID)

	if err != nil {
		log.Error(err, "failed to insert ", rpath)
		return nil, err
	}

	res := data.NewResource(rpath, &PGResourceAdapter{db: cs.cruds, resourcePath: rpath, UserID: cs.UserID})
	_ = res.GetPropertyValue("VEVENT", "ATTENDEE")
	title     := res.GetPropertyValue("VEVENT", "SUMMARY")
	start     := res.StartTimeUTC()
	end       := res.EndTimeUTC()
	subject   := "Invitation: " + title + " @ " + start.Weekday().String() + " " + start.Month().String() + " " + strconv.Itoa(start.Day()) + ", " + strconv.Itoa(start.Year()) + " " + strconv.Itoa(start.Hour()) + " - " + strconv.Itoa(end.Hour()) + " (" + start.Location().String() + ") (" + cs.Email + ")"

	actionRequestParameters := make(map[string]interface{})
	actionRequestParameters["to"] = cs.Email
	actionRequestParameters["subject"] = subject
	actionRequestParameters["from"] = "daptin.no-reply"
	actionRequestParameters["body"] = content


	_, _, mailerError := cs.Mailer.DoAction(Outcome{}, actionRequestParameters)
	if mailerError  != nil{
		log.Error("Unable To Send mail", mailerError)
	}

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
	c := base64.StdEncoding.EncodeToString([]byte(content))
	err = cs.cruds["calendar"].UpdateResource(rpath, c)
	if err != nil {
		log.Error(err, "failed to update ", rpath)
		return nil, err
	}

	res := data.NewResource(rpath, &PGResourceAdapter{db: cs.cruds, resourcePath: rpath,  UserID: cs.UserID})
	_ = res.GetPropertyValue("VEVENT", "ATTENDEE")
	title     := res.GetPropertyValue("VEVENT", "SUMMARY")
	start     := res.StartTimeUTC()
	end       := res.EndTimeUTC()
	subject   := "Updated invitation: " + title + " @ " + start.Weekday().String() + " " + start.Month().String() + " " + strconv.Itoa(start.Day()) + ", " + strconv.Itoa(start.Year()) + " " + strconv.Itoa(start.Hour()) + " - " + strconv.Itoa(end.Hour()) + " (" + start.Location().String() + ") (" + cs.Email + ")"

	actionRequestParameters := make(map[string]interface{})
	actionRequestParameters["to"] = cs.Email
	actionRequestParameters["subject"] = subject
	actionRequestParameters["from"] = "daptin.no-reply"
	actionRequestParameters["body"] = content


	_, _, mailerError := cs.Mailer.DoAction(Outcome{}, actionRequestParameters)
	if mailerError  != nil{
		log.Error("Unable To Send mail", mailerError)
	}

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

	err = cs.cruds["calendar"].DeleteCalendarEvent(cs.UserID, rpath)
	if err != nil {
		log.Info("failed to delete resource ", rpath, " ", err.Error())
		return err
	}

	return nil
}

func (cs *CalDavStorage) isResourcePresent(rpath string) bool {
	rrpath, err := cs.cruds["calendar"].GetRpath(cs.UserID)

	if err != nil {
		return false
	}

	if rrpath == rpath {
		return true
	}

	return false
}

type PGResourceAdapter struct {
	db map[string]*DbResource
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
	calRefId, err := pa.db["calendar"].GetReferenceIdByAccountId("calendar", pa.UserID)
	if err != nil {
		return false, err
	}

	IntcalRefId, err := strconv.ParseInt(calRefId, 10, 6)
	if err != nil {
		log.Error(err, pa.resourcePath)
		return false, err
	}
	permInst := pa.db[USER_ACCOUNT_TABLE_NAME].GetObjectPermissionById("calendar", IntcalRefId)
	uGroupId := permInst.UserGroupId

	userRefId, err := pa.db[USER_ACCOUNT_TABLE_NAME].GetIdToReferenceId(USER_ACCOUNT_TABLE_NAME, pa.UserID)
	if err != nil {
		return false, err
	}

	if perm == "read"{
		return permInst.CanRead(strconv.FormatInt(pa.UserID, 10), uGroupId), nil
	}

	if perm == "write"{
		return permInst.CanCreate(strconv.FormatInt(pa.UserID, 10), uGroupId), nil
	}

	if perm == "admin"{
		return pa.db[USER_ACCOUNT_TABLE_NAME].IsAdmin(userRefId), nil
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
	content,err := pa.db["calendar"].GetContent(pa.resourcePath,pa.UserID)
	if err != nil {
		log.Error(err, "failed to fetch content ", pa.resourcePath)
		return ""
	}

	return content
	//ret, err := base64.StdEncoding.DecodeString(content)
	//if err != nil {
	//	log.Error(err, "decode error ", pa.resourcePath)
	//	return ""
	//}
	//return string(ret)
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
	var mod time.Time
	mod, err = pa.db["calendar"].GetModTime(pa.resourcePath, pa.UserID)
	if err != nil {
		log.Error(err, "failed to fetch modTime ", pa.resourcePath)
		return time.Unix(0, 0)
	}
	return mod
}


