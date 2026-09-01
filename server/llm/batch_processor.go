package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	gateway "github.com/daptin/llmgateway"
	"github.com/daptin/llmgateway/contract"
	"github.com/daptin/llmgateway/protocol/openai"
	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

const (
	batchClaimDuration = 10 * time.Minute
	batchWorkers       = 4
	batchItemPageSize  = 100
)

type daptinBatchProcessor struct {
	cruds        map[string]*resource.DbResource
	files        daptinFiles
	batches      daptinBatches
	handler      http.Handler
	coordination gateway.CounterStore
}

func (processor *daptinBatchProcessor) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	slots := make(chan struct{}, batchWorkers)
	var workers sync.WaitGroup
	for {
		select {
		case <-ctx.Done():
			workers.Wait()
			return
		case <-ticker.C:
			select {
			case slots <- struct{}{}:
				workers.Add(1)
				go func() {
					defer workers.Done()
					defer func() { <-slots }()
					if err := processor.runOne(ctx); err != nil && ctx.Err() == nil {
						log.WithError(err).Error("[llm] batch processing failed")
					}
				}()
			default:
			}
		}
	}
}

func (processor *daptinBatchProcessor) runOne(ctx context.Context) error {
	row, user, err := processor.claim(ctx)
	if err != nil || row == nil {
		return err
	}
	ownerContext := context.WithValue(ctx, "user", user)
	status := contract.BatchStatus(resource.StringOrEmpty(row["status"]))
	if status == contract.BatchStatusCancelling {
		return processor.finishCancellation(ownerContext, row)
	}
	if status == contract.BatchStatusValidating {
		if err := processor.validate(ownerContext, row); err != nil {
			return processor.fail(ownerContext, row, "invalid_batch_file", err.Error(), nil)
		}
	}
	return processor.execute(ownerContext, contract.ID(daptinid.InterfaceToDIR(row["reference_id"]).String()),
		resource.StringOrEmpty(row["endpoint"]))
}

func (processor *daptinBatchProcessor) claim(ctx context.Context) (map[string]interface{}, *auth.SessionUser, error) {
	readTransaction, err := processor.cruds["llm_batch"].Connection().Beginx()
	if err != nil {
		return nil, nil, fmt.Errorf("begin batch claim scan: %w", err)
	}
	now := time.Now().UTC()
	rows, err := processor.claimCandidates(ctx, now, readTransaction)
	if err != nil {
		_ = readTransaction.Rollback()
		return nil, nil, err
	}
	if err := readTransaction.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit batch claim scan: %w", err)
	}
	for _, candidate := range rows {
		reference := daptinid.InterfaceToDIR(candidate["reference_id"])
		token, lockErr := processor.coordination.Acquire(ctx, "llm-batch-claim:"+reference.String(), 1, olricCoordinationTimeout)
		if errors.Is(lockErr, gateway.ErrCounterLimit) {
			continue
		}
		if lockErr != nil {
			return nil, nil, fmt.Errorf("coordinate batch claim: %w", lockErr)
		}
		claimed, user, claimErr := processor.claimCandidate(ctx, reference, now)
		releaseErr := processor.coordination.Release(context.WithoutCancel(ctx), token)
		if claimErr != nil {
			return nil, nil, claimErr
		}
		if releaseErr != nil {
			log.WithError(releaseErr).Warn("[llm] failed to release batch claim coordination")
		}
		if claimed != nil {
			return claimed, user, nil
		}
	}
	return nil, nil, nil
}

