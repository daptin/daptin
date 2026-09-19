package resource

import (
	"fmt"
	"strings"

	"github.com/artpar/rclone/fs/config"
	"github.com/jmoiron/sqlx"
)

func ConfigureCloudStoreCredential(credentialResource *DbResource, credentialName, rootPath string, required bool, transaction *sqlx.Tx) error {
	if !required {
		return nil
	}
	credentialName = strings.TrimSpace(credentialName)
	if credentialName == "" {
		return fmt.Errorf("credential_name is required for remote cloud storage")
	}

	credential, err := credentialResource.GetCredentialByName(credentialName, transaction)
	if err != nil {
		return fmt.Errorf("cloud store credential [%s] is unavailable: %w", credentialName, err)
	}
	if credential == nil || credential.DataMap == nil {
		return fmt.Errorf("cloud store credential [%s] has no configuration", credentialName)
	}
	credentialType, ok := credential.DataMap["type"].(string)
	if !ok || strings.TrimSpace(credentialType) == "" {
		return fmt.Errorf("cloud store credential [%s] has no rclone type", credentialName)
	}

	configName := strings.SplitN(rootPath, ":", 2)[0]
	if configName == "" {
		return fmt.Errorf("cloud store root_path has no configuration name")
	}
	for key, value := range credential.DataMap {
		config.Data().SetValue(configName, key, fmt.Sprintf("%s", value))
	}
	return nil
}
