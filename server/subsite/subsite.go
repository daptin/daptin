package subsite

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/artpar/api2go"
	_ "github.com/artpar/rclone/backend/all" // import all fs
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/dbresourceinterface"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type JsonApiError struct {
	Message string
}

type SubSite struct {
	Id           int64
	Name         string
	Hostname     string
	Path         string
	CloudStoreId *int64 `db:"cloud_store_id"`
	Permission   permission.PermissionInstance
	SiteType     string                     `db:"site_type"`
	FtpEnabled   bool                       `db:"ftp_enabled"`
	UserId       *int64                     `db:"user_account_id"`
	ReferenceId  daptinid.DaptinReferenceId `db:"reference_id"`
	Enable       bool                       `db:"enable"`
}

type HostRouterProvider interface {
	GetHostRouter(name string) *gin.Engine
}

func CreateTemplateHooks(transaction *sqlx.Tx, cruds map[string]dbresourceinterface.DbResourceInterface, hostSwitch HostRouterProvider) error {
	mainRouter := hostSwitch.GetHostRouter("dashboard")
	templateList, err := cruds["template"].GetAllObjects("template", transaction)
	if err != nil {
		return err
	}
	handlerCreator := CreateTemplateRouteHandler(cruds, transaction)
	for _, templateRow := range templateList {
		urlPattern := templateRow["url_pattern"].(string)
		strArray := make([]string, 0)
		err = json.Unmarshal([]byte(urlPattern), &strArray)
		if err != nil {
			return fmt.Errorf("Failed to parse url pattern ["+urlPattern+"] as string array: %s", err)
		}
		templateRenderHelper := handlerCreator(templateRow)
		for _, urlMatch := range strArray {
			mainRouter.Any(urlMatch, templateRenderHelper)
		}
	}
	return nil
}

func CreateTemplateRouteHandler(cruds map[string]dbresourceinterface.DbResourceInterface, transaction *sqlx.Tx) func(template map[string]interface{}) func(ginContext *gin.Context) {
	return func(templateInstance map[string]interface{}) func(ginContext *gin.Context) {

		templateName := templateInstance["name"].(string)
		actionConfigInterface := templateInstance["action_config"]
		cacheConfigInterface := templateInstance["cache_config"]
		actionRequest, err := GetActionConfig(actionConfigInterface)
		if err != nil {
			log.Errorf("Failed to get template instance for template [%v]", templateName)
		}

		var cacheConfig *CacheConfig
		cacheConfig, err = GetCacheConfig(cacheConfigInterface)
		if err != nil {
			log.Errorf("Failed to get template instance for template [%v]", templateName)
		}

		return func(ginContext *gin.Context) {

			if cacheConfig != nil {

			}

			actionRequest = actionresponse.ActionRequest{
				Type:       "article",
				Action:     "get_article_by_slug",
				Attributes: nil,
			}

			inFields := make(map[string]interface{})

			inFields["template"] = templateName
			queryMap := make(map[string]interface{})
			inFields["query"] = queryMap

			for _, param := range ginContext.Params {
				inFields[param.Key] = param.Value
			}

			queryParams := ginContext.Request.URL.Query()

			for key, valArray := range queryParams {
				if len(valArray) == 1 {
					inFields[key] = valArray[0]
					inFields[key+"[]"] = valArray
				} else {
					inFields[key] = valArray
					inFields[key+"[]"] = valArray
				}
			}

			var outcomeRequest actionresponse.Outcome
			var transaction1 *sqlx.Tx
			var errTxn error
			transaction1, errTxn = cruds["world"].Connection().Beginx()
			if errTxn != nil {
				_ = ginContext.AbortWithError(500, errTxn)
				return
			}
			defer func() {
				_ = transaction1.Commit()
			}()

			var api2goRequestData = api2go.Request{
				PlainRequest: ginContext.Request,
				QueryParams:  ginContext.Request.URL.Query(),
				Pagination:   nil,
				Header:       ginContext.Request.Header,
				Context:      nil,
			}

			if len(actionRequest.Action) > 0 && len(actionRequest.Type) > 0 {

				actionRequest = actionresponse.ActionRequest{
					Type:       "article",
					Action:     "get_article_by_slug",
					Attributes: inFields,
				}

				actionResponses, errAction := cruds["action"].HandleActionRequest(actionRequest, api2goRequestData, transaction1)
				if errAction != nil {
					_ = ginContext.AbortWithError(500, errAction)
					return
				}
				inFields["actionResponses"] = actionResponses

				for _, actionResponse := range actionResponses {
					inFields[actionResponse.ResponseType] = actionResponse.Attributes
				}
			}

			api2goResponder, _, err := cruds["world"].GetActionHandler("template.render").DoAction(
				outcomeRequest, inFields, transaction1)
			if err != nil && len(err) > 0 {
				_ = ginContext.AbortWithError(500, err[0])
				return
			}

			api2GoResult := api2goResponder.Result().(api2go.Api2GoModel)

			attrs := api2GoResult.GetAttributes()
			var content = attrs["content"].(string)
			var mimeType = attrs["mime_type"].(string)
			var headers = attrs["headers"].(map[string]string)

			ginContext.Writer.WriteHeader(http.StatusOK)
			ginContext.Writer.Header().Set("Content-Type", mimeType)
			for hKey, hValue := range headers {
				ginContext.Writer.Header().Set(hKey, hValue)
			}
			ginContext.Writer.Flush()

			// Render the rest of the DATA
			fmt.Fprint(ginContext.Writer, Atob(content))
			ginContext.Writer.Flush()

		}
	}
}

func Atob(data string) string {
	decodedData, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		log.Printf("Atob failed: %v", err)
		return ""
	}
	return string(decodedData)
}
