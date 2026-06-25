package actions

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/actionresponse"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	smtp "github.com/emersion/go-smtp"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

type outboxProcessActionPerformer struct {
	cruds map[string]*resource.DbResource
}

func (d *outboxProcessActionPerformer) Name() string {
	return "outbox.process"
}

func (d *outboxProcessActionPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	responses := make([]actionresponse.ActionResponse, 0)

	pendingMails, _, err := d.cruds["outbox"].GetRowsByWhereClauseWithTransaction("outbox", map[string]bool{"mail": true}, transaction,
		goqu.Ex{"sent": false},
		goqu.Ex{"retry_count": goqu.Op{"lt": 5}},
	)
	// Limit batch size to prevent OOM on large queues
	if len(pendingMails) > 100 {
		pendingMails = pendingMails[:100]
	}
	if err != nil {
		log.Errorf("Failed to query outbox: %v", err)
		return nil, responses, []error{err}
	}

	for _, pendingMail := range pendingMails {
		d.processPendingMail(pendingMail, transaction, true)
	}

	return nil, responses, nil
}

func (d *outboxProcessActionPerformer) processPendingMail(pendingMail map[string]interface{}, transaction *sqlx.Tx, respectNextRetry bool) bool {
	if respectNextRetry {
		if nextRetry, ok := pendingMail["next_retry_at"]; ok && nextRetry != nil {
			var retryTime time.Time
			switch v := nextRetry.(type) {
			case time.Time:
				retryTime = v
			case string:
				retryTime, _ = time.Parse(time.RFC3339, v)
			}
			if !retryTime.IsZero() && retryTime.After(time.Now()) {
				return false
			}
		}
	}

	mailId, ok := pendingMail["id"].(int64)
	if !ok {
		log.Errorf("Outbox entry has invalid id type: %T", pendingMail["id"])
		return false
	}

	// Claim this mail via Olric NX; if another node already claimed it, skip.
	claimKey := fmt.Sprintf("outbox-claim-%v", mailId)
	if resource.OlricCache != nil {
		err := resource.OlricCache.Put(context.Background(), claimKey, true, olric.EX(outboxClaimTTL()), olric.NX())
		if err != nil {
			log.Debugf("Outbox mail [%v] skipped because claim [%v] could not be acquired: %v", mailId, claimKey, err)
			return false
		}
	}

	fromAddress, ok := pendingMail["from_address"].(string)
	if !ok {
		log.Errorf("Outbox entry [%v] has invalid from_address type: %T", mailId, pendingMail["from_address"])
		return false
	}
	toAddress, ok := pendingMail["to_address"].(string)
	if !ok {
		log.Errorf("Outbox entry [%v] has invalid to_address type: %T", mailId, pendingMail["to_address"])
		return false
	}
	mailServerReference, ok := pendingMail["mail_server_id"]
	if !ok || mailServerReference == nil || fmt.Sprintf("%v", mailServerReference) == "" {
		err := fmt.Errorf("outbox mail [%v] missing mail_server_id", mailId)
		log.Errorf("%v", err)
		d.markFailed(mailId, err.Error(), pendingMail, transaction)
		return false
	}
	mailServerObj, mailServerDisplayId, err := d.getOutboxMailServer(mailServerReference, transaction)
	if err != nil {
		err = fmt.Errorf("outbox mail [%v] failed to resolve mail_server_id [%v]: %w", mailId, mailServerDisplayId, err)
		log.Errorf("%v", err)
		d.markFailed(mailId, err.Error(), pendingMail, transaction)
		return false
	}
	senderHost, ok := mailServerObj["hostname"].(string)
	if !ok || strings.TrimSpace(senderHost) == "" {
		err := fmt.Errorf("outbox mail [%v] mail_server [%v] has invalid hostname", mailId, mailServerDisplayId)
		log.Errorf("%v", err)
		d.markFailed(mailId, err.Error(), pendingMail, transaction)
		return false
	}
	senderHost = strings.TrimSpace(senderHost)

	claimStartedAt := time.Now()
	claimExpiresAt := claimStartedAt.Add(outboxClaimTTL())
	query, args, err := statementbuilder.Squirrel.
		Update("outbox").Prepared(true).
		Set(goqu.Record{"next_retry_at": claimExpiresAt}).
		Where(
			goqu.Ex{"id": mailId},
			goqu.Ex{"sent": false},
			goqu.Or(
				goqu.Ex{"next_retry_at": nil},
				goqu.Ex{"next_retry_at": goqu.Op{"lte": claimStartedAt}},
			),
		).ToSQL()
	if err != nil {
		log.Errorf("Failed to build outbox lease query for mail [%v]: %v", mailId, err)
		return false
	}
	leaseResult, err := transaction.Exec(query, args...)
	if err != nil {
		log.Errorf("Failed to lease outbox mail [%v]: %v", mailId, err)
		return false
	}
	rowsAffected, err := leaseResult.RowsAffected()
	if err != nil {
		log.Errorf("Failed to read lease result for outbox mail [%v]: %v", mailId, err)
		return false
	}
	if rowsAffected == 0 {
		log.Debugf("Outbox mail [%v] skipped because it was already leased or sent", mailId)
		if resource.OlricCache != nil {
			if _, deleteErr := resource.OlricCache.Delete(context.Background(), claimKey); deleteErr != nil {
				log.Debugf("Failed to release outbox claim [%v] after DB lease miss: %v", claimKey, deleteErr)
			}
		}
		return false
	}

	if transaction != nil {
		err := transaction.Commit()
		if err != nil {
			log.Errorf("Failed to commit transaction before sending outbox mail [%v]: %v", mailId, err)
			return false
		}
	}

	reloadTransaction, beginErr := d.cruds["outbox"].Connection().Beginx()
	if beginErr != nil {
		log.Errorf("Failed to begin reload transaction for outbox mail [%v]: %v", mailId, beginErr)
		return false
	}
	reloadedMails, _, err := d.cruds["outbox"].GetRowsByWhereClauseWithTransaction("outbox", map[string]bool{"mail": true}, reloadTransaction, goqu.Ex{"id": mailId})
	if err != nil || len(reloadedMails) == 0 {
		if err == nil {
			err = fmt.Errorf("outbox mail [%v] was not found after commit", mailId)
		}
		log.Errorf("Failed to reload outbox mail [%v]: %v", mailId, err)
		d.markFailed(mailId, "failed to reload mail body: "+err.Error(), pendingMail, reloadTransaction)
		if transaction != nil {
			*transaction = *reloadTransaction
		} else if commitErr := reloadTransaction.Commit(); commitErr != nil {
			log.Errorf("Failed to commit reload failure for outbox mail [%v]: %v", mailId, commitErr)
		}
		return false
	}

	reloadedMail := reloadedMails[0]
	mailStored, ok := reloadedMail["mail"]
	if !ok || mailStored == nil {
		err = fmt.Errorf("outbox entry [%v] has invalid mail type: %T", mailId, reloadedMail["mail"])
		log.Errorf("%v", err)
		d.markFailed(mailId, err.Error(), pendingMail, reloadTransaction)
		if transaction != nil {
			*transaction = *reloadTransaction
		} else if commitErr := reloadTransaction.Commit(); commitErr != nil {
			log.Errorf("Failed to commit invalid mail failure for outbox mail [%v]: %v", mailId, commitErr)
		}
		return false
	}

	mailBytes, err := d.cruds["outbox"].MailColumnBytes("outbox", "mail", mailStored)
	if err != nil {
		log.Errorf("Failed to read outbox mail [%v]: %v", mailId, err)
		d.markFailed(mailId, "failed to read mail body: "+err.Error(), pendingMail, reloadTransaction)
		if transaction != nil {
			*transaction = *reloadTransaction
		} else if commitErr := reloadTransaction.Commit(); commitErr != nil {
			log.Errorf("Failed to commit read failure for outbox mail [%v]: %v", mailId, commitErr)
		}
		return false
	}
	if err := reloadTransaction.Commit(); err != nil {
		log.Errorf("Failed to commit reload transaction for outbox mail [%v]: %v", mailId, err)
		if transaction != nil {
			newTransaction, beginErr := d.cruds["outbox"].Connection().Beginx()
			if beginErr == nil {
				*transaction = *newTransaction
			} else {
				log.Errorf("Failed to begin transaction after reload commit failure for outbox mail [%v]: %v", mailId, beginErr)
			}
		}
		return false
	}

	// Send with 30s timeout to prevent hanging on unreachable MX. No database
	// transaction is open while this external SMTP operation runs.
	sendDone := make(chan error, 1)
	go func() {
		sendDone <- sendOutboxMail(senderHost, fromAddress, []string{toAddress}, mailBytes)
	}()
	sendCtx, sendCancel := context.WithTimeout(context.Background(), 30*time.Second)
	select {
	case err = <-sendDone:
	case <-sendCtx.Done():
		err = fmt.Errorf("send timed out after 30s for [%v]", toAddress)
	}
	sendCancel()

	newTransaction, beginErr := d.cruds["outbox"].Connection().Beginx()
	if beginErr != nil {
		log.Errorf("Failed to begin transaction after sending outbox mail [%v]: %v", mailId, beginErr)
		return false
	}
	if transaction != nil {
		*transaction = *newTransaction
	} else {
		transaction = newTransaction
	}

	if err != nil {
		log.Errorf("Failed to send outbox mail [%v] to [%v]: %v", mailId, toAddress, err)
		d.markFailed(mailId, err.Error(), pendingMail, transaction)
		if commitErr := transaction.Commit(); commitErr != nil {
			log.Errorf("Failed to commit retry state for outbox mail [%v]: %v", mailId, commitErr)
			return false
		}
		if resource.OlricCache != nil {
			if _, deleteErr := resource.OlricCache.Delete(context.Background(), claimKey); deleteErr != nil {
				log.Debugf("Failed to release outbox claim [%v] after failed send: %v", claimKey, deleteErr)
			}
		}
		freshTransaction, beginErr := d.cruds["outbox"].Connection().Beginx()
		if beginErr != nil {
			log.Errorf("Failed to begin transaction after committing retry state for outbox mail [%v]: %v", mailId, beginErr)
			return false
		}
		if transaction != nil {
			*transaction = *freshTransaction
		}
		return false
	}

	query, args, err = statementbuilder.Squirrel.
		Update("outbox").Prepared(true).
		Set(goqu.Record{"sent": true}).
		Where(goqu.Ex{"id": mailId}).ToSQL()
	if err != nil {
		log.Errorf("Failed to build sent-state query for outbox mail [%v]: %v", mailId, err)
		return false
	}
	_, execErr := transaction.Exec(query, args...)
	if execErr != nil {
		log.Errorf("Failed to mark outbox mail [%v] as sent: %v", mailId, execErr)
		return false
	}
	if commitErr := transaction.Commit(); commitErr != nil {
		log.Errorf("Failed to commit sent state for outbox mail [%v]: %v", mailId, commitErr)
		return false
	}
	if resource.OlricCache != nil {
		if _, deleteErr := resource.OlricCache.Delete(context.Background(), claimKey); deleteErr != nil {
			log.Debugf("Failed to release outbox claim [%v] after sent state commit: %v", claimKey, deleteErr)
		}
	}
	freshTransaction, beginErr := d.cruds["outbox"].Connection().Beginx()
	if beginErr != nil {
		log.Errorf("Failed to begin transaction after committing sent state for outbox mail [%v]: %v", mailId, beginErr)
		return false
	}
	if transaction != nil {
		*transaction = *freshTransaction
	}
	log.Printf("Outbox mail [%v] sent to [%v]", mailId, toAddress)
	return true
}

