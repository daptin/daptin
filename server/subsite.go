package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/artpar/daptin/server/resource"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs"
	_ "github.com/artpar/rclone/fs/all" // import all fs
	"github.com/jmoiron/sqlx"
	"github.com/julienschmidt/httprouter"
	"github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"gopkg.in/gin-gonic/gin.v1"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type HostSwitch struct {
	handlerMap map[string]http.Handler
	siteMap    map[string]resource.SubSite
}

type JsonApiError struct {
	Message string
}

func CreateSubSites(config *resource.CmsConfig, db *sqlx.DB, cruds map[string]*resource.DbResource) HostSwitch {

	router := httprouter.New()
	router.ServeFiles("/*filepath", http.Dir("./scripts"))

	hs := HostSwitch{}
	hs.handlerMap = make(map[string]http.Handler)
	hs.siteMap = make(map[string]resource.SubSite)

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
		log.Infof("Site to subhost: %v", site)

		subSiteInformation.SubSite = site

		cloudStore, ok := cloudStoreMap[site.CloudStoreId]
		subSiteInformation.CloudStore = cloudStore
		storeProvider := cloudStore.StoreProvider
		if !ok {
			log.Infof("Site [%v] does not have a associated storage", site.Name)
			continue
		}

		oauthTokenId := cloudStore.OAutoTokenId

		token, err := cruds["oauth_token"].GetTokenByTokenReferenceId(oauthTokenId)
		oauthConf, err := cruds["oauth_token"].GetOauthDescriptionByTokenReferenceId(oauthTokenId)
		if err != nil {
			log.Errorf("Failed to get oauth token for store sync: %v", err)
			continue
		}

		if !token.Valid() {
			ctx := context.Background()
			tokenSource := oauthConf.TokenSource(ctx, token)
			token, err = tokenSource.Token()
			resource.CheckErr(err, "Failed to get new access token")
			err = cruds["oauth_token"].UpdateAccessTokenByTokenReferenceId(oauthTokenId, token.AccessToken, token.Expiry.Unix())
			resource.CheckErr(err, "failed to update access token")
		}

		sourceDirectoryName := uuid.NewV4().String()
		tempDirectoryPath, err := ioutil.TempDir("", sourceDirectoryName)

		hostRouter := httprouter.New()

		subSiteInformation.SourceRoot = tempDirectoryPath

		jsonToken, err := json.Marshal(token)
		resource.CheckErr(err, "Failed to convert token to json")
		fs.ConfigFileSet(storeProvider, "client_id", oauthConf.ClientID)
		fs.ConfigFileSet(storeProvider, "type", storeProvider)
		fs.ConfigFileSet(storeProvider, "client_secret", oauthConf.ClientSecret)
		fs.ConfigFileSet(storeProvider, "token", string(jsonToken))
		fs.ConfigFileSet(storeProvider, "client_scopes", strings.Join(oauthConf.Scopes, ","))
		fs.ConfigFileSet(storeProvider, "redirect_url", oauthConf.RedirectURL)

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
			dir := fs.CopyDir(fdst, fsrc)
			return dir
		})
		hostRouter.ServeFiles("/*filepath", http.Dir(tempDirectoryPath))

		hs.handlerMap[site.Hostname] = hostRouter
		siteMap[subSiteInformation.SubSite.Hostname] = subSiteInformation
	}

	config.SubSites = siteMap

	return hs
}

// Implement the ServerHTTP method on our new type
func (hs HostSwitch) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if a http.Handler is registered for the given host.
	// If yes, use it to handle the request.
	if handler := hs.handlerMap[r.Host]; handler != nil {
		handler.ServeHTTP(w, r)
	} else {
		pathParts := strings.Split(r.URL.Path, "/")
		if len(pathParts) > 1 {

			firstSubFolder := pathParts[1]
			subSite, isSubSite := hs.siteMap[firstSubFolder]
			if isSubSite {
				r.URL.Path = "/" + strings.Join(pathParts[2:], "/")
				handler := hs.handlerMap[subSite.Hostname]
				handler.ServeHTTP(w, r)
				return
			}
		}

		handler := hs.handlerMap["default"]
		handler.ServeHTTP(w, r)

		// Handle host names for wich no handler is registered
		//http.Error(w, "Forbidden", 403) // Or Redirect?
	}
}

type GrapeSaveRequest struct {
	Css    string       `json:"gjs-css"`
	Assets []GrapeAsset `json:"gjs-assets"`
	Html   string       `json:"gjs-html"`
}

func CreateSubSiteSaveContentHandler(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource, db *sqlx.DB) func(context *gin.Context) {

	return func(context *gin.Context) {

		//var grapeSaveRequest GrapeSaveRequest
		s, _ := context.GetRawData()
		//err := context.Bind(&grapeSaveRequest)
		//if err != nil {
		//	log.Errorf("Failed to create html document from html string: %v", err)
		//}
		//log.Infof("%s",string(s))

		query, err := url.ParseQuery(string(s))
		if err != nil {
			log.Errorf("Failed to parse query: [%v]", err)
			context.AbortWithStatus(400)
			return
		}

		cssString := query.Get("gjs-css")
		htmlString := query.Get("gjs-html")

		htmlDocument, err := goquery.NewDocumentFromReader(strings.NewReader(htmlString))
		if err != nil {
			log.Errorf("Failed to create html document from html string: %v", err)
			context.AbortWithStatus(400)
			return
		}

		if len(cssString) > 0 {

			htmlDocument.Find("head").Append(fmt.Sprintf("<style>\n%s\n</style>", cssString))
			//styleTag, err := goquery.NewDocumentFromReader(strings.NewReader(fmt.Sprintf("<style>\n%s\n</style>", cssString)))
			//if err != nil {
			//	log.Errorf("Failed to add styles to html")
			//}
		}

		assetsList := make([]GrapeAsset, 0)

		err = json.Unmarshal([]byte(query.Get("gjs-assets")), &assetsList)
		if err != nil {
			log.Errorf("Failed to unmarshal asset list from post body: %v", err)
			context.AbortWithStatus(400)
			return
		}

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

		referrer, _ := url.Parse(context.GetHeader("Referer"))
		subsite, ok := GetSubSiteFromContext(context, initConfig.SubSites)
		if !ok {
			log.Errorf("Invalid subsite: %v", context.GetHeader("Referer"))
			context.AbortWithStatus(400)
			return
		}

		path := referrer.Path

		log.Infof("%d assets to be added to %s", len(assetsList), path)
		fullpath, ok := GetFilePath(subsite.SourceRoot, path)
		if !ok {
			context.AbortWithStatus(404)
			return
		}

		//log.Infof("HTml: %v", htmlString)
		log.Infof("Writing contents to file: %v", fullpath)
		err = ioutil.WriteFile(fullpath, []byte(htmlString), 0644)
		if !ok {
			log.Errorf("Invalid subsite: %v", context.GetHeader("Referer"))
			context.AbortWithStatus(400)
			return
		}

		context.AbortWithStatusJSON(200, "ok")

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

func CreateSubSiteContentHandler(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource, db *sqlx.DB) func(context *gin.Context) {

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
		respMap["assets"] = assetsList

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
