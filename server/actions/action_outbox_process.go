package actions

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math"
	"net"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/actionresponse"
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
		err := resource.OlricCache.Put(context.Background(), claimKey, true, olric.EX(10*time.Minute), olric.NX())
		if err != nil {
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
	mailStored, ok := pendingMail["mail"]
	if !ok || mailStored == nil {
		log.Errorf("Outbox entry [%v] has invalid mail type: %T", mailId, pendingMail["mail"])
		return false
	}
	toHost := ""
	if h, ok := pendingMail["to_host"]; ok && h != nil {
		toHost = fmt.Sprintf("%v", h)
	}

	// Determine sender hostname for EHLO.
	senderHost := toHost
	if senderHost == "" {
		addr, err := mail.ParseAddress(fromAddress)
		if err == nil {
			parts := bytes.SplitN([]byte(addr.Address), []byte("@"), 2)
			if len(parts) == 2 {
				senderHost = string(parts[1])
			}
		}
	}

	mailBytes, err := d.cruds["outbox"].MailColumnBytes("outbox", "mail", mailStored)
	if err != nil {
		log.Errorf("Failed to read outbox mail [%v]: %v", mailId, err)
		d.markFailed(mailId, "failed to read mail body: "+err.Error(), pendingMail, transaction)
		return false
	}

	// Send with 30s timeout to prevent hanging on unreachable MX.
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

	if err != nil {
		log.Errorf("Failed to send outbox mail [%v] to [%v]: %v", mailId, toAddress, err)
		d.markFailed(mailId, err.Error(), pendingMail, transaction)
		return false
	}

	query, args, err := statementbuilder.Squirrel.
		Update("outbox").Prepared(true).
		Set(goqu.Record{"sent": true}).
		Where(goqu.Ex{"id": mailId}).ToSQL()
	if err == nil {
		_, execErr := transaction.Exec(query, args...)
		if execErr != nil {
			log.Errorf("Failed to mark outbox mail [%v] as sent: %v", mailId, execErr)
			return false
		}
	}
	log.Printf("Outbox mail [%v] sent to [%v]", mailId, toAddress)
	return true
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

func sendOutboxMail(hostname, from string, to []string, message []byte) error {
	return sendOutboxMailWith(hostname, from, to, message, net.LookupMX, sendOutboxSMTPData)
}

func sendOutboxMailWith(
	hostname, from string,
	to []string,
	message []byte,
	lookupMX func(string) ([]*net.MX, error),
	send func(mxHost, hostname, from string, to []string, message []byte) error,
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
			if err := send(mx.Host, hostname, from, []string{addr}, message); err != nil {
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

func sendOutboxSMTPData(mxHost, hostname, from string, to []string, message []byte) error {
	serverName := strings.TrimSuffix(mxHost, ".")
	c, err := smtp.Dial(net.JoinHostPort(serverName, "25"))
	if err != nil {
		return err
	}
	defer c.Close()

	if hostname != "" {
		if err := c.Hello(hostname); err != nil {
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
