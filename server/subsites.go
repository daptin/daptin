package server

import (
	limit "github.com/aviddiviner/gin-limit"
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/cloud_store"
	"github.com/daptin/daptin/server/hostswitch"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/rootpojo"
	"github.com/daptin/daptin/server/subsite"
	"github.com/daptin/daptin/server/task"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	limit2 "github.com/yangxikun/gin-limit-by-key"
	"golang.org/x/time/rate"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

func CreateSubSites(cmsConfig *resource.CmsConfig, transaction *sqlx.Tx,
	cruds map[string]*resource.DbResource, authMiddleware *auth.AuthMiddleware,
	rateConfig RateConfig, max_connections int, olricClient *olric.EmbeddedClient) (hostswitch.HostSwitch, map[daptinid.DaptinReferenceId]*assetcachepojo.AssetFolderCache) {

	hs := hostswitch.HostSwitch{
		AdministratorGroupId: cruds["usergroup"].AdministratorGroupId,
	}
	subsiteCacheFolders := make(map[daptinid.DaptinReferenceId]*assetcachepojo.AssetFolderCache)
	hs.HandlerMap = make(map[string]*gin.Engine)
	hs.SiteMap = make(map[string]subsite.SubSite)
	hs.AuthMiddleware = authMiddleware

	// Initialize the subsite cache with Olric client
	if olricClient != nil {
		err := InitSubsiteCache(olricClient)
		if err != nil {
			log.Errorf("Failed to initialize subsite cache: %v", err)
		} else {
			log.Infof("Subsite cache initialized with Olric")
		}
	} else {
		log.Warnf("Olric client is nil, subsite cache will not be available")
	}

	//log.Printf("Cruds before making sub sits: %v", cruds)
	sites, err := subsite.GetAllSites(cruds["site"], transaction)
	if err != nil {
		log.Printf("Failed to get all sites 117: %v", err)
	}
	stores, err := cloud_store.GetAllCloudStores(cruds["cloud_store"], transaction)
	if err != nil {
		log.Printf("Failed to get all cloudstores 121: %v", err)
	}
	cloudStoreMap := make(map[int64]rootpojo.CloudStore)

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

	rateLimiter := limit2.NewRateLimiter(func(c *gin.Context) string {
		requestPath := c.Request.Host + "/" + strings.Split(c.Request.RequestURI, "?")[0]
		return c.ClientIP() + requestPath // limit rate by client ip
	}, func(c *gin.Context) (*rate.Limiter, time.Duration) {
		requestPath := c.Request.Host + "/" + strings.Split(c.Request.RequestURI, "?")[0]
		limitValue, ok := rateConfig.limits[requestPath]
		if !ok {
			limitValue = 10000
		}

		return rate.NewLimiter(rate.Every(100*time.Millisecond), limitValue), time.Hour // limit 10 qps/clientIp and permit bursts of at most 10 tokens, and the limiter liveness time duration is 1 hour
	}, func(c *gin.Context) {
		c.AbortWithStatus(429) // handle exceed rate limit request
	})
	maxLimiter := limit.MaxAllowed(max_connections)
	middlewares := []gin.HandlerFunc{rateLimiter, maxLimiter, authMiddleware.AuthCheckMiddleware}

	for _, site := range sites {

		if !site.Enable {
			continue
		}

		subSiteInformation := resource.SubSiteInformation{}
		//hs.SiteMap[site.Path] = site

		for _, hostname := range strings.Split(site.Hostname, ",") {
			hs.SiteMap[hostname] = site
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
			log.Warnf("Site [%v] does not have a associated storage", site.Name)
			continue
		}

		err = cruds["task"].SyncStorageToPath(cloudStore, site.Path, tempDirectoryPath, transaction)
		if resource.CheckErr(err, "Failed to setup sync to path for subsite [%v]", site.Name) {
			continue
		}

		syncTask := task.Task{
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
		var credentials map[string]interface{}
		if cloudStore.CredentialName != "" {
			cred, err := cruds["credential"].GetCredentialByName(cloudStore.CredentialName, transaction)
			if err == nil && cred != nil {
				credentials = cred.DataMap
			}
		}

		subsiteAssetCache := &assetcachepojo.AssetFolderCache{
			LocalSyncPath: tempDirectoryPath,
			Keyname:       site.Path,
			CloudStore:    cloudStore,
			Credentials:   credentials,
		}
		subsiteCacheFolders[site.ReferenceId] = subsiteAssetCache

		resource.CheckErr(err, "Failed to register task to sync storage")

		hostRouter := CreateSubsiteEngine(site, subsiteAssetCache, middlewares)

		hs.HandlerMap[site.Hostname] = hostRouter
		siteMap[subSiteInformation.SubSite.Hostname] = subSiteInformation
		//SiteMap[subSiteInformation.SubSite.Path] = subSiteInformation
	}

	cmsConfig.SubSites = siteMap

	return hs, subsiteCacheFolders
}