func (processor *daptinBatchProcessor) claimCandidates(ctx context.Context, now time.Time, transaction *sqlx.Tx) ([]map[string]interface{}, error) {
	statuses := []string{string(contract.BatchStatusValidating), string(contract.BatchStatusInProgress),
		string(contract.BatchStatusFinalizing), string(contract.BatchStatusCancelling)}
	admin := &auth.SessionUser{Groups: auth.GroupPermissionList{{GroupReferenceId: processor.cruds["llm_batch"].AdministratorGroupId}}}
	adminContext := context.WithValue(ctx, "user", admin)
	candidates := make(map[daptinid.DaptinReferenceId]map[string]interface{}, batchWorkers*2)
	for _, expiry := range []resource.Query{
		{ColumnName: "claim_expires_at", Operator: "is empty"},
		{ColumnName: "claim_expires_at", Operator: "before", Value: now},
	} {
		encoded, err := json.Marshal([]resource.Query{{ColumnName: "status", Operator: "in", Value: statuses}, expiry})
		if err != nil {
			return nil, err
		}
		query := url.Values{"page[size]": {strconv.Itoa(batchWorkers)}, "sort": {"+created_at"}, "query": {string(encoded)}}
		listURL, _ := url.Parse("/llm_batch?" + query.Encode())
		request := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodGet, URL: listURL}).WithContext(adminContext), QueryParams: query}
		_, responder, err := processor.cruds["llm_batch"].PaginatedFindAllWithTransaction(request, transaction)
		if err != nil {
			return nil, fmt.Errorf("scan claimable batches: %w", err)
		}
		models, ok := responder.Result().([]api2go.Api2GoModel)
		if !ok {
			return nil, fmt.Errorf("batch claim scan returned [%T]", responder.Result())
		}
		for index := range models {
			row := models[index].GetAllAsAttributes()
			candidates[daptinid.InterfaceToDIR(row["reference_id"])] = row
		}
	}
	rows := make([]map[string]interface{}, 0, len(candidates))
	for _, row := range candidates {
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		left, _ := rows[i]["created_at"].(time.Time)
		right, _ := rows[j]["created_at"].(time.Time)
		return left.Before(right)
	})
	return rows, nil
}

func (processor *daptinBatchProcessor) claimCandidate(ctx context.Context, reference daptinid.DaptinReferenceId, now time.Time) (map[string]interface{}, *auth.SessionUser, error) {
	transaction, err := processor.cruds["llm_batch"].Connection().Beginx()
	if err != nil {
		return nil, nil, err
	}
	defer transaction.Rollback()
	rows, _, err := processor.cruds["llm_batch"].GetRowsByWhereClauseWithTransaction("llm_batch", nil, transaction, goqu.Ex{"reference_id": reference[:]})
	if err != nil {
		return nil, nil, err
	}
	if len(rows) != 1 {
		return nil, nil, nil
	}
	row := rows[0]
	status := contract.BatchStatus(resource.StringOrEmpty(row["status"]))
	if status != contract.BatchStatusValidating && status != contract.BatchStatusInProgress &&
		status != contract.BatchStatusFinalizing && status != contract.BatchStatusCancelling {
		return nil, nil, nil
	}
	if expiry, ok := row["claim_expires_at"].(time.Time); ok && expiry.After(now) {
		return nil, nil, nil
	}
	user, err := processor.batchOwner(row, transaction)
	if err != nil {
		return nil, nil, err
	}
	ownerContext := context.WithValue(ctx, "user", user)
	if expiresAt, ok := row["expires_at"].(time.Time); ok && !expiresAt.After(now) {
		if _, err := processor.batches.update(ownerContext, row, map[string]interface{}{
			"status": string(contract.BatchStatusExpired), "expired_at": now, "claim_expires_at": nil,
		}, transaction); err != nil {
			return nil, nil, err
		}
		if err := transaction.Commit(); err != nil {
			return nil, nil, err
		}
		return nil, nil, nil
	}
	updated, err := processor.batches.update(ownerContext, row, map[string]interface{}{
		"claim_expires_at": now.Add(batchClaimDuration),
	}, transaction)
	if err != nil {
		return nil, nil, err
	}
	if err := transaction.Commit(); err != nil {
		return nil, nil, err
	}
	return updated, user, nil
}

