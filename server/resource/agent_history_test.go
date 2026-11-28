package resource

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/artpar/api2go/v2"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/google/uuid"
)

func TestAgentHistoryOwnsResultsAndProjectsControlReads(t *testing.T) {
	db, cruds, user := newCanonicalMeteringDatabase(t)
	tx, err := db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	ctx := context.WithValue(context.Background(), "user", user)
	req := func(table, method string) api2go.Request {
		return api2go.Request{PlainRequest: (&http.Request{Method: method, URL: &url.URL{Path: "/" + table}}).WithContext(ctx)}
	}
	create := func(table string, attrs map[string]interface{}) map[string]interface{} {
		t.Helper()
		crud := cruds[table]
		model := api2go.NewApi2GoModelWithData(table, crud.TableInfo().Columns, int64(crud.TableInfo().DefaultPermission), crud.TableInfo().Relations, attrs)
		if table == "agent" {
			row, err := crud.CreateWithoutFilter(model, req(table, "POST"), tx)
			if err != nil {
				t.Fatal(err)
			}
			return row
		}
		response, err := crud.CreateAgentExecution(model, req(table, "POST"), tx)
		if err != nil {
			t.Fatal(err)
		}
		return response.Result().(api2go.Api2GoModel).GetAllAsAttributes()
	}
	update := func(table string, row, attrs map[string]interface{}) {
		t.Helper()
		crud := cruds[table]
		model := api2go.NewApi2GoModelWithData(table, crud.TableInfo().Columns, int64(crud.TableInfo().DefaultPermission), crud.TableInfo().Relations, row)
		model.SetAttributes(attrs)
		if _, err := crud.UpdateAgentExecution(model, req(table, "PATCH"), tx); err != nil {
			t.Fatal(err)
		}
	}
	input := strings.Repeat("input", 1<<20)
	definition := create("agent", map[string]interface{}{"name": "history", "config": "{}"})
	run := create("agentrun", map[string]interface{}{"agent_id": daptinid.InterfaceToDIR(definition["reference_id"]).String(), "status": "running", "state": "{}", "config": "{}", "input": ToJson([]string{input}), "auth_version": 1, "deadline_at": time.Now().Add(time.Hour)})
	runRef := daptinid.InterfaceToDIR(run["reference_id"])
	// A large immutable input must not be selected or rewritten for a checkpoint.
	control, err := cruds["agentrun"].AgentExecutionRow(runRef, []string{"id", "reference_id", "version", "permission", "user_account_id", "state"}, tx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := control["input"]; ok {
		t.Fatal("control lookup loaded immutable input")
	}
	update("agentrun", control, map[string]interface{}{"state": `{"accepted":0}`})
	full, err := cruds["agentrun"].AgentExecutionRow(runRef, []string{"reference_id", "input"}, tx)
	if err != nil || full["input"] != ToJson([]string{input}) {
		t.Fatal("checkpoint update changed input", err)
	}
	large := strings.Repeat("body", 1<<20)
	var firstStep map[string]interface{}
	for number := 1; number <= 101; number++ {
		step := create("agentstp", map[string]interface{}{"agentrun_id": runRef.String(), "number": number, "kind": "model", "operation": uuid.NewString()})
		if number == 1 {
			firstStep = step
		}
		body := `{"ok":true}`
		if number == 1 {
			body = ToJson(map[string]string{"body": large})
		}
		attempt := create("agenttry", map[string]interface{}{"agentrun_id": runRef.String(), "agentstp_id": daptinid.InterfaceToDIR(step["reference_id"]).String(), "number": 1, "status": "completed", "result": body})
		update("agentstp", step, map[string]interface{}{"accepted_try_id": daptinid.InterfaceToDIR(attempt["reference_id"]).String()})
	}
	page, err := cruds["agentrun"].AgentHistoryPage(runRef, 0, 101, tx)
	if err != nil || len(page) != 100 {
		t.Fatalf("first page: %d %v", len(page), err)
	}
	if StringOrEmpty(page[0]["result"]) != ToJson(map[string]string{"body": large}) {
		t.Fatal("large result was not retained")
	}
	if daptinid.InterfaceToDIR(page[0]["source_reference_id"]) != daptinid.NullReferenceId {
		t.Fatal("model step acquired a tool source")
	}
	page, err = cruds["agentrun"].AgentHistoryPage(runRef, 100, 101, tx)
	if err != nil || len(page) != 1 {
		t.Fatalf("second page: %d %v", len(page), err)
	}
	// Missing selections remain visible so reconstruction can fail explicitly.
	create("agentstp", map[string]interface{}{"agentrun_id": runRef.String(), "number": 102, "kind": "model", "operation": uuid.NewString()})
	page, err = cruds["agentrun"].AgentHistoryPage(runRef, 101, 102, tx)
	if err != nil || len(page) != 1 || daptinid.InterfaceToDIR(page[0]["accepted_reference_id"]) != daptinid.NullReferenceId {
		t.Fatalf("missing attempt was hidden: %+v %v", page, err)
	}
	otherRun := create("agentrun", map[string]interface{}{"agent_id": daptinid.InterfaceToDIR(definition["reference_id"]).String(), "status": "running", "state": "{}", "config": "{}", "input": "[]", "auth_version": 1, "deadline_at": time.Now().Add(time.Hour)})
	otherRunRef := daptinid.InterfaceToDIR(otherRun["reference_id"]).String()
	otherStep := create("agentstp", map[string]interface{}{"agentrun_id": otherRunRef, "number": 1, "kind": "model", "operation": uuid.NewString()})
	otherTry := create("agenttry", map[string]interface{}{"agentrun_id": otherRunRef, "agentstp_id": daptinid.InterfaceToDIR(otherStep["reference_id"]).String(), "number": 1, "status": "completed", "result": `{"private":"other run"}`})
	current, _, err := cruds["agentstp"].GetSingleRowByReferenceIdWithTransaction("agentstp", daptinid.InterfaceToDIR(firstStep["reference_id"]), nil, tx)
	if err != nil {
		t.Fatal(err)
	}
	update("agentstp", current, map[string]interface{}{"accepted_try_id": daptinid.InterfaceToDIR(otherTry["reference_id"]).String()})
	page, err = cruds["agentrun"].AgentHistoryPage(runRef, 0, 1, tx)
	if err != nil || len(page) != 1 || page[0]["result"] != nil || daptinid.InterfaceToDIR(page[0]["accepted_reference_id"]) != daptinid.NullReferenceId {
		t.Fatalf("another run's result entered history: %+v %v", page, err)
	}

}
