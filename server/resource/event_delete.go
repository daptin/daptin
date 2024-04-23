package resource

import "github.com/buraksezer/olric"

func NewDeleteEventHandler(cruds *map[string]*DbResource, dtopicMap *map[string]*olric.PubSub) DatabaseRequestInterceptor {
	return &eventHandlerMiddleware{
		cruds:     cruds,
		dtopicMap: dtopicMap,
	}
}