func (processor *daptinBatchProcessor) validate(ctx context.Context, row map[string]interface{}) error {
	inputID := contract.ID(daptinid.InterfaceToDIR(row["input_file_id"]).String())
	content, err := processor.files.Content(ctx, contract.Principal{}, inputID)
	if err != nil {
		return fmt.Errorf("read batch input: %w", err)
	}
	lines, err := openai.DecodeBatchInput(content.Data, resource.StringOrEmpty(row["endpoint"]))
	if err != nil {
		return err
	}
	transaction, err := processor.cruds["llm_batch_item"].Connection().Beginx()
	if err != nil {
		return fmt.Errorf("begin batch validation: %w", err)
	}
	batchReference := daptinid.InterfaceToDIR(row["reference_id"])
	existing, _, err := processor.cruds["llm_batch_item"].GetRowsByWhereClauseWithTransaction("llm_batch_item", nil, transaction,
		goqu.Ex{"llm_batch_id": row["id"]})
	if err != nil {
		_ = transaction.Rollback()
		return fmt.Errorf("load batch items: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit batch checkpoint read: %w", err)
	}
	inputByLine := make(map[int64]openai.BatchInputLine, len(lines))
	for _, line := range lines {
		inputByLine[line.Line] = line
	}
	for _, item := range existing {
		lineNumber, conversionErr := resource.ResourceRowInt64(item["line_number"])
		line, found := inputByLine[lineNumber]
		if conversionErr != nil || !found || resource.StringOrEmpty(item["custom_id"]) != line.CustomID ||
			resource.StringOrEmpty(item["body"]) != string(line.Body) {
			return fmt.Errorf("batch validation checkpoint is inconsistent")
		}
		delete(inputByLine, lineNumber)
	}
	missing := make([]openai.BatchInputLine, 0, len(inputByLine))
	for _, line := range lines {
		if _, found := inputByLine[line.Line]; found {
			missing = append(missing, line)
		}
	}
	itemURL, _ := url.Parse("/llm_batch_item")
	apiRequest := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodPost, URL: itemURL}).WithContext(ctx)}
	for offset := 0; offset < len(missing); offset += batchItemPageSize {
		end := offset + batchItemPageSize
		if end > len(missing) {
			end = len(missing)
		}
		transaction, err = processor.cruds["llm_batch_item"].Connection().Beginx()
		if err != nil {
			return fmt.Errorf("begin batch checkpoint: %w", err)
		}
		for _, line := range missing[offset:end] {
			_, err := processor.cruds["llm_batch_item"].CreateWithoutFilter(api2go.NewApi2GoModelWithData("llm_batch_item", nil, 0, nil, map[string]interface{}{
				"llm_batch_id": batchReference.String(), "line_number": line.Line, "custom_id": line.CustomID,
				"body": string(line.Body), "status": "pending",
			}), apiRequest, transaction)
			if err != nil {
				_ = transaction.Rollback()
				return fmt.Errorf("create batch item %d: %w", line.Line, err)
			}
		}
		current, loadErr := processor.batchRow(batchReference, transaction)
		if loadErr != nil {
			_ = transaction.Rollback()
			return loadErr
		}
		if _, err := processor.batches.update(ctx, current, map[string]interface{}{
			"claim_expires_at": time.Now().UTC().Add(batchClaimDuration),
		}, transaction); err != nil {
			_ = transaction.Rollback()
			return err
		}
		if err := transaction.Commit(); err != nil {
			return fmt.Errorf("commit batch checkpoint: %w", err)
		}
	}
	transaction, err = processor.cruds["llm_batch"].Connection().Beginx()
	if err != nil {
		return fmt.Errorf("begin validated batch transition: %w", err)
	}
	current, err := processor.batchRow(batchReference, transaction)
	if err != nil {
		_ = transaction.Rollback()
		return err
	}
	now := time.Now().UTC()
	counts, _ := json.Marshal(contract.BatchRequestCounts{Total: int64(len(lines))})
	if _, err := processor.batches.update(ctx, current, map[string]interface{}{
		"status": string(contract.BatchStatusInProgress), "in_progress_at": now, "request_counts": string(counts),
		"claim_expires_at": now.Add(batchClaimDuration),
	}, transaction); err != nil {
		_ = transaction.Rollback()
		return err
	}
	return transaction.Commit()
}

