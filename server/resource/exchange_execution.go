package resource

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const (
	exchangeExecutionPending  = "pending"
	exchangeExecutionRetrying = "retrying"
	exchangeExecutionRunning  = "running"
	exchangeExecutionComplete = "complete"
)

// ExchangeExecutionService is the single durable boundary for exchanges whose failure policy is retry.
// The source mutation and execution record use the caller's transaction. Scheduled
// processors on every node compete through the execution row in the shared database.
type ExchangeExecutionService struct {
	cruds     *map[string]*DbResource
	exchanges map[string]ExchangeContract
	now       func() time.Time
}

func NewExchangeExecutionService(config *CmsConfig, cruds *map[string]*DbResource) *ExchangeExecutionService {
	exchanges := make(map[string]ExchangeContract, len(config.ExchangeContracts))
	for _, exchange := range config.ExchangeContracts {
		exchanges[exchange.Name] = exchange
	}
	return &ExchangeExecutionService{cruds: cruds, exchanges: exchanges, now: time.Now}
}

func (service *ExchangeExecutionService) Enqueue(exchange ExchangeContract, method string,
	row map[string]interface{}, request api2go.Request, transaction *sqlx.Tx) error {
	if service == nil || service.cruds == nil || (*service.cruds)["data_exchange_execution"] == nil {
		return fmt.Errorf("data_exchange_execution resource is unavailable")
	}

	referenceID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("create exchange execution id: %w", err)
	}
	envelopeRow := make(map[string]interface{}, len(row))
	for key, value := range row {
		envelopeRow[key] = value
	}
	if sourceReference := exchangeSourceReference(row["reference_id"]); sourceReference != "" {
		envelopeRow["reference_id"] = sourceReference
	}
	envelope, err := json.Marshal(envelopeRow)
	if err != nil {
		return fmt.Errorf("encode exchange execution envelope: %w", err)
	}

	executionUser, err := exchangeSessionUser(*service.cruds, exchange.AsUserId, transaction)
	if err != nil {
		return fmt.Errorf("resolve data exchange execution user: %w", err)
	}
	attributes := map[string]interface{}{
		"reference_id":        referenceID.String(),
		"exchange_name":       exchange.Name,
		"source_type":         StringOrEmpty(row["__type"]),
		"source_reference_id": exchangeSourceReference(row["reference_id"]),
		"source_method":       strings.ToLower(method),
		"envelope":            string(envelope),
		"state":               exchangeExecutionPending,
		"attempt_count":       int64(0),
		"next_attempt_at":     service.now().UTC(),
	}
	request.PlainRequest = request.PlainRequest.WithContext(context.WithValue(request.PlainRequest.Context(), "user", executionUser))
	model := api2go.NewApi2GoModelWithData("data_exchange_execution", nil, 0, nil, attributes)
	_, err = (*service.cruds)["data_exchange_execution"].CreateWithoutFilter(model, request, transaction)
	if err != nil {
		return fmt.Errorf("create data exchange execution: %w", err)
	}
	return nil
}

func exchangeSourceReference(value interface{}) string {
	if value == nil {
		return ""
	}
	if reference := daptinid.InterfaceToDIR(value); reference != daptinid.NullReferenceId {
		return reference.String()
	}
	if reference := StringOrEmpty(value); reference != "" {
		return reference
	}
	if reference := fmt.Sprint(value); reference != "<nil>" {
		return reference
	}
	return ""
}

func (service *ExchangeExecutionService) ProcessPending(transaction *sqlx.Tx) error {
	if service == nil || service.cruds == nil || (*service.cruds)["data_exchange_execution"] == nil {
		return fmt.Errorf("data_exchange_execution resource is unavailable")
	}
	now := service.now().UTC()
	query, args, err := statementbuilder.Squirrel.Select("id", "exchange_name", "envelope", "attempt_count", USER_ACCOUNT_ID_COLUMN).
		Prepared(true).From("data_exchange_execution").Where(
		exchangeExecutionEligible(now),
	).Order(goqu.C("created_at").Asc()).Limit(1).ToSQL()
	if err != nil {
		return fmt.Errorf("build pending data exchange execution query: %w", err)
	}
	resultRows, err := transaction.Queryx(query, args...)
	if err != nil {
		return fmt.Errorf("list pending data exchange executions: %w", err)
	}
	defer resultRows.Close()
	rows := make([]map[string]interface{}, 0, 1)
	for resultRows.Next() {
		row := make(map[string]interface{})
		if err := resultRows.MapScan(row); err != nil {
			return fmt.Errorf("scan pending data exchange execution: %w", err)
		}
		rows = append(rows, row)
	}
	if err := resultRows.Err(); err != nil {
		return fmt.Errorf("iterate pending data exchange executions: %w", err)
	}
	if err := resultRows.Close(); err != nil {
		return fmt.Errorf("close pending data exchange executions: %w", err)
	}
	for _, row := range rows {
		if err := service.processOne(row, transaction, service.now().UTC()); err != nil {
			return err
		}
	}
	return nil
}

