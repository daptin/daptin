package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/artpar/go.uuid"
	_ "github.com/artpar/rclone/backend/all" // import all fs
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs/config"
	"github.com/artpar/rclone/fs/sync"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"github.com/thoas/stats"
	"golang.org/x/oauth2"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type HostSwitch struct {
	handlerMap     map[string]*gin.Engine
	siteMap        map[string]resource.SubSite
	authMiddleware *auth.AuthMiddleware
}

type JsonApiError struct {
	Message string
}

func CreateSubSites(cmsConfig *resource.CmsConfig, db database.DatabaseConnection, cruds map[string]*resource.DbResource, authMiddleware *auth.AuthMiddleware) HostSwitch {

	router := httprouter.New()
	router.ServeFiles("/*filepath", http.Dir("./scripts"))

	hs := HostSwitch{}
	hs.handlerMap = make(map[string]*gin.Engine)
	hs.siteMap = make(map[string]resource.SubSite)
	hs.authMiddleware = authMiddleware

	sites, err := cruds["site"].GetAllSites()
	stores, err := cruds["cloud_store"].GetAllCloudStores()
	cloudStoreMap := make(map[int64]resource.CloudStore)

	for _, store := range stores {
		cloudStoreMap[store.Id] = store
	}

	siteMap := make(map[string]resource.SubSiteInformation)

	if err != nil {
		log.Errorf("Failed to load sites from database: %v", err)
		return hs
	}

	for _, site := range sites {

		subSiteInformation := resource.SubSiteInformation{}
		hs.siteMap[site.Path] = site
		hs.siteMap[site.Hostname] = site
		//log.Infof("Site to subhost: %v", site)

		subSiteInformation.SubSite = site

		if site.CloudStoreId == nil {
			log.Infof("Site [%v] does not have a associated storage", site.Name)
			continue
		}

		cloudStore, ok := cloudStoreMap[*site.CloudStoreId]
		subSiteInformation.CloudStore = cloudStore
		storeProvider := cloudStore.StoreProvider
		if !ok {
			log.Infof("Site [%v] does not have a associated storage", site.Name)
			continue
		}

		oauthTokenId := cloudStore.OAutoTokenId

		token, err := cruds["oauth_token"].GetTokenByTokenReferenceId(oauthTokenId)
		oauthConf := &oauth2.Config{}
		if err != nil {
			log.Infof("Failed to get oauth token for store sync: %v", err)
		} else {
			oauthConf, err := cruds["oauth_token"].GetOauthDescriptionByTokenReferenceId(oauthTokenId)
			if !token.Valid() {
				ctx := context.Background()
				tokenSource := oauthConf.TokenSource(ctx, token)
				token, err = tokenSource.Token()
				resource.CheckErr(err, "Failed to get new access token")
				if token == nil {
					log.Errorf("We have no token to get the site from storage: %v", cloudStore.ReferenceId)
					continue
				}
				err = cruds["oauth_token"].UpdateAccessTokenByTokenReferenceId(oauthTokenId, token.AccessToken, token.Expiry.Unix())
				resource.CheckErr(err, "failed to update access token")
			}
		}

		u, _ := uuid.NewV4()
		sourceDirectoryName := u.String()
		tempDirectoryPath, err := ioutil.TempDir("", sourceDirectoryName)

		//hostRouter := httprouter.New()
		hostRouter := gin.New()

		subsiteStats := stats.New()

		hostRouter.Use(func() gin.HandlerFunc {
			return func(c *gin.Context) {
				beginning, recorder := subsiteStats.Begin(c.Writer)
				defer Stats.End(beginning, recorder)
				c.Next()
			}
		}())

		subSiteInformation.SourceRoot = tempDirectoryPath

		jsonToken, err := json.Marshal(token)
		resource.CheckErr(err, "Failed to convert token to json")
		config.FileSet(storeProvider, "client_id", oauthConf.ClientID)
		config.FileSet(storeProvider, "type", storeProvider)
		config.FileSet(storeProvider, "client_secret", oauthConf.ClientSecret)
		config.FileSet(storeProvider, "token", string(jsonToken))
		config.FileSet(storeProvider, "client_scopes", strings.Join(oauthConf.Scopes, ","))
		config.FileSet(storeProvider, "redirect_url", oauthConf.RedirectURL)

		args := []string{
			cloudStore.RootPath,
			tempDirectoryPath,
		}

		fsrc, fdst := cmd.NewFsSrcDst(args)
		log.Infof("Temp dir for site [%v] ==> %v", site.Name, tempDirectoryPath)
		go cmd.Run(true, true, nil, func() error {
			if fsrc == nil || fdst == nil {
				log.Errorf("Either source or destination is empty")
				return nil
			}
			log.Infof("Starting to copy drive for site base from [%v] to [%v]", fsrc.String(), fdst.String())
			if fsrc == nil || fdst == nil {
				log.Errorf("Source or destination is null")
				return nil
			}
			dir := sync.CopyDir(fdst, fsrc)
			return dir
		})
		//hostRouter.ServeFiles("/*filepath", http.Dir(tempDirectoryPath))
		hostRouter.Use(authMiddleware.AuthCheckMiddleware)
		hostRouter.Use(static.Serve("/", static.LocalFile(tempDirectoryPath, true)))
		hostRouter.NoRoute(func(c *gin.Context) {
			log.Printf("Found no route for %v", c.Request.URL)
			c.File(tempDirectoryPath + "/index.html")
			c.AbortWithStatus(200)
		})

		hostRouter.Handle("GET", "/statistics", func(c *gin.Context) {
			c.JSON(http.StatusOK, Stats.Data())
		})

		hs.handlerMap[site.Hostname] = hostRouter
		siteMap[subSiteInformation.SubSite.Hostname] = subSiteInformation
		siteMap[subSiteInformation.SubSite.Path] = subSiteInformation
	}

	cmsConfig.SubSites = siteMap

	return hs
}

