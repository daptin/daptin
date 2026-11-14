package resource

import (
	"errors"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
	"time"
)

const OutboxSMTPUncertainPrefix = "SMTP delivery uncertain: "

// ClaimOutboxMail records a conservative terminal retry state before the caller
// commits and starts SMTP. If that process crashes or loses its live claim, the
// database still prevents automatic redelivery. A confirmed SMTP result replaces
// the marker; a pre-delivery failure restores the normal bounded retry schedule.
func (dbResource *DbResource) ClaimOutboxMail(mailID int64, now time.Time, tx *sqlx.Tx) (bool, error) {
	if tx == nil {
		return false, errors.New("outbox claim requires a transaction")
	}
	query, args, err := statementbuilder.Squirrel.Update("outbox").Prepared(true).
		Set(goqu.Record{"retry_count": 5, "next_retry_at": nil, "last_error": OutboxSMTPUncertainPrefix + "attempt started; completion has not been recorded"}).
		Where(goqu.Ex{"id": mailID, "sent": false, "retry_count": goqu.Op{"lt": 5}}, goqu.Or(goqu.Ex{"next_retry_at": nil}, goqu.Ex{"next_retry_at": goqu.Op{"lte": now}})).ToSQL()
	if err != nil {
		return false, err
	}
	result, err := tx.Exec(query, args...)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}
