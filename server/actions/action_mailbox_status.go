package actions

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
)

type mailboxStatusActionPerformer struct {
	cruds map[string]*resource.DbResource
}

func (d *mailboxStatusActionPerformer) Name() string {
	return "mail_box.status"
}

func (d *mailboxStatusActionPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	sessionUser, ok := inFields["sessionUser"].(*auth.SessionUser)
	if !ok || sessionUser == nil {
		return nil, nil, []error{fmt.Errorf("mail_box.status requires an authenticated session")}
	}
	tx := transaction
	createdTx := false
	if tx == nil {
		var err error
		tx, err = d.cruds["mail_box"].Connection().Beginx()
		if err != nil {
			return nil, nil, []error{err}
		}
		createdTx = true
		defer tx.Rollback()
	}

	pageSize := parseMailboxStatusInt(inFields["page_size"], 50)
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 500 {
		pageSize = 500
	}
	pageNumber := parseMailboxStatusInt(inFields["page_number"], 1)
	if pageNumber < 1 {
		pageNumber = 1
	}
	queries := make([]resource.Query, 0, 2)
	if mailAccountRef := strings.TrimSpace(fmt.Sprintf("%v", inFields["mail_account_id"])); mailAccountRef != "" && mailAccountRef != "<nil>" {
		queries = append(queries, resource.Query{ColumnName: "mail_account_id", Operator: "eq", Value: mailAccountRef})
	}
	if mailBoxRef := strings.TrimSpace(fmt.Sprintf("%v", inFields["mail_box_id"])); mailBoxRef != "" && mailBoxRef != "<nil>" {
		queries = append(queries, resource.Query{ColumnName: "reference_id", Operator: "eq", Value: mailBoxRef})
	}

	total, boxes, err := d.listMailboxes(queries, pageSize, pageNumber, sessionUser, tx)
	if err != nil {
		return nil, nil, []error{err}
	}

	statusRows := make([]map[string]interface{}, 0, len(boxes))
	for _, box := range boxes {
		boxReferenceID := daptinid.InterfaceToDIR(box["reference_id"])
		boxId, err := resource.GetReferenceIdToIdWithTransaction("mail_box", boxReferenceID, tx)
		if err != nil {
			return nil, nil, []error{err}
		}
		mailAccountReferenceID := daptinid.InterfaceToDIR(box["mail_account_id"])
		mailAccountId, err := resource.GetReferenceIdToIdWithTransaction("mail_account", mailAccountReferenceID, tx)
		if err != nil {
			return nil, nil, []error{err}
		}
		status, err := d.cruds["mail_box"].GetMailBoxStatus(mailAccountId, boxId, tx)
		if err != nil {
			return nil, nil, []error{err}
		}
		row := map[string]interface{}{
			"mail_box_id":     box["reference_id"],
			"mail_account_id": box["mail_account_id"],
			"name":            box["name"],
			"messages":        status.Messages,
			"recent":          status.Recent,
			"unseen":          status.Unseen,
			"uidnext":         status.UidNext,
			"uidvalidity":     status.UidValidity,
		}
		latest, err := d.latestMailMetadata(boxReferenceID, sessionUser, tx)
		if err != nil {
			return nil, nil, []error{err}
		}
		if latest != nil {
			row["latest_message"] = latest
		}
		statusRows = append(statusRows, row)
	}

	if createdTx {
		if err := tx.Commit(); err != nil {
			return nil, nil, []error{err}
		}
	}

	payload := map[string]interface{}{
		"data": statusRows,
		"pagination": map[string]interface{}{
			"total":        total,
			"per_page":     pageSize,
			"current_page": pageNumber,
			"from":         (pageNumber-1)*pageSize + 1,
			"to":           (pageNumber-1)*pageSize + len(statusRows),
		},
	}
	return nil, []actionresponse.ActionResponse{resource.NewActionResponse("mail_box.status", payload)}, nil
}

func (d *mailboxStatusActionPerformer) listMailboxes(queries []resource.Query, pageSize int, pageNumber int,
	sessionUser *auth.SessionUser, tx *sqlx.Tx) (uint, []map[string]interface{}, error) {
	requestURL, _ := url.Parse("/api/mail_box")
	request := api2go.Request{
		PlainRequest: (&http.Request{Method: http.MethodGet, URL: requestURL}).WithContext(
			context.WithValue(context.Background(), "user", sessionUser)),
		QueryParams: map[string][]string{
			"page[size]":   {strconv.Itoa(pageSize)},
			"page[number]": {strconv.Itoa(pageNumber)},
			"sort":         {"+name"},
		},
	}
	if len(queries) > 0 {
		encoded, err := json.Marshal(queries)
		if err != nil {
			return 0, nil, err
		}
		request.QueryParams["query"] = []string{string(encoded)}
	}
	total, responder, err := d.cruds["mail_box"].PaginatedFindAllWithTransaction(request, tx)
	if err != nil {
		return 0, nil, err
	}
	models, ok := responder.Result().([]api2go.Api2GoModel)
	if !ok {
		return 0, nil, fmt.Errorf("mail_box resource read returned %T", responder.Result())
	}
	boxes := make([]map[string]interface{}, 0, len(models))
	for _, model := range models {
		attributes := model.GetAttributes()
		attributes["reference_id"] = model.GetID()
		boxes = append(boxes, attributes)
	}
	return total, boxes, nil
}

func (d *mailboxStatusActionPerformer) latestMailMetadata(mailBoxReferenceID daptinid.DaptinReferenceId,
	sessionUser *auth.SessionUser, tx *sqlx.Tx) (map[string]interface{}, error) {
	encoded, err := json.Marshal([]resource.Query{
		{ColumnName: "mail_box_id", Operator: "eq", Value: mailBoxReferenceID.String()},
		{ColumnName: "deleted", Operator: "eq", Value: false},
	})
	if err != nil {
		return nil, err
	}
	requestURL, _ := url.Parse("/api/mail")
	request := api2go.Request{
		PlainRequest: (&http.Request{Method: http.MethodGet, URL: requestURL}).WithContext(
			context.WithValue(context.Background(), "user", sessionUser)),
		QueryParams: map[string][]string{
			"query":      {string(encoded)},
			"page[size]": {"1"},
			"sort":       {"-internal_date", "-id"},
		},
	}
	_, responder, err := d.cruds["mail"].PaginatedFindAllWithTransaction(request, tx)
	if err != nil {
		return nil, err
	}
	models, ok := responder.Result().([]api2go.Api2GoModel)
	if !ok || len(models) == 0 {
		return nil, nil
	}
	attributes := models[0].GetAttributes()
	return map[string]interface{}{
		"reference_id":  models[0].GetID(),
		"subject":       attributes["subject"],
		"from_address":  attributes["from_address"],
		"internal_date": attributes["internal_date"],
		"message_id":    attributes["message_id"],
		"seen":          attributes["seen"],
		"recent":        attributes["recent"],
		"uid":           attributes["uid"],
	}, nil
}

func parseMailboxStatusInt(value interface{}, fallback int) int {
	if value == nil {
		return fallback
	}
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func NewMailboxStatusActionPerformer(cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {
	return &mailboxStatusActionPerformer{cruds: cruds}, nil
}
