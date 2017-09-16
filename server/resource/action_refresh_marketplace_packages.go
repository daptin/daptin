package resource

import (
	"fmt"
	//"golang.org/x/oauth2"
)

type RefreshMarketplacePackagelistPerformer struct {
	cruds     map[string]*DbResource
	marketMap map[string]*MarketplaceService
}

func (d *RefreshMarketplacePackagelistPerformer) Name() string {
	return "marketplace.package.refresh"
}

func (d *RefreshMarketplacePackagelistPerformer) DoAction(request ActionRequest, inFieldMap map[string]interface{}) ([]ActionResponse, []error) {

	marketReferenceId := inFieldMap["marketplace_id"].(string)
	marketplaceHandler, ok := d.marketMap[marketReferenceId]

	if !ok {
		return nil, []error{fmt.Errorf("Unknown market")}
	}

	err := marketplaceHandler.RefreshRepository()
	return  []ActionResponse{
		NewActionResponse("client.notify", map[string]interface{}{
			"type":    "success",
			"message": "Initiating system update.",
			"title":   "Success",
		}),
	}, []error{err}
}

func NewRefreshMarketplacePackagelistPerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := RefreshMarketplacePackagelistPerformer{
		cruds:     cruds,
		marketMap: initConfig.MarketplaceHandlers,
	}
	return &handler, nil

}
