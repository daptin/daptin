package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/artpar/api2go/v2"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/llmgateway/contract"
	"github.com/daptin/llmgateway/protocol/openai"
	"github.com/jmoiron/sqlx"
)

type daptinBatches struct {
	cruds map[string]*resource.DbResource
	files daptinFiles
}

func (store daptinBatches) Create(ctx context.Context, _ contract.Principal, request contract.CreateBatchRequest) (contract.Batch, error) {
	if _, err := daptinSessionUser(ctx); err != nil {
		return contract.Batch{}, err
	}
	transaction, err := store.cruds["llm_batch"].Connection().Beginx()
	if err != nil {
		return contract.Batch{}, fmt.Errorf("begin batch create: %w", err)
	}
	defer transaction.Rollback()
	input, _, err := store.files.load(ctx, request.InputFileID, http.MethodGet, transaction)
	if err != nil {
		return contract.Batch{}, err
	}
	if contract.FilePurpose(resource.StringOrEmpty(input["purpose"])) != contract.FilePurposeBatch {
		return contract.Batch{}, &contract.Error{Code: contract.ErrorInvalidRequest, Message: "input file purpose must be batch", HTTPStatus: http.StatusBadRequest}
	}
	metadata, err := json.Marshal(request.Metadata)
	if err != nil {
		return contract.Batch{}, fmt.Errorf("encode batch metadata: %w", err)
	}
	now := time.Now().UTC()
	expiresAt := now.Add(24 * time.Hour)
	attributes := map[string]interface{}{
		"input_file_id": string(request.InputFileID), "endpoint": request.Endpoint, "completion_window": request.CompletionWindow,
		"status": string(contract.BatchStatusValidating), "metadata": string(metadata),
		"request_counts": `{"total":0,"completed":0,"failed":0}`, "expires_at": expiresAt,
	}
	if request.OutputExpiresAfter != nil {
		attributes["output_expiration_seconds"] = request.OutputExpiresAfter.Seconds
	}
	createURL, _ := url.Parse("/llm_batch")
	apiRequest := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodPost, URL: createURL}).WithContext(ctx)}
	row, err := store.cruds["llm_batch"].CreateWithoutFilter(
		api2go.NewApi2GoModelWithData("llm_batch", nil, 0, nil, attributes), apiRequest, transaction,
	)
	if err != nil {
		return contract.Batch{}, fmt.Errorf("create batch: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return contract.Batch{}, fmt.Errorf("commit batch create: %w", err)
	}
	return contract.Batch{ID: contract.ID(daptinid.InterfaceToDIR(row["reference_id"]).String()), Endpoint: request.Endpoint,
		InputFileID: request.InputFileID, CompletionWindow: request.CompletionWindow, Status: contract.BatchStatusValidating,
		CreatedAt: now, ExpiresAt: &expiresAt, RequestCounts: contract.BatchRequestCounts{}, Metadata: request.Metadata}, nil
}

