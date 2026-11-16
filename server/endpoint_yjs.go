package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"

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

func prepareYjsRoom(store yjsStore, roomName string) ([][]byte, error) {
	journal, err := store.read(roomName)
	if err != nil {
		return nil, err
	}
	return yjsReplay(journal)
}

type YjsRuntime struct {
	relay *yjsRelay
}

func (r *YjsRuntime) Close(ctx context.Context) error {
	r.relay.mu.Lock()
	r.relay.closing = true
	r.relay.mu.Unlock()
	r.relay.cancel()
	r.relay.broadcast.Close()
	done := make(chan struct{})
	go func() { r.relay.active.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func InitializeYjsResources(ctx context.Context, store yjsStore, defaultRouter *gin.Engine,
	cruds map[string]*resource.DbResource) (*YjsRuntime, error) {
	var err error

	broadcaster, err := newOlricYjsBroadcaster(ctx, cruds["world"].PubSub)
	if err != nil {
		return nil, err
	}
	relayCtx, cancel := context.WithCancel(ctx)
	relay := &yjsRelay{ctx: relayCtx, cancel: cancel, store: store, broadcast: broadcaster}
	runtime := &YjsRuntime{relay: relay}

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
			relay.serve(ginContext.Writer, ginContext.Request, roomName, readOnly)
			return
		}

		relay.serve(ginContext.Writer, ginContext.Request, documentName, false)

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

					relay.serve(ginContext.Writer, ginContext.Request, roomName, readOnly)

				}
			}(typename, columnInfo.ColumnName))

		}

	}

	return runtime, err
}