func (d *outboxProcessActionPerformer) processPendingMailByReference(mailReferenceId daptinid.DaptinReferenceId, respectNextRetry bool) bool {
	transaction, err := d.cruds["outbox"].Connection().Beginx()
	if err != nil {
		log.Errorf("Failed to begin transaction for async outbox mail [%v]: %v", mailReferenceId.String(), err)
		return false
	}

	pendingMails, _, err := d.cruds["outbox"].GetRowsByWhereClauseWithTransaction("outbox", map[string]bool{"mail": true}, transaction, goqu.Ex{"reference_id": mailReferenceId[:]})
	if err != nil {
		_ = transaction.Rollback()
		log.Errorf("Failed to load async outbox mail [%v]: %v", mailReferenceId.String(), err)
		return false
	}
	if len(pendingMails) == 0 {
		_ = transaction.Rollback()
		log.Errorf("Async outbox mail [%v] was not found after mail.send commit", mailReferenceId.String())
		return false
	}

	processed := d.processPendingMail(pendingMails[0], transaction, respectNextRetry)
	if err := transaction.Commit(); err != nil {
		log.Errorf("Failed to commit async outbox mail [%v] state: %v", mailReferenceId.String(), err)
		return false
	}
	return processed
}

func outboxClaimTTL() time.Duration {
	ttl := 90 * time.Second
	if configured := strings.TrimSpace(os.Getenv("DAPTIN_OUTBOX_CLAIM_TTL_SECONDS")); configured != "" {
		seconds, err := strconv.Atoi(configured)
		if err == nil && seconds > 0 {
			ttl = time.Duration(seconds) * time.Second
		}
	}
	return ttl
}

