package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/database"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/jwt"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
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

const x = GroupPeek | GroupRefer

const (
	GuestCRUD = GuestPeek | GuestRead | GuestCreate | GuestUpdate | GuestDelete | GuestRefer
	UserCRUD  = UserPeek | UserRead | UserCreate | UserUpdate | UserDelete | UserRefer
	GroupCRUD = GroupPeek | GroupRead | GroupCreate | GroupUpdate | GroupDelete | GroupRefer
)

const (
	DEFAULT_PERMISSION               = GuestPeek | GuestExecute | UserRead | UserExecute | GroupRead | GroupExecute
	DEFAULT_PERMISSION_WHEN_NO_ADMIN = GuestCRUD | GuestExecute | UserCRUD | UserExecute | GroupCRUD | GroupExecute
	ALLOW_ALL_PERMISSIONS            = GuestCRUD | GuestExecute | UserCRUD | UserExecute | GroupCRUD | GroupExecute
)

func (a AuthPermission) String() string {
	return fmt.Sprintf("%d", a)
}

type ResourceAdapter interface {
	api2go.CRUD
	GetUserPassword(email string, transaction *sqlx.Tx) (string, error)
}

type AuthMiddleware struct {
	db                database.DatabaseConnection
	userCrud          ResourceAdapter
	userGroupCrud     ResourceAdapter
	userUserGroupCrud ResourceAdapter
	issuer            string
	olricDb           *olric.EmbeddedClient
}

