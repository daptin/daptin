package auth

import (
	"github.com/artpar/api2go"
	"github.com/dgrijalva/jwt-go"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"gopkg.in/gin-gonic/gin.v1"
	"net/http"
	"github.com/artpar/goms/server/jwt"
	"context"
)

const DEFAULT_PERMISSION int64 = 750

type CmsUser interface {
	GetName() string
	GetEmail() string
	IsGuest() bool
	IsLoggedIn() bool
}

type cmsUser struct {
	name       string
	email      string
	isLoggedIn bool
}

func (c *cmsUser) GetName() string {
	return c.name
}

func (c *cmsUser) GetEmail() string {
	return c.email
}

func (c *cmsUser) IsGuest() bool {
	return !c.isLoggedIn
}

func (c *cmsUser) IsLoggedIn() bool {
	return c.isLoggedIn
}

func GetUser(req *http.Request) *CmsUser {
	return nil
}

type AuthMiddleWare struct {
	db                *sqlx.DB
	userCrud          api2go.CRUD
	userGroupCrud     api2go.CRUD
	userUserGroupCrud api2go.CRUD
}

func NewAuthMiddlewareBuilder(db *sqlx.DB) *AuthMiddleWare {
	return &AuthMiddleWare{
		db: db,
	}
}

func (a *AuthMiddleWare) SetUserCrud(curd api2go.CRUD) {
	a.userCrud = curd
}

func (a *AuthMiddleWare) SetUserGroupCrud(curd api2go.CRUD) {
	a.userGroupCrud = curd
}

func (a *AuthMiddleWare) SetUserUserGroupCrud(curd api2go.CRUD) {
	a.userUserGroupCrud = curd
}

func NewAuthMiddleware(db *sqlx.DB, userCrud api2go.CRUD, userGroupCrud api2go.CRUD, userUserGroupCrud api2go.CRUD) *AuthMiddleWare {
	return &AuthMiddleWare{
		db:                db,
		userCrud:          userCrud,
		userGroupCrud:     userGroupCrud,
		userUserGroupCrud: userUserGroupCrud,
	}
}

var jwtMiddleware *jwtmiddleware.JWTMiddleware

func InitJwtMiddleware(secret []byte) {
	jwtMiddleware = jwtmiddleware.New(jwtmiddleware.Options{
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			return secret, nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err string) {
			log.Infof("Guest request [%v]: %v", err, r.Header)
		},
		//Debug: true,
		// When set, the middleware verifies that tokens are signed with the specific signing algorithm
		// If the signing method is not constant the ValidationKeyGetter callback can be used to implement additional checks
		// Important to avoid security issues described here: https://auth0.com/blog/2015/03/31/critical-vulnerabilities-in-json-web-token-libraries/
		SigningMethod: jwt.SigningMethodHS256,
		UserProperty:  "user",
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

func (a *AuthMiddleWare) AuthCheckMiddleware(c *gin.Context) {

	if StartsWith(c.Request.RequestURI, "/static") || StartsWith(c.Request.RequestURI, "/favicon.ico") {
		c.Next()
		return
	}

	user, err := jwtMiddleware.CheckJWT(c.Writer, c.Request)

	if err != nil {
		log.Infof("Auth failed: %v", err)
		c.Next()
	} else {

		//log.Infof("Set user: %v", user)
		if user == nil {

			newRequest := c.Request.WithContext(context.WithValue(c.Request.Context(), "user_id", ""))
			newRequest = newRequest.WithContext(context.WithValue(newRequest.Context(), "usergroup_id", []GroupPermission{}))
			c.Request = newRequest
			c.Next()
		} else {

			userToken := user
			email := userToken.Claims.(jwt.MapClaims)["email"].(string)
			name := userToken.Claims.(jwt.MapClaims)["name"].(string)
			//log.Infof("User is not nil: %v", email  )

			var referenceId string
			var userId int64
			var userGroups []GroupPermission
			err := a.db.QueryRowx("select u.id, u.reference_id from user u where email = ?", email).Scan(&userId, &referenceId)

			if err != nil {
				log.Errorf("Failed to scan user from db: %v", err)

				mapData := make(map[string]interface{})
				mapData["name"] = name
				mapData["email"] = email

				newUser := api2go.NewApi2GoModelWithData("user", nil, DEFAULT_PERMISSION, nil, mapData)

				req := api2go.Request{
					PlainRequest: &http.Request{
						Method: "POST",
					},
				}

				resp, err := a.userCrud.Create(newUser, req)
				if err != nil {
					log.Errorf("Failed to create new user: %v", err)
					c.AbortWithStatus(403)
					return
				}
				referenceId = resp.Result().(*api2go.Api2GoModel).Data["reference_id"].(string)

				mapData = make(map[string]interface{})
				mapData["name"] = "Home group of " + name

				newUserGroup := api2go.NewApi2GoModelWithData("usergroup", nil, DEFAULT_PERMISSION, nil, mapData)

				resp, err = a.userGroupCrud.Create(newUserGroup, req)
				if err != nil {
					log.Errorf("Failed to create new user group: %v", err)
				}
				userGroupId := resp.Result().(*api2go.Api2GoModel).Data["reference_id"].(string)

				userGroups = make([]GroupPermission, 0)
				mapData = make(map[string]interface{})
				mapData["user_id"] = referenceId
				mapData["usergroup_id"] = userGroupId

				newUserUserGroup := api2go.NewApi2GoModelWithData("user_user_id_has_usergroup_usergroup_id", nil, DEFAULT_PERMISSION, nil, mapData)

				uug, err := a.userUserGroupCrud.Create(newUserUserGroup, req)
				log.Infof("Userug: %v", uug)

			} else {
				rows, err := a.db.Queryx("select ug.reference_id as referenceid, uug.permission from usergroup ug join user_user_id_has_usergroup_usergroup_id uug on uug.usergroup_id = ug.id where uug.user_id = ?", userId)
				if err != nil {
					log.Errorf("Failed to get user group permissions: %v", err)
				} else {
					defer rows.Close()
					//cols, _ := rows.Columns()
					//log.Infof("Columns: %v", cols)
					for rows.Next() {
						var p GroupPermission
						err = rows.StructScan(&p)
						if err != nil {
							log.Errorf("failed to scan group permission struct: %v", err)
							continue
						}
						userGroups = append(userGroups, p)
					}

				}
			}

			//log.Infof("Group permissions :%v", userGroups)

			newRequest := c.Request.WithContext(context.WithValue(c.Request.Context(), "user_id", referenceId))
			newRequest = newRequest.WithContext(context.WithValue(newRequest.Context(), "user_id_integer", userId))
			newRequest = newRequest.WithContext(context.WithValue(newRequest.Context(), "usergroup_id", userGroups))
			c.Request = newRequest
			c.Next()

		}
	}

}

type GroupPermission struct {
	ReferenceId string
	Permission  int64
}
