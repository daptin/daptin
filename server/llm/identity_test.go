package llm

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/llmgateway/catalog"
	"github.com/daptin/llmgateway/contract"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

func TestLLMIdentityUsesPersistedGroupsAndModelExecutePermission(t *testing.T) {
	database, cruds, _, bootstrapReference := newCatalogTestResources(t)
	administrator := &auth.SessionUser{UserId: 1, UserReferenceId: bootstrapReference,
		Groups: auth.GroupPermissionList{{GroupReferenceId: cruds["user_account"].AdministratorGroupId}}}

	userReference := daptinid.DaptinReferenceId(uuid.New())
	groupReference := daptinid.DaptinReferenceId(uuid.New())
	allowedModelReference := daptinid.DaptinReferenceId(uuid.New())
	deniedModelReference := daptinid.DaptinReferenceId(uuid.New())
	transaction, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.Rollback()

	createLLMAuthorizationResource(t, cruds, administrator, transaction, "user_account", map[string]interface{}{
		"name": "LLM Member", "email": "llm-member@example.test", "reference_id": userReference.String(),
	})
	createLLMAuthorizationResource(t, cruds, administrator, transaction, "usergroup", map[string]interface{}{
		"name": "llm-executors-" + uuid.NewString(), "reference_id": groupReference.String(),
	})
	allowedModel := createLLMAuthorizationResource(t, cruds, administrator, transaction, "llm_model", map[string]interface{}{
		"name": "allowed-model-" + uuid.NewString(), "operations": `["chat"]`, "capabilities": `{}`,
		"routing_strategy": "priority_weighted", "fallback_models": `[]`, "default_parameters": `{}`,
		"unsupported_parameter_policy": "reject", "enable": true, "permission": int64(0),
		"reference_id": allowedModelReference.String(),
	})
	deniedModel := createLLMAuthorizationResource(t, cruds, administrator, transaction, "llm_model", map[string]interface{}{
		"name": "denied-model-" + uuid.NewString(), "operations": `["chat"]`, "capabilities": `{}`,
		"routing_strategy": "priority_weighted", "fallback_models": `[]`, "default_parameters": `{}`,
		"unsupported_parameter_policy": "reject", "enable": true, "permission": int64(0),
		"reference_id": deniedModelReference.String(),
	})
	_ = updateLLMAuthorizationResource(t, cruds, administrator, transaction, "llm_model", allowedModel, map[string]interface{}{"permission": int64(0)})
	_ = updateLLMAuthorizationResource(t, cruds, administrator, transaction, "llm_model", deniedModel, map[string]interface{}{"permission": int64(0)})
	userGroup := createLLMAuthorizationResource(t, cruds, administrator, transaction,
		"user_account_user_account_id_has_usergroup_usergroup_id", map[string]interface{}{
			"user_account_id": userReference.String(), "usergroup_id": groupReference.String(),
		})
	modelGroup := createLLMAuthorizationResource(t, cruds, administrator, transaction,
		"llm_model_llm_model_id_has_usergroup_usergroup_id", map[string]interface{}{
			"llm_model_id": allowedModelReference.String(), "usergroup_id": groupReference.String(),
		})
	modelGroup = updateLLMAuthorizationResource(t, cruds, administrator, transaction,
		"llm_model_llm_model_id_has_usergroup_usergroup_id", modelGroup, map[string]interface{}{
			"permission": int64(auth.GroupExecute),
		})
	userID, err := resource.GetReferenceIdToIdWithTransaction(resource.USER_ACCOUNT_TABLE_NAME, userReference, transaction)
	if err != nil {
		t.Fatal(err)
	}
	if err := transaction.Commit(); err != nil {
		t.Fatal(err)
	}
	contaminated := &auth.SessionUser{UserId: userID, UserReferenceId: userReference,
		Groups: auth.GroupPermissionList{{GroupReferenceId: cruds["user_account"].AdministratorGroupId}}}
	principal, err := (daptinIdentityResolver{users: cruds["user_account"]}).Resolve(contaminated)
	if err != nil {
		t.Fatal(err)
	}
	foundExpectedGroup := false
	for _, groupID := range principal.GroupIDs {
		if groupID == contract.ID(groupReference.String()) {
			foundExpectedGroup = true
		}
		if groupID == contract.ID(cruds["user_account"].AdministratorGroupId.String()) {
			t.Fatal("synthetic administrator group survived LLM identity resolution")
		}
	}
	if !foundExpectedGroup {
		t.Fatalf("canonical groups = %#v, missing %s", principal.GroupIDs, groupReference)
	}
	authorizer := daptinAuthorizer{cruds: cruds}
	if err := authorizer.Authorize(context.Background(), principal, catalog.Model{ID: contract.ID(allowedModelReference.String())}); err != nil {
		t.Fatalf("shared group with execute permission was denied: %v", err)
	}
	if err := authorizer.Authorize(context.Background(), principal, catalog.Model{ID: contract.ID(deniedModelReference.String())}); err == nil {
		t.Fatal("synthetic administrator group authorized an unrelated model")
	}

	deleteLLMAuthorizationResource(t, cruds, administrator,
		"user_account_user_account_id_has_usergroup_usergroup_id", userGroup)
	revokedPrincipal, err := (daptinIdentityResolver{users: cruds["user_account"]}).Resolve(contaminated)
	if err != nil {
		t.Fatal(err)
	}
	if err := authorizer.Authorize(context.Background(), revokedPrincipal, catalog.Model{ID: contract.ID(allowedModelReference.String())}); err == nil {
		t.Fatal("revoked persisted group continued to authorize the model")
	}

	restoreTransaction, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	createLLMAuthorizationResource(t, cruds, administrator, restoreTransaction,
		"user_account_user_account_id_has_usergroup_usergroup_id", map[string]interface{}{
			"user_account_id": userReference.String(), "usergroup_id": groupReference.String(),
		})
	if err := restoreTransaction.Commit(); err != nil {
		t.Fatal(err)
	}
	principal, err = (daptinIdentityResolver{users: cruds["user_account"]}).Resolve(contaminated)
	if err != nil {
		t.Fatal(err)
	}

	permissionTransaction, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	_ = updateLLMAuthorizationResource(t, cruds, administrator, permissionTransaction,
		"llm_model_llm_model_id_has_usergroup_usergroup_id", modelGroup, map[string]interface{}{
			"permission": int64(auth.GroupRead),
		})
	if err := permissionTransaction.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := authorizer.Authorize(context.Background(), principal, catalog.Model{ID: contract.ID(allowedModelReference.String())}); err == nil {
		t.Fatal("model relation without group execute permission was accepted")
	}

}

