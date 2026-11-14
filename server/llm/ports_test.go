package llm

import (
	"context"
	"errors"
	"fmt"
	"github.com/daptin/daptin/server/table_info"
	"strings"
	"testing"
	"time"

	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	gateway "github.com/daptin/llmgateway"
	"github.com/daptin/llmgateway/contract"
	"github.com/google/uuid"
)

func TestDaptinMeteringCapturesCanonicalPayloads(t *testing.T) {
	for _, bounded := range []bool{false, true} {
		t.Run(fmt.Sprint(bounded), func(t *testing.T) {
			database, cruds, _, _ := newCatalogTestResources(t)
			config := cruds["llm_model"].TableInfo().Metering
			config.OnActions["invoke"] = table_info.MeteringConfig{Capture: &table_info.PayloadCaptureConfig{Enabled: true, MaxBytes: 2048}}
			metering := daptinMetering{cruds: cruds, service: resource.NewMeteringService(&cruds)}
			text := "remember"
			if bounded {
				text = strings.Repeat("r", 4096)
			}
			request := contract.Request{Responses: &contract.ResponsesRequest{Input: []contract.ResponseInputItem{{Type: "message", Role: "user", Content: []contract.ContentPart{{Type: "input_text", Text: text}}}}}}
			response := contract.Response{Responses: &contract.ResponsesResponse{Status: "completed", Output: []contract.ResponseOutputItem{{Type: "message", Content: []contract.ContentPart{{Type: "output_text", Text: text}}}}}}
			requestBody, _ := json.Marshal(request)
			responseBody, _ := json.Marshal(response)
			token, err := metering.Admit(context.Background(), contract.Admission{RequestID: "logical-payload", Operation: contract.OperationResponses, Request: request})
			if err != nil {
				t.Fatal(err)
			}
			if err := metering.Complete(context.Background(), contract.Completion{Token: token, Status: "succeeded", HTTPStatus: 200, Response: &response}); err != nil {
				t.Fatal(err)
			}
			tx, err := database.Beginx()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			rows, _, err := cruds["api_usage"].GetRowsByWhereClauseWithTransaction("api_usage", nil, tx)
			if err != nil || len(rows) != 1 {
				t.Fatalf("usage rows=%d error=%v", len(rows), err)
			}
			row := rows[0]
			if resource.StringOrEmpty(row["request_body"]) != string(requestBody[:min(len(requestBody), 2048)]) || resource.StringOrEmpty(row["response_body"]) != string(responseBody[:min(len(responseBody), 2048)]) {
				t.Fatalf("logical payload mismatch: %#v", row)
			}
			requestBytes, _ := resource.ResourceRowInt64(row["request_bytes"])
			responseBytes, _ := resource.ResourceRowInt64(row["response_bytes"])
			if requestBytes != int64(len(requestBody)) || responseBytes != int64(len(responseBody)) {
				t.Fatalf("byte counts=%d,%d", requestBytes, responseBytes)
			}
			var metadata map[string]interface{}
			if err := json.UnmarshalFromString(resource.StringOrEmpty(row["metadata"]), &metadata); err != nil {
				t.Fatal(err)
			}
			if bounded && (metadata["request_body_truncated"] != true || metadata["response_body_truncated"] != true) {
				t.Fatalf("missing truncation facts: %#v", metadata)
			}
		})
	}
}

