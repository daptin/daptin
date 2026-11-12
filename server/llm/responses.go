package llm

import (
	"context"
	"net/http"
	"net/url"

	"github.com/artpar/api2go/v2"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	gateway "github.com/daptin/llmgateway"
	"github.com/daptin/llmgateway/contract"
	"github.com/jmoiron/sqlx"
)

type daptinResponses struct {
	cruds map[string]*resource.DbResource
}

func responseResourceRequest(ctx context.Context, method, path string) api2go.Request {
	requestURL, _ := url.Parse(path)
	return api2go.Request{PlainRequest: (&http.Request{Method: method, URL: requestURL}).WithContext(ctx)}
}

// Authorize runs before admission so denied retention cannot reserve quota or write usage.
func (store daptinResponses) Authorize(ctx context.Context, _ contract.Principal, previous string) error {
	user := daptinSessionUser(ctx)
	if user.UserReferenceId == daptinid.NullReferenceId {
		return &contract.Error{Code: contract.ErrorAuthentication, Message: "stored responses require a signed-in account", HTTPStatus: http.StatusUnauthorized}
	}
	config := resource.MeteringConfigForAction(store.cruds["llm_model"].TableInfo().Metering, "invoke")
	if resource.MeteringCaptureLimit(config) < gateway.MaxResponseHistoryBytes {
		return &contract.Error{Code: contract.ErrorUnavailable, Message: "capture policy does not allow complete response history", HTTPStatus: http.StatusServiceUnavailable}
	}
	if previous == "" {
		return nil
	}
	tx, err := store.cruds["api_usage"].Connection().Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	permission := store.cruds["api_usage"].GetObjectPermissionByReferenceId("api_usage", daptinid.InterfaceToDIR(previous), tx)
	if !permission.CanRefer(user.UserReferenceId, user.Groups, store.cruds["api_usage"].AdministratorGroupId) {
		return &contract.Error{Code: contract.ErrorPermission, Message: "response cannot be referenced", HTTPStatus: http.StatusForbidden}
	}
	return tx.Commit()
}

func (store daptinResponses) Begin(ctx context.Context, _ contract.Principal, token contract.ReservationToken, previousResponseID string) (contract.ID, error) {
	if daptinSessionUser(ctx).UserReferenceId == daptinid.NullReferenceId {
		return "", &contract.Error{Code: contract.ErrorAuthentication, Message: "stored responses require a signed-in account", HTTPStatus: http.StatusUnauthorized}
	}
	transaction, err := store.cruds["api_usage"].Connection().Beginx()
	if err != nil {
		return "", err
	}
	defer transaction.Rollback()
	reference, err := resource.NewMeteringService(&store.cruds).RetainPayload(daptinSessionUser(ctx), token.Opaque, previousResponseID, gateway.MaxResponseHistoryBytes, transaction)
	if err != nil {
		if status, ok := err.(interface{ Status() int }); ok {
			return "", &contract.Error{Code: contract.ErrorPermission, Message: "response cannot be referenced", HTTPStatus: status.Status()}
		}
		return "", err
	}
	if err := transaction.Commit(); err != nil {
		return "", err
	}
	return contract.ID(reference.String()), nil
}

func (store daptinResponses) Save(ctx context.Context, _ contract.Principal, stored gateway.StoredResponse) error {
	body, err := json.Marshal(stored.Response)
	if err != nil {
		return err
	}
	transaction, err := store.cruds["api_usage"].Connection().Beginx()
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	model, err := store.load(ctx, stored.ID, http.MethodPatch, transaction)
	if err != nil {
		return err
	}
	token := resource.StringOrEmpty(model.GetAllAsAttributes()["reservation_token"])
	if err := resource.NewMeteringService(&store.cruds).RecordResponseBody(daptinSessionUser(ctx), token, body, len(body), transaction); err != nil {
		return err
	}
	return transaction.Commit()
}