func (processor *daptinBatchProcessor) execute(ctx context.Context, batchID contract.ID, endpoint string) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		batch, err := processor.batches.Get(ctx, contract.Principal{}, batchID)
		if err != nil {
			return err
		}
		if batch.Status == contract.BatchStatusCancelling {
			transaction, beginErr := processor.cruds["llm_batch"].Connection().Beginx()
			if beginErr != nil {
				return beginErr
			}
			row, rowErr := processor.batchRow(daptinid.InterfaceToDIR(string(batchID)), transaction)
			_ = transaction.Rollback()
			if rowErr != nil {
				return rowErr
			}
			return processor.finishCancellation(ctx, row)
		}
		items, err := processor.pendingItems(ctx, batchID)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return processor.finalize(ctx, batchID)
		}
		for _, item := range items {
			if err := processor.executeItem(ctx, batchID, endpoint, item); err != nil {
				return err
			}
		}
	}
}

func (processor *daptinBatchProcessor) pendingItems(ctx context.Context, batchID contract.ID) ([]map[string]interface{}, error) {
	transaction, err := processor.cruds["llm_batch_item"].Connection().Beginx()
	if err != nil {
		return nil, err
	}
	defer transaction.Rollback()
	encoded, err := json.Marshal([]resource.Query{
		{ColumnName: "llm_batch_id", Operator: "is", Value: string(batchID)},
		{ColumnName: "status", Operator: "is", Value: "pending"},
	})
	if err != nil {
		return nil, err
	}
	query := url.Values{"page[size]": {strconv.Itoa(batchItemPageSize)}, "sort": {"+line_number"}, "query": {string(encoded)}}
	listURL, _ := url.Parse("/llm_batch_item?" + query.Encode())
	request := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodGet, URL: listURL}).WithContext(ctx), QueryParams: query}
	_, responder, err := processor.cruds["llm_batch_item"].PaginatedFindAllWithTransaction(request, transaction)
	if err != nil {
		return nil, err
	}
	models, ok := responder.Result().([]api2go.Api2GoModel)
	if !ok {
		return nil, fmt.Errorf("batch item scan returned [%T]", responder.Result())
	}
	if len(models) == 0 {
		return nil, transaction.Commit()
	}
	items := make([]map[string]interface{}, len(models))
	for index := range models {
		items[index] = models[index].GetAllAsAttributes()
	}
	return items, transaction.Commit()
}

func (processor *daptinBatchProcessor) executeItem(ctx context.Context, batchID contract.ID, endpoint string, item map[string]interface{}) error {
	line, err := resource.ResourceRowInt64(item["line_number"])
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		strings.NewReader(resource.StringOrEmpty(item["body"])))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer daptin-batch")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "batch."+string(batchID)+"."+strconv.FormatInt(line, 10))
	recorder := httptest.NewRecorder()
	processor.handler.ServeHTTP(recorder, request)
	responseBody := strings.TrimSpace(recorder.Body.String())
	if !json.Valid([]byte(responseBody)) {
		return fmt.Errorf("batch item %d returned non-JSON response", line)
	}
	state := "completed"
	if recorder.Code < 200 || recorder.Code >= 300 {
		state = "failed"
	}
	transaction, err := processor.cruds["llm_batch_item"].Connection().Beginx()
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	itemReference := daptinid.InterfaceToDIR(item["reference_id"])
	rows, _, err := processor.cruds["llm_batch_item"].GetRowsByWhereClauseWithTransaction("llm_batch_item", nil, transaction,
		goqu.Ex{"reference_id": itemReference[:]})
	if err != nil {
		return fmt.Errorf("load batch item: %w", err)
	}
	if len(rows) != 1 {
		return fmt.Errorf("batch item not found")
	}
	itemTable := processor.cruds["llm_batch_item"].TableInfo()
	model := api2go.NewApi2GoModelWithData("llm_batch_item", itemTable.Columns, int64(itemTable.DefaultPermission), itemTable.Relations, rows[0])
	model.SetAttributes(map[string]interface{}{"status": state, "response_status": recorder.Code,
		"response_request_id": recorder.Header().Get("X-Request-ID"), "response_body": responseBody})
	itemURL, _ := url.Parse("/llm_batch_item/" + itemReference.String())
	apiRequest := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodPatch, URL: itemURL}).WithContext(ctx)}
	if _, err := processor.cruds["llm_batch_item"].UpdateWithoutFilters(model, apiRequest, transaction); err != nil {
		return fmt.Errorf("record batch item result: %w", err)
	}
	batchRow, err := processor.batchRow(daptinid.InterfaceToDIR(string(batchID)), transaction)
	if err != nil {
		return err
	}
	var counts contract.BatchRequestCounts
	if err := json.Unmarshal([]byte(resource.StringOrEmpty(batchRow["request_counts"])), &counts); err != nil {
		return fmt.Errorf("decode batch request counts: %w", err)
	}
	if state == "completed" {
		counts.Completed++
	} else {
		counts.Failed++
	}
	encodedCounts, _ := json.Marshal(counts)
	if _, err := processor.batches.update(ctx, batchRow, map[string]interface{}{
		"request_counts": string(encodedCounts), "claim_expires_at": time.Now().UTC().Add(batchClaimDuration),
	}, transaction); err != nil {
		return err
	}
	return transaction.Commit()
}

