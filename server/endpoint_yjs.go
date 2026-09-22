package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/artpar/ydb"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

func canonicalYjsRoomParts(documentName string) ([]string, bool) {
	parts := strings.Split(documentName, ".")
	if len(parts) != 3 {
		return nil, false
	}
	if _, err := uuid.Parse(parts[1]); err != nil {
		return nil, false
	}
	return parts, true
}

func yjsPermissionAccess(objectPermission permission.PermissionInstance, user *auth.SessionUser,
	administratorGroupId daptinid.DaptinReferenceId) (bool, bool) {
	if objectPermission.CanUpdate(user.UserReferenceId, user.Groups, administratorGroupId) {
		return true, false
	}
	if objectPermission.CanRead(user.UserReferenceId, user.Groups, administratorGroupId) {
		return true, true
	}
	return false, false
}

func authorizeYjsRoom(cruds map[string]*resource.DbResource, user *auth.SessionUser,
	typename string, referenceId string, columnName string) (string, bool, int, error) {
	crud, ok := cruds[typename]
	if !ok || crud == nil {
		return "", false, http.StatusNotFound, nil
	}

	columnInfo, ok := crud.TableInfo().GetColumnByName(columnName)
	if !ok || !BeginsWithCheck(columnInfo.ColumnType, "file.") {
		return "", false, http.StatusNotFound, nil
	}

	parsedId, err := uuid.Parse(referenceId)
	if err != nil {
		return "", false, http.StatusBadRequest, err
	}

	tx, err := crud.Connection().Beginx()
	if err != nil {
		return "", false, http.StatusInternalServerError, err
	}
	defer tx.Rollback()

	object, _, err := crud.GetSingleRowByReferenceIdWithTransaction(typename,
		daptinid.DaptinReferenceId(parsedId), nil, tx)
	if err != nil || object == nil {
		return "", false, http.StatusNotFound, nil
	}

	objectPermission := crud.GetRowPermission(object, tx)
	allowed, readOnly := yjsPermissionAccess(objectPermission, user, crud.AdministratorGroupId)
	if allowed {
		return fmt.Sprintf("%s.%s.%s", typename, parsedId.String(), columnInfo.ColumnName), readOnly, 0, nil
	}

	return "", false, http.StatusNotFound, nil
}

func serveYjsRoom(ginContext *gin.Context, yjsConnectionHandler http.HandlerFunc,
	roomName string, readOnly bool) {
	requestContext := context.WithValue(ginContext.Request.Context(), "roomname", roomName)
	if readOnly {
		requestContext = ydb.WithReadOnlySession(requestContext)
	}
	ginContext.Request = ginContext.Request.WithContext(requestContext)
	yjsConnectionHandler(ginContext.Writer, ginContext.Request)
}

func prepareYjsRoom(store ydb.Store, roomName string) error {
	_, err := store.Size(ydb.YjsRoomName(roomName))
	return err
}

type YjsRuntime struct {
	database    *ydb.Ydb
	broadcaster *olricYjsBroadcaster
}

func (r *YjsRuntime) Close() {
	if r.database != nil {
		r.database.Close()
	}
	if r.broadcaster != nil {
		r.broadcaster.Close()
	}
}

func InitializeYjsResources(ctx context.Context, store ydb.Store, defaultRouter *gin.Engine,
	cruds map[string]*resource.DbResource) (*YjsRuntime, error) {
	var err error

	broadcaster, err := newOlricYjsBroadcaster(ctx, cruds["world"].PubSub)
	if err != nil {
		return nil, err
	}
	ydbInstance := ydb.InitYdb(store, broadcaster)
	runtime := &YjsRuntime{database: ydbInstance, broadcaster: broadcaster}

	yjsConnectionHandler := ydb.YdbWsConnectionHandler(ydbInstance)

	defaultRouter.GET("/yjs/:documentName", func(ginContext *gin.Context) {

		sessionUser := ginContext.Request.Context().Value("user")
		user, ok := sessionUser.(*auth.SessionUser)
		if !ok || user == nil {
			ginContext.AbortWithStatus(403)
			return
		}

		logrus.Tracef("Handle new YJS client")
		documentName := ginContext.Param("documentName")
		parts, isCanonicalRoom := canonicalYjsRoomParts(documentName)
		if isCanonicalRoom {
			roomName, readOnly, status, authorizeErr := authorizeYjsRoom(cruds, user, parts[0], parts[1], parts[2])
			if status != 0 {
				if authorizeErr != nil {
					logrus.Errorf("failed to authorize YJS room: %v", authorizeErr)
				}
				ginContext.AbortWithStatus(status)
				return
			}
			if err := prepareYjsRoom(store, roomName); err != nil {
				logrus.Errorf("failed to prepare YJS room: %v", err)
				ginContext.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			serveYjsRoom(ginContext, yjsConnectionHandler, roomName, readOnly)
			return
		}

		serveYjsRoom(ginContext, yjsConnectionHandler, documentName, false)

	})

	for typename, crud := range cruds {

		for _, columnInfo := range crud.TableInfo().Columns {
			if !BeginsWithCheck(columnInfo.ColumnType, "file.") {
				continue
			}

			path := fmt.Sprintf("/live/%v/:referenceId/%v/yjs", typename, columnInfo.ColumnName)
			logrus.Printf("[%v] YJS websocket endpoint for %v[%v]", path, typename, columnInfo.ColumnName)
			defaultRouter.GET(path, func(typename string, columnName string) func(ginContext *gin.Context) {
				return func(ginContext *gin.Context) {

					sessionUser := ginContext.Request.Context().Value("user")
					if sessionUser == nil {
						ginContext.AbortWithStatus(403)
						return
					}
					user, ok := sessionUser.(*auth.SessionUser)
					if !ok || user == nil {
						ginContext.AbortWithStatus(403)
						return
					}

					roomName, readOnly, status, authorizeErr := authorizeYjsRoom(cruds, user, typename,
						ginContext.Param("referenceId"), columnName)
					if status != 0 {
						if authorizeErr != nil {
							logrus.Errorf("failed to authorize YJS room: %v", authorizeErr)
						}
						ginContext.AbortWithStatus(status)
						return
					}
					if err := prepareYjsRoom(store, roomName); err != nil {
						logrus.Errorf("failed to prepare YJS room: %v", err)
						ginContext.AbortWithStatus(http.StatusInternalServerError)
						return
					}

					serveYjsRoom(ginContext, yjsConnectionHandler, roomName, readOnly)

				}
			}(typename, columnInfo.ColumnName))

		}

	}

	return runtime, err
}
