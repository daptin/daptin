package server

import (
	"fmt"
	"github.com/artpar/go.uuid"
	_ "github.com/artpar/rclone/backend/all" // import all fs
	"github.com/artpar/stats"
	"github.com/aviddiviner/gin-limit"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
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
	handlerMap     map[string]*gin.Engine
	siteMap        map[string]resource.SubSite
	authMiddleware *auth.AuthMiddleware
}

type JsonApiError struct {
	Message string
}

func CreateAssetColumnSync(cruds map[string]*resource.DbResource) map[string]map[string]*resource.AssetFolderCache {

	stores, err := cruds["cloud_store"].GetAllCloudStores()
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

				err = cruds["task"].SyncStorageToPath(cloudStore, column.ForeignKeyData.KeyName, tempDirectoryPath)
				if resource.CheckErr(err, "Failed to setup sync to path for table column [%v][%v]", tableName, column.ColumnName) {
					continue
				}

				assetCacheFolder := &resource.AssetFolderCache{
					CloudStore:    cloudStore,
					LocalSyncPath: tempDirectoryPath,
					Keyname:       column.ForeignKeyData.KeyName,
				}

				colCache[columnName] = assetCacheFolder
				log.Printf("Sync table columnd [%v][%v] at %v", tableName, columnName, tempDirectoryPath)

				err = TaskScheduler.AddTask(resource.Task{
					EntityName: "world",
					ActionName: "sync_column_storage",
					Attributes: map[string]interface{}{
						"table_name":  tableResource.TableInfo().TableName,
						"column_name": columnName,
					},
					AsUserEmail: cruds[resource.USER_ACCOUNT_TABLE_NAME].GetAdminEmailId(),
					Schedule:    "@every 30m",
				})

			}

		}

		assetCache[tableName] = colCache

	}

	return assetCache

}

