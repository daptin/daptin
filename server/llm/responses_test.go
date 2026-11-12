package llm

import (
	"context"
	"errors"
	"testing"

	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/table_info"
	gateway "github.com/daptin/llmgateway"
	"github.com/daptin/llmgateway/contract"
	"github.com/google/uuid"
)

func TestDaptinStoredResponsesUseResourceOwnershipAndPermissions(t *testing.T) {
	database, cruds, _, bootstrap := newCatalogTestResources(t)
	admin := &auth.SessionUser{UserId: 1, UserReferenceId: bootstrap, Groups: auth.GroupPermissionList{{GroupReferenceId: cruds["world"].AdministratorGroupId}}}
	reference := daptinid.DaptinReferenceId(uuid.New())
	tx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	createLLMAuthorizationResource(t, cruds, admin, tx, "user_account", map[string]interface{}{"reference_id": reference.String(), "email": "stored-owner@example.test", "name": "Stored owner"})
	id, err := resource.GetReferenceIdToIdWithTransaction("user_account", reference, tx)
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	member := &auth.SessionUser{UserId: id, UserReferenceId: reference}
	ctx := context.WithValue(context.Background(), "user", member)
	store := daptinResponses{cruds: cruds}
	cruds["llm_model"].TableInfo().Metering.OnActions["invoke"] = table_info.MeteringConfig{Capture: &table_info.PayloadCaptureConfig{Enabled: true, MaxBytes: gateway.MaxResponseHistoryBytes}}
	if err := store.Authorize(ctx, contract.Principal{}, ""); err != nil {
		t.Fatal(err)
	}
	request := contract.Request{Responses: &contract.ResponsesRequest{Store: true, Input: []contract.ResponseInputItem{{Type: "message", Role: "user", Content: []contract.ContentPart{{Type: "input_text", Text: "remember"}}}}}}
	requestBody, _ := json.Marshal(request)
	service := resource.NewMeteringService(&cruds)
	tx, err = database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	// Capture is deliberately enabled with quota enforcement disabled.
	decision, err := service.Admit(resource.MeteringContext{User: member, RequestBody: requestBody, Metering: &table_info.MeteringConfig{Capture: &table_info.PayloadCaptureConfig{Enabled: true, MaxBytes: gateway.MaxResponseHistoryBytes}}}, tx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	storedID, err := store.Begin(ctx, contract.Principal{}, contract.ReservationToken{Opaque: decision.ReservationToken}, "")
	if err != nil {
		t.Fatal(err)
	}
	record := gateway.StoredResponse{ID: storedID, Input: []contract.ResponseInputItem{{Type: "message", Role: "user", Content: []contract.ContentPart{{Type: "input_text", Text: "remember"}}}}, Response: contract.Response{Model: "model", Responses: &contract.ResponsesResponse{ID: string(storedID), Status: "completed", Store: true, Output: []contract.ResponseOutputItem{{Type: "message", Role: "assistant", ID: "message", Status: "completed", Content: []contract.ContentPart{{Type: "output_text", Text: "remembered"}}}}}}}
	if err = store.Save(ctx, contract.Principal{}, record); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(ctx, contract.Principal{}, storedID)
	if err != nil || len(loaded.Input) != 1 || loaded.Response.Responses.Output[0].Content[0].Text != "remembered" {
		t.Fatalf("saved response=%#v error=%v", loaded, err)
	}
	other := context.WithValue(context.Background(), "user", &auth.SessionUser{UserId: id + 1, UserReferenceId: daptinid.DaptinReferenceId(uuid.New())})
	for _, op := range []func() error{
		func() error { _, err := store.Load(other, contract.Principal{}, storedID); return err },
		func() error { return store.Delete(other, contract.Principal{}, storedID) },
	} {
		err := op()
		var typed *contract.Error
		if !errors.As(err, &typed) || typed.HTTPStatus != 404 {
			t.Fatalf("cross-account access: %v", err)
		}
	}
	tx, err = database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	row, err := cruds["api_usage"].GetReferenceIdToObjectWithTransaction("api_usage", daptinid.InterfaceToDIR(string(storedID)), tx)
	if err != nil {
		t.Fatal(err)
	}
	if daptinid.InterfaceToDIR(row["user_account_id"]) != reference {
		t.Fatalf("wrong owner: %#v", row)
	}
	updateLLMAuthorizationResource(t, cruds, admin, tx, "api_usage", row, map[string]interface{}{"permission": 0})
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Load(ctx, contract.Principal{}, storedID); err == nil {
		t.Fatal("revoked read permission was ignored")
	}
	if _, err = store.Begin(context.Background(), contract.Principal{}, contract.ReservationToken{}, ""); err == nil {
		t.Fatal("guest stored a private response")
	}
}