func NewStaticFsWithDefaultIndex(system http.Dir, pageOn404 string) http.FileSystem {
	return &StaticFsWithDefaultIndex{system: system, pageOn404: pageOn404}
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
	"jsmodel": true,
}

// Implement the ServerHTTP method on our new type
func (hs HostSwitch) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if a http.Handler is registered for the given host.
	// If yes, use it to handle the request.
	hostName := strings.Split(r.Host, ":")[0]
	pathParts := strings.Split(r.URL.Path, "/")
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
			w.Write([]byte("Unauthorised.\n"))
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
				w.Write([]byte("Unauthorised.\n"))
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

type GrapeSaveRequest struct {
	Css    string       `json:"gjs-css"`
	Assets []GrapeAsset `json:"gjs-assets"`
	Html   string       `json:"gjs-html"`
}

func CreateSubSiteSaveContentHandler(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource, db database.DatabaseConnection) func(context *gin.Context) {

	return func(context *gin.Context) {

		//var grapeSaveRequest GrapeSaveRequest
		s, _ := context.GetRawData()
		//err := context.Bind(&grapeSaveRequest)
		//if err != nil {
		//	log.Errorf("Failed to create html document from html string: %v", err)
		//}
		//log.Infof("%s",string(s))

		requestJson := make(map[string]interface{})
		err := json.Unmarshal(s, &requestJson)
		if err != nil {
			context.AbortWithError(403, err)
			return
		}

		//queryString := string(s)
		//query, err := url.ParseQuery(queryString)
		//if err != nil {
		//	log.Errorf("Failed to parse query: [%v]", err)
		//	context.AbortWithStatus(400)
		//	return
		//}
		//action := context.Request.FormValue("action")

		referrer, _ := url.Parse(context.GetHeader("Referer"))
		subsite, ok := GetSubSiteFromContext(context, initConfig.SubSites)
		if !ok {
			log.Errorf("Invalid subsite: %v", context.GetHeader("Referer"))
			context.AbortWithStatus(400)
			return
		}

		path := referrer.Path

		if strings.Index(path, subsite.SubSite.Path) == 1 {
			path = path[len(subsite.SubSite.Path)+1:]
		}

		fullpath, ok := GetFilePath(subsite.SourceRoot, path)
		if !ok {
			context.AbortWithStatus(404)
			return
		}

		//if action == "store" {

		cssString := requestJson["gjs-css"]
		htmlString := requestJson["gjs-html"]

		htmlDocument, err := goquery.NewDocumentFromReader(strings.NewReader(htmlString.(string)))
		if err != nil {
			log.Errorf("Failed to create html document from html string: %v", err)
			context.AbortWithStatus(400)
			return
		}

		if len(cssString.(string)) > 0 {
			htmlDocument.Find("head").Append(fmt.Sprintf("<style>\n%s\n</style>", cssString))
		}

		assetsList := make([]GrapeAsset, 0)

		//assets := requestJson["gjs-assets"].(string)

		err = json.Unmarshal([]byte(requestJson["gjs-assets"].(string)), &assetsList)
		//
		//for _, asset := range assets {
		//	assetItem := GrapeAsset{
		//		Src           : asset["src"].(string),
		//		Type          : asset["type"].(string),
		//		UnitDimension  : asset["unitDim"].(string),
		//		Height         : asset["height"].(int),
		//		Width          : asset["width"].(int),
		//	}
		//	assetsList = append(assetsList, assetItem)
		//}

		//if len(assets) > 1 {
		//
		//	if err != nil {
		//		log.Errorf("Failed to unmarshal asset list from post body: %v", err)
		//		context.AbortWithStatus(400)
		//		return
		//	}
		//}
		for _, asset := range assetsList {
			switch asset.Type {
			case "image":
				//htmlDocument.Find("head").Append("<")
			case "script":
				htmlDocument.Find("head").Append(fmt.Sprintf("<script src='%s'></script>", asset.Src))
			case "style":
				htmlDocument.Find("head").Append(fmt.Sprintf("<link rel='stylesheet' href='%s'></script>", asset.Src))
			}
		}

		htmlString, err = htmlDocument.Html()
		if err != nil {
			log.Errorf("Failed to convert to html document: %v", err)
			context.AbortWithStatus(400)
			return
		}

		log.Infof("Writing contents to file: %v", fullpath)
		err = ioutil.WriteFile(fullpath, []byte(htmlString.(string)), 0644)
		if !ok {
			log.Errorf("Invalid subsite: %v", context.GetHeader("Referer"))
			context.AbortWithStatus(400)
			return
		}
		//
		//} else if action == "load" {
		//	keys := strings.Split(context.Request.FormValue("keys"), ",")
		//	log.Infof("Keys to load", keys)
		//
		//	responseMap := make(map[string]interface{})
		//	for _, key := range keys {
		//
		//		switch key {
		//		case "gjs-html":
		//			htmlDoc, err := ioutil.ReadFile(fullpath)
		//			if err != nil {
		//				context.AbortWithError(403, err)
		//				return
		//			}
		//			responseMap[key] = string(htmlDoc)
		//
		//		}
		//
		//	}
		context.AbortWithStatusJSON(200, requestJson)
		//
		//}

	}

}

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

