package resource

import (
	"bytes"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/artpar/go-imap"
	"github.com/artpar/parsemail"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jmoiron/sqlx"
)

// MailSearchMetadata is the SQL-backed representation used by IMAP SEARCH.
// Raw RFC822 message data is deliberately not part of this representation.
type MailSearchMetadata struct {
	MessageID string
	From      string
	To        string
	Cc        string
	Bcc       string
	Sender    string
	ReplyTo   string
	Subject   string
	Body      string
	SentDate  time.Time
}

type MailSelection struct {
	ReferenceID    daptinid.DaptinReferenceId
	UID            uint32
	SequenceNumber uint32
}

func ExtractMailSearchMetadata(messageBytes []byte) (MailSearchMetadata, error) {
	parsed, err := parsemail.Parse(bytes.NewReader(messageBytes))
	if err != nil {
		return MailSearchMetadata{}, err
	}
	return mailSearchMetadataFromParsed(parsed), nil
}

func mailSearchMetadataFromParsed(parsed parsemail.Email) MailSearchMetadata {
	sender := ""
	if parsed.Sender != nil {
		sender = parsed.Sender.String()
	}
	return MailSearchMetadata{
		MessageID: strings.TrimSpace(parsed.MessageID),
		From:      joinMailAddresses(parsed.From),
		To:        joinMailAddresses(parsed.To),
		Cc:        joinMailAddresses(parsed.Cc),
		Bcc:       joinMailAddresses(parsed.Bcc),
		Sender:    sender,
		ReplyTo:   joinMailAddresses(parsed.ReplyTo),
		Subject:   parsed.Subject,
		Body:      parsed.TextBody,
		SentDate:  parsed.Date,
	}
}

func joinMailAddresses(addresses []*mail.Address) string {
	values := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if address != nil {
			values = append(values, address.String())
		}
	}
	return strings.Join(values, ", ")
}

func nullableMailDate(value time.Time) interface{} {
	if value.IsZero() {
		return nil
	}
	return value
}

