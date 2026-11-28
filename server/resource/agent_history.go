package resource

import (
	"errors"
	"fmt"

	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

// AgentExecutionRow projects backend execution reads. Large immutable inputs and
// attempt results are requested explicitly, not read by claims or heartbeats.
func (dr *DbResource) AgentExecutionRow(ref daptinid.DaptinReferenceId, columns []string, tx *sqlx.Tx) (map[string]interface{}, error) {
	if dr.tableInfo.TableName != "agentrun" && dr.tableInfo.TableName != "agentstp" && dr.tableInfo.TableName != "agenttry" {
		return nil, errors.New("not an agent execution resource")
	}
	rows, err := GetLimitedOrderedRowsWithTransaction(dr.tableInfo.TableName, columns, "id", tx, 1, goqu.Ex{"reference_id": ref[:]})
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, errors.New("agent execution record not found")
	}
	rows, _, err = dr.ResultToArrayOfMapWithTransaction(rows, dr.model.GetColumnMap(), nil, tx)
	if err != nil {
		return nil, err
	}
	return rows[0], nil
}

// AgentHistoryPage reads an ordered prefix and its selected results in one query.
// The controller authorizes the run before this backend read. Joins require every
// selected attempt and tool source to belong to that run; incomplete data is an
// error during reconstruction rather than an omitted conversation turn.
func (dr *DbResource) AgentHistoryPage(run daptinid.DaptinReferenceId, after, through int, tx *sqlx.Tx) ([]map[string]interface{}, error) {
	if dr.tableInfo.TableName != "agentrun" {
		return nil, errors.New("not an agent run")
	}
	runID, err := GetReferenceIdToIdWithTransaction("agentrun", run, tx)
	if err != nil {
		return nil, err
	}
	query, args, err := statementbuilder.Squirrel.From(goqu.T("agentstp").As("s")).Prepared(true).
		Select(goqu.I("s.reference_id"), goqu.I("s.number"), goqu.I("s.kind"), goqu.I("s.call_id"),
			goqu.COALESCE(goqu.I("a.reference_id"), daptinid.NullReferenceId[:]).As("accepted_reference_id"), goqu.I("a.result"), goqu.I("a.status"),
			goqu.COALESCE(goqu.I("src.reference_id"), daptinid.NullReferenceId[:]).As("source_reference_id")).
		LeftJoin(goqu.T("agenttry").As("a"), goqu.On(goqu.Ex{"a.id": goqu.I("s.accepted_try_id"), "a.agentstp_id": goqu.I("s.id"), "a.agentrun_id": runID})).
		LeftJoin(goqu.T("agentstp").As("src"), goqu.On(goqu.Ex{"src.id": goqu.I("s.source_step_id"), "src.agentrun_id": runID})).
		Where(goqu.Ex{"s.agentrun_id": runID}, goqu.I("s.number").Gt(after), goqu.I("s.number").Lte(through)).
		Order(goqu.I("s.number").Asc()).Limit(100).ToSQL()
	if err != nil {
		return nil, err
	}
	rows, err := tx.Queryx(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result, err := RowsToMap(rows, "agentstp")
	if err != nil {
		return nil, fmt.Errorf("read agent history: %w", err)
	}
	return result, rows.Err()
}
