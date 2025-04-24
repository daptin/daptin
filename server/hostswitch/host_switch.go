package hostswitch

import (
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/constants"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/subsite"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strings"
)

type HostSwitch struct {
	HandlerMap           map[string]*gin.Engine
	SiteMap              map[string]subsite.SubSite
	AuthMiddleware       *auth.AuthMiddleware
	AdministratorGroupId daptinid.DaptinReferenceId
}

func (hs HostSwitch) GetHostRouter(name string) *gin.Engine {
	//TODO implement me
	return hs.HandlerMap[name]
}

func (hs HostSwitch) GetAllRouter() []*gin.Engine {
	//TODO implement me
	arr := make([]*gin.Engine, len(hs.HandlerMap))
	i := 0
	for _, h := range hs.HandlerMap {
		arr[i] = h
		i += 1
	}
	return arr
}

func (hs HostSwitch) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Debugf("HostSwitch.ServeHTTP RequestUrl: %v", r.URL)
	// Check if a http.Handler is registered for the given host.
	// If yes, use it to handle the request.
	hostName := strings.Split(r.Host, ":")[0]
	pathParts := strings.Split(r.URL.Path, "/")

	if BeginsWithCheck(r.URL.Path, "/.well-known") {
		hs.HandlerMap["dashboard"].ServeHTTP(w, r)
		return
	}

	if handler := hs.HandlerMap[hostName]; handler != nil && !(len(pathParts) > 1 && constants.WellDefinedApiPaths[pathParts[1]]) {

		ok, abort, modifiedRequest := hs.AuthMiddleware.AuthCheckMiddlewareWithHttp(r, w, true)
		if ok {
			r = modifiedRequest
		}

		subSite := hs.SiteMap[hostName]

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
		if len(pathParts) > 1 && !constants.WellDefinedApiPaths[pathParts[1]] {

			firstSubFolder := pathParts[1]
			subSite, isSubSite := hs.SiteMap[firstSubFolder]
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
					handler := hs.HandlerMap[subSite.Hostname]
					handler.ServeHTTP(w, r)
				} else {
					w.WriteHeader(403)
					w.Write([]byte("Unauthorized"))
				}
				return
			}
		}

		if !BeginsWithCheck(r.Host, "dashboard.") && !BeginsWithCheck(r.Host, "api.") {
			handler, ok := hs.HandlerMap["default"]
			if !ok {
				//log.Errorf("Failed to find default route")
			} else {
				handler.ServeHTTP(w, r)
				return
			}
		}

		//log.Printf("Serving from dashboard")
		handler, ok := hs.HandlerMap["dashboard"]
		if !ok {
			log.Errorf("Failed to find dashboard route")
			return
		}

		handler.ServeHTTP(w, r)

		// Handle host names for which no handler is registered
		//http.Error(w, "Forbidden", 403) // Or Redirect?
	}
}
