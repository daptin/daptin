package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/jwt"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"strings"
)

type AuthPermission int64

const None AuthPermission = iota

const (
	GuestPeek AuthPermission = 1 << iota
	GuestRead
	GuestCreate
	GuestUpdate
	GuestDelete
	GuestExecute
	GuestRefer
	UserPeek
	UserRead
	UserCreate
	UserUpdate
	UserDelete
	UserExecute
	UserRefer
	GroupPeek
	GroupRead
	GroupCreate
	GroupUpdate
	GroupDelete
	GroupExecute
	GroupRefer
)

const (
	GuestCRUD = GuestPeek | GuestRead | GuestCreate | GuestUpdate | GuestDelete | GuestRefer
	UserCRUD  = UserPeek | UserRead | UserCreate | UserUpdate | UserDelete | UserRefer
	GroupCRUD = GroupPeek | GroupRead | GroupCreate | GroupUpdate | GroupDelete | GroupRefer
)

var DEFAULT_PERMISSION = GuestPeek | GuestExecute | UserCRUD | UserExecute | GroupCreate | GroupExecute | GroupRefer | GroupCRUD
var ALLOW_ALL_PERMISSIONS = GuestCRUD | GuestExecute | UserCRUD | UserExecute | GroupCRUD | GroupExecute

func (a AuthPermission) String() string {
	return fmt.Sprintf("%d", a)
}

type ResourceAdapter interface {
	api2go.CRUD
	GetUserPassword(email string) (string, error)
}

type AuthMiddleware struct {
	db                database.DatabaseConnection
	userCrud          ResourceAdapter
	userGroupCrud     ResourceAdapter
	userUserGroupCrud ResourceAdapter
	issuer            string
}

func NewAuthMiddlewareBuilder(db database.DatabaseConnection, issuer string) *AuthMiddleware {
	return &AuthMiddleware{
		db:     db,
		issuer: issuer,
	}
}

func (a *AuthMiddleware) SetUserCrud(curd ResourceAdapter) {
	a.userCrud = curd
}

func (a *AuthMiddleware) SetUserGroupCrud(curd ResourceAdapter) {
	a.userGroupCrud = curd
}

func (a *AuthMiddleware) SetUserUserGroupCrud(curd ResourceAdapter) {
	a.userUserGroupCrud = curd
}

var jwtMiddleware *jwtmiddleware.JWTMiddleware

func InitJwtMiddleware(secret []byte, issuer string) {
	jwtMiddleware = jwtmiddleware.New(jwtmiddleware.Options{
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			return secret, nil
		},
		Issuer: issuer,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err string) {
			//log.Infof("Guest request [%v]: %v", err, r.Header)
		},
		Debug: false,
		// When set, the middleware verifies that tokens are signed with the specific signing algorithm
		// If the signing method is not constant the ValidationKeyGetter callback can be used to implement additional checks
		// Important to avoid security issues described here: https://auth0.com/blog/2015/03/31/critical-vulnerabilities-in-json-web-token-libraries/
		SigningMethod: jwt.SigningMethodHS256,
		UserProperty:  "user",
		Extractor: jwtmiddleware.FromFirst(
			jwtmiddleware.FromAuthHeader,
			jwtmiddleware.FromParameter("token"),
			func(r *http.Request) (string, error) {
				cookie, e := r.Cookie("token")
				if cookie == nil {
					return "", nil
				}
				return cookie.Value, e
			},
		),
	})
}

func StartsWith(bigStr string, smallString string) bool {
	if len(bigStr) < len(smallString) {
		return false
	}

	if bigStr[0:len(smallString)] == smallString {
		return true
	}

	return false

}

func (a *AuthMiddleware) BasicAuthCheckMiddlewareWithHttp(req *http.Request, writer http.ResponseWriter) (token *jwt.Token, err error) {
	token = nil
	authHeaderValue := req.Header.Get("Authorization")
	bearerValueParts := strings.Split(authHeaderValue, " ")
	if len(bearerValueParts) < 2 {
		return
	}

	tokenString := bearerValueParts[1]
	tokenValue, err := base64.StdEncoding.DecodeString(tokenString)
	if err != nil {
		return
	}
	tokenValueParts := strings.Split(string(tokenValue), ":")
	username := tokenValueParts[0]
	password := ""
	if len(tokenValueParts) > 1 {
		password = tokenValueParts[1]
	}
	existingPasswordHash, err := a.userCrud.GetUserPassword(username)
	if err != nil {
		return
	}

	if BcryptCheckStringHash(password, existingPasswordHash) {
		token = &jwt.Token{
			Claims: jwt.MapClaims{
				"name":  strings.Split(username, "@")[0],
				"email": username,
			},
		}
	}

	return
}

func BcryptCheckStringHash(newString, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(newString))
	return err == nil
}

func CheckErr(err error, message ...interface{}) {
	if err != nil {
		fmtString := message[0].(string)
		args := make([]interface{}, 0)
		if len(message) > 1 {
			args = message[1:]
		}
		args = append(args, err)
		log.Errorf(fmtString+": %v", args...)
	}
}

