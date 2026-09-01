package llm

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/buraksezer/olric"
	olricconfig "github.com/buraksezer/olric/config"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/daptin/daptin/server/table_info"
	gateway "github.com/daptin/llmgateway"
	"github.com/daptin/llmgateway/catalog"
	"github.com/daptin/llmgateway/contract"
	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestDaptinCatalogUsesCanonicalResourcesAndContentFingerprint(t *testing.T) {
	database, cruds, olricClient, userReference := newCatalogTestResources(t)
	credentialReference := daptinid.DaptinReferenceId(uuid.New())
	providerReference := daptinid.DaptinReferenceId(uuid.New())
	modelReference := daptinid.DaptinReferenceId(uuid.New())
	deploymentReference := daptinid.DaptinReferenceId(uuid.New())
	now := time.Date(2026, time.September, 1, 10, 0, 0, 0, time.UTC)

	tx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	encryptedCredential, err := resource.Encrypt([]byte("0123456789abcdef0123456789abcdef"), `{"api_key":"test-key"}`)
	if err != nil {
		t.Fatal(err)
	}
	for tableName, record := range map[string]goqu.Record{
		"credential": {
			"id": 20, "name": "provider-key", "content": encryptedCredential,
			"reference_id": credentialReference[:], "permission": auth.DEFAULT_PERMISSION, "user_account_id": 1,
			"created_at": now, "updated_at": now,
		},
		"llm_provider": {
			"id": 11, "name": "provider", "provider_type": "openai-compatible", "base_url": "https://example.test/v1",
			"provider_parameters": "{}", "allow_insecure": false, "allow_private_network": false, "enable": true, "credential_id": 20,
			"reference_id": providerReference[:], "permission": auth.DEFAULT_PERMISSION, "user_account_id": 1,
			"created_at": now, "updated_at": now,
		},
		"llm_model": {
			"id": 10, "name": "public-model", "operations": `["chat"]`, "capabilities": `{}`, "routing_strategy": "priority_weighted",
			"fallback_models": `[]`, "default_parameters": `{}`, "unsupported_parameter_policy": "reject", "enable": true,
			"reference_id": modelReference[:], "permission": auth.DEFAULT_PERMISSION, "user_account_id": 1,
			"created_at": now, "updated_at": now,
		},
		"llm_deployment": {
			"id": 12, "name": "deployment", "llm_model_id": 10, "llm_provider_id": 11, "upstream_model": "upstream-model",
			"operations": `["chat"]`, "priority": 1, "weight": 2, "request_timeout_ms": 90000,
			"connect_timeout_ms": 10000, "max_concurrency": 8, "rpm": 60, "tpm": 10000,
			"pricing": `{}`, "parameters": `{}`, "health_check": `{}`, "enable": true,
			"reference_id": deploymentReference[:], "permission": auth.DEFAULT_PERMISSION, "user_account_id": 1,
			"created_at": now, "updated_at": now,
		},
	} {
		insert := statementbuilder.Squirrel.Insert(tableName).Prepared(true).Rows(record)
		query, arguments, buildErr := insert.ToSQL()
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		if _, execErr := tx.Exec(query, arguments...); execErr != nil {
			t.Fatalf("insert %s: %v", tableName, execErr)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	source := &daptinCatalog{cruds: cruds}
	document, err := source.Load(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if document.Revision != 1 || len(document.Providers) != 1 || len(document.Models) != 1 || len(document.Deployments) != 1 {
		t.Fatalf("unexpected catalog document: %#v", document)
	}
	if document.Providers[0].SecretRef != credentialReference.String() {
		t.Fatalf("provider secret reference = %q, want %q", document.Providers[0].SecretRef, credentialReference.String())
	}
	deployment := document.Deployments[0]
	if deployment.ModelID != contract.ID(modelReference.String()) || deployment.ProviderID != contract.ID(providerReference.String()) ||
		deployment.Priority != 1 || deployment.Weight != 2 || deployment.MaxConcurrency != 8 {
		t.Fatalf("deployment did not use canonical relation/numeric values: %#v", deployment)
	}
	if _, err := source.Load(context.Background(), document.Revision); !errors.Is(err, catalog.ErrStaleRevision) {
		t.Fatalf("unchanged catalog reload = %v, want stale revision", err)
	}
	for field, value := range map[string]string{
		"pricing":      `{"unknown_rate":1}`,
		"health_check": `{"unknown_probe":true}`,
	} {
		updateCatalogField(t, database, "llm_deployment", field, value)
		if _, err := source.Load(context.Background(), document.Revision); err == nil {
			t.Fatalf("catalog accepted unknown %s field", field)
		}
		updateCatalogField(t, database, "llm_deployment", field, `{}`)
	}

	updateCatalogField(t, database, "llm_model", "fallback_models", `["missing-model"]`)
	if _, err := source.Load(context.Background(), document.Revision); err == nil {
		t.Fatal("catalog accepted an unresolved fallback relation")
	}
	updateCatalogField(t, database, "llm_model", "fallback_models", `[]`)
	updateCatalogField(t, database, "llm_deployment", "weight", 3)
	reloaded, err := source.Load(context.Background(), document.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Revision != 2 || reloaded.Deployments[0].Weight != 3 {
		t.Fatalf("catalog reload = revision %d deployment %#v", reloaded.Revision, reloaded.Deployments[0])
	}

	hostA, err := NewGateway(context.Background(), cruds, olricClient)
	if err != nil {
		t.Fatal(err)
	}
	hostB, err := NewGateway(context.Background(), cruds, olricClient)
	if err != nil {
		t.Fatal(err)
	}
	waitForCatalogSubscribers(t, cruds["world"].PubSub, 2)
	initialStatus := hostA.Status()
	if initialStatus.Revision != 1 || !initialStatus.Ready || initialStatus.Degraded {
		t.Fatalf("initial gateway status = %#v", initialStatus)
	}
	if secondStatus := hostB.Status(); secondStatus.Revision != initialStatus.Revision || secondStatus.Ready != initialStatus.Ready ||
		secondStatus.Draining != initialStatus.Draining || secondStatus.Degraded != initialStatus.Degraded ||
		secondStatus.RejectedRevision != initialStatus.RejectedRevision || secondStatus.ReloadStage != initialStatus.ReloadStage {
		t.Fatalf("gateway hosts started with different snapshot state: first %#v, second %#v", initialStatus, secondStatus)
	}

	updateCatalogField(t, database, "llm_deployment", "weight", 0)
	publishCatalogEvent(t, cruds["world"].PubSub, "llm_deployment")
	for index, host := range []*Gateway{hostA, hostB} {
		rejectedStatus := waitForGatewayStatus(t, host, func(status gateway.Status) bool {
			return status.RejectedRevision > status.Revision
		})
		if rejectedStatus.Revision != initialStatus.Revision || !rejectedStatus.Ready || !rejectedStatus.Degraded ||
			rejectedStatus.ReloadStage != "validate" {
			t.Fatalf("gateway host %d replaced or disabled its active snapshot after a rejected catalog: %#v", index+1, rejectedStatus)
		}
	}

	updateCatalogField(t, database, "llm_deployment", "weight", 4)
	publishCatalogEvent(t, cruds["world"].PubSub, "llm_deployment")
	var recoveredRevision uint64
	for index, host := range []*Gateway{hostA, hostB} {
		recoveredStatus := waitForGatewayStatus(t, host, func(status gateway.Status) bool {
			return status.Revision > initialStatus.Revision && status.RejectedRevision == 0
		})
		if !recoveredStatus.Ready || recoveredStatus.Degraded || recoveredStatus.ReloadStage != "" {
			t.Fatalf("gateway host %d did not recover after a valid catalog event: %#v", index+1, recoveredStatus)
		}
		if recoveredRevision == 0 {
			recoveredRevision = recoveredStatus.Revision
		} else if recoveredStatus.Revision != recoveredRevision {
			t.Fatalf("gateway hosts recovered to different revisions: first %d, second %d", recoveredRevision, recoveredStatus.Revision)
		}
	}
	rotatedCredential, err := resource.Encrypt([]byte("0123456789abcdef0123456789abcdef"), `{"api_key":"rotated-key"}`)
	if err != nil {
		t.Fatal(err)
	}
	updateCatalogField(t, database, "credential", "content", rotatedCredential)
	updateCatalogField(t, database, "credential", "version", 2)
	publishCatalogEvent(t, cruds["world"].PubSub, "credential")
	for index, host := range []*Gateway{hostA, hostB} {
		rotatedStatus := waitForGatewayStatus(t, host, func(status gateway.Status) bool {
			return status.Revision > recoveredRevision
		})
		if !rotatedStatus.Ready || rotatedStatus.Degraded {
			t.Fatalf("credential rotation did not rebuild a ready snapshot on gateway host %d: %#v", index+1, rotatedStatus)
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "Bearer daptin-session-token")
	request = request.WithContext(context.WithValue(request.Context(), "user", &auth.SessionUser{
		UserId: 1, UserReferenceId: userReference,
	}))
	response := httptest.NewRecorder()
	hostA.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("model listing status = %d, body = %s", response.Code, response.Body.String())
	}
	readyRequest := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	readyResponse := httptest.NewRecorder()
	hostA.Handler().ServeHTTP(readyResponse, readyRequest)
	if readyResponse.Code != http.StatusOK {
		t.Fatalf("ready status = %d, body = %s", readyResponse.Code, readyResponse.Body.String())
	}
	upstreamCancelled := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "text/event-stream")
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte("data: {\"id\":\"chatcmpl-cancel\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"upstream-model\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"partial\"},\"finish_reason\":null}]}\n\n"))
		response.(http.Flusher).Flush()
		<-request.Context().Done()
		upstreamCancelled <- struct{}{}
	}))
	defer upstream.Close()
	streamRevision := hostA.Status().Revision
	updateCatalogField(t, database, "llm_provider", "base_url", upstream.URL+"/v1")
	updateCatalogField(t, database, "llm_provider", "allow_insecure", true)
	updateCatalogField(t, database, "llm_provider", "allow_private_network", true)
	publishCatalogEvent(t, cruds["world"].PubSub, "llm_provider")
	waitForGatewayStatus(t, hostA, func(status gateway.Status) bool { return status.Revision > streamRevision && status.Ready })
	legacyCacheKey := "itr-user_account-1"
	_, _ = resource.OlricCache.Delete(context.Background(), legacyCacheKey)
	if err := resource.OlricCache.Put(context.Background(), legacyCacheKey, userReference, olric.EX(time.Minute)); err != nil {
		t.Fatalf("seed pre-fix reference cache value: %v", err)
	}
	authorizationTransaction, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	ownerReference, err := cruds["llm_model"].GetIdToReferenceId("user_account", 1, authorizationTransaction)
	if err != nil || ownerReference != userReference {
		_ = authorizationTransaction.Rollback()
		t.Fatalf("canonical model owner lookup = %s, %v; want %s", ownerReference.String(), err, userReference.String())
	}
	modelPermission := cruds["llm_model"].GetObjectPermissionByReferenceId("llm_model", modelReference, authorizationTransaction)
	_ = authorizationTransaction.Rollback()
	if !modelPermission.CanRead(userReference, nil, cruds["llm_model"].AdministratorGroupId) {
		t.Fatalf("canonical model owner cannot read model: permission=%#v user=%s", modelPermission, userReference.String())
	}

	httpGateway := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		request = request.WithContext(context.WithValue(request.Context(), "user", &auth.SessionUser{
			UserId: 1, UserReferenceId: userReference,
		}))
		hostA.Handler().ServeHTTP(response, request)
	}))
	defer httpGateway.Close()
	streamContext, cancelStream := context.WithCancel(context.Background())
	streamRequest, err := http.NewRequestWithContext(streamContext, http.MethodPost, httpGateway.URL+"/v1/chat/completions",
		strings.NewReader(`{"model":"public-model","messages":[{"role":"user","content":"hello"}],"stream":true}`))
	if err != nil {
		t.Fatal(err)
	}
	streamRequest.Header.Set("Authorization", "Bearer daptin-session-token")
	streamRequest.Header.Set("Content-Type", "application/json")
	streamRequest.Header.Set("X-Request-ID", "host-stream-cancel")
	streamResponse, err := http.DefaultClient.Do(streamRequest)
	if err != nil {
		t.Fatal(err)
	}
	if streamResponse.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(streamResponse.Body)
		_ = streamResponse.Body.Close()
		t.Fatalf("stream status = %d, body = %s", streamResponse.StatusCode, body)
	}
	firstDrainContext, cancelFirstDrain := context.WithTimeout(context.Background(), 10*time.Millisecond)
	if err := hostA.Drain(firstDrainContext); !errors.Is(err, context.DeadlineExceeded) {
		cancelFirstDrain()
		t.Fatalf("drain with active stream = %v", err)
	}
	cancelFirstDrain()
	cancelStream()
	_ = streamResponse.Body.Close()
	select {
	case <-upstreamCancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("provider stream context remained open after client cancellation")
	}

	terminalDeadline := time.Now().Add(5 * time.Second)
	for {
		queryContext, cancelQuery := context.WithTimeout(context.Background(), time.Second)
		queryTransaction, beginErr := database.BeginTxx(queryContext, nil)
		if beginErr != nil {
			cancelQuery()
			t.Fatalf("metering connection remained unavailable after stream cancellation: %v", beginErr)
		}
		usageRows, _, queryErr := cruds["api_usage"].GetRowsByWhereClauseWithTransaction(
			"api_usage", nil, queryTransaction, goqu.Ex{"request_id": "host-stream-cancel"},
		)
		_ = queryTransaction.Rollback()
		cancelQuery()
		if queryErr != nil {
			t.Fatal(queryErr)
		}
		if len(usageRows) == 1 && resource.StringOrEmpty(usageRows[0]["state"]) == "cancelled" {
			break
		}
		if time.Now().After(terminalDeadline) {
			t.Fatalf("stream cancellation did not terminalize generic usage: %#v", usageRows)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if inUse := database.Stats().InUse; inUse != 0 {
		t.Fatalf("database connections still in use after stream cancellation: %d", inUse)
	}
	var healthProbes atomic.Int64
	healthUpstream := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/models" {
			http.NotFound(response, request)
			return
		}
		healthProbes.Add(1)
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer healthUpstream.Close()
	healthRevision := hostB.Status().Revision
	updateCatalogField(t, database, "llm_provider", "base_url", healthUpstream.URL+"/v1")
	updateCatalogField(t, database, "llm_provider", "allow_insecure", true)
	updateCatalogField(t, database, "llm_provider", "allow_private_network", true)
	updateCatalogField(t, database, "llm_deployment", "health_check", `{"enabled":true,"interval_ms":1000,"timeout_ms":500,"failure_threshold":1}`)
	publishCatalogEvent(t, cruds["world"].PubSub, "llm_deployment")
	waitForGatewayStatus(t, hostB, func(status gateway.Status) bool { return status.Revision > healthRevision && status.Ready })
	healthDeadline := time.Now().Add(3 * time.Second)
	for healthProbes.Load() == 0 && time.Now().Before(healthDeadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if healthProbes.Load() == 0 {
		t.Fatal("Daptin gateway did not schedule the configured provider health check")
	}
	drainContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := hostA.Drain(drainContext); err != nil {
		t.Fatal(err)
	}
	if err := hostB.Drain(drainContext); err != nil {
		t.Fatal(err)
	}
}

func newCatalogTestResources(t *testing.T) (*sqlx.DB, map[string]*resource.DbResource, *olric.EmbeddedClient, daptinid.DaptinReferenceId) {
	t.Helper()
	statementbuilder.InitialiseStatementBuilder("sqlite3")
	database, err := sqlx.Open("sqlite3", "file:llm-catalog-"+uuid.NewString()+"?mode=memory&cache=shared&_busy_timeout=10000")
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = database.Close() })

	tables := make([]table_info.TableInfo, 0, len(resource.StandardTables))
	for _, table := range resource.StandardTables {
		copyOfTable := table
		copyOfTable.Columns = append([]api2go.ColumnInfo(nil), table.Columns...)
		copyOfTable.Relations = append([]api2go.TableRelation(nil), table.Relations...)
		tables = append(tables, copyOfTable)
	}
	config := resource.CmsConfig{Tables: tables}
	resource.CheckRelations(&config)
	required := map[string]bool{
		"world": true, "usergroup": true, "user_account": true, "credential": true,
		"llm_provider": true, "llm_model": true, "llm_deployment": true,
		"api_plan": true, "api_member": true, "api_usage": true, "api_quota": true,
		"llm_model_llm_model_id_has_usergroup_usergroup_id": true,
	}
	selected := make([]table_info.TableInfo, 0, len(required))
	for _, table := range config.Tables {
		if required[table.TableName] {
			selected = append(selected, table)
		}
	}
	config.Tables = selected
	resource.CheckAllTableStatus(&config, database)
	resource.CreateRelations(&config, database)
	constraintTransaction, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	resource.CreateUniqueConstraints(&config, constraintTransaction)
	if err := constraintTransaction.Commit(); err != nil {
		t.Fatal(err)
	}
	resource.CreateIndexes(&config, database)

	adminReference := daptinid.DaptinReferenceId(uuid.New())
	userReference := daptinid.DaptinReferenceId(uuid.New())
	for tableName, record := range map[string]goqu.Record{
		"usergroup": {
			"id": 2, "name": "administrators", "reference_id": adminReference[:], "permission": auth.DEFAULT_PERMISSION,
		},
		"user_account": {
			"id": 1, "name": "Catalog Test", "email": "catalog@example.test", "reference_id": userReference[:],
			"permission": auth.DEFAULT_PERMISSION,
		},
	} {
		query, arguments, buildErr := statementbuilder.Squirrel.Insert(tableName).Prepared(true).Rows(record).ToSQL()
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		if _, execErr := database.Exec(query, arguments...); execErr != nil {
			t.Fatalf("insert %s: %v", tableName, execErr)
		}
	}
	usersReference := daptinid.DaptinReferenceId(uuid.New())
	query, arguments, err := statementbuilder.Squirrel.Insert("usergroup").Prepared(true).Rows(goqu.Record{
		"id": 3, "name": "users", "reference_id": usersReference[:], "permission": auth.DEFAULT_PERMISSION,
	}).ToSQL()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(query, arguments...); err != nil {
		t.Fatalf("insert users group: %v", err)
	}

	configStore, err := resource.NewConfigStore(database)
	if err != nil {
		t.Fatal(err)
	}
	configTransaction, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	if err := configStore.SetConfigValueFor("encryption.secret", "0123456789abcdef0123456789abcdef", "backend", configTransaction); err != nil {
		_ = configTransaction.Rollback()
		t.Fatal(err)
	}
	if err := configTransaction.Commit(); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	started := make(chan struct{})
	olricConfiguration := olricconfig.New("local")
	olricConfiguration.BindAddr = "127.0.0.1"
	olricConfiguration.BindPort = port
	olricConfiguration.MemberlistConfig.BindAddr = "127.0.0.1"
	olricConfiguration.MemberlistConfig.BindPort = 0
	olricConfiguration.MemberlistConfig.Name = net.JoinHostPort(olricConfiguration.BindAddr, strconv.Itoa(port))
	olricConfiguration.LogOutput = nil
	olricConfiguration.Started = func() { close(started) }
	embedded, err := olric.New(olricConfiguration)
	if err != nil {
		t.Fatal(err)
	}
	olricErrors := make(chan error, 1)
	go func() { olricErrors <- embedded.Start() }()
	select {
	case <-started:
	case err := <-olricErrors:
		t.Fatalf("start Olric: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Olric startup")
	}
	t.Cleanup(func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = embedded.Shutdown(shutdownContext)
		select {
		case <-olricErrors:
		case <-time.After(5 * time.Second):
			t.Error("timed out waiting for Olric shutdown")
		}
	})
	client := embedded.NewEmbeddedClient()
	pubsub, err := client.NewPubSub(olric.ToAddress(net.JoinHostPort(olricConfiguration.BindAddr, strconv.Itoa(port))))
	if err != nil {
		t.Fatalf("create catalog pubsub: %v", err)
	}

	previousCache := resource.OlricCache
	resource.OlricCache = nil
	t.Cleanup(func() { resource.OlricCache = previousCache })
	cruds := make(map[string]*resource.DbResource, len(config.Tables))
	previousCruds := make(map[string]*resource.DbResource, len(config.Tables))
	for _, table := range config.Tables {
		previousCruds[table.TableName] = resource.CRUD_MAP[table.TableName]
		model := api2go.NewApi2GoModel(table.TableName, table.Columns, int64(table.DefaultPermission), table.Relations)
		crud, createErr := resource.NewDbResource(model, database, &resource.MiddlewareSet{}, cruds, configStore, client, table)
		if createErr != nil {
			t.Fatalf("create %s resource: %v", table.TableName, createErr)
		}
		cruds[table.TableName] = crud
	}
	for _, crud := range cruds {
		crud.PubSub = pubsub
	}
	t.Cleanup(func() {
		for tableName, previous := range previousCruds {
			if previous == nil {
				delete(resource.CRUD_MAP, tableName)
			} else {
				resource.CRUD_MAP[tableName] = previous
			}
		}
	})
	return database, cruds, client, userReference
}

