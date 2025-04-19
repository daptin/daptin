package action_provider

import (
	"github.com/artpar/go-guerrilla"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/actions"
	"github.com/daptin/daptin/server/hostswitch"
	"github.com/daptin/daptin/server/resource"
	log "github.com/sirupsen/logrus"
)

func GetActionPerformers(initConfig *resource.CmsConfig, configStore *resource.ConfigStore,
	cruds map[string]*resource.DbResource, mailDaemon *guerrilla.Daemon,
	hostSwitch hostswitch.HostSwitch, certificateManager *resource.CertificateManager) []actionresponse.ActionPerformerInterface {
	log.Tracef("GetActionPerformers")
	transaction, err := cruds["world"].Connection().Beginx()
	resource.CheckErr(err, "Failed to begin transaction [14]")
	if err != nil {
		return nil
	}
	defer transaction.Commit()

	performers := make([]actionresponse.ActionPerformerInterface, 0)

	becomeAdminPerformer, err := actions.NewBecomeAdminPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create become admin performer")
	performers = append(performers, becomeAdminPerformer)

	cloudStoreFileImportPerformer, err := actions.NewImportCloudStoreFilesPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create cloudStoreFileImportPerformer")
	performers = append(performers, cloudStoreFileImportPerformer)

	otpGenerateActionPerformer, err := actions.NewOtpGenerateActionPerformer(cruds, configStore, transaction)
	resource.CheckErr(err, "Failed to create otp generator")
	performers = append(performers, otpGenerateActionPerformer)

	otpLoginVerifyActionPerformer, err := actions.NewOtpLoginVerifyActionPerformer(cruds, configStore, transaction)
	resource.CheckErr(err, "Failed to create otp verify performer")
	performers = append(performers, otpLoginVerifyActionPerformer)

	renderTempalteActionPerformer, err := actions.NewRenderTemplateActionPerformer(cruds, configStore, transaction)
	resource.CheckErr(err, "Failed to create render template performer")
	performers = append(performers, renderTempalteActionPerformer)

	makeResponsePerformer, err := actions.NewMakeResponsePerformer()
	resource.CheckErr(err, "Failed to create make response performer")
	performers = append(performers, makeResponsePerformer)

	downloadConfigPerformer, err := actions.NewDownloadCmsConfigPerformer(initConfig)
	resource.CheckErr(err, "Failed to create download config performer")
	performers = append(performers, downloadConfigPerformer)

	exportDataPerformer, err := actions.NewExportDataPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create data export performer")
	performers = append(performers, exportDataPerformer)

	exportCsvDataPerformer, err := actions.NewExportCsvDataPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create csv data export performer")
	performers = append(performers, exportCsvDataPerformer)

	importDataPerformer, err := actions.NewImportDataPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create data import performer")
	performers = append(performers, importDataPerformer)

	oauth2redirect, err := actions.NewOauthLoginBeginActionPerformer(initConfig, cruds, configStore, transaction)
	resource.CheckErr(err, "Failed to create oauth2 request performer")
	performers = append(performers, oauth2redirect)

	oauth2response, err := actions.NewOauthLoginResponseActionPerformer(initConfig, cruds, configStore, transaction)
	resource.CheckErr(err, "Failed to create oauth2 response handler")
	performers = append(performers, oauth2response)

	storeSyncAction, err := actions.NewSyncSiteStorageActionPerformer(cruds)
	resource.CheckErr(err, "Failed to site sync action performer")
	performers = append(performers, storeSyncAction)

	columnStoreSyncAction, err := actions.NewSyncColumnStorageActionPerformer(cruds)
	resource.CheckErr(err, "Failed to create column storage sync performer")
	performers = append(performers, columnStoreSyncAction)

	cloudStoreFileListActionPerformer, err := actions.NewCloudStoreFileListActionPerformer(cruds)
	resource.CheckErr(err, "Failed to create cloudStoreFileListActionPerformer")
	performers = append(performers, cloudStoreFileListActionPerformer)

	cloudStoreFileGetActionPerformer, err := actions.NewCloudStoreFileGetActionPerformer(cruds)
	resource.CheckErr(err, "Failed to create cloudStoreFileGetActionPerformer")
	performers = append(performers, cloudStoreFileGetActionPerformer)

	cloudStoreFileDeleteActionPerformer, err := actions.NewCloudStoreFileDeleteActionPerformer(cruds)
	resource.CheckErr(err, "Failed to create cloudStoreFileDeleteActionPerformer")
	performers = append(performers, cloudStoreFileDeleteActionPerformer)

	oauthProfileExchangePerformer, err := actions.NewOuathProfileExchangePerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create oauth2 profile exchange handler")
	performers = append(performers, oauthProfileExchangePerformer)

	generateJwtPerformer, err := actions.NewGenerateJwtTokenPerformer(configStore, cruds, transaction)
	resource.CheckErr(err, "Failed to create generate jwt performer")
	performers = append(performers, generateJwtPerformer)

	NewNetworkRequestPerformer, err := actions.NewNetworkRequestPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create generate network request performer")
	performers = append(performers, NewNetworkRequestPerformer)

	randomDataGenerator, err := actions.NewRandomDataGeneratePerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create random data generator")
	performers = append(performers, randomDataGenerator)

	oauth2tokenGenerator, err := actions.NewGenerateOauth2TokenPerformer(configStore, cruds)
	resource.CheckErr(err, "Failed to create oauth2 token generator")
	performers = append(performers, oauth2tokenGenerator)

	//marketplacePackage, err := resource.NewMarketplacePackageInstaller(initConfig, cruds)
	//resource.CheckErr(err, "Failed to create marketplace package install performer")
	//performers = append(performers, marketplacePackage)

	mailServerSync, err := actions.NewMailServersSyncActionPerformer(cruds, mailDaemon, certificateManager)
	resource.CheckErr(err, "Failed to create mail server sync performer")
	performers = append(performers, mailServerSync)

	mailSendAction, err := actions.NewMailSendActionPerformer(cruds, mailDaemon, certificateManager)
	resource.CheckErr(err, "Failed to create mail send performer")
	performers = append(performers, mailSendAction)

	awsMailSendActionPerformer, err := actions.NewAwsMailSendActionPerformer(cruds, mailDaemon, configStore, transaction)
	resource.CheckErr(err, "Failed to create mail send performer")
	performers = append(performers, awsMailSendActionPerformer)

	restartPerformer, err := actions.NewRestartSystemPerformer(initConfig)
	resource.CheckErr(err, "Failed to create restart performer")
	performers = append(performers, restartPerformer)

	xlsUploadPerformer, err := actions.NewUploadFileToEntityPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create xls upload performer")
	performers = append(performers, xlsUploadPerformer)

	actionTransactionPerformer, err := actions.NewActionCommitTransactionPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create action Transaction Performer")
	performers = append(performers, actionTransactionPerformer)

	csvUploadPerformer, err := actions.NewUploadCsvFileToEntityPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create csv upload performer")
	performers = append(performers, csvUploadPerformer)

	columnDeletePerformer, err := actions.NewDeleteWorldColumnPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create column delete performer")
	performers = append(performers, columnDeletePerformer)

	tableDeletePerformer, err := actions.NewDeleteWorldPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create table delete performer")
	performers = append(performers, tableDeletePerformer)

	columnRenamePerformer, err := actions.NewRenameWorldColumnPerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create column rename performer")
	performers = append(performers, columnRenamePerformer)

	randomValueGeneratePerformer, err := actions.NewRandomValueGeneratePerformer()
	resource.CheckErr(err, "Failed to create random value generate performer")
	performers = append(performers, randomValueGeneratePerformer)

	enableGraphqlPerformer, err := actions.NewGraphqlEnablePerformer(initConfig, cruds)
	resource.CheckErr(err, "Failed to create enable graphql performer")
	performers = append(performers, enableGraphqlPerformer)

	commandExecutePerformer, err := actions.NewCommandExecuteActionPerformer(cruds)
	resource.CheckErr(err, "Failed to create command execute performer")
	performers = append(performers, commandExecutePerformer)

	fileUploadPerformer, err := actions.NewFileUploadActionPerformer(cruds)
	resource.CheckErr(err, "Failed to create restart performer")
	performers = append(performers, fileUploadPerformer)

	cloudStoreFolderCreateActionPerformer, err := actions.NewCloudStoreFolderCreateActionPerformer(cruds)
	resource.CheckErr(err, "Failed to create cloudStoreFolderCreateActionPerformer")
	performers = append(performers, cloudStoreFolderCreateActionPerformer)

	cloudStorePathMoveActionPerformer, err := actions.NewCloudStorePathMoveActionPerformer(cruds)
	resource.CheckErr(err, "Failed to create cloudStorePathMoveActionPerformer")
	performers = append(performers, cloudStorePathMoveActionPerformer)

	cloudStoreSiteCreateActionPerformer, err := actions.NewCloudStoreSiteCreateActionPerformer(cruds)
	resource.CheckErr(err, "Failed to create cloudStoreSiteCreateActionPerformer")
	performers = append(performers, cloudStoreSiteCreateActionPerformer)

	acmeTlsCertificateGenerateActionPerformer, err := actions.NewAcmeTlsCertificateGenerateActionPerformer(cruds, configStore, hostSwitch.HandlerMap["api"], transaction)
	resource.CheckErr(err, "Failed to create acme tls certificate generator")
	performers = append(performers, acmeTlsCertificateGenerateActionPerformer)

	selfTlsCertificateGenerateActionPerformer, err := actions.NewSelfTlsCertificateGenerateActionPerformer(cruds, configStore, certificateManager, transaction)
	resource.CheckErr(err, "Failed to create self tls certificate generator")
	performers = append(performers, selfTlsCertificateGenerateActionPerformer)

	integrationInstallationPerformer, err := actions.NewIntegrationInstallationPerformer(initConfig, cruds, configStore, transaction)
	resource.CheckErr(err, "Failed to create integration installation performer")
	performers = append(performers, integrationInstallationPerformer)

	integrations, err := cruds["world"].GetActiveIntegrations(transaction)
	if err == nil {

		for _, integration := range integrations {

			performer, err := actions.NewIntegrationActionPerformer(integration, initConfig, cruds, configStore, transaction)

			if err != nil {

				log.Printf("Failed to create integration action performer for: %v", integration.Name)
				continue
			}

			performers = append(performers, performer)

		}

	}
	log.Tracef("Completed GetActionPerformers")

	for _, performer := range performers {
		resource.ActionHandlerMap[performer.Name()] = performer
	}

	return performers
}
