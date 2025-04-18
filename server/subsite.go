package server

import (
	"fmt"
	"github.com/artpar/api2go"
	_ "github.com/artpar/rclone/backend/all" // import all fs
	"github.com/artpar/stats"
	"github.com/aviddiviner/gin-limit"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	uuid "github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	limit2 "github.com/yangxikun/gin-limit-by-key"
	"golang.org/x/time/rate"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

type HostSwitch struct {
	handlerMap           map[string]*gin.Engine
	siteMap              map[string]resource.SubSite
	authMiddleware       *auth.AuthMiddleware
	AdministratorGroupId daptinid.DaptinReferenceId
}

type JsonApiError struct {
	Message string
}

func CreateAssetColumnSync(cruds map[string]*resource.DbResource, transaction *sqlx.Tx) map[string]map[string]*resource.AssetFolderCache {
	log.Tracef("CreateAssetColumnSync")

	stores, err := cruds["cloud_store"].GetAllCloudStores(transaction)
	assetCache := make(map[string]map[string]*resource.AssetFolderCache)

	if err != nil || len(stores) == 0 {
		return assetCache
	}
	cloudStoreMap := make(map[string]resource.CloudStore)

	for _, store := range stores {
		cloudStoreMap[store.Name] = store
	}

	for tableName, tableResource := range cruds {

		colCache := make(map[string]*resource.AssetFolderCache)

		tableInfo := tableResource.TableInfo()
		for _, column := range tableInfo.Columns {

			if column.IsForeignKey && column.ForeignKeyData.DataSource == "cloud_store" {

				columnName := column.ColumnName

				cloudStore := cloudStoreMap[column.ForeignKeyData.Namespace]
				tempDirectoryPath, err := ioutil.TempDir(os.Getenv("DAPTIN_CACHE_FOLDER"), tableName+"_"+columnName)

				if cloudStore.StoreProvider != "local" {
					err = cruds["task"].SyncStorageToPath(cloudStore, column.ForeignKeyData.KeyName, tempDirectoryPath, transaction)
					if resource.CheckErr(err, "Failed to setup sync to path for table column [%v][%v]", tableName, column.ColumnName) {
						continue
					}
				} else {
					tempDirectoryPath = cloudStore.RootPath + "/" + column.ForeignKeyData.KeyName
				}

				assetCacheFolder := &resource.AssetFolderCache{
					CloudStore:    cloudStore,
					LocalSyncPath: tempDirectoryPath,
					Keyname:       column.ForeignKeyData.KeyName,
				}

				colCache[columnName] = assetCacheFolder
				log.Infof("Sync table column [%v][%v] at %v", tableName, columnName, tempDirectoryPath)

				if cloudStore.StoreProvider != "local" {
					err = TaskScheduler.AddTask(resource.Task{
						EntityName: "world",
						ActionName: "sync_column_storage",
						Attributes: map[string]interface{}{
							"table_name":  tableResource.TableInfo().TableName,
							"column_name": columnName,
						},
						AsUserEmail: cruds[resource.USER_ACCOUNT_TABLE_NAME].GetAdminEmailId(transaction),
						Schedule:    "@every 30m",
					})
				}

			}

		}

		assetCache[tableName] = colCache

	}
	log.Tracef("Completed CreateAssetColumnSync")

	return assetCache

}

func CreateTemplateHooks(cmsConfig *resource.CmsConfig,
	transaction *sqlx.Tx, cruds map[string]*resource.DbResource,
	rateConfig RateConfig, hs HostSwitch) error {
	mainRouter := hs.handlerMap["dashboard"]
	templateList, err := cruds["template"].GetAllObjects("template", transaction)
	if err != nil {
		return err
	}
	handlerCreator := CreateTemplateRouteHandler(cruds)
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

func CreateTemplateRouteHandler(cruds map[string]*resource.DbResource) func(template map[string]interface{}) func(ginContext *gin.Context) {
	return func(template map[string]interface{}) func(ginContext *gin.Context) {
		return func(ginContext *gin.Context) {
			inFields := make(map[string]interface{})

			inFields["template"] = template["name"].(string)
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

			var outcomeRequest resource.Outcome
			var transaction *sqlx.Tx
			var errTxn error
			transaction, errTxn = cruds["world"].Connection.Beginx()
			if errTxn != nil {
				ginContext.AbortWithError(500, errTxn)
				return
			}
			defer func() {
				transaction.Rollback()
			}()
			api2goResponder, _, err := cruds["action"].ActionHandlerMap["template.render"].DoAction(
				outcomeRequest, inFields, transaction)
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
			fmt.Fprint(ginContext.Writer, resource.Atob(content))
			ginContext.Writer.Flush()

		}
	}
}

// CreateSubSites creates a router which can route based on hostname to one of the hosted static subsites
func CreateSubSites(cmsConfig *resource.CmsConfig, transaction *sqlx.Tx,
	cruds map[string]*resource.DbResource, authMiddleware *auth.AuthMiddleware,
	rateConfig RateConfig, max_connections int) (HostSwitch, map[daptinid.DaptinReferenceId]*resource.AssetFolderCache) {

	router := httprouter.New()
	router.ServeFiles("/*filepath", http.Dir("./scripts"))

	hs := HostSwitch{
		AdministratorGroupId: cruds["usergroup"].AdministratorGroupId,
	}
	subsiteCacheFolders := make(map[daptinid.DaptinReferenceId]*resource.AssetFolderCache)
	hs.handlerMap = make(map[string]*gin.Engine)
	hs.siteMap = make(map[string]resource.SubSite)
	hs.authMiddleware = authMiddleware

	//log.Printf("Cruds before making sub sits: %v", cruds)
	sites, err := cruds["site"].GetAllSites(transaction)
	if err != nil {
		log.Printf("Failed to get all sites 117: %v", err)
	}
	stores, err := cruds["cloud_store"].GetAllCloudStores(transaction)
	if err != nil {
		log.Printf("Failed to get all cloudstores 121: %v", err)
	}
	cloudStoreMap := make(map[int64]resource.CloudStore)

	adminEmailId := cruds[resource.USER_ACCOUNT_TABLE_NAME].GetAdminEmailId(transaction)
	log.Printf("Admin email id: %s", adminEmailId)

	for _, store := range stores {
		cloudStoreMap[store.Id] = store
	}

	siteMap := make(map[string]resource.SubSiteInformation)

	if err != nil {
		log.Errorf("Failed to load sites from database: %v", err)
		return hs, subsiteCacheFolders
	}

	//max_connections, err := configStore.GetConfigIntValueFor("limit.max_connections", "backend")
	//rate_limit, err := configStore.GetConfigIntValueFor("limit.rate", "backend")

	for _, site := range sites {

		if !site.Enable {
			continue
		}

		subSiteInformation := resource.SubSiteInformation{}
		//hs.siteMap[site.Path] = site

		for _, hostname := range strings.Split(site.Hostname, ",") {
			hs.siteMap[hostname] = site
		}

		subSiteInformation.SubSite = site

		if site.CloudStoreId == nil {
			log.Printf("Site [%v] does not have a associated storage", site.Name)
			continue
		}

		u, _ := uuid.NewV7()
		sourceDirectoryName := u.String()
		tempDirectoryPath, err := ioutil.TempDir(os.Getenv("DAPTIN_CACHE_FOLDER"), sourceDirectoryName)
		if resource.CheckErr(err, "Failed to create temp directory") {
			continue
		}
		subSiteInformation.SourceRoot = tempDirectoryPath
		cloudStore, ok := cloudStoreMap[*site.CloudStoreId]
		subSiteInformation.CloudStore = cloudStore
		if !ok {
			log.Printf("Site [%v] does not have a associated storage", site.Name)
			continue
		}

		err = cruds["task"].SyncStorageToPath(cloudStore, site.Path, tempDirectoryPath, transaction)
		if resource.CheckErr(err, "Failed to setup sync to path for subsite [%v]", site.Name) {
			continue
		}

		syncTask := resource.Task{
			EntityName: "site",
			ActionName: "sync_site_storage",
			Attributes: map[string]interface{}{
				"site_id": site.ReferenceId.String(),
				"path":    tempDirectoryPath,
			},
			AsUserEmail: adminEmailId,
			Schedule:    "@every 1h",
		}

		activeTask := cruds["site"].NewActiveTaskInstance(syncTask)

		func(task *resource.ActiveTaskInstance) {
			go func() {
				log.Info("Sleep 5 sec for running new sync task")
				time.Sleep(5 * time.Second)
				activeTask.Run()
			}()
		}(activeTask)

		err = TaskScheduler.AddTask(syncTask)

		subsiteCacheFolders[site.ReferenceId] = &resource.AssetFolderCache{
			LocalSyncPath: tempDirectoryPath,
			Keyname:       site.Path,
			CloudStore:    cloudStore,
		}

		resource.CheckErr(err, "Failed to register task to sync storage")

		subsiteStats := stats.New()
		hostRouter := gin.New()

		hostRouter.Use(func() gin.HandlerFunc {
			return func(c *gin.Context) {
				beginning, recorder := subsiteStats.Begin(c.Writer)
				defer Stats.End(beginning, stats.WithRecorder(recorder))
				c.Next()
			}
		}())

		hostRouter.Use(limit.MaxAllowed(max_connections))
		hostRouter.Use(limit2.NewRateLimiter(func(c *gin.Context) string {
			requestPath := c.Request.Host + "/" + strings.Split(c.Request.RequestURI, "?")[0]
			return c.ClientIP() + requestPath // limit rate by client ip
		}, func(c *gin.Context) (*rate.Limiter, time.Duration) {
			requestPath := c.Request.Host + "/" + strings.Split(c.Request.RequestURI, "?")[0]
			limitValue, ok := rateConfig.limits[requestPath]
			if !ok {
				limitValue = 100
			}

			return rate.NewLimiter(rate.Every(100*time.Millisecond), limitValue), time.Hour // limit 10 qps/clientIp and permit bursts of at most 10 tokens, and the limiter liveness time duration is 1 hour
		}, func(c *gin.Context) {
			c.AbortWithStatus(429) // handle exceed rate limit request
		}))

		hostRouter.GET("/stats", func(c *gin.Context) {
			c.JSON(200, subsiteStats.Data())
		})

		//hostRouter.ServeFiles("/*filepath", http.Dir(tempDirectoryPath))
		hostRouter.Use(authMiddleware.AuthCheckMiddleware)

		if site.SiteType == "hugo" {
			hostRouter.Use(static.Serve("/", static.LocalFile(tempDirectoryPath+"/public", true)))
		} else {
			hostRouter.Use(static.Serve("/", static.LocalFile(tempDirectoryPath, true)))
		}

		faviconPath := tempDirectoryPath + "/favicon.ico"
		if site.SiteType == "hugo" {
			faviconPath = tempDirectoryPath + "/public/favicon.ico"
		}

		hostRouter.GET("/favicon.ico", func(c *gin.Context) {
			c.File(faviconPath)
		})
		hostRouter.NoRoute(func(c *gin.Context) {
			log.Printf("Found no route for [%v] [%v] [%v]", c.ClientIP(), c.Request.Header.Get("User-Agent"), c.Request.URL)
			c.File(tempDirectoryPath + "/index.html")
		})

		hostRouter.Handle("GET", "/statistics", func(c *gin.Context) {
			c.JSON(http.StatusOK, Stats.Data())
		})

		hs.handlerMap[site.Hostname] = hostRouter
		siteMap[subSiteInformation.SubSite.Hostname] = subSiteInformation
		//siteMap[subSiteInformation.SubSite.Path] = subSiteInformation
	}

	cmsConfig.SubSites = siteMap

	return hs, subsiteCacheFolders
}

type StaticFsWithDefaultIndex struct {
	system    http.FileSystem
	pageOn404 string
}

func (spf *StaticFsWithDefaultIndex) Open(name string) (http.File, error) {
	//log.Printf("Service file from static path: %s/%s", spf.subPath, name)

	f, err := spf.system.Open(name)
	if err != nil {
		return spf.system.Open(spf.pageOn404)
	}
	return f, nil
}

var apiPaths = map[string]bool{
	"api":     true,
	"action":  true,
	"meta":    true,
	"stats":   true,
	"feed":    true,
	"asset":   true,
	"jsmodel": true,
}

// Implement the ServerHTTP method on our new type
func (hs HostSwitch) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Debugf("HostSwitch.ServeHTTP RequestUrl: %v", r.URL)
	// Check if a http.Handler is registered for the given host.
	// If yes, use it to handle the request.
	hostName := strings.Split(r.Host, ":")[0]
	pathParts := strings.Split(r.URL.Path, "/")

	if BeginsWithCheck(r.URL.Path, "/.well-known") {
		hs.handlerMap["dashboard"].ServeHTTP(w, r)
		return
	}

	if handler := hs.handlerMap[hostName]; handler != nil && !(len(pathParts) > 1 && apiPaths[pathParts[1]]) {

		ok, abort, modifiedRequest := hs.authMiddleware.AuthCheckMiddlewareWithHttp(r, w, true)
		if ok {
			r = modifiedRequest
		}

		subSite := hs.siteMap[hostName]

		permission := subSite.Permission
		if abort {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+hostName+`"`)
			w.WriteHeader(401)
			w.Write([]byte("unauthorized"))
		} else {
			userI := r.Context().Value("user")
			var user *auth.SessionUser

			if userI != nil {
				user = userI.(*auth.SessionUser)
			} else {
				user = &auth.SessionUser{
					UserReferenceId: daptinid.NullReferenceId,
					Groups:          auth.GroupPermissionList{},
				}
			}

			if permission.CanExecute(user.UserReferenceId, user.Groups, hs.AdministratorGroupId) {
				handler.ServeHTTP(w, r)
			} else {
				w.Header().Set("WWW-Authenticate", `Basic realm="`+hostName+`"`)
				w.WriteHeader(401)
				w.Write([]byte("unauthorised"))
			}
		}
		return
	} else {
		if len(pathParts) > 1 && !apiPaths[pathParts[1]] {

			firstSubFolder := pathParts[1]
			subSite, isSubSite := hs.siteMap[firstSubFolder]
			if isSubSite {

				permission := subSite.Permission
				userI := r.Context().Value("user")
				var user *auth.SessionUser

				if userI != nil {
					user = userI.(*auth.SessionUser)
				} else {
					user = &auth.SessionUser{
						UserReferenceId: daptinid.NullReferenceId,
						Groups:          auth.GroupPermissionList{},
					}
				}
				if permission.CanExecute(user.UserReferenceId, user.Groups, hs.AdministratorGroupId) {
					r.URL.Path = "/" + strings.Join(pathParts[2:], "/")
					handler := hs.handlerMap[subSite.Hostname]
					handler.ServeHTTP(w, r)
				} else {
					w.WriteHeader(403)
					w.Write([]byte("Unauthorized"))
				}
				return
			}
		}

		if !BeginsWithCheck(r.Host, "dashboard.") && !BeginsWithCheck(r.Host, "api.") {
			handler, ok := hs.handlerMap["default"]
			if !ok {
				//log.Errorf("Failed to find default route")
			} else {
				handler.ServeHTTP(w, r)
				return
			}
		}

		//log.Printf("Serving from dashboard")
		handler, ok := hs.handlerMap["dashboard"]
		if !ok {
			log.Errorf("Failed to find dashboard route")
			return
		}

		handler.ServeHTTP(w, r)

		// Handle host names for which no handler is registered
		//http.Error(w, "Forbidden", 403) // Or Redirect?
	}
}

func EndsWithCheck(str string, endsWith string) bool {
	if len(endsWith) > len(str) {
		return false
	}

	if len(endsWith) == len(str) && endsWith != str {
		return false
	}

	suffix := str[len(str)-len(endsWith):]
	i := suffix == endsWith
	return i

}

func BeginsWithCheck(str string, beginsWith string) bool {
	if len(beginsWith) > len(str) {
		return false
	}

	if len(beginsWith) == len(str) && beginsWith != str {
		return false
	}

	prefix := str[:len(beginsWith)]
	i := prefix == beginsWith
	//log.Printf("Check [%v] begins with [%v]: %v", str, beginsWith, i)
	return i

}