func updateCatalogField(t *testing.T, database *sqlx.DB, tableName, field string, value interface{}) {
	t.Helper()
	query, arguments, err := statementbuilder.Squirrel.Update(tableName).Prepared(true).Set(goqu.Record{field: value}).ToSQL()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(query, arguments...); err != nil {
		t.Fatalf("update %s.%s: %v", tableName, field, err)
	}
}

func waitForCatalogSubscribers(t *testing.T, pubsub *olric.PubSub, expected int64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		subscribers, err := pubsub.PubSubNumSub(context.Background(), "credential", "llm_provider", "llm_model", "llm_deployment")
		if err == nil && subscribers["credential"] >= expected && subscribers["llm_provider"] >= expected && subscribers["llm_model"] >= expected && subscribers["llm_deployment"] >= expected {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d LLM catalog subscribers", expected)
}

func publishCatalogEvent(t *testing.T, pubsub *olric.PubSub, tableName string) {
	t.Helper()
	if _, err := pubsub.Publish(context.Background(), tableName, resource.WsOutMessage{
		Type: "event", Topic: tableName, Event: "update", Source: "database",
	}); err != nil {
		t.Fatalf("publish %s catalog event: %v", tableName, err)
	}
}

func waitForGatewayStatus(t *testing.T, host *Gateway, matches func(gateway.Status) bool) gateway.Status {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status := host.Status()
		if matches(status) {
			return status
		}
		time.Sleep(10 * time.Millisecond)
	}
	status := host.Status()
	t.Fatalf("timed out waiting for gateway status; last status: %#v", status)
	return status
}
