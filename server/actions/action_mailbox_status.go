package actions

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

type mailboxStatusActionPerformer struct {
	cruds map[string]*resource.DbResource
}

func (d *mailboxStatusActionPerformer) Name() string {
	return "mail_box.status"
}

func (d *mailboxStatusActionPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
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
	offset := (pageNumber - 1) * pageSize

	where := make([]goqu.Ex, 0)
	if mailAccountRef := strings.TrimSpace(fmt.Sprintf("%v", inFields["mail_account_id"])); mailAccountRef != "" && mailAccountRef != "<nil>" {
		mailAccountId, err := resource.GetReferenceIdToIdWithTransaction("mail_account", daptinid.InterfaceToDIR(mailAccountRef), tx)
		if err != nil {
			return nil, nil, []error{err}
		}
		where = append(where, goqu.Ex{"mail_account_id": mailAccountId})
	}
	if mailBoxRef := strings.TrimSpace(fmt.Sprintf("%v", inFields["mail_box_id"])); mailBoxRef != "" && mailBoxRef != "<nil>" {
		mailBoxId, err := resource.GetReferenceIdToIdWithTransaction("mail_box", daptinid.InterfaceToDIR(mailBoxRef), tx)
		if err != nil {
			return nil, nil, []error{err}
		}
		where = append(where, goqu.Ex{"id": mailBoxId})
	}

	total, err := d.countMailboxes(where, tx)
	if err != nil {
		return nil, nil, []error{err}
	}
	boxes, err := d.listMailboxes(where, pageSize, offset, tx)
	if err != nil {
		return nil, nil, []error{err}
	}

	statusRows := make([]map[string]interface{}, 0, len(boxes))
	for _, box := range boxes {
		boxId, _ := box["id"].(int64)
		mailAccountId, _ := box["mail_account_id"].(int64)
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
		latest, err := d.latestMailMetadata(boxId, tx)
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
			"from":         offset + 1,
			"to":           offset + len(statusRows),
		},
	}
	return nil, []actionresponse.ActionResponse{resource.NewActionResponse("mail_box.status", payload)}, nil
}

func (d *mailboxStatusActionPerformer) countMailboxes(where []goqu.Ex, tx *sqlx.Tx) (int64, error) {
	query := statementbuilder.Squirrel.Select(goqu.COUNT("*")).Prepared(true).From("mail_box")
	for _, w := range where {
		query = query.Where(w)
	}
	sql, args, err := query.ToSQL()
	if err != nil {
		return 0, err
	}
	var total int64
	err = tx.QueryRowx(sql, args...).Scan(&total)
	return total, err
}

func (d *mailboxStatusActionPerformer) listMailboxes(where []goqu.Ex, pageSize int, offset int, tx *sqlx.Tx) ([]map[string]interface{}, error) {
	query := statementbuilder.Squirrel.Select("*").Prepared(true).From("mail_box").
		Order(goqu.C("name").Asc()).
		Limit(uint(pageSize)).
		Offset(uint(offset))
	for _, w := range where {
		query = query.Where(w)
	}
	sql, args, err := query.ToSQL()
	if err != nil {
		return nil, err
	}
	rows, err := tx.Queryx(sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rawRows, err := resource.RowsToMap(rows, "mail_box")
	if err != nil {
		return nil, err
	}
	boxes, _, err := d.cruds["mail_box"].ResultToArrayOfMapWithTransaction(rawRows, d.cruds["mail_box"].ColumnMap(), nil, tx)
	return boxes, err
}

func (d *mailboxStatusActionPerformer) latestMailMetadata(mailBoxId int64, tx *sqlx.Tx) (map[string]interface{}, error) {
	query, args, err := statementbuilder.Squirrel.
		Select("reference_id", "subject", "from_address", "internal_date", "message_id", "seen", "recent", "uid").
		Prepared(true).
		From("mail").
		Where(goqu.Ex{"mail_box_id": mailBoxId, "deleted": false}).
		Order(goqu.C("internal_date").Desc(), goqu.C("id").Desc()).
		Limit(1).
		ToSQL()
	if err != nil {
		return nil, err
	}
	rows, err := tx.Queryx(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rawRows, err := resource.RowsToMap(rows, "mail")
	if err != nil || len(rawRows) == 0 {
		return nil, err
	}
	mails, _, err := d.cruds["mail"].ResultToArrayOfMapWithTransaction(rawRows, d.cruds["mail"].ColumnMap(), nil, tx)
	if err != nil || len(mails) == 0 {
		return nil, err
	}
	return mails[0], nil
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
