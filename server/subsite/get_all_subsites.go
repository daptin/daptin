package subsite

import (
	"github.com/daptin/daptin/server/dbresourceinterface"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

func GetAllSites(resourceInterface dbresourceinterface.DbResourceInterface, transaction *sqlx.Tx) ([]SubSite, error) {

	var sites []SubSite

	s, v, err := statementbuilder.Squirrel.Select(
		goqu.I("s.name"), goqu.I("s.hostname"),
		goqu.I("s.cloud_store_id"),
		goqu.I("s."+"user_account_id"), goqu.I("s.path"),
		goqu.I("s.reference_id"), goqu.I("s.id"), goqu.I("s.enable"),
		goqu.I("s.site_type"), goqu.I("s.ftp_enabled")).Prepared(true).
		From(goqu.T("site").As("s")).ToSQL()
	if err != nil {
		return sites, err
	}

	stmt1, err := transaction.Preparex(s)
	if err != nil {
		log.Errorf("[424] failed to prepare statment: %v", err)
		return nil, err
	}

	rows, err := stmt1.Queryx(v...)
	if err != nil {
		return sites, err
	}

	for rows.Next() {
		var site SubSite
		err = rows.StructScan(&site)
		if err != nil {
			log.Errorf("Failed to scan site from db to struct: %v", err)
		}
		sites = append(sites, site)
	}

	err = rows.Close()
	if err != nil {
		log.Error("Failed to close rows after getting all sites", err)
		return nil, err
	}

	err = stmt1.Close()
	if err != nil {
		log.Errorf("failed to close prepared statement: %v", err)
		return nil, err
	}

	for i, site := range sites {
		perm := resourceInterface.GetObjectPermissionByReferenceId("site", site.ReferenceId, transaction)
		site.Permission = perm
		sites[i] = site
	}

	return sites, nil

}