func (store daptinResponses) Load(ctx context.Context, _ contract.Principal, id contract.ID) (gateway.StoredResponse, error) {
	transaction, err := store.cruds["api_usage"].Connection().Beginx()
	if err != nil {
		return gateway.StoredResponse{}, err
	}
	defer transaction.Rollback()
	model, err := store.load(ctx, id, http.MethodGet, transaction)
	if err != nil {
		return gateway.StoredResponse{}, err
	}
	row := model.GetAllAsAttributes()
	stored := gateway.StoredResponse{ID: id, PreviousResponseID: resource.StringOrEmpty(row["parent_usage_id"])}
	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(resource.StringOrEmpty(row["metadata"])), &metadata); err != nil {
		return gateway.StoredResponse{}, err
	}
	if metadata["payload_retained"] != true || metadata["payload_deleted"] == true || metadata["request_body_truncated"] == true || metadata["response_body_truncated"] == true {
		return gateway.StoredResponse{}, responseNotFound()
	}
	var request contract.Request
	if err := json.Unmarshal([]byte(resource.StringOrEmpty(row["request_body"])), &request); err != nil {
		return gateway.StoredResponse{}, err
	}
	if request.Responses == nil {
		return gateway.StoredResponse{}, responseNotFound()
	}
	stored.Input = request.Responses.Input
	stored.PreviousResponseID = request.Responses.PreviousResponseID
	if raw := resource.StringOrEmpty(row["response_body"]); raw != "" {
		if err := json.Unmarshal([]byte(raw), &stored.Response); err != nil {
			return gateway.StoredResponse{}, err
		}
	}
	if stored.Response.Responses == nil {
		stored.Response.Responses = &contract.ResponsesResponse{ID: string(id), Status: "in_progress", Store: true}
	}
	stored.Response.Responses.ID = string(id)
	stored.Response.Responses.Store = true
	if err := transaction.Commit(); err != nil {
		return gateway.StoredResponse{}, err
	}
	return stored, nil
}

func (store daptinResponses) Delete(ctx context.Context, _ contract.Principal, id contract.ID) error {
	transaction, err := store.cruds["api_usage"].Connection().Beginx()
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	reference := daptinid.InterfaceToDIR(string(id))
	if reference == daptinid.NullReferenceId {
		return responseNotFound()
	}
	if _, err := store.load(ctx, id, http.MethodDelete, transaction); err != nil {
		return err
	}
	err = resource.NewMeteringService(&store.cruds).ForgetPayload(daptinSessionUser(ctx), reference, transaction)
	if err != nil {
		return responseResourceError(err)
	}
	return transaction.Commit()
}

func (store daptinResponses) load(ctx context.Context, id contract.ID, method string, transaction *sqlx.Tx) (api2go.Api2GoModel, error) {
	reference := daptinid.InterfaceToDIR(string(id))
	if reference == daptinid.NullReferenceId {
		return api2go.Api2GoModel{}, responseNotFound()
	}
	responder, err := store.cruds["api_usage"].FindOneWithTransaction(reference, responseResourceRequest(ctx, http.MethodGet, "/api_usage/"+string(id)), transaction)
	if err != nil {
		return api2go.Api2GoModel{}, responseResourceError(err)
	}
	model, ok := responder.Result().(api2go.Api2GoModel)
	if !ok || model.GetAllAsAttributes() == nil {
		return api2go.Api2GoModel{}, responseNotFound()
	}
	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(resource.StringOrEmpty(model.GetAllAsAttributes()["metadata"])), &metadata); err != nil {
		return api2go.Api2GoModel{}, err
	}
	if metadata["payload_retained"] != true || metadata["payload_deleted"] == true {
		return api2go.Api2GoModel{}, responseNotFound()
	}
	// Use the resource permission matching the actual operation, including for direct engine callers.
	user := daptinSessionUser(ctx)
	permission := store.cruds["api_usage"].GetObjectPermissionByReferenceId("api_usage", reference, transaction)
	allowed := false
	switch method {
	case http.MethodGet:
		allowed = permission.CanRead(user.UserReferenceId, user.Groups, store.cruds["api_usage"].AdministratorGroupId)
	case http.MethodPatch:
		allowed = permission.CanUpdate(user.UserReferenceId, user.Groups, store.cruds["api_usage"].AdministratorGroupId)
	case http.MethodDelete:
		allowed = permission.CanDelete(user.UserReferenceId, user.Groups, store.cruds["api_usage"].AdministratorGroupId)
	}
	if !allowed {
		return api2go.Api2GoModel{}, responseNotFound()
	}
	return model, nil
}

func responseResourceError(err error) error {
	if status, ok := err.(interface{ Status() int }); ok && (status.Status() == http.StatusForbidden || status.Status() == http.StatusNotFound) {
		return responseNotFound()
	}
	return err
}

func responseNotFound() *contract.Error {
	return &contract.Error{Code: contract.ErrorPermission, Message: "response not found", HTTPStatus: http.StatusNotFound}
}

var _ gateway.ResponseStore = daptinResponses{}
