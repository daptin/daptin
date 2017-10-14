package server

import "github.com/daptin/daptin/server/resource"

func GetActionPerformers(initConfig *resource.CmsConfig, configStore *resource.ConfigStore) []resource.ActionPerformerInterface {
	performers := make([]resource.ActionPerformerInterface, 0)

	becomeAdminPerformer, err := resource.NewBecomeAdminPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create become admin performer")
	performers = append(performers, becomeAdminPerformer)

	downloadConfigPerformer, err := resource.NewDownloadCmsConfigPerformer(initConfig)
	resource.CheckErr(err, "Failed to create download config performer")
	performers = append(performers, downloadConfigPerformer)

	exportDataPerformer, err := resource.NewExportDataPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create data export performer")
	performers = append(performers, exportDataPerformer)

	importDataPerformer, err := resource.NewImportDataPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create data import performer")
	performers = append(performers, importDataPerformer)

	oauth2redirect, err := resource.NewOauthLoginBeginActionPerformer(initConfig, cruds, configStore)
	resource.CheckErr(err, "Failed to create oauth2 request performer")
	performers = append(performers, oauth2redirect)

	oauth2response, err := resource.NewOauthLoginResponseActionPerformer(initConfig, cruds, configStore)
	resource.CheckErr(err, "Failed to create oauth2 response handler")
	performers = append(performers, oauth2response)

	generateJwtPerformer, err := resource.NewGenerateJwtTokenPerformer(configStore, cruds)
	resource.CheckErr(err, "Failed to create generate jwt performer")
	performers = append(performers, generateJwtPerformer)

	randomDataGenerator, err := resource.NewRandomDataGeneratePerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create random data generator")
	performers = append(performers, randomDataGenerator)

	marketplacePackage, err := resource.NewMarketplacePackageInstaller(initConfig, cruds)
	resource.CheckErr(err, "Failed to create marketplace package install performer")
	performers = append(performers, marketplacePackage)

	refreshMarketPlaceHandler, err := resource.NewRefreshMarketplacePackagelistPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create marketplace package refresh performer")
	performers = append(performers, refreshMarketPlaceHandler)

	restartPerformer, err := resource.NewRestarSystemPerformer(initConfig)
	resource.CheckErr(err, "Failed to create restart performer")
	performers = append(performers, restartPerformer)

	xlsUploadPerformer, err := resource.NewUploadFileToEntityPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create xls upload performer")
	performers = append(performers, xlsUploadPerformer)

	fileUploadPerformer, err := resource.NewFileUploadActionPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create restart performer")
	performers = append(performers, fileUploadPerformer)

	return performers
}