func NewAuthMiddlewareBuilder(db database.DatabaseConnection, issuer string, olricDb *olric.EmbeddedClient) *AuthMiddleware {
	return &AuthMiddleware{
		db:      db,
		issuer:  issuer,
		olricDb: olricDb,
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

func InitJwtMiddleware(secret []byte, issuer string, db *olric.EmbeddedClient) {
	if jwtmiddleware.TokenCache == nil {
		jwtmiddleware.TokenCache, _ = db.NewDMap("token-cache")
	}
	jwtMiddleware = jwtmiddleware.New(jwtmiddleware.Options{
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			return secret, nil
		},
		Issuer: issuer,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err string) {
			//log.Printf("Guest request [%v]: %v", err, r.Header)
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
	transaction, err := a.db.Beginx()
	if err != nil {
		CheckErr(err, "Failed to begin transaction [168]")
		return
	}

	existingPasswordHash, err := a.userCrud.GetUserPassword(username, transaction)
	transaction.Rollback()
	if err != nil {
		return
	}

	if BcryptCheckStringHash(password, existingPasswordHash) {
		token = &jwt.Token{
			Claims: jwt.MapClaims{
				"name":  strings.Split(username, "@")[0],
				"email": username,
				"sub":   username,
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

var UserGroupSelectQuery = statementbuilder.Squirrel.Select(
	goqu.I("ug.reference_id").As("groupreferenceid"),
	goqu.I("uug.reference_id").As("relationreferenceid"),
	goqu.I("uug.permission")).Prepared(true).
	From(goqu.T("usergroup").As("ug")).
	Join(goqu.T("user_account_user_account_id_has_usergroup_usergroup_id").As("uug"),
		goqu.On(goqu.Ex{
			"uug.usergroup_id": goqu.I("ug.id"),
		}))

func PrepareAuthQueries() {
	UserGroupSelectQuery = statementbuilder.Squirrel.Select(
		goqu.I("ug.reference_id").As("groupreferenceid"),
		goqu.I("uug.reference_id").As("relationreferenceid"),
		goqu.I("uug.permission")).Prepared(true).
		From(goqu.T("usergroup").As("ug")).
		Join(
			goqu.T("user_account_user_account_id_has_usergroup_usergroup_id").As("uug"),
			goqu.On(goqu.Ex{
				"uug.usergroup_id": goqu.I("ug.id"),
			}))

}

type CachedUserAccount struct {
	Account SessionUser
	Expiry  time.Time
}

// var LocalUserCacheMap = make(map[string]CachedUserAccount)
var LocalUserCacheLock = sync.Mutex{}
var olricCache olric.DMap

func (a *AuthMiddleware) AuthCheckMiddlewareWithHttp(req *http.Request, writer http.ResponseWriter,
	doBasicAuthCheck bool) (okToContinue, abortRequest bool, returnRequest *http.Request) {

	okToContinue = true
	abortRequest = false

	if StartsWith(req.RequestURI, "/static") || StartsWith(req.RequestURI, "/favicon.ico") {
		okToContinue = true
		return okToContinue, abortRequest, req
	}

	if olricCache == nil {
		log.Infof("Create olric default-cache for auth")
		olricCache, _ = a.olricDb.NewDMap("auth-cache")
	}

	hasUser := false

	userJwtToken, err := jwtMiddleware.CheckJWT(writer, req)

	if err != nil {
		//log.Warnf("failed to identify user in auth middleware: %v", err)
		if doBasicAuthCheck {
			userJwtToken, err = a.BasicAuthCheckMiddlewareWithHttp(req, writer)
			if err != nil || userJwtToken == nil {
				okToContinue = false
				CheckErr(err, "JWT middleware auth check failed")
				CheckErr(err, "BASIC middleware auth check failed")
			} else {
				hasUser = true
			}
		} else {
			//hasUser = true
			//log.Printf("JWT auth failed: %v", err)
		}
	} else {
		hasUser = true
	}

	if hasUser {

		if userJwtToken == nil {

			newRequest := req.WithContext(context.WithValue(req.Context(), "user_id", ""))
			newRequest = newRequest.WithContext(context.WithValue(newRequest.Context(), "usergroup_id", []GroupPermission{}))
			req = newRequest
			okToContinue = true
		} else {

			userToken := userJwtToken
			email := userToken.Claims.(jwt.MapClaims)["email"].(string)
			name := userToken.Claims.(jwt.MapClaims)["name"].(string)
			//log.Printf("User is not nil: %v", email)

			var sessionUser *SessionUser

			ok := false
			//LocalUserCacheLock.Lock()
			//localCachedUser, ok := LocalUserCacheMap[email]
			//
			//if ok && time.Now().After(localCachedUser.Expiry) {
			//	delete(LocalUserCacheMap, email)
			//	ok = false
			//}
			//LocalUserCacheLock.Unlock()

			if !ok {

				cachedUser, err := olricCache.Get(context.Background(), email)
				var referenceIdBytes []byte
				var userId int64
				var userGroups []GroupPermission
				if err != nil || cachedUser == nil {
					//log.Errorf("cached user [%v] is nil", email)

					sql, args, err := statementbuilder.Squirrel.Select(goqu.I("u.id"),
						goqu.I("u.reference_id")).Prepared(true).
						From(goqu.T("user_account").As("u")).Where(
						goqu.Ex{"email": email}).ToSQL()
					if err != nil {
						log.Errorf("Failed to create select query for user table")
						return false, true, req
					}

					stmt1, err := a.db.Preparex(sql)
					if err != nil {
						log.Errorf("[315] failed to prepare statment: %v", err)
						return false, true, req
					}

					rowx := stmt1.QueryRowx(args...)
					err = rowx.Scan(&userId, &referenceIdBytes)
					err = stmt1.Close()
					if err != nil {
						log.Errorf("failed to close prepared statement: %v", err)
					}

					uu, _ := uuid.FromBytes(referenceIdBytes[:])

					referenceId := daptinid.DaptinReferenceId(uu)

					if err != nil {
						// if a user logged in from third party oauth login
						log.Errorf("Failed to scan user [%v] from db: %v", email, err)

						mapData := make(map[string]interface{})
						mapData["name"] = name
						mapData["email"] = email

						newUser := api2go.NewApi2GoModelWithData("user_account", nil, int64(DEFAULT_PERMISSION), nil, mapData)
						ur, _ := url.Parse("/user_account")

						req1 := api2go.Request{
							PlainRequest: &http.Request{
								Method: "POST",
								URL:    ur,
							},
						}

						resp, err := a.userCrud.Create(newUser, req1)
						if err != nil {
							log.Errorf("Failed to create new user: %v", err)
							abortRequest = true
							return okToContinue, abortRequest, req
						}
						referenceId = daptinid.DaptinReferenceId([]byte((resp.Result().(api2go.Api2GoModel)).GetID()))

						mapData = make(map[string]interface{})
						mapData["name"] = "Home group of " + name

						newUserGroup := api2go.NewApi2GoModelWithData("usergroup", nil, int64(DEFAULT_PERMISSION), nil, mapData)

						resp, err = a.userGroupCrud.Create(newUserGroup, req1)
						if err != nil {
							log.Errorf("Failed to create new user group: %v", err)
						}
						userGroupId := daptinid.DaptinReferenceId([]byte((resp.Result().(api2go.Api2GoModel)).GetID()))

						userGroups = make([]GroupPermission, 0)
						mapData = make(map[string]interface{})
						mapData["user_account_id"] = referenceId
						mapData["usergroup_id"] = userGroupId

						newUserUserGroup := api2go.NewApi2GoModelWithData("user_account_user_account_id_has_usergroup_usergroup_id", nil, int64(DEFAULT_PERMISSION), nil, mapData)

						uug, err := a.userUserGroupCrud.Create(newUserUserGroup, req1)
						if err != nil {
							log.Errorf("Failed to create user-usergroup relation: %v", err)
						}
						log.Printf("User ug: %v", uug)

					} else {

						query, args1, err := UserGroupSelectQuery.Where(goqu.Ex{"uug.user_account_id": userId}).ToSQL()

						stmt1, err := a.db.Preparex(query)
						if err != nil {
							log.Errorf("[382] failed to prepare statment: %v", err)
							return false, true, nil
						}
						defer func(stmt1 *sqlx.Stmt) {
							err := stmt1.Close()
							if err != nil {
								log.Errorf("failed to close prepared statement: %v", err)
							}
						}(stmt1)

						rows, err := stmt1.Queryx(args1...)
						if err != nil {
							log.Errorf("Failed to get user group permissions: %v", err)
						} else {
							defer func() {
								if err := rows.Close(); err != nil {
									log.Errorf("failed to close result after fetching user in auth: %v", err)
								}
							}()

							//cols, _ := rows.Columns()
							//log.Debugf("Usergroup selection query for user [%v] : [%v]", email, query)
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

					//log.Printf("Group permissions :%v", userGroups)

					sessionUser = &SessionUser{
						UserId:          userId,
						UserReferenceId: referenceId,
						Groups:          userGroups,
					}
					//
					LocalUserCacheLock.Lock()
					//LocalUserCacheMap[email] = CachedUserAccount{
					//	Account: *sessionUser,
					//	Expiry:  time.Now().Add(2 * time.Minute),
					//}

					//if rand.Int() % 10 == 0 {
					//j, _ := json.Marshal(*sessionUser)
					//strJ := string(j)
					//log.Errorf("cache user account auth [%v] -> %v", len(strJ), strJ)
					repeatCheck, err := olricCache.Get(context.Background(), email)
					if err != nil || repeatCheck == nil {
						err = olricCache.Put(context.Background(), email, *sessionUser, olric.EX(10*time.Minute), olric.NX())
						CheckErr(err, "failed to put user in cache %s", email)
					}
					//}
					LocalUserCacheLock.Unlock()

				} else {
					var sessionUserValue SessionUser
					err = cachedUser.Scan(&sessionUserValue)
					if err == nil {
						sessionUser = &sessionUserValue
					}
					//LocalUserCacheLock.Lock()
					//LocalUserCacheMap[email] = CachedUserAccount{
					//	Account: *sessionUser,
					//	Expiry:  time.Now().Add(2 * time.Minute),
					//}
					//LocalUserCacheLock.Unlock()
				}
			} else {
				//sessionUser = &localCachedUser.Account
			}

			//log.Tracef("User cache map size: %v", len(LocalUserCacheMap))

			ct := req.Context()
			ct = context.WithValue(ct, "user", sessionUser)
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
	UserReferenceId daptinid.DaptinReferenceId
	Groups          GroupPermissionList
}

func (s SessionUser) MarshalBinary() ([]byte, error) {
	var data []byte

	userIdData := make([]byte, 8)
	binary.LittleEndian.PutUint64(userIdData, uint64(s.UserId))
	data = append(data, userIdData...)

	userRefData, _ := s.UserReferenceId.MarshalBinary()
	data = append(data, userRefData...)

	for _, group := range s.Groups {
		groupData, _ := group.MarshalBinary()
		data = append(data, groupData...)
	}

	return data, nil
}

func (s *SessionUser) UnmarshalBinary(data []byte) error {
	if len(data) < 24 { // 8 bytes + 16 bytes
		return errors.New("insufficient data for SessionUser")
	}

	s.UserId = int64(binary.LittleEndian.Uint64(data[:8]))
	err := s.UserReferenceId.UnmarshalBinary(data[8:24])
	if err != nil {
		return err
	}

	position := 24
	for position < len(data) {
		var group GroupPermission
		if err := group.UnmarshalBinary(data[position:]); err != nil {
			return err
		}
		s.Groups = append(s.Groups, group)
		position += 56 // size of one GroupPermission block
	}
	return nil
}

type GroupPermission struct {
	GroupReferenceId    daptinid.DaptinReferenceId `db:"groupreferenceid"`
	ObjectReferenceId   daptinid.DaptinReferenceId `db:"objectreferenceid"`
	RelationReferenceId daptinid.DaptinReferenceId `db:"relationreferenceid"`
	Permission          AuthPermission
}

type GroupPermissionList []GroupPermission

func (g GroupPermissionList) MarshalBinary() (data []byte, err error) {
	var buf bytes.Buffer
	for _, gp := range g {
		buf.Write(gp.GroupReferenceId[:])
		buf.Write(gp.ObjectReferenceId[:])
		buf.Write(gp.RelationReferenceId[:])
		if err := binary.Write(&buf, binary.LittleEndian, gp.Permission); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func (g *GroupPermissionList) UnmarshalBinary(data []byte) error {
	const size = 16 + 16 + 16 + 8 // Size of three DaptinReferenceIds and one AuthPermission
	if len(data)%size != 0 {
		return errors.New("invalid data length")
	}

	count := len(data) / size
	*g = make([]GroupPermission, count)
	for i := 0; i < count; i++ {
		offset := i * size
		(*g)[i].GroupReferenceId = *(*daptinid.DaptinReferenceId)(data[offset : offset+16])
		offset += 16
		(*g)[i].ObjectReferenceId = *(*daptinid.DaptinReferenceId)(data[offset : offset+16])
		offset += 16
		(*g)[i].RelationReferenceId = *(*daptinid.DaptinReferenceId)(data[offset : offset+16])
		offset += 16
		(*g)[i].Permission = AuthPermission(binary.LittleEndian.Uint64(data[offset : offset+8]))
	}
	return nil
}

const AuthGroupBinaryRepresentationSize = (3 * 16) + 8

func (g *GroupPermission) UnmarshalBinary(data []byte) error {
	if len(data) < (AuthGroupBinaryRepresentationSize) {
		return fmt.Errorf("insufficient data: expected at least %d bytes, got %d bytes", 3*16+8, len(data))
	}

	// Unmarshal UUIDs
	g.GroupReferenceId = daptinid.DaptinReferenceId(data[0:16])
	g.ObjectReferenceId = daptinid.DaptinReferenceId(data[16:32])
	g.RelationReferenceId = daptinid.DaptinReferenceId(data[32:48])

	// Unmarshal AuthPermission
	g.Permission = AuthPermission(binary.LittleEndian.Uint64(data[48:56]))

	return nil
}

func (g GroupPermission) MarshalBinary() ([]byte, error) {
	//TODO implement me
	// Create a byte slice of size 3*16 (for 3 UUIDs each of 16 bytes) + 8 for the int64 AuthPermission value
	var data = make([]byte, AuthGroupBinaryRepresentationSize)

	// Copy each UUID to the byte slice
	copy(data[0:16], g.GroupReferenceId[:])
	copy(data[16:32], g.ObjectReferenceId[:])
	copy(data[32:48], g.RelationReferenceId[:])

	// Convert AuthPermission to bytes and copy to the byte slice
	binary.LittleEndian.PutUint64(data[48:56], uint64(g.Permission))

	return data, nil
}
