package auth

import (
	"context"
	"database/sql/driver"
	"fmt"
	"github.com/artpar/api2go"
	"github.com/artpar/daptin/server/jwt"
	"github.com/dgrijalva/jwt-go"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"gopkg.in/gin-gonic/gin.v1"
	"net/http"
	"strings"
)

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

type AuthPermission int64

const None AuthPermission = iota

const (
	Peek          AuthPermission = 1 << iota
	ReadStrict
	CreateStrict
	UpdateStrict
	DeleteStrict
	ExecuteStrict
	ReferStrict
)

const (
	Read    = ReadStrict | Peek
	Refer   = ReferStrict | Read
	Create  = CreateStrict | Read
	Update  = UpdateStrict | Read
	Delete  = DeleteStrict | Read
	Execute = ExecuteStrict | Peek
	CRUD    = Read | Create | Update | Delete | Refer
)

type ObjectPermission struct {
	OwnerPermission AuthPermission
	GroupPermission AuthPermission
	GuestPermission AuthPermission
}

func (op *ObjectPermission) Scan(value interface{}) error {

	newOp := ParsePermission(value.(int64))
	op.GroupPermission = newOp.GroupPermission
	op.OwnerPermission = newOp.OwnerPermission
	op.GuestPermission = newOp.GuestPermission
	return nil
}
func (op ObjectPermission) Value() (driver.Value, error) {
	return op.IntValue(), nil
}

var DEFAULT_PERMISSION ObjectPermission = NewPermission(Peek|Execute, Read, CRUD|Execute)

func (op ObjectPermission) OwnerCan(a AuthPermission) bool {
	return op.OwnerPermission&a == a
}

func (op ObjectPermission) GroupCan(a AuthPermission) bool {
	return op.GroupPermission&a == a
}

func (op ObjectPermission) GuestCan(a AuthPermission) bool {
	return op.GuestPermission&a == a
}

func (al ObjectPermission) IntValue() int64 {
	return int64(al.OwnerPermission)*1000*1000 + int64(al.GroupPermission)*1000 + int64(al.GuestPermission)
}

func ParsePermission(p int64) ObjectPermission {
	al := ObjectPermission{}
	al.GuestPermission = AuthPermission(p % 1000)
	p = p / 1000
	al.GroupPermission = AuthPermission(p % 1000)
	p = p / 1000
	al.OwnerPermission = AuthPermission(p % 1000)
	return al
}

func NewPermission(guest AuthPermission, group AuthPermission, owner AuthPermission) ObjectPermission {
	al := ObjectPermission{}
	al.GuestPermission = guest
	al.GroupPermission = group
	al.OwnerPermission = owner
	return al
}

func (al ObjectPermission) String() string {
	return fmt.Sprintf("Owner[%v], Group[%v], Guest[%v]", al.OwnerPermission, al.GroupPermission, al.GuestPermission)
}

func (a AuthPermission) String() string {

	vals := []string{}

	if a == None {
		vals = append(vals, "Can None")
		return "Can Do None"
	}

	if Peek&a == Peek {
		vals = append(vals, "Can Peek")
	}
	if ReadStrict&a == ReadStrict {
		vals = append(vals, "Can ReadStrict")
	}
	if CreateStrict&a == CreateStrict {
		vals = append(vals, "Can CreateStrict")
	}
	if UpdateStrict&a == UpdateStrict {
		vals = append(vals, "Can UpdateStrict")
	}
	if DeleteStrict&a == DeleteStrict {
		vals = append(vals, "Can DeleteStrict")
	}
	if ExecuteStrict&a == ExecuteStrict {
		vals = append(vals, "Can ExecuteStrict")
	}
	if ReferStrict&a == ReferStrict {
		vals = append(vals, "Can ReferStrict")
	}
	if Read&a == Read {
		vals = append(vals, "Can Read")
	}
	if Create&a == Create {
		vals = append(vals, "Can Create")
	}
	if Update&a == Update {
		vals = append(vals, "Can Update")
	}
	if Delete&a == Delete {
		vals = append(vals, "Can Delete")
	}
	if Execute&a == Execute {
		vals = append(vals, "Can Execute")
	}
	if Refer&a == Refer {
		vals = append(vals, "Can Refer")
	}
	if CRUD&a == CRUD {
		vals = append(vals, "Can CRUD")
	}

	return strings.Join(vals, ", ")
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

				newUser := api2go.NewApi2GoModelWithData("user", nil, DEFAULT_PERMISSION.IntValue(), nil, mapData)

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

				newUserGroup := api2go.NewApi2GoModelWithData("usergroup", nil, DEFAULT_PERMISSION.IntValue(), nil, mapData)

				resp, err = a.userGroupCrud.Create(newUserGroup, req)
				if err != nil {
					log.Errorf("Failed to create new user group: %v", err)
				}
				userGroupId := resp.Result().(*api2go.Api2GoModel).Data["reference_id"].(string)

				userGroups = make([]GroupPermission, 0)
				mapData = make(map[string]interface{})
				mapData["user_id"] = referenceId
				mapData["usergroup_id"] = userGroupId

				newUserUserGroup := api2go.NewApi2GoModelWithData("user_user_id_has_usergroup_usergroup_id", nil, DEFAULT_PERMISSION.IntValue(), nil, mapData)

				uug, err := a.userUserGroupCrud.Create(newUserUserGroup, req)
				if err != nil {
					log.Errorf("Failed to create user-usergroup relation: %v", err)
				}
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

			user := SessionUser{
				UserId:          userId,
				UserReferenceId: referenceId,
				Groups:          userGroups,
			}
			ct := c.Request.Context()
			ct = context.WithValue(ct, "user", user)
			newRequest := c.Request.WithContext(ct)
			c.Request = newRequest
			c.Next()

		}
	}

}

type SessionUser struct {
	UserId          int64
	UserReferenceId string
	Groups          []GroupPermission
}

type GroupPermission struct {
	ReferenceId string
	Permission  ObjectPermission
}
