package resource

import (
	"sync"

	"github.com/daptin/daptin/server/actionresponse"
)

var ActionHandlerMap = map[string]actionresponse.ActionPerformerInterface{}

var actionHandlerMapLock sync.RWMutex

func RegisterGlobalActionHandler(name string, performer actionresponse.ActionPerformerInterface) {
	actionHandlerMapLock.Lock()
	defer actionHandlerMapLock.Unlock()
	ActionHandlerMap[name] = performer
}

func GetGlobalActionHandler(name string) (actionresponse.ActionPerformerInterface, bool) {
	actionHandlerMapLock.RLock()
	defer actionHandlerMapLock.RUnlock()
	performer, ok := ActionHandlerMap[name]
	return performer, ok
}

func RegisterActionHandlerOnAll(cruds map[string]*DbResource, name string, performer actionresponse.ActionPerformerInterface) {
	actionHandlerMapLock.Lock()
	defer actionHandlerMapLock.Unlock()
	ActionHandlerMap[name] = performer
	for _, crud := range cruds {
		if crud.ActionHandlerMap == nil {
			crud.ActionHandlerMap = make(map[string]actionresponse.ActionPerformerInterface)
		}
		crud.ActionHandlerMap[name] = performer
	}
}

func DeleteActionHandlerOnAll(cruds map[string]*DbResource, name string) {
	actionHandlerMapLock.Lock()
	defer actionHandlerMapLock.Unlock()
	delete(ActionHandlerMap, name)
	for _, crud := range cruds {
		if crud.ActionHandlerMap != nil {
			delete(crud.ActionHandlerMap, name)
		}
	}
}

func GetActionHandler(dbResource *DbResource, name string) (actionresponse.ActionPerformerInterface, bool) {
	actionHandlerMapLock.RLock()
	defer actionHandlerMapLock.RUnlock()
	if dbResource != nil && dbResource.ActionHandlerMap != nil {
		performer, ok := dbResource.ActionHandlerMap[name]
		return performer, ok
	}
	performer, ok := ActionHandlerMap[name]
	return performer, ok
}
