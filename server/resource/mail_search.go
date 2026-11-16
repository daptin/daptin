package resource

import (
	"database/sql"
	"strings"
	"time"

	"github.com/artpar/go-imap"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jmoiron/sqlx"
)

type MailSelection struct {
	ReferenceID    daptinid.DaptinReferenceId
	UID            uint32
	SequenceNumber uint32
}

func nullableMailDate(value time.Time) interface{} {
	if value.IsZero() {
		return nil
	}
	return value
}

type imapSearchCandidate struct {
	MailSelection
	predicates []int
	size       int64
}

// selectIMAPSearchCandidates projects metadata predicates and prunes rows that
// cannot match. It never reads MIME. The protocol adapter applies authorization
// before the residual content predicates are evaluated.
func (dbResource *DbResource) selectIMAPSearchCandidates(mailboxID int64, criteria *imap.SearchCriteria, tx *sqlx.Tx, view *IMAPSequenceView) ([]imapSearchCandidate, *imapSearchNode, error) {
	const alias = "searchable"
	plan := planIMAPSearch(criteria, alias, view)
	columns := []interface{}{alias + ".reference_id", alias + ".effective_uid", alias + ".sequence_number",
		goqu.L("COALESCE(?, 0)", goqu.I(alias+".size"))}
	var predicates []interface{}
	if !plan.exact {
		plan.project(&predicates)
	}
	dataset := statementbuilder.Squirrel.Select(append(columns, predicates...)...).
		From(imapMailboxRows(mailboxID, "internal_date", "sent_date", "size", "seen", "recent", "deleted", "flags").As(alias))
	if view != nil {
		if len(*view) == 0 {
			return nil, plan, nil
		}
		dataset = dataset.Where(goqu.I(alias + ".effective_uid").Lte(int64((*view)[len(*view)-1])))
	}
	if plan.possible != nil {
		dataset = dataset.Where(plan.possible)
	}
	query, args, err := dataset.Order(goqu.I(alias + ".sequence_number").Asc()).Prepared(true).ToSQL()
	if err != nil {
		return nil, nil, err
	}
	rows, err := tx.Queryx(query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var candidates []imapSearchCandidate
	for rows.Next() {
		row := imapSearchCandidate{predicates: make([]int, len(predicates))}
		dest := []interface{}{&row.ReferenceID, &row.UID, &row.SequenceNumber, &row.size}
		for index := range row.predicates {
			dest = append(dest, &row.predicates[index])
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, nil, err
		}
		if view != nil {
			row.SequenceNumber = view.sequence(row.UID)
			if row.SequenceNumber == 0 {
				continue
			}
		}
		candidates = append(candidates, row)
	}
	return candidates, plan, rows.Err()
}

// SelectMailBoxCandidates resolves an IMAP sequence set to ordered public mail
// identities. Entity data is deliberately loaded later through DbResource.
func (dbResource *DbResource) SelectMailBoxCandidates(mailBoxID int64, uid bool, sequenceSet *imap.SeqSet, transaction *sqlx.Tx) ([]MailSelection, error) {
	mailboxRows := imapMailboxRows(mailBoxID)
	valueColumn := goqu.I("selected.sequence_number")
	maximumColumn := goqu.I("selected.message_count")
	if uid {
		valueColumn = goqu.I("selected.effective_uid")
		maximumColumn = goqu.I("selected.maximum_uid")
	}
	queryBuilder := statementbuilder.Squirrel.
		Select("selected.reference_id", "selected.effective_uid", "selected.sequence_number").
		From(mailboxRows.As("selected")).
		Where(sequenceSetExpression(sequenceSet, valueColumn, maximumColumn)).
		Order(goqu.I("selected.sequence_number").Asc()).
		Prepared(true)

	query, args, err := queryBuilder.ToSQL()
	if err != nil {
		return nil, err
	}
	rows, err := transaction.Queryx(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	selections := make([]MailSelection, 0)
	for rows.Next() {
		var selection MailSelection
		if err := rows.Scan(&selection.ReferenceID, &selection.UID, &selection.SequenceNumber); err != nil {
			return nil, err
		}
		selections = append(selections, selection)
	}
	return selections, rows.Err()
}

func imapMailboxRows(mailboxID int64, additionalColumns ...interface{}) *goqu.SelectDataset {
	columns := append([]interface{}{
		"id", "reference_id", "uid",
		goqu.L("ROW_NUMBER() OVER (ORDER BY COALESCE(NULLIF(uid, 0), id), id)").As("sequence_number"),
		goqu.L("MAX(COALESCE(NULLIF(uid, 0), id)) OVER ()").As("maximum_uid"),
		goqu.L("COUNT(*) OVER ()").As("message_count"),
	}, additionalColumns...)
	inner := statementbuilder.Squirrel.Select(columns...).From("mail").Where(goqu.Ex{"mail_box_id": mailboxID})
	return statementbuilder.Squirrel.Select("mailbox_messages.*", effectiveUIDExpression("mailbox_messages").As("effective_uid")).From(inner.As("mailbox_messages"))
}

func effectiveUIDExpression(prefix string) exp.SQLFunctionExpression {
	return goqu.COALESCE(
		goqu.Func("NULLIF", goqu.I(prefix+".uid"), 0),
		goqu.I(prefix+".id"),
	)
}

func imapSearchMetadata(criteria *imap.SearchCriteria, prefix string, view *IMAPSequenceView) goqu.Expression {
	column := func(name string) exp.IdentifierExpression { return goqu.I(prefix + "." + name) }
	expressions := make([]goqu.Expression, 0)

	if criteria.SeqNum != nil {
		if view != nil {
			expressions = append(expressions, sequenceSetExpression(view.resolve(criteria.SeqNum, false), column("effective_uid"), column("maximum_uid")))
		} else {
			expressions = append(expressions, sequenceSetExpression(criteria.SeqNum, column("sequence_number"), column("message_count")))
		}
	}
	if criteria.Uid != nil {
		uids := criteria.Uid
		if view != nil {
			uids = view.resolve(uids, true)
		}
		expressions = append(expressions, sequenceSetExpression(uids, column("effective_uid"), column("maximum_uid")))
	}
	if !criteria.Since.IsZero() {
		expressions = append(expressions, column("internal_date").Gte(criteria.Since))
	}
	if !criteria.Before.IsZero() {
		expressions = append(expressions, column("internal_date").Lt(criteria.Before))
	}
	if !criteria.SentSince.IsZero() {
		expressions = append(expressions, column("sent_date").Gte(criteria.SentSince))
	}
	if !criteria.SentBefore.IsZero() {
		expressions = append(expressions, column("sent_date").Lt(criteria.SentBefore))
	}
	if criteria.Larger > 0 {
		expressions = append(expressions, column("size").Gt(criteria.Larger))
	}
	if criteria.Smaller > 0 {
		expressions = append(expressions, column("size").Lt(criteria.Smaller))
	}

	for _, flag := range criteria.WithFlags {
		expressions = append(expressions, flagExpression(prefix, flag, true))
	}
	for _, flag := range criteria.WithoutFlags {
		expressions = append(expressions, flagExpression(prefix, flag, false))
	}
	if len(expressions) == 0 {
		return nil
	}
	// Unknown metadata (such as a missing Date field) does not match a
	// positive criterion, and is therefore a match under NOT.
	return goqu.L("COALESCE(?, FALSE)", goqu.And(expressions...))
}

func sequenceSetExpression(set *imap.SeqSet, value, maximum exp.IdentifierExpression) goqu.Expression {
	ranges := make([]goqu.Expression, 0, len(set.Set))
	number := func(value uint32) exp.LiteralExpression {
		return goqu.L("CAST(? AS DECIMAL(10, 0))", int64(value))
	}
	for _, item := range set.Set {
		switch {
		case item.Start == 0 && item.Stop == 0:
			ranges = append(ranges, value.Eq(maximum))
		case item.Stop == 0:
			ranges = append(ranges, goqu.Or(value.Gte(number(item.Start)), value.Eq(maximum)))
		case item.Start == item.Stop:
			ranges = append(ranges, value.Eq(number(item.Start)))
		default:
			ranges = append(ranges, goqu.And(value.Gte(number(item.Start)), value.Lte(number(item.Stop))))
		}
	}
	if len(ranges) == 0 {
		return goqu.L("1 = 0")
	}
	return goqu.Or(ranges...)
}

func escapeMailSearchLike(value string) string {
	value = strings.ReplaceAll(value, "!", "!!")
	value = strings.ReplaceAll(value, "%", "!%")
	return strings.ReplaceAll(value, "_", "!_")
}

func flagExpression(prefix, flag string, present bool) goqu.Expression {
	column := func(name string) exp.IdentifierExpression { return goqu.I(prefix + "." + name) }
	var expression goqu.Expression
	switch strings.ToLower(flag) {
	case strings.ToLower(imap.SeenFlag):
		expression = column("seen").Eq(true)
	case strings.ToLower(imap.RecentFlag):
		expression = column("recent").Eq(true)
	case strings.ToLower(imap.DeletedFlag):
		expression = column("deleted").Eq(true)
	default:
		normalized := strings.ToLower(strings.TrimSpace(flag))
		escaped := escapeMailSearchLike(normalized)
		expression = goqu.Or(
			goqu.L("LOWER(COALESCE(?, '')) = ?", column("flags"), normalized),
			goqu.L("LOWER(COALESCE(?, '')) LIKE ? ESCAPE '!'", column("flags"), escaped+",%"),
			goqu.L("LOWER(COALESCE(?, '')) LIKE ? ESCAPE '!'", column("flags"), "%,"+escaped),
			goqu.L("LOWER(COALESCE(?, '')) LIKE ? ESCAPE '!'", column("flags"), "%,"+escaped+",%"),
		)
	}
	if present {
		return expression
	}
	return goqu.L("NOT (?)", expression)
}

// imapSelectionUIDs projects only the metadata needed to announce precise
// membership changes. Use the same effective UID and order as FETCH/SEARCH.
func (dbResource *DbResource) imapSelectionUIDs(mailBoxID int64, tx *sqlx.Tx) ([]uint32, uint32, error) {
	uid := effectiveUIDExpression("mail")
	query, args, err := statementbuilder.Squirrel.Select(uid, goqu.I("mail.recent")).
		From("mail").Where(goqu.Ex{"mail_box_id": mailBoxID}).Order(uid.Asc(), goqu.I("mail.id").Asc()).Prepared(true).ToSQL()
	if err != nil {
		return nil, 0, err
	}
	rows, err := tx.Queryx(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var uids []uint32
	var recent uint32
	for rows.Next() {
		var uid uint32
		var isRecent sql.NullBool
		if err = rows.Scan(&uid, &isRecent); err != nil {
			return nil, 0, err
		}
		uids = append(uids, uid)
		if isRecent.Bool {
			recent++
		}
	}
	return uids, recent, rows.Err()
}
