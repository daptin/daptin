package resource

import (
	"bytes"
	encodingjson "encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/doug-martin/goqu/v9"
	"github.com/emersion/go-ical"
	"github.com/jmoiron/sqlx"
)

type itipReconcileDeliveryAction struct{ cruds map[string]*DbResource }

func NewITIPReconcileDeliveryAction(cruds map[string]*DbResource) actionresponse.ActionPerformerInterface {
	return &itipReconcileDeliveryAction{cruds: cruds}
}

func (*itipReconcileDeliveryAction) Name() string { return "itip.reconcile_delivery" }

func (a *itipReconcileDeliveryAction) DoAction(_ actionresponse.Outcome, fields map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	caller, _ := fields["sessionUser"].(*auth.SessionUser)
	if tx == nil || caller == nil || !IsAdminWithTransaction(caller, tx) {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("calendar delivery reconciliation denied"), "calendar delivery reconciliation denied", http.StatusForbidden)}
	}
	b := NewCalDAVBackend(a.cruds, caller, nil)
	query, err := encodingjson.Marshal([]Query{{ColumnName: "direction", Operator: "=", Value: "outbound"}, {ColumnName: "state", Operator: "=", Value: "submitted"}})
	if err != nil {
		return nil, nil, []error{err}
	}
	processed := 0
	// Each run processes a bounded number of terminal messages. Pending
	// messages remain in the result set, so scan successive pages for work.
	for page := 1; ; page++ {
		req := b.request(http.MethodGet, "/api/cal_mail")
		req.QueryParams = map[string][]string{"query": {string(query)}, "page[size]": {"100"}, "page[number]": {fmt.Sprint(page)}}
		_, response, err := a.cruds["cal_mail"].PaginatedFindAllWithTransaction(req, tx)
		if err != nil {
			return nil, nil, []error{err}
		}
		models, ok := response.Result().([]api2go.Api2GoModel)
		if !ok {
			return nil, nil, []error{errors.New("invalid calendar mail list")}
		}
		if len(models) == 0 {
			break
		}
		for _, model := range models {
			messageRef := daptinid.InterfaceToDIR(model.GetID())
			if messageRef == daptinid.NullReferenceId {
				return nil, nil, []error{errors.New("calendar mail has no reference ID")}
			}
			done, err := a.reconcileMessage(b, messageRef, tx)
			if err != nil {
				return nil, nil, []error{err}
			}
			if done {
				processed++
				if processed == 100 {
					return nil, []actionresponse.ActionResponse{NewActionResponse("itip.reconcile_delivery", map[string]interface{}{"processed": processed})}, nil
				}
			}
		}
		if len(models) < 100 {
			break
		}
	}
	return nil, []actionresponse.ActionResponse{NewActionResponse("itip.reconcile_delivery", map[string]interface{}{"processed": processed})}, nil
}

