package server

import (
	"context"
	"fmt"
	"github.com/artpar/api2go"
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

func InitializeYjsResources(documentProvider ydb.DocumentProvider, defaultRouter *gin.Engine,
	cruds map[string]*resource.DbResource, dtopicMap map[string]*olric.PubSub) error {
	var err error
	//var sessionFetcher *YjsConnectionSessionFetcher
	//sessionFetcher = &YjsConnectionSessionFetcher{}
	var ydbInstance = ydb.InitYdb(documentProvider)

	yjsConnectionHandler := ydb.YdbWsConnectionHandler(ydbInstance)

	defaultRouter.GET("/yjs/:documentName", func(ginContext *gin.Context) {

		sessionUser := ginContext.Request.Context().Value("user")
		if sessionUser == nil {
			ginContext.AbortWithStatus(403)
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

				redisPubSub := dtopicMap[typename].Subscribe(context.Background(), typename)
				go func(rps *redis.PubSub) {
					channel := rps.Channel()
					for {
						msg := <-channel
						var eventMessage resource.EventMessage
						//log.Infof("Message received: %s", msg.Payload)
						err = ProcessEventMessage(eventMessage, msg, typename, cruds, columnInfo, documentProvider)
						CheckErr(err, "Failed to process message on OlricTopic[%v]", typename)

					}
				}(redisPubSub)

				return func(ginContext *gin.Context) {

					sessionUser := ginContext.Request.Context().Value("user")
					if sessionUser == nil {
						ginContext.AbortWithStatus(403)
						return
					}
					user := sessionUser.(*auth.SessionUser)

					referenceId := ginContext.Param("referenceId")

					tx, err := cruds[typename].Connection().Beginx()
					if err != nil {
						resource.CheckErr(err, "Failed to begin transaction [840]")
						return
					}

					object, _, err := cruds[typename].GetSingleRowByReferenceIdWithTransaction(typename,
						daptinid.DaptinReferenceId(uuid.MustParse(referenceId)), nil, tx)
					tx.Rollback()
					if err != nil {
						ginContext.AbortWithStatus(404)
						return
					}

					tx, err = cruds[typename].Connection().Beginx()
					objectPermission := cruds[typename].GetRowPermission(object, tx)
					tx.Rollback()
					if err != nil {
						ginContext.AbortWithStatus(500)
						return
					}

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
