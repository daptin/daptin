package resource

import "github.com/buraksezer/olric"

func NewUpdateEventHandler(cruds *map[string]*DbResource, dtopicMap *map[string]*olric.PubSub) DatabaseRequestInterceptor {
	return &eventHandlerMiddleware{
		cruds:     cruds,
		dtopicMap: dtopicMap,
	}
}