func (processor *daptinBatchProcessor) finalize(ctx context.Context, batchID contract.ID) error {
	transaction, err := processor.cruds["llm_batch"].Connection().Beginx()
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	batchReference := daptinid.InterfaceToDIR(string(batchID))
	row, err := processor.batchRow(batchReference, transaction)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	row, err = processor.batches.update(ctx, row, map[string]interface{}{
		"status": string(contract.BatchStatusFinalizing), "finalizing_at": now, "claim_expires_at": now.Add(batchClaimDuration),
	}, transaction)
	if err != nil {
		return err
	}
	items, err := processor.allItems(row, transaction)
	if err != nil {
		return err
	}
	outputLines := make([]openai.BatchOutputLine, 0, len(items))
	for _, item := range items {
		line, conversionErr := resource.ResourceRowInt64(item["line_number"])
		if conversionErr != nil {
			return fmt.Errorf("decode batch item line: %w", conversionErr)
		}
		responseStatus, conversionErr := resource.ResourceRowInt64(item["response_status"])
		if conversionErr != nil || responseStatus < 100 || responseStatus > 599 {
			return fmt.Errorf("batch item %d has invalid response status", line)
		}
		outputLines = append(outputLines, openai.BatchOutputLine{ID: "batch_req_" + uuid.NewSHA1(uuid.Nil, []byte(string(batchID)+":"+strconv.FormatInt(line, 10))).String(),
			CustomID: resource.StringOrEmpty(item["custom_id"]), StatusCode: int(responseStatus),
			RequestID: resource.StringOrEmpty(item["response_request_id"]), Body: []byte(resource.StringOrEmpty(item["response_body"]))})
	}
	output, err := openai.EncodeBatchOutput(outputLines)
	if err != nil {
		return err
	}
	var expiration *contract.FileExpiration
	if row["output_expiration_seconds"] != nil {
		seconds, conversionErr := resource.ResourceRowInt64(row["output_expiration_seconds"])
		if conversionErr != nil {
			return conversionErr
		}
		expiration = &contract.FileExpiration{Anchor: "created_at", Seconds: seconds}
	}
	file, err := processor.files.create(ctx, contract.CreateFileRequest{Filename: "batch_" + string(batchID) + "_output.jsonl",
		ContentType: "application/jsonl", Purpose: contract.FilePurposeBatchOutput, Data: output, ExpiresAfter: expiration}, transaction)
	if err != nil {
		return err
	}
	row, err = processor.batchRow(batchReference, transaction)
	if err != nil {
		return err
	}
	completedAt := time.Now().UTC()
	if _, err := processor.batches.update(ctx, row, map[string]interface{}{"status": string(contract.BatchStatusCompleted),
		"completed_at": completedAt, "output_file_id": string(file.ID),
		"claim_expires_at": nil}, transaction); err != nil {
		return err
	}
	return transaction.Commit()
}