func (a *AuthMiddleware) AuthCheckMiddlewareWithHttp(req *http.Request, writer http.ResponseWriter, doBasicAuthCheck bool) (okToContinue, abortRequest bool, returnRequest *http.Request) {
	okToContinue = true
	abortRequest = false

	if StartsWith(req.RequestURI, "/static") || StartsWith(req.RequestURI, "/favicon.ico") {
		okToContinue = true
		return okToContinue, abortRequest, req
	}

	hasUser := false
	user, err := jwtMiddleware.CheckJWT(writer, req)

	if err != nil {
		if doBasicAuthCheck {
			user, err = a.BasicAuthCheckMiddlewareWithHttp(req, writer)
			if err != nil || user == nil {
				CheckErr(err, "JWT middleware auth check failed")
				CheckErr(err, "BASIC middleware auth check failed")
			} else {
				hasUser = true
			}
		} else {
			//hasUser = true
			//log.Infof("JWT auth failed: %v", err)
		}
	} else {
		hasUser = true
	}

	if hasUser {

		//log.Infof("Set user: %v", user)
		if user == nil {

			newRequest := req.WithContext(context.WithValue(req.Context(), "user_id", ""))
			newRequest = newRequest.WithContext(context.WithValue(newRequest.Context(), "usergroup_id", []GroupPermission{}))
			req = newRequest
			okToContinue = true
		} else {

			userToken := user
			email := userToken.Claims.(jwt.MapClaims)["email"].(string)
			name := userToken.Claims.(jwt.MapClaims)["name"].(string)
			//log.Infof("User is not nil: %v", email  )

			var referenceId string
			var userId int64
			var userGroups []GroupPermission

			sql, args, err := statementbuilder.Squirrel.Select("u.id", "u.reference_id").From("user_account u").Where("email = ?", email).ToSql()
			if err != nil {
				log.Errorf("Failed to create select query for user table")
				return false, true, req
			}

			rowx := a.db.QueryRowx(sql, args...)
			err = rowx.Scan(&userId, &referenceId)

			if err != nil {
				// if a user logged in from third party oauth login
				log.Errorf("Failed to scan user from db: %v", err)

				mapData := make(map[string]interface{})
				mapData["name"] = name
				mapData["email"] = email

				newUser := api2go.NewApi2GoModelWithData("user_account", nil, int64(DEFAULT_PERMISSION), nil, mapData)

				req1 := api2go.Request{
					PlainRequest: &http.Request{
						Method: "POST",
					},
				}

				resp, err := a.userCrud.Create(newUser, req1)
				if err != nil {
					log.Errorf("Failed to create new user: %v", err)
					abortRequest = true
					return okToContinue, abortRequest, req
				}
				referenceId = resp.Result().(*api2go.Api2GoModel).Data["reference_id"].(string)

				mapData = make(map[string]interface{})
				mapData["name"] = "Home group of " + name

				newUserGroup := api2go.NewApi2GoModelWithData("usergroup", nil, int64(DEFAULT_PERMISSION), nil, mapData)

				resp, err = a.userGroupCrud.Create(newUserGroup, req1)
				if err != nil {
					log.Errorf("Failed to create new user group: %v", err)
				}
				userGroupId := resp.Result().(*api2go.Api2GoModel).Data["reference_id"].(string)

				userGroups = make([]GroupPermission, 0)
				mapData = make(map[string]interface{})
				mapData["user_account_id"] = referenceId
				mapData["usergroup_id"] = userGroupId

				newUserUserGroup := api2go.NewApi2GoModelWithData("user_account_user_account_id_has_usergroup_usergroup_id", nil, int64(DEFAULT_PERMISSION), nil, mapData)

				uug, err := a.userUserGroupCrud.Create(newUserUserGroup, req1)
				if err != nil {
					log.Errorf("Failed to create user-usergroup relation: %v", err)
				}
				log.Infof("User ug: %v", uug)

			} else {

				sql, args, err := statementbuilder.Squirrel.Select("ug.reference_id as \"groupreferenceid\"",
					"uug.reference_id as \"relationreferenceid\"", "uug.permission").From("usergroup ug").
					Join("user_account_user_account_id_has_usergroup_usergroup_id uug on uug.usergroup_id = ug.id").Where("uug.user_account_id = ?", userId).ToSql()

				rows, err := a.db.Queryx(sql, args...)

				if err != nil {
					log.Errorf("Failed to get user group permissions: %v", err)
				} else {
					defer rows.Close()
					//cols, _ := rows.Columns()
					//log.Infof("Columns: %v", cols)
					for rows.Next() {
						var p GroupPermission
						err = rows.StructScan(&p)
						p.ObjectReferenceId = referenceId
						if err != nil {
							log.Errorf("failed to scan group permission struct: %v", err)
							continue
						}
						userGroups = append(userGroups, p)
					}

				}
			}

			//log.Infof("Group permissions :%v", userGroups)

			user := &SessionUser{
				UserId:          userId,
				UserReferenceId: referenceId,
				Groups:          userGroups,
			}
			ct := req.Context()
			ct = context.WithValue(ct, "user", user)
			newRequest := req.WithContext(ct)
			req = newRequest
			okToContinue = true
		}
	}

	return okToContinue, abortRequest, req
}

func (a *AuthMiddleware) AuthCheckMiddleware(c *gin.Context) {

	ok, abort, newRequest := a.AuthCheckMiddlewareWithHttp(c.Request, c.Writer, false)
	if abort {
		c.Abort()
	} else if ok {
		c.Request = newRequest
		c.Next()
	} else {
		c.AbortWithStatus(401)
	}

}

type SessionUser struct {
	UserId          int64
	UserReferenceId string
	Groups          []GroupPermission
}

type GroupPermission struct {
	GroupReferenceId    string `db:"groupreferenceid"`
	ObjectReferenceId   string `db:"objectreferenceid"`
	RelationReferenceId string `db:"relationreferenceid"`
	Permission          AuthPermission
}