func (d *outboxProcessActionPerformer) getOutboxMailServer(mailServerReference interface{}, transaction *sqlx.Tx) (map[string]interface{}, string, error) {
	switch v := mailServerReference.(type) {
	case int64:
		mailServerObj, _, err := d.cruds["mail_server"].GetSingleRowById("mail_server", v, nil, transaction)
		return mailServerObj, strconv.FormatInt(v, 10), err
	case int:
		id := int64(v)
		mailServerObj, _, err := d.cruds["mail_server"].GetSingleRowById("mail_server", id, nil, transaction)
		return mailServerObj, strconv.FormatInt(id, 10), err
	case int32:
		id := int64(v)
		mailServerObj, _, err := d.cruds["mail_server"].GetSingleRowById("mail_server", id, nil, transaction)
		return mailServerObj, strconv.FormatInt(id, 10), err
	case float64:
		id := int64(v)
		if float64(id) == v {
			mailServerObj, _, err := d.cruds["mail_server"].GetSingleRowById("mail_server", id, nil, transaction)
			return mailServerObj, strconv.FormatInt(id, 10), err
		}
	case string:
		trimmed := strings.TrimSpace(v)
		if parsed, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			mailServerObj, _, err := d.cruds["mail_server"].GetSingleRowById("mail_server", parsed, nil, transaction)
			return mailServerObj, trimmed, err
		}
		mailServerRef := daptinid.InterfaceToDIR(trimmed)
		mailServerObj, err := d.cruds["mail_server"].GetObjectByWhereClause("mail_server", "reference_id", mailServerRef[:], transaction)
		return mailServerObj, mailServerRef.String(), err
	}

	mailServerRef := daptinid.InterfaceToDIR(mailServerReference)
	mailServerObj, err := d.cruds["mail_server"].GetObjectByWhereClause("mail_server", "reference_id", mailServerRef[:], transaction)
	return mailServerObj, mailServerRef.String(), err
}