func (service *ExchangeExecutionService) processOne(row map[string]interface{}, transaction *sqlx.Tx, now time.Time) error {
	id, err := ResourceRowInt64(row["id"])
	if err != nil {
		return fmt.Errorf("invalid data exchange execution id: %w", err)
	}
	attempts, err := ResourceRowInt64(row["attempt_count"])
	if err != nil {
		return fmt.Errorf("invalid attempt count for data exchange execution [%d]: %w", id, err)
	}
	claimSQL, claimArgs, err := statementbuilder.Squirrel.Update("data_exchange_execution").Prepared(true).
		Set(goqu.Record{
			"state":         exchangeExecutionRunning,
			"attempt_count": attempts + 1,
			"updated_at":    now,
		}).Where(
		goqu.Ex{"id": id},
		exchangeExecutionEligible(now),
	).ToSQL()
	if err != nil {
		return fmt.Errorf("build data exchange execution claim: %w", err)
	}
	result, err := transaction.Exec(claimSQL, claimArgs...)
	if err != nil {
		return fmt.Errorf("claim data exchange execution [%d]: %w", id, err)
	}
	claimed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read data exchange execution claim [%d]: %w", id, err)
	}
	if claimed == 0 {
		return nil
	}

	exchangeName := StringOrEmpty(row["exchange_name"])
	exchange, ok := service.exchanges[exchangeName]
	if !ok {
		return service.markRetry(id, attempts+1, now, fmt.Errorf("data exchange [%s] is unavailable", exchangeName), transaction)
	}
	asUserID, err := ResourceRowInt64(row[USER_ACCOUNT_ID_COLUMN])
	if err != nil || asUserID <= 0 {
		return service.markRetry(id, attempts+1, now, fmt.Errorf("data exchange execution user is unavailable"), transaction)
	}
	exchange.AsUserId = asUserID
	envelopeJSON, err := Decrypt((*service.cruds)["data_exchange_execution"].EncryptionSecret, StringOrEmpty(row["envelope"]))
	if err != nil {
		return service.markRetry(id, attempts+1, now, fmt.Errorf("decrypt exchange envelope: %w", err), transaction)
	}
	var envelope map[string]interface{}
	if err := json.Unmarshal([]byte(envelopeJSON), &envelope); err != nil {
		return service.markRetry(id, attempts+1, now, fmt.Errorf("decode exchange envelope: %w", err), transaction)
	}

	if _, err := transaction.Exec("SAVEPOINT data_exchange_execution_attempt"); err != nil {
		return fmt.Errorf("create data exchange execution savepoint: %w", err)
	}
	_, executeErr := NewExchangeExecution(exchange, service.cruds).Execute([]map[string]interface{}{envelope}, transaction)
	if executeErr != nil {
		if _, err := transaction.Exec("ROLLBACK TO SAVEPOINT data_exchange_execution_attempt"); err != nil {
			return fmt.Errorf("rollback failed data exchange execution [%d]: %w", id, err)
		}
		if _, err := transaction.Exec("RELEASE SAVEPOINT data_exchange_execution_attempt"); err != nil {
			return fmt.Errorf("release failed data exchange execution savepoint [%d]: %w", id, err)
		}
		return service.markRetry(id, attempts+1, now, executeErr, transaction)
	}
	if _, err := transaction.Exec("RELEASE SAVEPOINT data_exchange_execution_attempt"); err != nil {
		return fmt.Errorf("release data exchange execution savepoint [%d]: %w", id, err)
	}

	completeSQL, completeArgs, err := statementbuilder.Squirrel.Update("data_exchange_execution").Prepared(true).
		Set(goqu.Record{
			"state":        exchangeExecutionComplete,
			"completed_at": service.now().UTC(),
			"last_error":   nil,
			"updated_at":   service.now().UTC(),
		}).Where(goqu.Ex{"id": id, "state": exchangeExecutionRunning}).ToSQL()
	if err != nil {
		return fmt.Errorf("build data exchange completion: %w", err)
	}
	if _, err := transaction.Exec(completeSQL, completeArgs...); err != nil {
		return fmt.Errorf("complete data exchange execution [%d]: %w", id, err)
	}
	return nil
}

func (service *ExchangeExecutionService) markRetry(id int64, attempts int64, now time.Time,
	executionErr error, transaction *sqlx.Tx) error {
	// Provider errors can contain URLs, headers, or credentials. Keep durable
	// diagnostics deliberately non-sensitive.
	errorMessage := fmt.Sprintf("exchange attempt failed (%T)", executionErr)
	if executionErr == nil {
		errorMessage = "exchange attempt failed without an error"
	}
	retrySQL, retryArgs, err := statementbuilder.Squirrel.Update("data_exchange_execution").Prepared(true).
		Set(goqu.Record{
			"state":           exchangeExecutionRetrying,
			"next_attempt_at": now.Add(exchangeRetryDelay(attempts)),
			"last_error":      errorMessage,
			"updated_at":      service.now().UTC(),
		}).Where(goqu.Ex{"id": id, "state": exchangeExecutionRunning}).ToSQL()
	if err != nil {
		return fmt.Errorf("build data exchange retry: %w", err)
	}
	if _, err := transaction.Exec(retrySQL, retryArgs...); err != nil {
		return fmt.Errorf("schedule retry for data exchange execution [%d]: %w", id, err)
	}
	return nil
}

func exchangeExecutionEligible(now time.Time) exp.Expression {
	return goqu.And(
		goqu.Ex{"state": goqu.Op{"in": []string{exchangeExecutionPending, exchangeExecutionRetrying}}},
		goqu.Or(goqu.Ex{"next_attempt_at": nil}, goqu.Ex{"next_attempt_at": goqu.Op{"lte": now}}),
	)
}

func exchangeRetryDelay(attempts int64) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	if attempts > 12 {
		attempts = 12
	}
	delay := time.Duration(1<<attempts) * time.Second
	if delay > time.Hour {
		return time.Hour
	}
	return delay
}