func CreateSubSiteContentHandler(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource, db database.DatabaseConnection) func(context *gin.Context) {

	siteMap := initConfig.SubSites

	return func(context *gin.Context) {

		keys, _ := context.GetQueryArray("keys[]")
		path, _ := context.GetQuery("path")

		log.Infof("Keys: %v", keys)
		log.Infof("Path: %v", path)

		subsite, ok := GetSubSiteFromContext(context, siteMap)
		if !ok {
			context.JSON(404, JsonApiError{Message: fmt.Sprintf("Invalid subsite: %v", context.GetHeader("Referer"))})
			return
		}

		if path == "/" || path == "" {
			path = "/index.html"
		}

		fullpath := subsite.SourceRoot + path

		exists, isDir := exists(fullpath)

		if !exists {
			context.AbortWithStatus(404)
			return
		}
		if isDir {
			if EndsWithCheck(fullpath, "/") {
				fullpath = fullpath + "index.html"
			} else {
				fullpath = fullpath + "/index.html"
			}
		}
		fileContents, err := ioutil.ReadFile(fullpath)
		if err != nil {
			log.Errorf("Failed to read file: %v", err)
			context.JSON(500, JsonApiError{Message: fmt.Sprintf("Failed  to read file: %v", err)})
			return
		}

		if !EndsWithCheck(fullpath, ".html") {
			log.Errorf("Not a html file")
			context.JSON(400, JsonApiError{Message: "Not a html file"})
			return
		}
		cts := string(fileContents)
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(cts))
		if err != nil {
			log.Errorf("Failed to read file as html doc: %v", err)
			context.JSON(500, JsonApiError{Message: fmt.Sprintf("Failed to read file as html doc: %v", err)})
			return
		}

		cssContents := make([]string, 0)

		doc.Find("style").Each(func(i int, s *goquery.Selection) {
			// For each item found, get the band and title
			cssContent := s.Text()
			cssContents = append(cssContents, cssContent)
		})

		allCss := strings.Join(cssContents, "\n")

		cssPaths := make([]string, 0)

		doc.Find("link").Each(func(i int, s *goquery.Selection) {
			// For each item found, get the band and title
			relType := s.AttrOr("rel", "none")

			if relType != "stylesheet" {
				return
			}

			srcPath := s.AttrOr("href", "")
			if len(srcPath) > 0 {
				cssPaths = append(cssPaths, srcPath)
			}
		})

		scriptPaths := make([]string, 0)
		doc.Find("script").Each(func(i int, s *goquery.Selection) {
			// For each item found, get the band and title

			txt := s.Text()

			if strings.TrimSpace(txt) != "" {
				return
			}

			srcPath := s.AttrOr("src", "")
			if len(srcPath) > 0 {
				scriptPaths = append(scriptPaths, srcPath)
			}
		})

		imagePaths := make([]string, 0)
		doc.Find("img").Each(func(i int, s *goquery.Selection) {
			// For each item found, get the band and title

			txt := s.Text()

			if strings.TrimSpace(txt) != "" {
				return
			}

			srcPath := s.AttrOr("src", "")
			//styleValue := s.AttrOr("style", "")
			//width := styleValue
			//height := s.AttrOr("height", "")
			if len(srcPath) > 0 {
				imagePaths = append(imagePaths, srcPath)
			}
		})

		doc.RemoveFiltered("link")
		doc.RemoveFiltered("script")

		htmlContent, err := doc.Html()
		if err != nil {
			log.Errorf("Failed to convert to html: %v", err)
			context.JSON(500, JsonApiError{Message: fmt.Sprintf("Failed to convert to html: %v", err)})
			return
		}

		respMap := make(map[string]interface{})

		assetsList := make([]GrapeAsset, 0)

		for _, asset := range cssPaths {
			assetsList = append(assetsList, NewStyleGrapeAsset(asset))
		}

		for _, asset := range scriptPaths {
			assetsList = append(assetsList, NewScriptGrapeAsset(asset))
		}

		respMap["gjs-html"] = htmlContent
		respMap["gjs-css"] = allCss
		respMap["gjs-assets"] = assetsList

		context.Header("Content-type", "application/json")
		context.JSON(200, respMap)
	}
}

