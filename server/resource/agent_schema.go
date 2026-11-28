package resource

import (
	"context"
	"fmt"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/table_info"
	"github.com/jmoiron/sqlx"
)

// AgentDatabaseTime shares the SQL clock across workers with fractional precision
// for active-time accounting. Scheduled claims supply the common time decoder.
func AgentDatabaseTime(tx *sqlx.Tx) (time.Time, error) {
	var value interface{}
	query := "SELECT CURRENT_TIMESTAMP"
	switch tx.DriverName() {
	case "sqlite3":
		query = "SELECT strftime('%Y-%m-%d %H:%M:%f', 'now')"
	case "mysql":
		query = "SELECT UTC_TIMESTAMP(3)"
	case "postgres":
		query = "SELECT clock_timestamp()"
	}
	if err := tx.QueryRowx(query).Scan(&value); err != nil {
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
		return api2go.ColumnInfo{Name: name, ColumnName: name, ColumnType: kind, DataType: datatype, IsNullable: nullable, IsIndexed: name == "status" || name == "wake_at" || name == "lease_until" || name == "deadline_at"}
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
				col("state", "json", "text", false),
				col("config", "json", "longtext", false),
				col("input", "json", "longtext", false),
				col("auth_version", "measurement", "bigint", false),
				col("deadline_at", "datetime", "timestamp", false),
				col("claim", "label", "varchar(64)", true),
				col("lease_until", "datetime", "timestamp", true),
				col("wake_at", "datetime", "timestamp", true)}}, table_info.TableInfo{TableName: "agentstp", DefaultGroups: adminQueueGroup, DefaultPermission: auth.UserRead | auth.UserPeek,
			CompositeKeys: [][]string{{"agentrun_id", "number"}},
			Columns: []api2go.ColumnInfo{
				col("number", "measurement", "bigint", false),
				col("kind", "label", "varchar(30)", false),
				col("operation", "alias", "varchar(40)", false),
				col("call_id", "label", "text", true)}}, table_info.TableInfo{TableName: "agenttry", CompositeKeys: [][]string{{"agentstp_id", "number"}}, DefaultGroups: adminQueueGroup, DefaultPermission: auth.UserRead | auth.UserPeek,
			Columns: []api2go.ColumnInfo{
				col("status", "label", "varchar(30)", false),
				col("number", "measurement", "bigint", false),
				col("result", "json", "longtext", true)}})
	StandardRelations = append(StandardRelations,
		api2go.NewTableRelation("agentrun", "belongs_to", "agent"),
		api2go.NewTableRelation("agentstp", "belongs_to", "agentrun"),
		api2go.NewTableRelation("agenttry", "belongs_to", "agentrun"),
		api2go.NewTableRelation("agenttry", "belongs_to", "agentstp"),
		api2go.NewTableRelationWithNames("agentrun", "active_run", "has_one", "agenttry", "active_try_id"),
		api2go.NewTableRelationWithNames("agentrun", "current_run", "has_one", "agentstp", "current_step_id"),
		api2go.NewTableRelationWithNames("agentrun", "pending_run", "has_one", "agentstp", "source_step_id"),
		api2go.NewTableRelationWithNames("agentstp", "accepted_step", "has_one", "agenttry", "accepted_try_id"),
		api2go.NewTableRelationWithNames("agentstp", "tool_step", "has_one", "agentstp", "source_step_id"),
		api2go.NewTableRelation("api_quota", "has_one", "agentrun"),
		api2go.NewTableRelation("api_usage", "has_one", "agenttry"))
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

// AgentRunCandidates is the backend queue scan, like the scheduler's due-task
// lookup. It returns identities only; claims and writes use the resource lifecycle.
func (dr *DbResource) AgentRunCandidates(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	if dr.tableInfo.TableName != "agentrun" || limit < 1 || limit > 100 {
		return nil, fmt.Errorf("invalid agent queue scan")
	}
	tx, err := dr.BeginTransaction(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	now, err := AgentDatabaseTime(tx)
	if err != nil {
		return nil, err
	}
	query, args, err := statementbuilder.Squirrel.Select("reference_id").From("agentrun").Prepared(true).
		Where(goqu.Or(
			goqu.And(goqu.Ex{"status": []string{"queued", "running"}}, goqu.Or(goqu.Ex{"wake_at": nil}, goqu.C("wake_at").Lte(now))),
			goqu.And(goqu.Ex{"status": []string{"queued", "running", "waiting_decision", "unknown"}}, goqu.C("deadline_at").Lte(now)),
		)).Where(goqu.Or(goqu.Ex{"lease_until": nil}, goqu.C("lease_until").Lte(now))).
		Order(goqu.C("created_at").Asc()).Limit(uint(limit)).ToSQL()
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []map[string]interface{}
	for rows.Next() {
		var ref daptinid.DaptinReferenceId
		if err := rows.Scan(&ref); err != nil {
			return nil, err
		}
		result = append(result, map[string]interface{}{"reference_id": ref})
	}
	return result, rows.Err()
}
