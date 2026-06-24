package actions

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/go-guerrilla"
	"github.com/artpar/go-guerrilla/mail"
	"github.com/daptin/daptin/server/actionresponse"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/emersion/go-msgauth/dkim"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

type mailSendActionPerformer struct {
	cruds              map[string]*resource.DbResource
	mailDaemon         *guerrilla.Daemon
	certificateManager *resource.CertificateManager
}

func (d *mailSendActionPerformer) Name() string {
	return "mail.send"
}

func (d *mailSendActionPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	responses := make([]actionresponse.ActionResponse, 0)

	mailTo := GetValueAsArrayString(inFields, "to")
	subject, _ := inFields["subject"].(string)
	mailFrom, _ := inFields["from"].(string)
	mailBody, _ := inFields["body"].(string)
	if mailFrom == "" {
		return nil, nil, []error{fmt.Errorf("missing required field: from")}
	}
	mailServerHostname := ""
	if mailServer, useMailServer := inFields["mail_server_hostname"]; useMailServer && mailServer != nil {
		mailServerHostname = strings.TrimSpace(fmt.Sprintf("%v", mailServer))
	}
	if mailServerHostname == "" {
		configuredHostname, err := d.cruds["mail"].ConfigStore.GetConfigValueFor("mail.default_server_hostname", "backend", transaction)
		if err == nil {
			mailServerHostname = strings.TrimSpace(configuredHostname)
		}
	}
	if mailServerHostname == "" {
		return nil, nil, []error{fmt.Errorf("missing required field: mail_server_hostname or backend config mail.default_server_hostname")}
	}
	attemptDelivery := mailSendAttemptDelivery(inFields)
	createdOutboxMails := make([]map[string]interface{}, 0)

	outboxUrl, _ := url.Parse("/api/outbox")
	outboxReq := api2go.Request{
		PlainRequest: &http.Request{
			Method: "POST",
			URL:    outboxUrl,
		},
	}

	mailServerObj, err := d.cruds["mail_server"].GetObjectByWhereClause("mail_server", "hostname", mailServerHostname, transaction)
	if err != nil {
		log.Errorf("Failed to get mail server details for sending as: %v", mailServerHostname)
		return nil, nil, []error{fmt.Errorf("failed to get mail server details for sending as: %v", mailServerHostname)}
	}

	mailFromAddress, err := mail.NewAddress(mailFrom)
	if err != nil {
		log.Errorf("Invalid mail from address [%v]: %v", mailFrom, err)
		return nil, nil, []error{err}
	}
	toAddresses := make([]mail.Address, 0, len(mailTo))
	for _, adr := range mailTo {
		mailToAddress, err := mail.NewAddress(adr)
		if err != nil {
			log.Errorf("Invalid mail to address [%v]: %v", adr, err)
			return nil, nil, []error{err}
		}
		toAddresses = append(toAddresses, *mailToAddress)
	}

	cert, err := d.certificateManager.GetTLSConfig(mailFromAddress.Host, false, transaction)
	if err != nil {
		log.Errorf("Failed to get private key for domain [%v]", mailFromAddress.Host)
		log.Errorf("Refusing to send mail without signing")
		return nil, nil, []error{err}
	}

	block, _ := pem.Decode([]byte(cert.PrivatePEMDecrypted))
	if block == nil {
		return nil, nil, []error{fmt.Errorf("failed to decode PEM block for domain [%v]", mailFromAddress.Host)}
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, nil, []error{err}
	}

	dkimSelector := "d1"
	if sel, err := d.cruds["mail"].ConfigStore.GetConfigValueFor("mail.dkim_selector", "backend", transaction); err == nil && sel != "" {
		dkimSelector = sel
	}

	options := &dkim.SignOptions{
		Selector:               dkimSelector,
		HeaderCanonicalization: dkim.CanonicalizationRelaxed,
		BodyCanonicalization:   dkim.CanonicalizationRelaxed,
		Domain:                 mailFromAddress.Host,
		Signer:                 privateKey,
	}

	messageID := fmt.Sprintf("<%s@%s>", uuid.NewString(), mailFromAddress.Host)
	newMailString := fmt.Sprintf("From: %s\r\nSubject: %s\r\nTo: %s\r\nDate: %s\r\nMessage-ID: %s\r\n",
		mailFromAddress.String(), subject, strings.Join(mailTo, ","), time.Now().Format(time.RFC822Z), messageID)
	newMailString = newMailString + "\r\n" + mailBody

	var b bytes.Buffer
	if err := dkim.Sign(&b, bytes.NewReader([]byte(newMailString)), options); err != nil {
		log.Errorf("Failed to sign outgoing mail via dkim, not sending it ahead [%v]", err)
		return nil, nil, []error{err}
	}

	finalMail := b.Bytes()
	log.Printf("Final Mail: From [%v] to [%v] via [%v]", mailFromAddress.String(), strings.Join(mailTo, ","), mailServerHostname)

	for _, toAddress := range toAddresses {
		outboxMailBody := d.cruds["outbox"].MailColumnValue("outbox", "mail", finalMail, subject)

		outboxModel := api2go.NewApi2GoModelWithData("outbox", nil, 0, nil, map[string]interface{}{
			"from_address":   mailFromAddress.String(),
			"to_address":     toAddress.String(),
			"to_host":        toAddress.Host,
			"mail_server_id": mailServerObj["reference_id"],
			"mail":           outboxMailBody,
			"sent":           false,
			"retry_count":    0,
			"next_retry_at":  time.Now(),
		})
		createdOutboxMail, err := d.cruds["outbox"].CreateWithoutFilter(outboxModel, outboxReq, transaction)
		if err != nil {
			log.Errorf("Failed to queue mail to outbox for [%v]: %v", toAddress.String(), err)
			return nil, nil, []error{err}
		}
		if attemptDelivery {
			createdOutboxMail = d.outboxMailWithNativeID(createdOutboxMail, transaction)
			createdOutboxMails = append(createdOutboxMails, createdOutboxMail)
		}
	}

	if attemptDelivery && len(createdOutboxMails) > 0 {
		outboxProcessor := &outboxProcessActionPerformer{cruds: d.cruds}
		for _, createdOutboxMail := range createdOutboxMails {
			outboxProcessor.processPendingMail(createdOutboxMail, transaction, false)
		}
	}

	return nil, responses, nil
}

