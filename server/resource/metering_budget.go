package resource

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

// MeteringBudget is backend execution context, never an HTTP or action input.
// The controller supplies the immutable run snapshot and its current claim.
type MeteringBudget struct {
	Run, Attempt, Claim string
	Tokens, CostMicros  int64
	Started, Deadline   time.Time
}

type meteringBudgetKey struct{}

func WithMeteringBudget(ctx context.Context, budget MeteringBudget) context.Context {
	return context.WithValue(ctx, meteringBudgetKey{}, budget)
}

func MeteringBudgetFromContext(ctx context.Context) *MeteringBudget {
	budget, ok := ctx.Value(meteringBudgetKey{}).(MeteringBudget)
	if !ok {
		return nil
	}
	return &budget
}

func (m *MeteringService) lockBudget(budget *MeteringBudget, tx *sqlx.Tx) error {
	ref := daptinid.InterfaceToDIR(budget.Run)
	if ref == daptinid.NullReferenceId || budget.Claim == "" || budget.Tokens < 0 || budget.CostMicros < 0 || !budget.Deadline.After(budget.Started) {
		return errors.New("invalid execution budget")
	}
	query, args, err := statementbuilder.Squirrel.Select("status", "claim", "lease_until", "deadline_at").From("agentrun").Prepared(true).
		Where(goqu.Ex{"reference_id": ref[:]}).Limit(1).ForUpdate(goqu.Wait).ToSQL()
	if err != nil {
		return err
	}
	row, err := querySingleMeteringRow("agentrun", tx, query, args...)
	if err != nil {
		return err
	}
	now, err := AgentDatabaseTime(tx)
	if err != nil {
		return err
	}
	if StringOrEmpty(row["status"]) != "running" || StringOrEmpty(row["claim"]) != budget.Claim || !toTime(row["lease_until"]).After(now) || !toTime(row["deadline_at"]).After(now) {
		return errors.New("execution budget is no longer active")
	}
	return nil
}

func budgetBucket(run, metric string) string { return "agentrun:" + run + ":" + metric }

func (m *MeteringService) reserveBudget(budget *MeteringBudget, decision *MeteringDecision, user *auth.SessionUser, tx *sqlx.Tx) error {
	for _, limit := range []struct {
		metric  string
		maximum int64
	}{{"total_tokens", budget.Tokens}, {"cost_micros", budget.CostMicros}} {
		if limit.maximum == 0 {
			continue
		}
		key := budgetBucket(budget.Run, limit.metric)
		quota, err := m.findQuota(key, tx)
		if errors.Is(err, errMeteringRowNotFound) {
			quota = map[string]interface{}{"bucket_key": key, "metric": limit.metric, "maximum": limit.maximum,
				"window_start": budget.Started, "window_end": budget.Deadline, "reserved": int64(0), "consumed": int64(0), "agentrun_id": budget.Run}
		} else if err != nil {
			return err
		}
		reservation, err := m.reserveQuota(quota, key, limit.metric, decision.ReservedMeasures[limit.metric], true, user, tx)
		if err != nil {
			return fmt.Errorf("run budget: %w", err)
		}
		decision.reservation[key] = reservation
	}
	return nil
}

// BudgetProgress reads the same quota rows used for admission; it owns no
// duplicate counters. The run checkpoint exposes this snapshot to resource GETs.
func (m *MeteringService) BudgetProgress(budget MeteringBudget, tx *sqlx.Tx) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	for _, limit := range []struct {
		metric  string
		maximum int64
	}{{"total_tokens", budget.Tokens}, {"cost_micros", budget.CostMicros}} {
		if limit.maximum == 0 {
			continue
		}
		var reserved, consumed int64
		query, args, err := statementbuilder.Squirrel.Select("reserved", "consumed").From("api_quota").Prepared(true).Where(goqu.Ex{"bucket_key": budgetBucket(budget.Run, limit.metric)}).Limit(1).ToSQL()
		if err != nil {
			return nil, err
		}
		quota, err := querySingleMeteringRow("api_quota", tx, query, args...)
		if err == nil {
			reserved, err = ResourceRowInt64(quota["reserved"])
			if err != nil {
				return nil, err
			}
			consumed, err = ResourceRowInt64(quota["consumed"])
		}
		if err != nil && !errors.Is(err, errMeteringRowNotFound) {
			return nil, err
		}
		remaining := limit.maximum - consumed
		if remaining < reserved {
			remaining = 0
		} else {
			remaining -= reserved
		}
		result[limit.metric] = map[string]int64{"maximum": limit.maximum, "consumed": consumed, "reserved": reserved, "remaining": remaining}
	}
	return result, nil
}