// CreateSubSites creates a router which can route based on hostname to one of the hosted static subsites
func CreateSubSites(cmsConfig *resource.CmsConfig, db database.DatabaseConnection,
	cruds map[string]*resource.DbResource, authMiddleware *auth.AuthMiddleware,
	configStore *resource.ConfigStore) (HostSwitch, map[string]*resource.AssetFolderCache) {

	router := httprouter.New()
	router.ServeFiles("/*filepath", http.Dir("./scripts"))

	hs := HostSwitch{}
	subsiteCacheFolders := make(map[string]*resource.AssetFolderCache)
	hs.handlerMap = make(map[string]*gin.Engine)
	hs.siteMap = make(map[string]resource.SubSite)
	hs.authMiddleware = authMiddleware

	//log.Printf("Cruds before making sub sits: %v", cruds)
	sites, err := cruds["site"].GetAllSites()
	stores, err := cruds["cloud_store"].GetAllCloudStores()
	cloudStoreMap := make(map[int64]resource.CloudStore)

	adminEmailId := cruds[resource.USER_ACCOUNT_TABLE_NAME].GetAdminEmailId()
	log.Printf("Admin email id: %s", adminEmailId)

	for _, store := range stores {
		cloudStoreMap[store.Id] = store
	}

	siteMap := make(map[string]resource.SubSiteInformation)

	if err != nil {
		log.Errorf("Failed to load sites from database: %v", err)
		return hs, subsiteCacheFolders
	}

	max_connections, err := configStore.GetConfigIntValueFor("limit.max_connections", "backend")
	rate_limit, err := configStore.GetConfigIntValueFor("limit.rate", "backend")

	for _, site := range sites {

		if !site.Enable {
			continue
		}

		subSiteInformation := resource.SubSiteInformation{}
		hs.siteMap[site.Path] = site

		for _, hostname := range strings.Split(site.Hostname, ",") {
			hs.siteMap[hostname] = site
		}

		subSiteInformation.SubSite = site

		if site.CloudStoreId == nil {
			log.Infof("Site [%v] does not have a associated storage", site.Name)
			continue
		}

		u, _ := uuid.NewV4()
		sourceDirectoryName := u.String()
		tempDirectoryPath, err := ioutil.TempDir(os.Getenv("DAPTIN_CACHE_FOLDER"), sourceDirectoryName)
		if resource.CheckErr(err, "Failed to create temp directory") {
			continue
		}
		subSiteInformation.SourceRoot = tempDirectoryPath
		cloudStore, ok := cloudStoreMap[*site.CloudStoreId]
		subSiteInformation.CloudStore = cloudStore
		if !ok {
			log.Infof("Site [%v] does not have a associated storage", site.Name)
			continue
		}

		err = cruds["task"].SyncStorageToPath(cloudStore, site.Path, tempDirectoryPath)
		if resource.CheckErr(err, "Failed to setup sync to path for subsite [%v]", site.Name) {
			continue
		}

		syncTask := resource.Task{
			EntityName: "site",
			ActionName: "sync_site_storage",
			Attributes: map[string]interface{}{
				"site_id": site.ReferenceId,
				"path":    tempDirectoryPath,
			},
			AsUserEmail: adminEmailId,
			Schedule:    "@every 1h",
		}

		activeTask := cruds["site"].NewActiveTaskInstance(syncTask)

		func(task *resource.ActiveTaskInstance){
			go func() {
				log.Info("Sleep 5 sec for running new sync task")
				time.Sleep(1 * time.Second)
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
			return c.ClientIP() + strings.Split(c.Request.RequestURI, "?")[0] // limit rate by client ip
		}, func(c *gin.Context) (*rate.Limiter, time.Duration) {
			return rate.NewLimiter(rate.Every(100*time.Millisecond), rate_limit), time.Hour // limit 10 qps/clientIp and permit bursts of at most 10 tokens, and the limiter liveness time duration is 1 hour
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
			log.Printf("Found no route for %v", c.Request.URL)
			log.Printf("Found no route for user agent %v", c.Request.Header.Get("User-Agent"))
			log.Printf("Found no route for ip %v", c.ClientIP())
			c.File(tempDirectoryPath + "/index.html")
			c.AbortWithStatus(200)
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
	//log.Infof("Service file from static path: %s/%s", spf.subPath, name)

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
		} else if ok {
			userI := r.Context().Value("user")
			var user *auth.SessionUser
			if userI != nil {
				user = userI.(*auth.SessionUser)
			} else {
				user = &auth.SessionUser{
					UserReferenceId: "",
					Groups:          []auth.GroupPermission{},
				}
			}

			if permission.CanExecute(user.UserReferenceId, user.Groups) {
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
						UserReferenceId: "",
						Groups:          []auth.GroupPermission{},
					}
				}
				if permission.CanExecute(user.UserReferenceId, user.Groups) {
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

		//log.Infof("Serving from dashboard")
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

//type GrapeSaveRequest struct {
//	Css    string       `json:"gjs-css"`
//	Assets []GrapeAsset `json:"gjs-assets"`
//	Html   string       `json:"gjs-html"`
//}

//func CreateSubSiteSaveContentHandler(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource, db database.DatabaseConnection) func(context *gin.Context) {
//
//	return func(context *gin.Context) {
//
//		//var grapeSaveRequest GrapeSaveRequest
//		s, _ := context.GetRawData()
//		//err := context.Bind(&grapeSaveRequest)
//		//if err != nil {
//		//	log.Errorf("Failed to create html document from html string: %v", err)
//		//}
//		//log.Infof("%s",string(s))
//
//		requestJson := make(map[string]interface{})
//		err := json.Unmarshal(s, &requestJson)
//		if err != nil {
//			context.AbortWithError(403, err)
//			return
//		}
//
//		//queryString := string(s)
//		//query, err := url.ParseQuery(queryString)
//		//if err != nil {
//		//	log.Errorf("Failed to parse query: [%v]", err)
//		//	context.AbortWithStatus(400)
//		//	return
//		//}
//		//action := context.Request.FormValue("action")
//
//		referrer, _ := url.Parse(context.GetHeader("Referer"))
//		subsite, ok := GetSubSiteFromContext(context, initConfig.SubSites)
//		if !ok {
//			log.Errorf("Invalid subsite: %v", context.GetHeader("Referer"))
//			context.AbortWithStatus(400)
//			return
//		}
//
//		path := referrer.Path
//
//		if strings.Index(path, subsite.SubSite.Path) == 1 {
//			path = path[len(subsite.SubSite.Path)+1:]
//		}
//
//		fullpath, ok := GetFilePath(subsite.SourceRoot, path)
//		if !ok {
//			context.AbortWithStatus(404)
//			return
//		}
//
//		//if action == "store" {
//
//		cssString := requestJson["gjs-css"]
//		htmlString := requestJson["gjs-html"]
//
//		htmlDocument, err := goquery.NewDocumentFromReader(strings.NewReader(htmlString.(string)))
//		if err != nil {
//			log.Errorf("Failed to create html document from html string: %v", err)
//			context.AbortWithStatus(400)
//			return
//		}
//
//		if len(cssString.(string)) > 0 {
//			htmlDocument.Find("head").Append(fmt.Sprintf("<style>\n%s\n</style>", cssString))
//		}
//
//		assetsList := make([]GrapeAsset, 0)
//
//		//assets := requestJson["gjs-assets"].(string)
//
//		err = json.Unmarshal([]byte(requestJson["gjs-assets"].(string)), &assetsList)
//		//
//		//for _, asset := range assets {
//		//	assetItem := GrapeAsset{
//		//		Src           : asset["src"].(string),
//		//		Type          : asset["type"].(string),
//		//		UnitDimension  : asset["unitDim"].(string),
//		//		Height         : asset["height"].(int),
//		//		Width          : asset["width"].(int),
//		//	}
//		//	assetsList = append(assetsList, assetItem)
//		//}
//
//		//if len(assets) > 1 {
//		//
//		//	if err != nil {
//		//		log.Errorf("Failed to unmarshal asset list from post body: %v", err)
//		//		context.AbortWithStatus(400)
//		//		return
//		//	}
//		//}
//		for _, asset := range assetsList {
//			switch asset.Type {
//			case "image":
//				//htmlDocument.Find("head").Append("<")
//			case "script":
//				htmlDocument.Find("head").Append(fmt.Sprintf("<script src='%s'></script>", asset.Src))
//			case "style":
//				htmlDocument.Find("head").Append(fmt.Sprintf("<link rel='stylesheet' href='%s'></script>", asset.Src))
//			}
//		}
//
//		htmlString, err = htmlDocument.Html()
//		if err != nil {
//			log.Errorf("Failed to convert to html document: %v", err)
//			context.AbortWithStatus(400)
//			return
//		}
//
//		log.Infof("Writing contents to file: %v", fullpath)
//		err = ioutil.WriteFile(fullpath, []byte(htmlString.(string)), 0644)
//		if !ok {
//			log.Errorf("Invalid subsite: %v", context.GetHeader("Referer"))
//			context.AbortWithStatus(400)
//			return
//		}
//		//
//		//} else if action == "load" {
//		//	keys := strings.Split(context.Request.FormValue("keys"), ",")
//		//	log.Infof("Keys to load", keys)
//		//
//		//	responseMap := make(map[string]interface{})
//		//	for _, key := range keys {
//		//
//		//		switch key {
//		//		case "gjs-html":
//		//			htmlDoc, err := ioutil.ReadFile(fullpath)
//		//			if err != nil {
//		//				context.AbortWithError(403, err)
//		//				return
//		//			}
//		//			responseMap[key] = string(htmlDoc)
//		//
//		//		}
//		//
//		//	}
//		context.AbortWithStatusJSON(200, requestJson)
//		//
//		//}
//
//	}
//
//}

func GetFilePath(sourceRoot string, path string) (string, bool) {
	fullpath := sourceRoot + path

	exists, isDir := exists(fullpath)

	if !exists {
		return "", false
	}
	if isDir {
		if EndsWithCheck(fullpath, "/") {
			fullpath = fullpath + "index.html"
		} else {
			fullpath = fullpath + "/index.html"
		}
	}
	return fullpath, true

}

func exists(path string) (Exists bool, IsDir bool) {
	Exists = false
	IsDir = false
	fi, err := os.Stat(path)
	if err != nil {
		fmt.Println(err)
		return
	}
	switch mode := fi.Mode(); {
	case mode.IsDir():
		// do directory stuff
		Exists = true
		IsDir = true
		return
	case mode.IsRegular():
		// do file stuff
		Exists = true
		return
	}
	return
}

//func CreateSubSiteContentHandler(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource, db database.DatabaseConnection) func(context *gin.Context) {
//
//	siteMap := initConfig.SubSites
//
//	return func(context *gin.Context) {
//
//		keys, _ := context.GetQueryArray("keys[]")
//		path, _ := context.GetQuery("path")
//
//		log.Infof("Keys: %v", keys)
//		log.Infof("Path: %v", path)
//
//		subsite, ok := GetSubSiteFromContext(context, siteMap)
//		if !ok {
//			context.JSON(404, JsonApiError{Message: fmt.Sprintf("Invalid subsite: %v", context.GetHeader("Referer"))})
//			return
//		}
//
//		if path == "/" || path == "" {
//			path = "/index.html"
//		}
//
//		fullpath := subsite.SourceRoot + path
//
//		exists, isDir := exists(fullpath)
//
//		if !exists {
//			context.AbortWithStatus(404)
//			return
//		}
//		if isDir {
//			if EndsWithCheck(fullpath, "/") {
//				fullpath = fullpath + "index.html"
//			} else {
//				fullpath = fullpath + "/index.html"
//			}
//		}
//		fileContents, err := ioutil.ReadFile(fullpath)
//		if err != nil {
//			log.Errorf("Failed to read file: %v", err)
//			context.JSON(500, JsonApiError{Message: fmt.Sprintf("Failed  to read file: %v", err)})
//			return
//		}
//
//		if !EndsWithCheck(fullpath, ".html") {
//			log.Errorf("Not a html file")
//			context.JSON(400, JsonApiError{Message: "Not a html file"})
//			return
//		}
//		cts := string(fileContents)
//		doc, err := goquery.NewDocumentFromReader(strings.NewReader(cts))
//		if err != nil {
//			log.Errorf("Failed to read file as html doc: %v", err)
//			context.JSON(500, JsonApiError{Message: fmt.Sprintf("Failed to read file as html doc: %v", err)})
//			return
//		}
//
//		cssContents := make([]string, 0)
//
//		doc.Find("style").Each(func(i int, s *goquery.Selection) {
//			// For each item found, get the band and title
//			cssContent := s.Text()
//			cssContents = append(cssContents, cssContent)
//		})
//
//		allCss := strings.Join(cssContents, "\n")
//
//		cssPaths := make([]string, 0)
//
//		doc.Find("link").Each(func(i int, s *goquery.Selection) {
//			// For each item found, get the band and title
//			relType := s.AttrOr("rel", "none")
//
//			if relType != "stylesheet" {
//				return
//			}
//
//			srcPath := s.AttrOr("href", "")
//			if len(srcPath) > 0 {
//				cssPaths = append(cssPaths, srcPath)
//			}
//		})
//
//		scriptPaths := make([]string, 0)
//		doc.Find("script").Each(func(i int, s *goquery.Selection) {
//			// For each item found, get the band and title
//
//			txt := s.Text()
//
//			if strings.TrimSpace(txt) != "" {
//				return
//			}
//
//			srcPath := s.AttrOr("src", "")
//			if len(srcPath) > 0 {
//				scriptPaths = append(scriptPaths, srcPath)
//			}
//		})
//
//		imagePaths := make([]string, 0)
//		doc.Find("img").Each(func(i int, s *goquery.Selection) {
//			// For each item found, get the band and title
//
//			txt := s.Text()
//
//			if strings.TrimSpace(txt) != "" {
//				return
//			}
//
//			srcPath := s.AttrOr("src", "")
//			//styleValue := s.AttrOr("style", "")
//			//width := styleValue
//			//height := s.AttrOr("height", "")
//			if len(srcPath) > 0 {
//				imagePaths = append(imagePaths, srcPath)
//			}
//		})
//
//		doc.RemoveFiltered("link")
//		doc.RemoveFiltered("script")
//
//		htmlContent, err := doc.Html()
//		if err != nil {
//			log.Errorf("Failed to convert to html: %v", err)
//			context.JSON(500, JsonApiError{Message: fmt.Sprintf("Failed to convert to html: %v", err)})
//			return
//		}
//
//		respMap := make(map[string]interface{})
//
//		assetsList := make([]GrapeAsset, 0)
//
//		for _, asset := range cssPaths {
//			assetsList = append(assetsList, NewStyleGrapeAsset(asset))
//		}
//
//		for _, asset := range scriptPaths {
//			assetsList = append(assetsList, NewScriptGrapeAsset(asset))
//		}
//
//		respMap["gjs-html"] = htmlContent
//		respMap["gjs-css"] = allCss
//		respMap["gjs-assets"] = assetsList
//
//		context.Header("Content-type", "application/json")
//		context.JSON(200, respMap)
//	}
//}

//func GetSubSiteFromContext(context *gin.Context, siteMap map[string]resource.SubSiteInformation) (resource.SubSiteInformation, bool) {
//	referrer := context.GetHeader("Referer")
//	log.Infof("Referrer: %v", referrer)
//
//	parsed, err := url.Parse(referrer)
//	if err != nil {
//		log.Infof("Failed to parse referrer as url: %v", err)
//	}
//
//	subsite, ok := siteMap[strings.Split(parsed.Host, ":")[0]]
//
//	if !ok {
//		pathParts := strings.Split(parsed.Path, "/")
//		if len(pathParts) > 1 {
//			subSiteName := pathParts[1]
//			subsite, ok = siteMap[subSiteName]
//		}
//	}
//
//	return subsite, ok
//}

//type GrapeAsset struct {
//	Src           string `json:"src"`
//	Type          string `json:"type"`
//	UnitDimension string `json:"unitDim"`
//	Height        int    `json:"height"`
//	Width         int    `json:"width"`
//}
//
//func NewImageGrapeAsset(src string) GrapeAsset {
//	return GrapeAsset{
//		Type: "image",
//		Src:  src,
//	}
//}
//func NewStyleGrapeAsset(src string) GrapeAsset {
//	return GrapeAsset{
//		Type: "style",
//		Src:  src,
//	}
//}
//
//func NewScriptGrapeAsset(src string) GrapeAsset {
//	return GrapeAsset{
//		Type: "script",
//		Src:  src,
//	}
//}

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
