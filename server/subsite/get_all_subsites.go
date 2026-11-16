package subsite

import (
	"fmt"
	"strconv"

	"github.com/daptin/daptin/server/dbresourceinterface"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/jmoiron/sqlx"
)

// GetAllSites adapts canonical resource rows to the runtime snapshot shared by
// HTTP dispatch, TLS provisioning and FTP. Relationships retain public IDs.
func GetAllSites(resourceInterface dbresourceinterface.DbResourceInterface, transaction *sqlx.Tx) ([]SubSite, error) {
	rows, err := resourceInterface.GetAllObjects("site", transaction)
	if err != nil {
		return nil, err
	}
	sites := make([]SubSite, 0, len(rows))
	for _, row := range rows {
		site := SubSite{
			Name:         siteString(row["name"]),
			Hostname:     siteString(row["hostname"]),
			Path:         siteString(row["path"]),
			SiteType:     siteString(row["site_type"]),
			ReferenceId:  daptinid.InterfaceToDIR(row["reference_id"]),
			CloudStoreId: daptinid.InterfaceToDIR(row["cloud_store_id"]),
			UserId:       daptinid.InterfaceToDIR(row["user_account_id"]),
		}
		site.Id, err = strconv.ParseInt(fmt.Sprint(row["id"]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("site %q id: %w", site.Name, err)
		}
		site.Enable, err = siteBool(row["enable"])
		if err != nil {
			return nil, fmt.Errorf("site %q enable: %w", site.Name, err)
		}
		site.FtpEnabled, err = siteBool(row["ftp_enabled"])
		if err != nil {
			return nil, fmt.Errorf("site %q ftp_enabled: %w", site.Name, err)
		}
		site.Headers, err = ParseResponseHeaders(row["headers"])
		if err != nil {
			return nil, fmt.Errorf("site %q headers: %w", site.Name, err)
		}
		site.Permission = resourceInterface.GetObjectPermissionByReferenceId("site", site.ReferenceId, transaction)
		sites = append(sites, site)
	}
	return sites, nil
}

func siteString(value interface{}) string {
	text, _ := value.(string)
	return text
}

func siteBool(value interface{}) (bool, error) {
	if value == nil {
		return false, nil
	}
	return strconv.ParseBool(fmt.Sprint(value))
}