func (store daptinBatches) List(ctx context.Context, _ contract.Principal, request contract.ListBatchesRequest) (contract.BatchPage, error) {
	if _, err := daptinSessionUser(ctx); err != nil {
		return contract.BatchPage{}, err
	}
	transaction, err := store.cruds["llm_batch"].Connection().Beginx()
	if err != nil {
		return contract.BatchPage{}, fmt.Errorf("begin batch list: %w", err)
	}
	defer transaction.Rollback()
	query := url.Values{"page[size]": {strconv.Itoa(request.Limit + 1)}, "sort": {"-created_at"}}
	if request.After != "" {
		if daptinid.InterfaceToDIR(string(request.After)) == daptinid.NullReferenceId {
			return contract.BatchPage{}, &contract.Error{Code: contract.ErrorInvalidRequest, Message: "invalid batch cursor", HTTPStatus: http.StatusBadRequest}
		}
		query["page[before]"] = []string{string(request.After)}
	}
	listURL, _ := url.Parse("/llm_batch?" + query.Encode())
	apiRequest := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodGet, URL: listURL}).WithContext(ctx), QueryParams: query}
	_, responder, err := store.cruds["llm_batch"].PaginatedFindAllWithTransaction(apiRequest, transaction)
	if err != nil {
		return contract.BatchPage{}, fmt.Errorf("list batches: %w", err)
	}
	models, ok := responder.Result().([]api2go.Api2GoModel)
	if !ok {
		return contract.BatchPage{}, fmt.Errorf("batch list returned [%T]", responder.Result())
	}
	hasMore := len(models) > request.Limit
	if hasMore {
		models = models[:request.Limit]
	}
	batches := make([]contract.Batch, 0, len(models))
	for index := range models {
		batch, conversionErr := daptinBatch(models[index].GetAllAsAttributes())
		if conversionErr != nil {
			return contract.BatchPage{}, conversionErr
		}
		batches = append(batches, batch)
	}
	if err := transaction.Commit(); err != nil {
		return contract.BatchPage{}, fmt.Errorf("commit batch list: %w", err)
	}
	return contract.BatchPage{Data: batches, HasMore: hasMore}, nil
}

func (store daptinBatches) Get(ctx context.Context, _ contract.Principal, id contract.ID) (contract.Batch, error) {
	transaction, err := store.cruds["llm_batch"].Connection().Beginx()
	if err != nil {
		return contract.Batch{}, fmt.Errorf("begin batch read: %w", err)
	}
	defer transaction.Rollback()
	row, err := store.load(ctx, id, false, transaction)
	if err != nil {
		return contract.Batch{}, err
	}
	batch, err := daptinBatch(row)
	if err != nil {
		return contract.Batch{}, err
	}
	if err := transaction.Commit(); err != nil {
		return contract.Batch{}, fmt.Errorf("commit batch read: %w", err)
	}
	return batch, nil
}

func (store daptinBatches) Cancel(ctx context.Context, _ contract.Principal, id contract.ID) (contract.Batch, error) {
	transaction, err := store.cruds["llm_batch"].Connection().Beginx()
	if err != nil {
		return contract.Batch{}, fmt.Errorf("begin batch cancel: %w", err)
	}
	defer transaction.Rollback()
	row, err := store.load(ctx, id, true, transaction)
	if err != nil {
		return contract.Batch{}, err
	}
	status := contract.BatchStatus(resource.StringOrEmpty(row["status"]))
	switch status {
	case contract.BatchStatusValidating, contract.BatchStatusInProgress, contract.BatchStatusFinalizing:
	default:
		return contract.Batch{}, &contract.Error{Code: contract.ErrorInvalidRequest, Message: "batch cannot be cancelled in its current state", HTTPStatus: http.StatusBadRequest}
	}
	now := time.Now().UTC()
	updated, err := store.update(ctx, row, map[string]interface{}{"status": string(contract.BatchStatusCancelling), "cancelling_at": now}, transaction)
	if err != nil {
		return contract.Batch{}, err
	}
	if err := transaction.Commit(); err != nil {
		return contract.Batch{}, fmt.Errorf("commit batch cancel: %w", err)
	}
	return daptinBatch(updated)
}

func (store daptinBatches) load(ctx context.Context, id contract.ID, requireUpdate bool, transaction *sqlx.Tx) (map[string]interface{}, error) {
	reference := daptinid.InterfaceToDIR(string(id))
	if reference == daptinid.NullReferenceId {
		return nil, batchNotFound()
	}
	method := http.MethodGet
	if requireUpdate {
		method = http.MethodPatch
	}
	readURL, _ := url.Parse("/llm_batch/" + reference.String())
	apiRequest := api2go.Request{PlainRequest: (&http.Request{Method: method, URL: readURL}).WithContext(ctx)}
	responder, err := store.cruds["llm_batch"].FindOneWithTransaction(reference, apiRequest, transaction)
	if err != nil {
		if status, ok := err.(interface{ Status() int }); ok && (status.Status() == http.StatusForbidden || status.Status() == http.StatusNotFound) {
			return nil, batchNotFound()
		}
		return nil, fmt.Errorf("load batch: %w", err)
	}
	model, ok := responder.Result().(api2go.Api2GoModel)
	if !ok || model.GetAllAsAttributes() == nil {
		return nil, batchNotFound()
	}
	return model.GetAllAsAttributes(), nil
}

