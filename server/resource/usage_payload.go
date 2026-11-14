package resource

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/jmoiron/sqlx"
)

// RetainPayload makes an admitted capture addressable by its resource reference.
// The backend capture policy must allow a complete replayable payload.
func (m *MeteringService) RetainPayload(user *auth.SessionUser, token, parent string, requiredLimit int, tx *sqlx.Tx) (daptinid.DaptinReferenceId, error) {
	if user == nil || user.UserReferenceId == daptinid.NullReferenceId {
		return daptinid.NullReferenceId, errors.New("payload retention requires a signed-in account")
	}
	row, err := m.findUsageByToken(token, tx)
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	ownerID, err := ResourceRowInt64(row["user_account_id"])
	if err != nil || ownerID != user.UserId {
		return daptinid.NullReferenceId, errors.New("usage capture belongs to another user")
	}
	if usageCaptureLimit(row) < requiredLimit {
		return daptinid.NullReferenceId, errors.New("capture policy does not allow complete response history")
	}
	attrs := map[string]interface{}{}
	if parent != "" {
		reference := daptinid.InterfaceToDIR(parent)
		permission := (*m.cruds)["api_usage"].GetObjectPermissionByReferenceId("api_usage", reference, tx)
		if reference == daptinid.NullReferenceId || !permission.CanRefer(user.UserReferenceId, user.Groups, (*m.cruds)["api_usage"].AdministratorGroupId) {
			return daptinid.NullReferenceId, api2go.NewHTTPError(errors.New("response cannot be referenced"), "response cannot be referenced", http.StatusForbidden)
		}
		attrs["parent_usage_id"] = parent
	}
	metadata := map[string]interface{}{}
	if err := json.UnmarshalFromString(StringOrEmpty(row["metadata"]), &metadata); err != nil {
		return daptinid.NullReferenceId, err
	}
	if metadata["request_body_truncated"] == true {
		return daptinid.NullReferenceId, errors.New("captured request exceeds history size limit")
	}
	metadata["payload_retained"] = true
	attrs["metadata"] = ToJson(metadata)
	if err := m.updateUsagePayload(user, row, attrs, tx); err != nil {
		return daptinid.NullReferenceId, err
	}
	return daptinid.InterfaceToDIR(row["reference_id"]), nil
}

// ForgetPayload removes captured content without deleting accounting facts.
func (m *MeteringService) ForgetPayload(user *auth.SessionUser, reference daptinid.DaptinReferenceId, tx *sqlx.Tx) error {
	crud := (*m.cruds)["api_usage"]
	permission := crud.GetObjectPermissionByReferenceId("api_usage", reference, tx)
	if user == nil || !permission.CanDelete(user.UserReferenceId, user.Groups, crud.AdministratorGroupId) {
		return errors.New("payload deletion is not permitted")
	}
	row, err := crud.GetReferenceIdToObjectWithTransaction("api_usage", reference, tx)
	if err != nil {
		return err
	}
	metadata := map[string]interface{}{}
	if err := json.UnmarshalFromString(StringOrEmpty(row["metadata"]), &metadata); err != nil {
		return err
	}
	metadata["payload_retained"] = false
	metadata["payload_deleted"] = true
	return m.updateUsagePayload(user, row, map[string]interface{}{"request_body": nil, "response_body": nil, "request_body_encoding": nil, "response_body_encoding": nil, "metadata": ToJson(metadata)}, tx)
}

func (m *MeteringService) updateUsagePayload(user *auth.SessionUser, row, attrs map[string]interface{}, tx *sqlx.Tx) error {
	crud := (*m.cruds)["api_usage"]
	model := api2go.NewApi2GoModelWithData("api_usage", crud.TableInfo().Columns, int64(crud.TableInfo().DefaultPermission), crud.TableInfo().Relations, row)
	model.SetAttributes(attrs)
	request := api2go.Request{PlainRequest: (&http.Request{Method: http.MethodPatch, URL: &url.URL{Path: "/api_usage/" + daptinid.InterfaceToDIR(row["reference_id"]).String()}}).WithContext(context.WithValue(WithMeteringInternal(context.Background()), "user", m.internalUser(user)))}
	_, err := crud.UpdateWithTransaction(model, request, tx)
	return err
}
