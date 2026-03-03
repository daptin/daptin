package server

import (
	"context"
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/artpar/ydb"
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

func InitializeYjsResources(store ydb.Store, defaultRouter *gin.Engine,
	cruds map[string]*resource.DbResource, dtopicMap map[string]*olric.PubSub) error {
	var err error

	broadcaster := ydb.NewLocalBroadcaster(64)
	ydbInstance := ydb.InitYdb(store, broadcaster)

	yjsConnectionHandler := ydb.YdbWsConnectionHandler(ydbInstance)

	defaultRouter.GET("/yjs/:documentName", func(ginContext *gin.Context) {

		sessionUser := ginContext.Request.Context().Value("user")
		if sessionUser == nil {
			ginContext.AbortWithStatus(403)
			return
		}

		logrus.Tracef("Handle new YJS client")
		yjsConnectionHandler(ginContext.Writer, ginContext.Request)

	})

	for typename, crud := range cruds {

		for _, columnInfo := range crud.TableInfo().Columns {
			if !BeginsWithCheck(columnInfo.ColumnType, "file.") {
				continue
			}

			path := fmt.Sprintf("/live/%v/:referenceId/%v/yjs", typename, columnInfo.ColumnName)
			logrus.Printf("[%v] YJS websocket endpoint for %v[%v]", path, typename, columnInfo.ColumnName)
			defaultRouter.GET(path, func(typename string, columnInfo api2go.ColumnInfo) func(ginContext *gin.Context) {

				pubSub, ok := dtopicMap[typename]
				if !ok || pubSub == nil {
					logrus.Warnf("no pub/sub topic for type %v, skipping subscription", typename)
				} else {
					redisPubSub := pubSub.Subscribe(context.Background(), typename)
					go func(rps *redis.PubSub) {
						channel := rps.Channel()
						for msg := range channel {
							var eventMessage resource.WsOutMessage
							processErr := ProcessEventMessage(eventMessage, msg, typename, cruds, columnInfo, store)
							CheckErr(processErr, "Failed to process message on OlricTopic[%v]", typename)
						}
					}(redisPubSub)
				}

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

					referenceId := ginContext.Param("referenceId")

					parsedId, parseErr := uuid.Parse(referenceId)
					if parseErr != nil {
						ginContext.AbortWithStatus(400)
						return
					}

					tx, txErr := cruds[typename].Connection().Beginx()
					if txErr != nil {
						resource.CheckErr(txErr, "Failed to begin transaction [840]")
						return
					}

					object, _, getErr := cruds[typename].GetSingleRowByReferenceIdWithTransaction(typename,
						daptinid.DaptinReferenceId(parsedId), nil, tx)
					tx.Rollback()
					if getErr != nil {
						ginContext.AbortWithStatus(404)
						return
					}

					tx2, txErr2 := cruds[typename].Connection().Beginx()
					if txErr2 != nil {
						resource.CheckErr(txErr2, "Failed to begin transaction [850]")
						ginContext.AbortWithStatus(500)
						return
					}
					objectPermission := cruds[typename].GetRowPermission(object, tx2)
					tx2.Rollback()

					if !objectPermission.CanUpdate(user.UserReferenceId, user.Groups, cruds[typename].AdministratorGroupId) {
						ginContext.AbortWithStatus(401)
						return
					}

					roomName := fmt.Sprintf("%v%v%v%v%v", typename, ".", referenceId, ".", columnInfo.ColumnName)
					ginContext.Request = ginContext.Request.WithContext(context.WithValue(ginContext.Request.Context(), "roomname", roomName))

					yjsConnectionHandler(ginContext.Writer, ginContext.Request)

				}
			}(typename, columnInfo))

		}

	}

	return err
}