func (d *outboxProcessActionPerformer) markFailed(mailId int64, lastError string, pendingMail map[string]interface{}, transaction *sqlx.Tx) {
	retryCount := int64(0)
	if rc, ok := pendingMail["retry_count"]; ok && rc != nil {
		switch v := rc.(type) {
		case int64:
			retryCount = v
		case float64:
			retryCount = int64(v)
		case string:
			parsed, _ := strconv.ParseInt(v, 10, 64)
			retryCount = parsed
		}
	}
	retryCount++

	backoffMinutes := math.Pow(2, float64(retryCount))
	nextRetry := time.Now().Add(time.Duration(backoffMinutes) * time.Minute)

	query, args, err := statementbuilder.Squirrel.
		Update("outbox").Prepared(true).
		Set(goqu.Record{
			"retry_count":   retryCount,
			"last_error":    lastError,
			"next_retry_at": nextRetry,
		}).
		Where(goqu.Ex{"id": mailId}).ToSQL()
	if err == nil {
		_, execErr := transaction.Exec(query, args...)
		if execErr != nil {
			log.Errorf("Failed to update outbox retry for mail [%v]: %v", mailId, execErr)
		}
	}
}

func sendOutboxMail(ehloHostname, from string, to []string, message []byte) error {
	return sendOutboxMailWith(ehloHostname, from, to, message, net.LookupMX, sendOutboxSMTPData)
}

func sendOutboxMailWith(
	ehloHostname, from string,
	to []string,
	message []byte,
	lookupMX func(string) ([]*net.MX, error),
	send func(mxHost, ehloHostname, from string, to []string, message []byte) error,
) error {
	for _, addr := range to {
		_, domain, err := splitOutboxAddress(addr)
		if err != nil {
			return err
		}

		mxs, err := lookupMX(domain)
		if err != nil {
			return err
		}
		if len(mxs) == 0 {
			mxs = []*net.MX{{Host: domain}}
		}

		var lastErr error
		delivered := false
		for _, mx := range mxs {
			if err := send(mx.Host, ehloHostname, from, []string{addr}, message); err != nil {
				lastErr = err
				continue
			}
			delivered = true
			break
		}
		if !delivered {
			if lastErr != nil {
				return lastErr
			}
			return fmt.Errorf("no MX accepted mail for [%v]", addr)
		}
	}

	return nil
}

func sendOutboxSMTPData(mxHost, ehloHostname, from string, to []string, message []byte) error {
	serverName := strings.TrimSuffix(mxHost, ".")
	c, err := smtp.Dial(net.JoinHostPort(serverName, "25"))
	if err != nil {
		return err
	}
	defer c.Close()

	if ehloHostname != "" {
		if err := c.Hello(ehloHostname); err != nil {
			return err
		}
	}

	if ok, _ := c.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{ServerName: serverName}
		if err := c.StartTLS(tlsConfig); err != nil {
			return err
		}
	}

	if err := c.Mail(from, nil); err != nil {
		return err
	}
	for _, addr := range to {
		if err := c.Rcpt(addr); err != nil {
			return err
		}
	}

	wc, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := io.Copy(wc, bytes.NewReader(message)); err != nil {
		_ = wc.Close()
		return err
	}
	if err := wc.Close(); err != nil {
		return err
	}

	return c.Quit()
}

func splitOutboxAddress(addr string) (local, domain string, err error) {
	parts := strings.SplitN(addr, "@", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("mta: invalid mail address")
	}
	return parts[0], parts[1], nil
}

func NewOutboxProcessActionPerformer(cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {

	handler := outboxProcessActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
