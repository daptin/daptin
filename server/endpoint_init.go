package server

import (
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/resource"
)

func InitialiseServerResources(initConfig *resource.CmsConfig, db database.DatabaseConnection) {
	resource.CheckRelations(initConfig)
	resource.CheckAuditTables(initConfig)
	resource.CheckTranslationTables(initConfig)
	//lock := new(sync.Mutex)
	//AddStateMachines(&initConfig, db)

	var errc error

	resource.CheckAllTableStatus(initConfig, db)
	resource.CheckErr(errc, "Failed to commit transaction after creating tables")

	//resource.CreateRelations(initConfig, db)
	//resource.CheckErr(errc, "Failed to commit transaction after creating relations")

	transaction, err := db.Beginx()
	if err != nil {
		resource.CheckErr(err, "Failed to begin transaction [1017]")
		return
	}

	if transaction != nil {
		resource.CreateUniqueConstraints(initConfig, transaction)
		errc = transaction.Commit()
		resource.CheckErr(errc, "Failed to commit transaction after creating unique constrains")
	}

	resource.CreateIndexes(initConfig, db)

	var errb error
	transaction, err = db.Beginx()
	resource.CheckErr(errb, "Failed to begin transaction [1031]")

	if transaction != nil {
		errb = resource.UpdateWorldTable(initConfig, transaction)
		resource.CheckErr(errb, "Failed to update world tables")
		errc := transaction.Commit()
		resource.CheckErr(errc, "Failed to commit transaction after updating world tables")
	}

	transaction, err = db.Beginx()
	if err != nil {
		resource.CheckErr(err, "Failed to begin transaction [1042]")
		return
	}

	resource.UpdateExchanges(initConfig, transaction)
	//go func() {
	resource.UpdateStateMachineDescriptions(initConfig, transaction)
	resource.UpdateStreams(initConfig, transaction)
	//resource.UpdateMarketplaces(initConfig, db)
	err = resource.UpdateTasksData(initConfig, transaction)
	resource.CheckErr(err, "[870] Failed to update cron jobs")
	err = resource.UpdateActionTable(initConfig, transaction)
	resource.CheckErr(err, "Failed to update action table")
	if err == nil {
		transaction.Commit()
	} else {
		transaction.Rollback()
	}
	//}()

}
