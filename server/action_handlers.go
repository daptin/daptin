package server

import "github.com/artpar/goms/server/resource"

func GetActionPerformers(initConfig *resource.CmsConfig, configStore *resource.ConfigStore) []resource.ActionPerformerInterface {
  performers := make([]resource.ActionPerformerInterface, 0)

  becomeAdminPerformer, err := resource.NewBecomeAdminPerformer(initConfig, cruds)
  resource.CheckErr(err, "Failed to create become admin performer")
  performers = append(performers, becomeAdminPerformer)

  downloadConfigPerformer, err := resource.NewDownloadCmsConfigPerformer(initConfig)
  resource.CheckErr(err, "Failed to create download config performer")
  performers = append(performers, downloadConfigPerformer)

  oauth2redirect, err := resource.NewOauthLoginBeginActionPerformer(initConfig, cruds, configStore)
  resource.CheckErr(err, "Failed to create oauth2 request performer")
  performers = append(performers, oauth2redirect)

  oauth2response, err := resource.NewOauthLoginResponseActionPerformer(initConfig, cruds, configStore)
  resource.CheckErr(err, "Failed to create oauth2 response handler")
  performers = append(performers, oauth2response)

  generateJwtPerformer, err := resource.NewGenerateJwtTokenPerformer(configStore, cruds)
  resource.CheckErr(err, "Failed to create generate jwt performer")
  performers = append(performers, generateJwtPerformer)

  restartPerformer, err := resource.NewRestarSystemPerformer(initConfig)
  resource.CheckErr(err, "Failed to create restart performer")
  performers = append(performers, restartPerformer)

  return performers
}
