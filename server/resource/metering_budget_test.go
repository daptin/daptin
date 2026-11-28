package resource

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/google/uuid"
)

func TestRunBudgetsReserveAndSettleWithoutAccountPlan(t *testing.T) {
	for _, limits := range []struct{ tokens, cost int64 }{{100, 0}, {0, 100}, {100, 100}} {
		t.Run(fmt.Sprint(limits), func(t *testing.T) {
			db, cruds, user := newCanonicalMeteringDatabase(t)
			service := NewMeteringService(&cruds)
			tx, err := db.Beginx()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			now, err := AgentDatabaseTime(tx)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.WithValue(context.Background(), "user", user)
			create := func(table string, attrs map[string]interface{}) string {
				t.Helper()
				crud := cruds[table]
				model := api2go.NewApi2GoModelWithData(table, crud.TableInfo().Columns, int64(crud.TableInfo().DefaultPermission), crud.TableInfo().Relations, attrs)
				req := api2go.Request{PlainRequest: (&http.Request{Method: "POST", URL: &url.URL{Path: "/" + table}}).WithContext(ctx)}
				var response api2go.Responder
				var err error
				if table == "agent" {
					record, createErr := crud.CreateWithoutFilter(model, req, tx)
					if createErr != nil {
						t.Fatal(createErr)
					}
					return fmt.Sprint(record["reference_id"])
				} else {
					response, err = crud.CreateAgentExecution(model, req, tx)
				}
				if err != nil {
					t.Fatal(err)
				}
				created := response.Result().(api2go.Api2GoModel)
				return created.GetID()
			}
			agent := create("agent", map[string]interface{}{"name": "budget-test", "config": "{}"})
			run := create("agentrun", map[string]interface{}{"agent_id": agent, "status": "running", "claim": "worker", "state": "{}", "config": "{}", "input": "[]", "auth_version": 1, "deadline_at": now.Add(time.Hour), "lease_until": now.Add(time.Minute)})
			step := create("agentstp", map[string]interface{}{"agentrun_id": run, "number": 1, "kind": "model", "operation": uuid.NewString()})
			attempt := create("agenttry", map[string]interface{}{"agentrun_id": run, "agentstp_id": step, "status": "dispatched", "number": 1})
			budget := &MeteringBudget{Run: run, Attempt: attempt, Claim: "worker", Tokens: limits.tokens, CostMicros: limits.cost, Started: now, Deadline: now.Add(time.Hour)}
			admit := func(id string, amount int64) (*MeteringDecision, error) {
				return service.Admit(MeteringContext{RequestID: id, User: user, Budget: budget, EstimatedMeasures: map[string]int64{"total_tokens": amount, "cost_micros": amount}}, tx)
			}
			id := uuid.NewString()
			decision, err := admit(id, 70)
			if err != nil || !decision.Enabled {
				t.Fatalf("budget-only admission: %+v %v", decision, err)
			}
			duplicate, err := admit(id, 70)
			if err != nil || duplicate.ReservationToken != decision.ReservationToken {
				t.Fatalf("duplicate: %+v %v", duplicate, err)
			}
			if _, err := admit(uuid.NewString(), 40); err == nil {
				t.Fatal("concurrent exposure exceeded run ceiling")
			}
			if err := service.Complete(MeteringContext{User: user, Measures: map[string]int64{"total_tokens": 50, "cost_micros": 50}}, decision, tx); err != nil {
				t.Fatal(err)
			}
			if err := service.Complete(MeteringContext{User: user}, decision, tx); err != nil {
				t.Fatal(err)
			}
			progress, err := service.BudgetProgress(*budget, tx)
			if err != nil {
				t.Fatal(err)
			}
			for metric, raw := range progress {
				counts := raw.(map[string]int64)
				if counts["consumed"] != 50 || counts["reserved"] != 0 || counts["remaining"] != 50 {
					t.Fatalf("%s: %+v", metric, counts)
				}
			}
			if _, err := admit(uuid.NewString(), 51); err == nil {
				t.Fatal("settled consumption was not enforced")
			}
			budget.Claim = "another-worker"
			if _, err := admit(uuid.NewString(), 1); err == nil {
				t.Fatal("stale claim admitted inference")
			}
		})
	}
}