func (a *itipReconcileDeliveryAction) reconcileMessage(admin *DaptinDAVBackend, messageRef daptinid.DaptinReferenceId, tx *sqlx.Tx) (bool, error) {
	message, _, err := a.cruds["cal_mail"].GetSingleRowByReferenceIdWithTransaction("cal_mail", messageRef, nil, tx)
	if err != nil {
		return false, err
	}
	if message["direction"] != "outbound" || message["state"] != "submitted" {
		return false, nil
	}
	outboxRef := daptinid.InterfaceToDIR(message["outbox_id"])
	if outboxRef == daptinid.NullReferenceId {
		return false, errors.New("outbound calendar mail has no outbox")
	}
	outbox, _, err := a.cruds["outbox"].GetSingleRowByReferenceIdWithTransaction("outbox", outboxRef, nil, tx)
	if err != nil {
		return false, err
	}
	status, state := outboxScheduleStatus(outbox)
	if state == "" {
		return false, nil
	}
	collectionRef := daptinid.InterfaceToDIR(message["collection_id"])
	eventRef := daptinid.InterfaceToDIR(message["event_reference"])
	if collectionRef != daptinid.NullReferenceId && eventRef != daptinid.NullReferenceId {
		present, err := GetReferenceIdByWhereClauseWithTransaction(calendarCollectionTable, tx, goqu.Ex{"reference_id": collectionRef[:]})
		if err != nil {
			return false, err
		}
		if len(present) == 0 {
			goto updateMessage
		}
		collection, _, err := a.cruds[calendarCollectionTable].GetSingleRowByReferenceIdWithTransaction(calendarCollectionTable, collectionRef, nil, tx)
		if err != nil {
			return false, err
		}
		if err := admin.lockCalendarCollection(collection, tx); err != nil {
			return false, err
		}
		// A later resend, edit, or reply owns the visible status. Finishing an
		// older SMTP attempt must never replace its state.
		collectionID, err := GetReferenceIdToIdWithTransaction(calendarCollectionTable, collectionRef, tx)
		if err != nil {
			return false, err
		}
		related, _, err := a.cruds["cal_mail"].GetRowsByWhereClauseWithTransaction("cal_mail", nil, tx, goqu.Ex{
			"collection_id": collectionID, "event_reference": eventRef.String(), "direction": "outbound",
			"uid": message["uid"], "recurrence_id": message["recurrence_id"], "recipient_address": message["recipient_address"],
		})
		if err != nil {
			return false, err
		}
		messageID, err := ResourceRowInt64(message["id"])
		if err != nil {
			return false, err
		}
		latest := true
		for _, other := range related {
			id, err := ResourceRowInt64(other["id"])
			if err != nil {
				return false, err
			}
			if id > messageID {
				latest = false
				break
			}
		}
		if latest {
			existing, err := GetReferenceIdByWhereClauseWithTransaction(calendarObjectTable, tx, goqu.Ex{"reference_id": eventRef[:]})
			if err != nil {
				return false, err
			}
			if len(existing) != 0 {
				event, _, err := a.cruds[calendarObjectTable].GetSingleRowByReferenceIdWithTransaction(calendarObjectTable, eventRef, nil, tx)
				if err != nil {
					return false, err
				}
				if daptinid.InterfaceToDIR(event["collection_id"]) != collectionRef {
					goto updateMessage
				}
				ownerRef := daptinid.InterfaceToDIR(collection["user_account_id"])
				ownerID, err := GetReferenceIdToIdWithTransaction("user_account", ownerRef, tx)
				if err != nil {
					return false, err
				}
				owner, err := exchangeSessionUser(a.cruds, ownerID, tx)
				if err != nil {
					return false, err
				}
				b := NewCalDAVBackend(a.cruds, owner, nil)
				content, err := b.contentBytes(calendarObjectTable, event)
				if err != nil {
					return false, err
				}
				calendar, err := ical.NewDecoder(bytes.NewReader(content)).Decode()
				if err != nil {
					return false, err
				}
				if reconcileScheduleStatus(calendar, StringOrEmpty(message["uid"]), StringOrEmpty(message["recurrence_id"]), StringOrEmpty(message["recipient_address"]), status) {
					var encoded bytes.Buffer
					if err := ical.NewEncoder(&encoded).Encode(calendar); err != nil {
						return false, err
					}
					requestPath := StringOrEmpty(event["rpath"])
					delete(event, "version")
					if err := b.calendarUpdate(calendarObjectTable, requestPath, event, map[string]interface{}{
						"content": b.contentValue(calendarObjectTable, requestPath, ical.MIMEType, encoded.Bytes()),
					}, tx); err != nil {
						return false, err
					}
				}
			}
		}
	}
updateMessage:
	model := api2go.NewApi2GoModelWithData("cal_mail", nil, 0, nil, map[string]interface{}{"reference_id": messageRef.String(), "state": state})
	_, err = a.cruds["cal_mail"].updateAfterAuthorizationWithTransaction(model, admin.request(http.MethodPatch, "/api/cal_mail/"+messageRef.String()), tx)
	if err != nil {
		return false, fmt.Errorf("update calendar mail delivery: %w", err)
	}
	return true, nil
}

func outboxScheduleStatus(outbox map[string]interface{}) (string, string) {
	sent := fmt.Sprint(outbox["sent"])
	if sent == "true" || sent == "1" {
		return "1.1", "sent"
	}
	if retries, err := ResourceRowInt64(outbox["retry_count"]); err == nil && retries >= 5 {
		return "5.1", "failed"
	}
	return "", ""
}

func reconcileScheduleStatus(calendar *ical.Calendar, uid, recurrence, recipient, status string) bool {
	for _, event := range itipComponents(calendar) {
		if itipProp(event.Props.Get("UID")) != uid || itipProp(event.Props.Get("RECURRENCE-ID")) != recurrence {
			continue
		}
		for _, name := range []string{"ATTENDEE", "ORGANIZER"} {
			for i := range event.Props[name] {
				property := &event.Props[name][i]
				if itipAddress(property) != strings.ToLower(recipient) || property.Params.Get("SCHEDULE-STATUS") != "1.0" {
					continue
				}
				property.Params.Set("SCHEDULE-STATUS", status)
				return true
			}
		}
	}
	return false
}