func (processor *daptinBatchProcessor) finishCancellation(ctx context.Context, row map[string]interface{}) error {
	transaction, err := processor.cruds["llm_batch"].Connection().Beginx()
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	current, err := processor.batchRow(daptinid.InterfaceToDIR(row["reference_id"]), transaction)
	if err != nil {
		return err
	}
	if _, err := processor.batches.update(ctx, current, map[string]interface{}{"status": string(contract.BatchStatusCancelled),
		"cancelled_at": time.Now().UTC(), "claim_expires_at": nil}, transaction); err != nil {
		return err
	}
	return transaction.Commit()
}

func (processor *daptinBatchProcessor) fail(ctx context.Context, row map[string]interface{}, code, message string, line *int64) error {
	transaction, err := processor.cruds["llm_batch"].Connection().Beginx()
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	current, err := processor.batchRow(daptinid.InterfaceToDIR(row["reference_id"]), transaction)
	if err != nil {
		return err
	}
	errorsJSON, _ := json.Marshal([]contract.BatchError{{Code: code, Message: message, Line: line}})
	if _, err := processor.batches.update(ctx, current, map[string]interface{}{"status": string(contract.BatchStatusFailed),
		"failed_at": time.Now().UTC(), "errors": string(errorsJSON), "claim_expires_at": nil}, transaction); err != nil {
		return err
	}
	return transaction.Commit()
}

func (processor *daptinBatchProcessor) batchOwner(row map[string]interface{}, transaction *sqlx.Tx) (*auth.SessionUser, error) {
	reference := daptinid.InterfaceToDIR(row["user_account_id"])
	userRow, err := processor.cruds["user_account"].GetReferenceIdToObjectWithTransaction("user_account", reference, transaction)
	if err != nil {
		return nil, fmt.Errorf("load batch owner: %w", err)
	}
	id, err := resource.ResourceRowInt64(userRow["id"])
	if err != nil {
		return nil, fmt.Errorf("decode batch owner: %w", err)
	}
	return &auth.SessionUser{UserId: id, UserReferenceId: reference,
		Groups: processor.cruds["user_account"].GetObjectUserGroupsByWhereWithTransaction("user_account", transaction, "id", id)}, nil
}

func (processor *daptinBatchProcessor) batchRow(reference daptinid.DaptinReferenceId, transaction *sqlx.Tx) (map[string]interface{}, error) {
	rows, _, err := processor.cruds["llm_batch"].GetRowsByWhereClauseWithTransaction("llm_batch", nil, transaction, goqu.Ex{"reference_id": reference[:]})
	if err != nil {
		return nil, fmt.Errorf("load batch: %w", err)
	}
	if len(rows) != 1 {
		return nil, fmt.Errorf("batch not found")
	}
	return rows[0], nil
}

func (processor *daptinBatchProcessor) allItems(batch map[string]interface{}, transaction *sqlx.Tx) ([]map[string]interface{}, error) {
	rows, _, err := processor.cruds["llm_batch_item"].GetRowsByWhereClauseWithTransaction("llm_batch_item", nil, transaction,
		goqu.Ex{"llm_batch_id": batch["id"]})
	if err != nil {
		return nil, err
	}
	lineNumbers := make(map[daptinid.DaptinReferenceId]int64, len(rows))
	for _, row := range rows {
		line, conversionErr := resource.ResourceRowInt64(row["line_number"])
		if conversionErr != nil {
			return nil, fmt.Errorf("decode batch item line: %w", conversionErr)
		}
		lineNumbers[daptinid.InterfaceToDIR(row["reference_id"])] = line
	}
	sort.Slice(rows, func(i, j int) bool {
		return lineNumbers[daptinid.InterfaceToDIR(rows[i]["reference_id"])] < lineNumbers[daptinid.InterfaceToDIR(rows[j]["reference_id"])]
	})
	return rows, nil
}