func TestDaptinMeteringRecordsGuestInvocation(t *testing.T) {
	database, cruds, _, _ := newCatalogTestResources(t)
	metering := daptinMetering{cruds: cruds, service: resource.NewMeteringService(&cruds)}
	token, err := metering.Admit(context.Background(), contract.Admission{
		RequestID: "guest-request", Operation: contract.OperationResponses,
		EstimatedUsage: contract.Usage{InputTokens: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if token.Opaque == "" {
		t.Fatal("guest invocation did not reserve usage")
	}
	duplicate, err := metering.Admit(context.Background(), contract.Admission{
		RequestID: "guest-request", Operation: contract.OperationResponses,
	})
	if err != nil || duplicate.Opaque != token.Opaque {
		t.Fatalf("duplicate guest admission = %q, %v", duplicate.Opaque, err)
	}
	if err := metering.Complete(context.Background(), contract.Completion{
		Token: token, HTTPStatus: 200, Status: "completed",
		Usage: contract.Usage{InputTokens: 3, OutputTokens: 2, TotalTokens: 5, CostMicros: 7},
	}); err != nil {
		t.Fatal(err)
	}
	transaction, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.Rollback()
	rows, _, err := cruds["api_usage"].GetRowsByWhereClauseWithTransaction("api_usage", nil, transaction)
	if err != nil {
		t.Fatal(err)
	}
	guest, err := cruds["user_account"].GetUserAccountRowByEmailWithTransaction("guest@cms.go", transaction)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("guest invocation created %d usage rows, want 1", len(rows))
	}
	ownerReference := daptinid.InterfaceToDIR(rows[0]["user_account_id"])
	guestReference := daptinid.InterfaceToDIR(guest["reference_id"])
	status, err := resource.ResourceRowInt64(rows[0]["status_code"])
	if err != nil {
		t.Fatal(err)
	}
	if ownerReference != guestReference || rows[0]["endpoint"] != "/v1/responses" || rows[0]["state"] != "completed" || status != 200 {
		t.Fatalf("guest usage attribution or status is wrong: %#v", rows[0])
	}
	var measures map[string]int64
	if err := json.UnmarshalFromString(resource.StringOrEmpty(rows[0]["measures"]), &measures); err != nil {
		t.Fatal(err)
	}
	if measures["total_tokens"] != 5 || measures["cost_micros"] != 7 {
		t.Fatalf("guest measures = %#v", measures)
	}
}

func TestDaptinMeteringTerminalizesGuestCancellation(t *testing.T) {
	database, cruds, _, _ := newCatalogTestResources(t)
	metering := daptinMetering{cruds: cruds, service: resource.NewMeteringService(&cruds)}
	token, err := metering.Admit(context.Background(), contract.Admission{
		RequestID: "guest-embedding-cancel", Operation: contract.OperationEmbeddings,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := metering.Cancel(context.Background(), contract.Cancellation{Token: token, Reason: "client_cancelled"}); err != nil {
		t.Fatal(err)
	}
	transaction, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.Rollback()
	rows, _, err := cruds["api_usage"].GetRowsByWhereClauseWithTransaction("api_usage", nil, transaction)
	if err != nil || len(rows) != 1 {
		t.Fatalf("guest cancellation usage rows = %d, %v", len(rows), err)
	}
	guest, err := cruds["user_account"].GetUserAccountRowByEmailWithTransaction("guest@cms.go", transaction)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0]["state"] != "cancelled" || rows[0]["endpoint"] != "/v1/embeddings" ||
		daptinid.InterfaceToDIR(rows[0]["user_account_id"]) != daptinid.InterfaceToDIR(guest["reference_id"]) {
		t.Fatalf("guest cancellation was not terminalized against the guest account: %#v", rows[0])
	}
}

func TestDaptinMeteringRecordsGuestEmbedding(t *testing.T) {
	database, cruds, _, _ := newCatalogTestResources(t)
	metering := daptinMetering{cruds: cruds, service: resource.NewMeteringService(&cruds)}
	token, err := metering.Admit(context.Background(), contract.Admission{
		RequestID: "guest-embedding", Operation: contract.OperationEmbeddings,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := metering.Complete(context.Background(), contract.Completion{
		Token: token, HTTPStatus: 200, Status: "completed", Usage: contract.Usage{InputTokens: 2, TotalTokens: 2},
	}); err != nil {
		t.Fatal(err)
	}
	transaction, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.Rollback()
	rows, _, err := cruds["api_usage"].GetRowsByWhereClauseWithTransaction("api_usage", nil, transaction)
	if err != nil || len(rows) != 1 {
		t.Fatalf("guest embedding usage rows = %d, %v", len(rows), err)
	}
	var measures map[string]int64
	if err := json.UnmarshalFromString(resource.StringOrEmpty(rows[0]["measures"]), &measures); err != nil {
		t.Fatal(err)
	}
	if rows[0]["state"] != "completed" || rows[0]["endpoint"] != "/v1/embeddings" || measures["input_tokens"] != 2 {
		t.Fatalf("guest embedding usage = %#v", rows[0])
	}
}

func TestDaptinOlricPortsImplementGatewayContract(t *testing.T) {
	_, _, embedded, _ := newCatalogTestResources(t)
	client := newCatalogClusterClient(t, embedded)
	values, err := client.NewDMap("llmgateway-port-values-" + uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	leases, err := client.NewDMap("llmgateway-port-leases-" + uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	cacheValues, err := client.NewDMap("llmgateway-port-cache-" + uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	counters := olricCounterStore{values: values, leases: leases}
	if value, err := counters.Add(ctx, "rpm", 1, time.Second); err != nil || value != 1 {
		t.Fatalf("first fixed-window add = %d, %v", value, err)
	}
	time.Sleep(600 * time.Millisecond)
	if value, err := counters.Add(ctx, "rpm", 1, time.Second); err != nil || value != 2 {
		t.Fatalf("second fixed-window add = %d, %v", value, err)
	}
	time.Sleep(600 * time.Millisecond)
	if value, found, err := counters.Get(ctx, "rpm"); err != nil || found || value != 0 {
		t.Fatalf("fixed-window expiry was extended: value=%d found=%v err=%v", value, found, err)
	}

	lease, err := counters.Acquire(ctx, "deployment", 1, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := counters.Acquire(ctx, "deployment", 1, time.Minute); !errors.Is(err, gateway.ErrCounterLimit) {
		t.Fatalf("second acquire = %v, want counter limit", err)
	}
	if err := counters.Release(ctx, lease); err != nil {
		t.Fatal(err)
	}
	if err := counters.Release(ctx, lease); err != nil {
		t.Fatalf("duplicate release must be idempotent: %v", err)
	}
	if _, err := counters.Acquire(ctx, "deployment", 1, time.Minute); err != nil {
		t.Fatalf("released capacity was not reusable: %v", err)
	}

	first, err := counters.Acquire(ctx, "staggered", 2, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(600 * time.Millisecond)
	second, err := counters.Acquire(ctx, "staggered", 2, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(600 * time.Millisecond)
	if _, err := counters.Acquire(ctx, "staggered", 2, time.Second); !errors.Is(err, gateway.ErrCounterLimit) {
		t.Fatalf("staggered lease counter expired before its newest lease: %v", err)
	}
	if err := counters.Release(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := counters.Release(ctx, second); err != nil {
		t.Fatal(err)
	}

	cache := olricResponseCache{values: cacheValues}
	if err := cache.Set(ctx, "response", []byte("payload"), time.Minute); err != nil {
		t.Fatal(err)
	}
	payload, found, err := cache.Get(ctx, "response")
	if err != nil || !found || string(payload) != "payload" {
		t.Fatalf("cache get = %q, %v, %v", payload, found, err)
	}
	if err := cache.Delete(ctx, "response"); err != nil {
		t.Fatal(err)
	}
	if _, found, err := cache.Get(ctx, "response"); err != nil || found {
		t.Fatalf("cache delete left entry: found=%v err=%v", found, err)
	}
}