// SearchMailBoxCandidates evaluates IMAP criteria in SQL. The returned public
// identities must still be read through the mail DbResource before they are
// exposed by a protocol adapter.
func (dbResource *DbResource) SearchMailBoxCandidates(mailBoxID int64, criteria *imap.SearchCriteria, transaction *sqlx.Tx) ([]MailSelection, error) {
	if criteria == nil {
		criteria = imap.NewSearchCriteria()
	}

	inner := statementbuilder.Squirrel.
		Select(
			"id", "reference_id", "uid", "subject", "from_address", "to_address", "cc_address", "bcc_address",
			"sender_address", "reply_to_address", "message_id", "body", "internal_date", "sent_date",
			"size", "seen", "recent", "deleted", "flags",
			goqu.L("ROW_NUMBER() OVER (ORDER BY COALESCE(NULLIF(uid, 0), id), id)").As("sequence_number"),
			goqu.L("MAX(COALESCE(NULLIF(uid, 0), id)) OVER ()").As("maximum_uid"),
			goqu.L("COUNT(*) OVER ()").As("message_count"),
		).
		From("mail").
		Where(goqu.Ex{"mail_box_id": mailBoxID})

	expression, err := compileIMAPSearchCriteria(criteria, "searchable")
	if err != nil {
		return nil, err
	}

	mailboxRows := statementbuilder.Squirrel.
		Select("mailbox_messages.*", effectiveUIDExpression("mailbox_messages").As("effective_uid")).
		From(inner.As("mailbox_messages"))
	queryBuilder := statementbuilder.Squirrel.
		Select("searchable.reference_id", "searchable.effective_uid", "searchable.sequence_number").
		From(mailboxRows.As("searchable")).
		Where(expression).
		Order(goqu.I("searchable.sequence_number").Asc()).
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

// SelectMailBoxCandidates resolves an IMAP sequence set to ordered public mail
// identities. Entity data is deliberately loaded later through DbResource.
func (dbResource *DbResource) SelectMailBoxCandidates(mailBoxID int64, uid bool, sequenceSet *imap.SeqSet, transaction *sqlx.Tx) ([]MailSelection, error) {
	inner := statementbuilder.Squirrel.
		Select(
			"id", "reference_id", "uid",
			goqu.L("ROW_NUMBER() OVER (ORDER BY COALESCE(NULLIF(uid, 0), id), id)").As("sequence_number"),
			goqu.L("MAX(COALESCE(NULLIF(uid, 0), id)) OVER ()").As("maximum_uid"),
			goqu.L("COUNT(*) OVER ()").As("message_count"),
		).
		From("mail").
		Where(goqu.Ex{"mail_box_id": mailBoxID})

	mailboxRows := statementbuilder.Squirrel.
		Select("mailbox_messages.*", effectiveUIDExpression("mailbox_messages").As("effective_uid")).
		From(inner.As("mailbox_messages"))
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

func effectiveUIDExpression(prefix string) exp.SQLFunctionExpression {
	return goqu.COALESCE(
		goqu.Func("NULLIF", goqu.I(prefix+".uid"), 0),
		goqu.I(prefix+".id"),
	)
}

func compileIMAPSearchCriteria(criteria *imap.SearchCriteria, prefix string) (goqu.Expression, error) {
	column := func(name string) exp.IdentifierExpression { return goqu.I(prefix + "." + name) }
	expressions := make([]goqu.Expression, 0)

	if criteria.SeqNum != nil {
		expressions = append(expressions, sequenceSetExpression(criteria.SeqNum, column("sequence_number"), column("message_count")))
	}
	if criteria.Uid != nil {
		expressions = append(expressions, sequenceSetExpression(criteria.Uid, column("effective_uid"), column("maximum_uid")))
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

	headerColumns := map[string]string{
		"bcc": "bcc_address", "cc": "cc_address", "from": "from_address", "to": "to_address",
		"subject": "subject", "message-id": "message_id", "sender": "sender_address", "reply-to": "reply_to_address",
	}
	for name, values := range criteria.Header {
		columnName, ok := headerColumns[strings.ToLower(name)]
		if !ok {
			return nil, fmt.Errorf("IMAP SEARCH HEADER %q is not supported by SQL-backed mail metadata", name)
		}
		for _, value := range values {
			if value == "" {
				expressions = append(expressions, goqu.L("COALESCE(?, '') <> ''", column(columnName)))
			} else {
				expressions = append(expressions, containsTextExpression(column(columnName), value))
			}
		}
	}
	for _, value := range criteria.Body {
		expressions = append(expressions, containsTextExpression(column("body"), value))
	}
	for _, value := range criteria.Text {
		textColumns := []string{"subject", "from_address", "to_address", "cc_address", "bcc_address", "sender_address", "reply_to_address", "message_id", "body"}
		matches := make([]goqu.Expression, 0, len(textColumns))
		for _, name := range textColumns {
			matches = append(matches, containsTextExpression(column(name), value))
		}
		expressions = append(expressions, goqu.Or(matches...))
	}
	for _, flag := range criteria.WithFlags {
		expressions = append(expressions, flagExpression(prefix, flag, true))
	}
	for _, flag := range criteria.WithoutFlags {
		expressions = append(expressions, flagExpression(prefix, flag, false))
	}
	for _, child := range criteria.Not {
		childExpression, err := compileIMAPSearchCriteria(child, prefix)
		if err != nil {
			return nil, err
		}
		expressions = append(expressions, goqu.L("NOT (?)", childExpression))
	}
	for _, pair := range criteria.Or {
		left, err := compileIMAPSearchCriteria(pair[0], prefix)
		if err != nil {
			return nil, err
		}
		right, err := compileIMAPSearchCriteria(pair[1], prefix)
		if err != nil {
			return nil, err
		}
		expressions = append(expressions, goqu.Or(left, right))
	}

	if len(expressions) == 0 {
		return goqu.L("1 = 1"), nil
	}
	return goqu.And(expressions...), nil
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

func containsTextExpression(column exp.IdentifierExpression, value string) goqu.Expression {
	if value == "" {
		return goqu.L("1 = 1")
	}
	value = escapeMailSearchLike(value)
	return goqu.L("LOWER(COALESCE(?, '')) LIKE ? ESCAPE '!'", column, "%"+strings.ToLower(value)+"%")
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
		normalized := escapeMailSearchLike(strings.ToLower(strings.TrimSpace(flag)))
		expression = goqu.Or(
			goqu.L("LOWER(COALESCE(?, '')) = ?", column("flags"), normalized),
			goqu.L("LOWER(COALESCE(?, '')) LIKE ? ESCAPE '!'", column("flags"), normalized+",%"),
			goqu.L("LOWER(COALESCE(?, '')) LIKE ? ESCAPE '!'", column("flags"), "%,"+normalized),
			goqu.L("LOWER(COALESCE(?, '')) LIKE ? ESCAPE '!'", column("flags"), "%,"+normalized+",%"),
		)
	}
	if present {
		return expression
	}
	return goqu.L("NOT (?)", expression)
}