func (d *mailSendActionPerformer) outboxMailWithNativeID(outboxMail map[string]interface{}, transaction *sqlx.Tx) map[string]interface{} {
	if id, ok := outboxMail["id"].(int64); ok && id > 0 {
		return outboxMail
	}
	referenceID, ok := outboxMail["reference_id"]
	if !ok || referenceID == nil {
		return outboxMail
	}
	outboxID, err := resource.GetReferenceIdToIdWithTransaction("outbox", daptinid.InterfaceToDIR(referenceID), transaction)
	if err != nil {
		log.Errorf("Failed to resolve outbox reference [%v] for immediate delivery: %v", referenceID, err)
		return outboxMail
	}
	outboxMail["id"] = outboxID
	return outboxMail
}

func mailSendAttemptDelivery(inFields map[string]interface{}) bool {
	for _, key := range []string{"send_immediately", "attempt_delivery"} {
		val, ok := inFields[key]
		if !ok || val == nil {
			continue
		}
		switch v := val.(type) {
		case bool:
			return v
		case string:
			parsed, err := strconv.ParseBool(strings.TrimSpace(v))
			if err == nil {
				return parsed
			}
		case int:
			return v != 0
		case int64:
			return v != 0
		case float64:
			return v != 0
		}
	}
	return false
}

func NewMailSendActionPerformer(cruds map[string]*resource.DbResource, mailDaemon *guerrilla.Daemon, certificateManager *resource.CertificateManager) (actionresponse.ActionPerformerInterface, error) {

	handler := mailSendActionPerformer{
		cruds:              cruds,
		mailDaemon:         mailDaemon,
		certificateManager: certificateManager,
	}

	return &handler, nil

}
