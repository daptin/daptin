package server

import (
	"github.com/artpar/go-guerrilla"
	"github.com/daptin/daptin/server/resource"
	"log"
)

func GetActionPerformers(initConfig *resource.CmsConfig, configStore *resource.ConfigStore, cruds map[string]*resource.DbResource, mailDaemon *guerrilla.Daemon, hostSwitch HostSwitch, certificateManager *resource.CertificateManager) []resource.ActionPerformerInterface {

	performers := make([]resource.ActionPerformerInterface, 0)

	becomeAdminPerformer, err := resource.NewBecomeAdminPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create become admin performer")
	performers = append(performers, becomeAdminPerformer)

	cloudStoreFileImportPerformer, err := resource.NewImportCloudStoreFilesPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create cloudStoreFileImportPerformer")
	performers = append(performers, cloudStoreFileImportPerformer)

	otpGenerateActionPerformer, err := resource.NewOtpGenerateActionPerformer(cruds, configStore)
	resource.CheckErr(err, "Failed to create otp generator")
	performers = append(performers, otpGenerateActionPerformer)

	otpLoginVerifyActionPerformer, err := resource.NewOtpLoginVerifyActionPerformer(cruds, configStore)
	resource.CheckErr(err, "Failed to create otp verify performer")
	performers = append(performers, otpLoginVerifyActionPerformer)

	makeResponsePerformer, err := resource.NewMakeResponsePerformer()
	resource.CheckErr(err, "Failed to create make response performer")
	performers = append(performers, makeResponsePerformer)

	downloadConfigPerformer, err := resource.NewDownloadCmsConfigPerformer(initConfig)
	resource.CheckErr(err, "Failed to create download config performer")
	performers = append(performers, downloadConfigPerformer)

	exportDataPerformer, err := resource.NewExportDataPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create data export performer")
	performers = append(performers, exportDataPerformer)

	exportCsvDataPerformer, err := resource.NewExportCsvDataPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create csv data export performer")
	performers = append(performers, exportCsvDataPerformer)

	importDataPerformer, err := resource.NewImportDataPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create data import performer")
	performers = append(performers, importDataPerformer)

	oauth2redirect, err := resource.NewOauthLoginBeginActionPerformer(initConfig, cruds, configStore)
	resource.CheckErr(err, "Failed to create oauth2 request performer")
	performers = append(performers, oauth2redirect)

	oauth2response, err := resource.NewOauthLoginResponseActionPerformer(initConfig, cruds, configStore)
	resource.CheckErr(err, "Failed to create oauth2 response handler")
	performers = append(performers, oauth2response)

	storeSyncAction, err := resource.NewSyncSiteStorageActionPerformer(cruds)
	resource.CheckErr(err, "Failed to site sync action performer")
	performers = append(performers, storeSyncAction)

	columnStoreSyncAction, err := resource.NewSyncColumnStorageActionPerformer(cruds)
	resource.CheckErr(err, "Failed to create column storage sync performer")
	performers = append(performers, columnStoreSyncAction)

	oauthProfileExchangePerformer, err := resource.NewOuathProfileExchangePerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create oauth2 profile exchange handler")
	performers = append(performers, oauthProfileExchangePerformer)

	generateJwtPerformer, err := resource.NewGenerateJwtTokenPerformer(configStore, cruds)
	resource.CheckErr(err, "Failed to create generate jwt performer")
	performers = append(performers, generateJwtPerformer)

	NewNetworkRequestPerformer, err := resource.NewNetworkRequestPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create generate network request performer")
	performers = append(performers, NewNetworkRequestPerformer)

	randomDataGenerator, err := resource.NewRandomDataGeneratePerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create random data generator")
	performers = append(performers, randomDataGenerator)

	oauth2tokenGenerator, err := resource.NewGenerateOauth2TokenPerformer(configStore, cruds)
	resource.CheckErr(err, "Failed to create oauth2 token generator")
	performers = append(performers, oauth2tokenGenerator)

	//marketplacePackage, err := resource.NewMarketplacePackageInstaller(initConfig, cruds)
	//resource.CheckErr(err, "Failed to create marketplace package install performer")
	//performers = append(performers, marketplacePackage)

	mailServerSync, err := resource.NewMailServersSyncActionPerformer(cruds, mailDaemon, certificateManager)
	resource.CheckErr(err, "Failed to create mail server sync performer")
	performers = append(performers, mailServerSync)

	restartPerformer, err := resource.NewRestarSystemPerformer(initConfig)
	resource.CheckErr(err, "Failed to create restart performer")
	performers = append(performers, restartPerformer)

	xlsUploadPerformer, err := resource.NewUploadFileToEntityPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create xls upload performer")
	performers = append(performers, xlsUploadPerformer)

	csvUploadPerformer, err := resource.NewUploadCsvFileToEntityPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create csv upload performer")
	performers = append(performers, csvUploadPerformer)

	columnDeletePerformer, err := resource.NewDeleteWorldColumnPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create column delete performer")
	performers = append(performers, columnDeletePerformer)

	tableDeletePerformer, err := resource.NewDeleteWorldPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create table delete performer")
	performers = append(performers, tableDeletePerformer)

	columnRenamePerformer, err := resource.NewRenameWorldColumnPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create column rename performer")
	performers = append(performers, columnRenamePerformer)

	enableGraphqlPerformer, err := resource.NewGraphqlEnablePerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create enable graphql performer")
	performers = append(performers, enableGraphqlPerformer)

	fileUploadPerformer, err := resource.NewFileUploadActionPerformer(cruds)
	resource.CheckErr(err, "Failed to create restart performer")
	performers = append(performers, fileUploadPerformer)

	acmeTlsCertificateGenerateActionPerformer, err := resource.NewAcmeTlsCertificateGenerateActionPerformer(cruds, configStore, hostSwitch.handlerMap["api"])
	resource.CheckErr(err, "Failed to create acme tls certificate generator")
	performers = append(performers, acmeTlsCertificateGenerateActionPerformer)

	selfTlsCertificateGenerateActionPerformer, err := resource.NewSelfTlsCertificateGenerateActionPerformer(cruds, configStore, certificateManager)
	resource.CheckErr(err, "Failed to create self tls certificate generator")
	performers = append(performers, selfTlsCertificateGenerateActionPerformer)

	integrationInstallationPerformer, err := resource.NewIntegrationInstallationPerformer(initConfig, cruds, configStore)
	resource.CheckErr(err, "Failed to create integration installation performer")
	performers = append(performers, integrationInstallationPerformer)

	integrations, err := cruds["world"].GetActiveIntegrations()
	if err == nil {

		for _, integration := range integrations {

			performer, err := resource.NewIntegrationActionPerformer(integration, initConfig, cruds, configStore)

			if err != nil {

				log.Printf("Failed to create integration action performer for: %v", integration.Name)
				continue
			}

			performers = append(performers, performer)

		}

	}

	return performers
}
