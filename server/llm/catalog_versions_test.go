package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/llmgateway/catalog"
	"github.com/jmoiron/sqlx"
	sqlite3 "github.com/mattn/go-sqlite3"
)

func catalogVersionFixtures(t *testing.T, database *sqlx.DB, cruds map[string]*resource.DbResource, owner *auth.SessionUser) map[string]map[string]interface{} {
	t.Helper()
	tx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	rows := make(map[string]map[string]interface{})
	rows["credential"] = createLLMAuthorizationResource(t, cruds, owner, tx, "credential", map[string]interface{}{"name": "reconciliation-key", "content": `{"api_key":"test-key"}`})
	rows["llm_provider"] = createLLMAuthorizationResource(t, cruds, owner, tx, "llm_provider", map[string]interface{}{
		"name":                  "reconciliation-provider",
		"provider_type":         "openai-compatible",
		"base_url":              "https://example.test/v1",
		"provider_parameters":   "{}",
		"allow_insecure":        false,
		"allow_private_network": false,
		"enable":                true,
		"credential_id":         daptinid.InterfaceToDIR(rows["credential"]["reference_id"]).String(),
	})
	rows["llm_model"] = createLLMAuthorizationResource(t, cruds, owner, tx, "llm_model", map[string]interface{}{
		"name":                         "reconciliation-model",
		"operations":                   `["chat"]`,
		"capabilities":                 "{}",
		"routing_strategy":             "priority_weighted",
		"fallback_models":              "[]",
		"default_parameters":           "{}",
		"unsupported_parameter_policy": "reject",
		"enable":                       true,
	})
	rows["llm_deployment"] = createLLMAuthorizationResource(t, cruds, owner, tx, "llm_deployment", map[string]interface{}{
		"name":               "reconciliation-deployment",
		"operations":         `["chat"]`,
		"upstream_model":     "upstream-model",
		"priority":           0,
		"weight":             1,
		"request_timeout_ms": 90000,
		"connect_timeout_ms": 10000,
		"max_concurrency":    8,
		"rpm":                -1,
		"tpm":                -1,
		"pricing":            "{}",
		"parameters":         "{}",
		"health_check":       "{}",
		"enable":             true,
		"llm_provider_id":    daptinid.InterfaceToDIR(rows["llm_provider"]["reference_id"]).String(),
		"llm_model_id":       daptinid.InterfaceToDIR(rows["llm_model"]["reference_id"]).String(),
	})
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestUnchangedCatalogReadsOnlyResourceVersions(t *testing.T) {
	database, cruds, _, reference := newCatalogTestResources(t)
	owner := &auth.SessionUser{UserId: 1, UserReferenceId: reference, Groups: auth.GroupPermissionList{{GroupReferenceId: cruds["world"].AdministratorGroupId}}}
	catalogVersionFixtures(t, database, cruds, owner)
	source := &daptinCatalog{cruds: cruds}
	first, err := source.Load(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	observed := map[string]bool{}
	// Restrict SQL reads, rather than mocking Load: payload or secret reads must
	// fail even if their results would produce the same catalog fingerprint.
	connection, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	err = connection.Raw(func(raw interface{}) error {
		raw.(*sqlite3.SQLiteConn).RegisterAuthorizer(func(operation int, table, column, _ string) int {
			if operation != sqlite3.SQLITE_READ {
				return sqlite3.SQLITE_OK
			}
			switch table {
			case "llm_provider", "llm_model", "llm_deployment", "credential":
				observed[table] = true
				if column != "reference_id" && column != "version" && column != "enable" {
					return sqlite3.SQLITE_DENY
				}
			}
			return sqlite3.SQLITE_OK
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	connection.Close()
	defer func() {
		connection, err := database.Conn(context.Background())
		if err != nil {
			t.Error(err)
			return
		}
		defer connection.Close()
		if err := connection.Raw(func(raw interface{}) error { raw.(*sqlite3.SQLiteConn).RegisterAuthorizer(nil); return nil }); err != nil {
			t.Error(err)
		}
	}()
	_, err = source.Load(context.Background(), first.Revision)
	if !errors.Is(err, catalog.ErrStaleRevision) {
		t.Fatalf("metadata-only check: %v", err)
	}
	for _, table := range []string{"llm_provider", "llm_model", "llm_deployment", "credential"} {
		if !observed[table] {
			t.Errorf("did not check %s versions", table)
		}
	}
}

func TestCatalogReconciliationDetectsResourceAndCredentialChanges(t *testing.T) {
	database, cruds, _, reference := newCatalogTestResources(t)
	owner := &auth.SessionUser{UserId: 1, UserReferenceId: reference, Groups: auth.GroupPermissionList{{GroupReferenceId: cruds["world"].AdministratorGroupId}}}
	rows := catalogVersionFixtures(t, database, cruds, owner)
	source := &daptinCatalog{cruds: cruds}
	current, err := source.Load(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	update := func(table string, changes map[string]interface{}) {
		t.Helper()
		tx, err := database.Beginx()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		rows[table] = updateLLMAuthorizationResource(t, cruds, owner, tx, table, rows[table], changes)
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	reload := func() catalog.Document {
		t.Helper()
		next, err := source.Load(context.Background(), current.Revision)
		if err != nil || next.Revision <= current.Revision {
			t.Fatalf("changed catalog not reloaded: revision=%d err=%v", next.Revision, err)
		}
		current = next
		return next
	}
	// An unrelated credential does not invalidate this provider's adapter.
	tx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	replacement := createLLMAuthorizationResource(t, cruds, owner, tx, "credential", map[string]interface{}{"name": "replacement-key", "content": `{"api_key":"replacement"}`})
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := source.Load(context.Background(), current.Revision); !errors.Is(err, catalog.ErrStaleRevision) {
		t.Fatalf("unrelated credential reloaded catalog: %v", err)
	}
	update("credential", map[string]interface{}{"content": `{"api_key":"rotated"}`})
	reload()
	update("llm_provider", map[string]interface{}{"credential_id": daptinid.InterfaceToDIR(replacement["reference_id"]).String()})
	if next := reload(); next.Providers[0].SecretRef != daptinid.InterfaceToDIR(replacement["reference_id"]).String() {
		t.Fatal("provider credential reassignment was missed")
	}
	update("credential", map[string]interface{}{"content": `{"api_key":"old-unused-key"}`})
	if _, err := source.Load(context.Background(), current.Revision); !errors.Is(err, catalog.ErrStaleRevision) {
		t.Fatalf("unused credential reloaded catalog: %v", err)
	}
	rows["credential"] = replacement
	update("credential", map[string]interface{}{"content": `{"api_key":"replacement-rotated"}`})
	reload()
	update("llm_deployment", map[string]interface{}{"enable": false})
	if next := reload(); len(next.Deployments) != 0 {
		t.Fatal("disabled deployment retained")
	}
	update("llm_deployment", map[string]interface{}{"enable": true})
	reload()
	update("llm_model", map[string]interface{}{"name": "renamed-model"})
	if next := reload(); next.Models[0].Name != "renamed-model" {
		t.Fatal("model edit missed")
	}
	// Delete and recreate a deployment: the count is unchanged, and the new row
	// may have a lower version. Its identity must invalidate the saved stamp.
	tx, err = database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	old := daptinid.InterfaceToDIR(rows["llm_deployment"]["reference_id"])
	request := responseResourceRequest(context.WithValue(context.Background(), "user", owner), http.MethodDelete, "/llm_deployment/"+old.String())
	if err := cruds["llm_deployment"].DeleteWithoutFilters(old, request, tx); err != nil {
		t.Fatal(err)
	}
	createLLMAuthorizationResource(t, cruds, owner, tx, "llm_deployment", map[string]interface{}{
		"name":               "recreated-deployment",
		"operations":         `["chat"]`,
		"upstream_model":     "replacement-upstream",
		"priority":           0,
		"weight":             1,
		"request_timeout_ms": 90000,
		"connect_timeout_ms": 10000,
		"max_concurrency":    8,
		"rpm":                -1,
		"tpm":                -1,
		"pricing":            "{}",
		"parameters":         "{}",
		"health_check":       "{}",
		"enable":             true,
		"llm_provider_id":    daptinid.InterfaceToDIR(rows["llm_provider"]["reference_id"]).String(),
		"llm_model_id":       daptinid.InterfaceToDIR(rows["llm_model"]["reference_id"]).String(),
	})
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if next := reload(); len(next.Deployments) != 1 || next.Deployments[0].UpstreamModel != "replacement-upstream" {
		t.Fatal("delete/recreate missed")
	}
}

func TestCatalogVersionProjectionReadsEveryPage(t *testing.T) {
	database, cruds, _, reference := newCatalogTestResources(t)
	owner := &auth.SessionUser{UserId: 1, UserReferenceId: reference, Groups: auth.GroupPermissionList{{GroupReferenceId: cruds["world"].AdministratorGroupId}}}
	tx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	expected := map[string]bool{}
	for index := 0; index < 257; index++ {
		row := createLLMAuthorizationResource(t, cruds, owner, tx, "llm_model", map[string]interface{}{"name": fmt.Sprintf("page-model-%03d", index), "operations": `["chat"]`, "enable": true})
		expected[daptinid.InterfaceToDIR(row["reference_id"]).String()] = true
	}
	var credentials []string
	for index := 0; index < 257; index++ {
		row := createLLMAuthorizationResource(t, cruds, owner, tx, "credential", map[string]interface{}{"name": fmt.Sprintf("page-credential-%03d", index), "content": `{"api_key":"test-key"}`})
		credentials = append(credentials, daptinid.InterfaceToDIR(row["reference_id"]).String())
	}
	credentialVersions, err := readCatalogCredentialVersions(context.Background(), tx, credentials)
	if err != nil || len(credentialVersions) != 257 {
		t.Fatalf("credential batching returned %d versions: %v", len(credentialVersions), err)
	}
	versions, err := readCatalogRecordVersions(context.Background(), tx, "llm_model")
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range versions {
		if !expected[version.Reference] {
			t.Fatalf("unexpected or duplicate reference %s", version.Reference)
		}
		delete(expected, version.Reference)
	}
	if len(expected) != 0 {
		t.Fatalf("pagination missed %d rows", len(expected))
	}
}