func (store daptinBatches) update(ctx context.Context, row map[string]interface{}, changes map[string]interface{}, transaction *sqlx.Tx) (map[string]interface{}, error) {
	table := store.cruds["llm_batch"].TableInfo()
	model := api2go.NewApi2GoModelWithData("llm_batch", table.Columns, int64(table.DefaultPermission), table.Relations, row)
	model.SetAttributes(changes)
	reference := daptinid.InterfaceToDIR(row["reference_id"])
	updateURL, _ := url.Parse("/llm_batch/" + reference.String())
	apiRequest := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodPatch, URL: updateURL}).WithContext(ctx)}
	updated, err := store.cruds["llm_batch"].UpdateWithoutFilters(model, apiRequest, transaction)
	if err != nil {
		return nil, fmt.Errorf("update batch: %w", err)
	}
	return updated, nil
}

func daptinBatch(row map[string]interface{}) (contract.Batch, error) {
	createdAt, ok := row["created_at"].(time.Time)
	if !ok || createdAt.IsZero() {
		return contract.Batch{}, fmt.Errorf("batch has invalid creation time")
	}
	status := contract.BatchStatus(resource.StringOrEmpty(row["status"]))
	if !status.Valid() {
		return contract.Batch{}, fmt.Errorf("batch has invalid status")
	}
	var metadata map[string]string
	if err := json.Unmarshal([]byte(resource.StringOrEmpty(row["metadata"])), &metadata); err != nil {
		return contract.Batch{}, fmt.Errorf("decode batch metadata: %w", err)
	}
	var counts contract.BatchRequestCounts
	if err := json.Unmarshal([]byte(resource.StringOrEmpty(row["request_counts"])), &counts); err != nil {
		return contract.Batch{}, fmt.Errorf("decode batch request counts: %w", err)
	}
	var errorsList []contract.BatchError
	if encoded := resource.StringOrEmpty(row["errors"]); encoded != "" {
		if err := json.Unmarshal([]byte(encoded), &errorsList); err != nil {
			return contract.Batch{}, fmt.Errorf("decode batch errors: %w", err)
		}
	}
	batch := contract.Batch{ID: contract.ID(daptinid.InterfaceToDIR(row["reference_id"]).String()), Endpoint: resource.StringOrEmpty(row["endpoint"]),
		InputFileID: contract.ID(daptinid.InterfaceToDIR(row["input_file_id"]).String()), CompletionWindow: resource.StringOrEmpty(row["completion_window"]),
		Status:    status,
		CreatedAt: createdAt, RequestCounts: counts, Metadata: metadata, Errors: errorsList}
	if reference := daptinid.InterfaceToDIR(row["output_file_id"]); reference != daptinid.NullReferenceId {
		batch.OutputFileID = contract.ID(reference.String())
	}
	for column, target := range map[string]**time.Time{"in_progress_at": &batch.InProgressAt, "expires_at": &batch.ExpiresAt,
		"finalizing_at": &batch.FinalizingAt, "completed_at": &batch.CompletedAt, "failed_at": &batch.FailedAt,
		"expired_at": &batch.ExpiredAt, "cancelling_at": &batch.CancellingAt, "cancelled_at": &batch.CancelledAt} {
		if row[column] == nil {
			continue
		}
		value, valid := row[column].(time.Time)
		if !valid {
			return contract.Batch{}, fmt.Errorf("batch has invalid %s", column)
		}
		*target = &value
	}
	return batch, nil
}

func batchNotFound() *contract.Error {
	return &contract.Error{Code: contract.ErrorPermission, Message: "batch not found", HTTPStatus: http.StatusNotFound}
}

var _ openai.BatchStore = daptinBatches{}
