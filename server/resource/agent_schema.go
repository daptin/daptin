package resource

import (
	"fmt"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/table_info"
	"github.com/jmoiron/sqlx"
)

// AgentDatabaseTime uses the same database clock and decoding as scheduled claims.
func AgentDatabaseTime(tx *sqlx.Tx) (time.Time, error) {
	var value interface{}
	if err := tx.QueryRowx("SELECT CURRENT_TIMESTAMP").Scan(&value); err != nil {
		return time.Time{}, err
	}
	instant, err := taskDatabaseTime(value)
	if err != nil {
		return time.Time{}, fmt.Errorf("agent database clock: %w", err)
	}
	return instant, nil
}

func init() {
	col := func(name, kind, datatype string, nullable bool) api2go.ColumnInfo {
		return api2go.ColumnInfo{Name: name, ColumnName: name, ColumnType: kind, DataType: datatype, IsNullable: nullable, IsIndexed: name == "status" || name == "wake_at" || name == "lease_until"}
	}
	StandardTables = append(StandardTables, table_info.TableInfo{
		TableName:         "agent",
		DefaultGroups:     adminsGroup,
		DefaultPermission: auth.UserRead | auth.UserPeek | auth.UserExecute,
		Columns: []api2go.ColumnInfo{
			col("name", "label", "varchar(200)", false),
			col("config", "json", "longtext", false)}},
		table_info.TableInfo{
			TableName:         "agentrun",
			DefaultGroups:     adminQueueGroup,
			DefaultPermission: auth.UserRead | auth.UserPeek | auth.UserExecute,
			Columns: []api2go.ColumnInfo{col("status", "label", "varchar(30)", false),
				col("state", "json", "longtext", false),
				col("auth_version", "measurement", "bigint", false),
				col("claim", "label", "varchar(64)", true),
				col("lease_until", "datetime", "timestamp", true),
				col("wake_at", "datetime", "timestamp", true)}}, table_info.TableInfo{TableName: "agentstp", DefaultGroups: adminQueueGroup, DefaultPermission: auth.UserRead | auth.UserPeek,
			CompositeKeys: [][]string{{"agentrun_id", "number"}},
			Columns: []api2go.ColumnInfo{
				col("number", "measurement", "bigint", false),
				col("kind", "label", "varchar(30)", false),
				col("payload", "json", "longtext", false)}}, table_info.TableInfo{TableName: "agenttry", DefaultGroups: adminQueueGroup, DefaultPermission: auth.UserRead | auth.UserPeek,
			Columns: []api2go.ColumnInfo{
				col("status", "label", "varchar(30)", false),
				col("intent", "json", "longtext", false),
				col("result", "json", "longtext", true)}})
	StandardRelations = append(StandardRelations,
		api2go.NewTableRelation("agentrun", "belongs_to", "agent"),
		api2go.NewTableRelation("agentstp", "belongs_to", "agentrun"),
		api2go.NewTableRelation("agenttry", "belongs_to", "agentrun"),
		api2go.NewTableRelation("agenttry", "belongs_to", "agentstp"))
	// Callers execute controls; only the backend performer writes execution rows.
	for _, control := range []struct {
		name, table string
		optional    bool
		fields      []api2go.ColumnInfo
	}{
		{"start", "agent", false, []api2go.ColumnInfo{col("messages", "json", "text", false)}},
		{"cancel", "agentrun", false, nil},
		{"decide", "agentrun", false, []api2go.ColumnInfo{col("decision", "json", "text", false)}},
		{"reconcile", "agentrun", false, []api2go.ColumnInfo{col("attempt", "alias", "varchar(40)", false), col("result", "json", "text", false)}},
	} {
		attrs := map[string]interface{}{"subject": "~subject"}
		for _, field := range control.fields {
			attrs[field.ColumnName] = "~" + field.ColumnName
		}
		fields := append([]api2go.ColumnInfo{
			col(control.table+"_id", "alias", "varchar(40)", false)}, control.fields...)
		SystemActions = append(SystemActions,
			actionresponse.Action{
				Name:             control.name,
				Label:            control.name + " agent run",
				OnType:           control.table,
				InstanceOptional: control.optional,
				Permission:       &authenticatedOTPActionPermission,
				InFields:         fields,
				OutFields: []actionresponse.Outcome{
					{
						Type:       "agent." + control.name,
						Method:     "EXECUTE",
						Attributes: attrs}}})
	}
}

// Agent execution rows are written only after the controller has authorized a
// start/control or fenced a worker. These adapters preserve the CRUD lifecycle
// without granting callers general create/update access to execution state.
func (dr *DbResource) CreateAgentExecution(model api2go.Api2GoModel, req api2go.Request, tx *sqlx.Tx) (api2go.Responder, error) {
	if dr.tableInfo.TableName != "agentrun" && dr.tableInfo.TableName != "agentstp" && dr.tableInfo.TableName != "agenttry" {
		return nil, fmt.Errorf("not an agent execution resource")
	}
	return dr.createAfterAuthorizationWithTransaction(model, req, tx)
}
func (dr *DbResource) UpdateAgentExecution(model api2go.Api2GoModel, req api2go.Request, tx *sqlx.Tx) (api2go.Responder, error) {
	if dr.tableInfo.TableName != "agentrun" && dr.tableInfo.TableName != "agentstp" && dr.tableInfo.TableName != "agenttry" {
		return nil, fmt.Errorf("not an agent execution resource")
	}
	return dr.updateAfterAuthorizationWithTransaction(model, req, tx)
}
