package resource

import "github.com/buraksezer/olric"

func NewCreateEventHandler(cruds *map[string]*DbResource, dtopicMap *map[string]*olric.DTopic) DatabaseRequestInterceptor {

	return &eventHandlerMiddleware{
		cruds:     cruds,
		dtopicMap: dtopicMap,
	}
}
