package actions

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"net/mail"
	"strconv"
	"time"

	"github.com/artpar/api2go/v2"
	mta "github.com/artpar/go-smtp-mta"
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
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

	pendingMails, err := d.cruds["outbox"].GetAllObjectsWithWhereWithTransaction("outbox", transaction,
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

	now := time.Now()
	for _, pendingMail := range pendingMails {
		// Check next_retry_at
		if nextRetry, ok := pendingMail["next_retry_at"]; ok && nextRetry != nil {
			var retryTime time.Time
			switch v := nextRetry.(type) {
			case time.Time:
				retryTime = v
			case string:
				retryTime, _ = time.Parse(time.RFC3339, v)
			}
			if !retryTime.IsZero() && retryTime.After(now) {
				continue
			}
		}

		mailId := pendingMail["id"].(int64)

		// Claim this mail via Olric NX — if another node already claimed it, skip
		claimKey := fmt.Sprintf("outbox-claim-%v", mailId)
		if resource.OlricCache != nil {
			err := resource.OlricCache.Put(context.Background(), claimKey, true, olric.EX(10*time.Minute), olric.NX())
			if err != nil {
				continue
			}
		}

		fromAddress := pendingMail["from_address"].(string)
		toAddress := pendingMail["to_address"].(string)
		mailBase64 := pendingMail["mail"].(string)
		toHost := ""
		if h, ok := pendingMail["to_host"]; ok && h != nil {
			toHost = fmt.Sprintf("%v", h)
		}

		// Determine sender hostname for EHLO
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

		mailBytes, err := base64.StdEncoding.DecodeString(mailBase64)
		if err != nil {
			log.Errorf("Failed to decode outbox mail [%v]: %v", mailId, err)
			d.markFailed(mailId, "failed to decode mail body: "+err.Error(), pendingMail, transaction)
			continue
		}

		sender := mta.Sender{
			Hostname: senderHost,
		}

		// Send with 30s timeout to prevent hanging on unreachable MX
		sendDone := make(chan error, 1)
		go func() {
			sendDone <- (&sender).Send(fromAddress, []string{toAddress}, bytes.NewReader(mailBytes))
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
			continue
		}

		// Mark as sent
		query, args, err := statementbuilder.Squirrel.
			Update("outbox").Prepared(true).
			Set(goqu.Record{"sent": true}).
			Where(goqu.Ex{"id": mailId}).ToSQL()
		if err == nil {
			_, execErr := transaction.Exec(query, args...)
			if execErr != nil {
				log.Errorf("Failed to mark outbox mail [%v] as sent: %v", mailId, execErr)
			}
		}
		log.Printf("Outbox mail [%v] sent to [%v]", mailId, toAddress)
	}

	return nil, responses, nil
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

func NewOutboxProcessActionPerformer(cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {

	handler := outboxProcessActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