func GetSubSiteFromContext(context *gin.Context, siteMap map[string]resource.SubSiteInformation) (resource.SubSiteInformation, bool) {
	referrer := context.GetHeader("Referer")
	log.Infof("Referrer: %v", referrer)

	parsed, err := url.Parse(referrer)
	if err != nil {
		log.Infof("Failed to parse referrer as url: %v", err)
	}

	subsite, ok := siteMap[parsed.Host]

	if !ok {
		pathParts := strings.Split(parsed.Path, "/")
		if len(pathParts) > 1 {
			subSiteName := pathParts[1]
			subsite, ok = siteMap[subSiteName]
		}
	}

	return subsite, ok
}

type GrapeAsset struct {
	Src           string `json:"src"`
	Type          string `json:"type"`
	UnitDimension string `json:"unitDim"`
	Height        int    `json:"height"`
	Width         int    `json:"width"`
}

func NewImageGrapeAsset(src string) GrapeAsset {
	return GrapeAsset{
		Type: "image",
		Src:  src,
	}
}
func NewStyleGrapeAsset(src string) GrapeAsset {
	return GrapeAsset{
		Type: "style",
		Src:  src,
	}
}

func NewScriptGrapeAsset(src string) GrapeAsset {
	return GrapeAsset{
		Type: "script",
		Src:  src,
	}
}

func EndsWith(str string, endsWith string) (string, bool) {
	if len(endsWith) > len(str) {
		return "", false
	}

	if len(endsWith) == len(str) && endsWith != str {
		return "", false
	}

	suffix := str[len(str)-len(endsWith):]
	prefix := str[:len(str)-len(endsWith)]

	i := suffix == endsWith
	return prefix, i

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
	return i

}