func createLLMAuthorizationResource(t *testing.T, cruds map[string]*resource.DbResource, administrator *auth.SessionUser,
	transaction *sqlx.Tx, tableName string, attributes map[string]interface{}) map[string]interface{} {
	t.Helper()
	requestURL, _ := url.Parse("/" + tableName)
	request := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodPost, URL: requestURL}).
		WithContext(context.WithValue(context.Background(), "user", administrator))}
	row, err := cruds[tableName].CreateWithoutFilter(
		api2go.NewApi2GoModelWithData(tableName, nil, 0, nil, attributes), request, transaction,
	)
	if err != nil {
		t.Fatalf("create %s through canonical resource path: %v", tableName, err)
	}
	return row
}

func updateLLMAuthorizationResource(t *testing.T, cruds map[string]*resource.DbResource, administrator *auth.SessionUser,
	transaction *sqlx.Tx, tableName string, row map[string]interface{}, changes map[string]interface{}) map[string]interface{} {
	t.Helper()
	reference := daptinid.InterfaceToDIR(row["reference_id"])
	requestURL, _ := url.Parse("/" + tableName + "/" + reference.String())
	request := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodPatch, URL: requestURL}).
		WithContext(context.WithValue(context.Background(), "user", administrator))}
	responder, err := cruds[tableName].FindOneWithTransaction(reference, request, transaction)
	if err != nil {
		t.Fatalf("load %s through canonical resource path: %v", tableName, err)
	}
	model, ok := responder.Result().(api2go.Api2GoModel)
	if !ok {
		t.Fatalf("load %s returned %T", tableName, responder.Result())
	}
	model.SetAttributes(changes)
	updated, err := cruds[tableName].UpdateWithoutFilters(model, request, transaction)
	if err != nil {
		t.Fatalf("update %s through canonical resource path: %v", tableName, err)
	}
	return updated
}

func deleteLLMAuthorizationResource(t *testing.T, cruds map[string]*resource.DbResource, administrator *auth.SessionUser,
	tableName string, row map[string]interface{}) {
	t.Helper()
	reference := daptinid.InterfaceToDIR(row["reference_id"])
	requestURL, _ := url.Parse("/" + tableName + "/" + reference.String())
	request := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodDelete, URL: requestURL}).
		WithContext(context.WithValue(context.Background(), "user", administrator))}
	if _, err := cruds[tableName].Delete(reference.String(), request); err != nil {
		t.Fatalf("delete %s through canonical resource path: %v", tableName, err)
	}
}
