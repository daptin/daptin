package actions

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
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
	attemptDelivery := mailSendAttemptDelivery(inFields)
	createdOutboxMailReferences := make([]daptinid.DaptinReferenceId, 0)

	mailFromAddress, err := mail.NewAddress(mailFrom)
	if err != nil {
		log.Errorf("Invalid mail from address [%v]: %v", mailFrom, err)
		return nil, nil, []error{err}
	}
	mailAccount, err := d.cruds["mail"].GetUserMailAccountRowByEmail(mailFromAddress.String(), transaction)
	if err != nil {
		return nil, nil, []error{err}
	}
	mailServerReference := daptinid.InterfaceToDIR(mailAccount["mail_server_id"])
	if mailServerReference == daptinid.NullReferenceId {
		return nil, nil, []error{fmt.Errorf("sender mail account has no mail server")}
	}
	mailServerObj, _, err := d.cruds["mail_server"].GetSingleRowByReferenceIdWithTransaction("mail_server", mailServerReference, nil, transaction)
	if err != nil {
		return nil, nil, []error{fmt.Errorf("failed to resolve sender mail server: %w", err)}
	}
	mailServerHostname, err := mailSendServerHostname(inFields, mailServerObj)
	if err != nil {
		return nil, nil, []error{err}
	}
	messageID := fmt.Sprintf("<%s@%s>", uuid.NewString(), mailFromAddress.Host)
	newMailString := fmt.Sprintf("From: %s\r\nSubject: %s\r\nTo: %s\r\nDate: %s\r\nMessage-ID: %s\r\n",
		mailFromAddress.String(), subject, strings.Join(mailTo, ","), time.Now().Format(time.RFC822Z), messageID)
	newMailString = newMailString + "\r\n" + mailBody

	finalMail, err := d.signMail(mailFromAddress.Host, []byte(newMailString), transaction)
	if err != nil {
		return nil, nil, []error{err}
	}
	log.Printf("Final Mail: From [%v] to [%v] via [%v]", mailFromAddress.String(), strings.Join(mailTo, ","), mailServerHostname)

	sent, err := d.cruds["mail"].AppendSentMailForSender(mailFromAddress.String(), finalMail, transaction)
	if err != nil {
		return nil, nil, []error{err}
	}
	createdOutboxMailReferences, err = d.cruds["mail"].QueueOutgoingMail(mailAccount, daptinid.InterfaceToDIR(sent["reference_id"]), finalMail, mailTo, transaction)
	if err != nil {
		return nil, nil, []error{err}
	}

	if attemptDelivery && len(createdOutboxMailReferences) > 0 {
		if transaction != nil {
			err = transaction.Commit()
			if err != nil {
				return nil, nil, []error{err}
			}

			newTransaction, beginErr := d.cruds["world"].Connection().Beginx()
			if beginErr != nil {
				return nil, nil, []error{beginErr}
			}
			*transaction = *newTransaction
		}
		d.deliverOutboxMailsAsync(createdOutboxMailReferences)
	}

	return nil, responses, nil
}

func mailSendServerHostname(inFields map[string]interface{}, mailServer map[string]interface{}) (string, error) {
	hostname, ok := mailServer["hostname"].(string)
	hostname = strings.TrimSpace(hostname)
	if !ok || hostname == "" {
		return "", fmt.Errorf("sender mail server has no hostname")
	}
	if asserted, present := inFields["mail_server_hostname"]; present && asserted != nil {
		assertedHostname := strings.TrimSpace(fmt.Sprintf("%v", asserted))
		if assertedHostname != "" && !strings.EqualFold(assertedHostname, hostname) {
			return "", fmt.Errorf("mail_server_hostname [%s] does not match sender mail server [%s]", assertedHostname, hostname)
		}
	}
	return hostname, nil
}

func (d *mailSendActionPerformer) deliverOutboxMailsAsync(outboxMailReferences []daptinid.DaptinReferenceId) {
	references := append([]daptinid.DaptinReferenceId(nil), outboxMailReferences...)
	go func() {
		outboxProcessor := &outboxProcessActionPerformer{cruds: d.cruds}
		for _, outboxMailReference := range references {
			outboxProcessor.processPendingMailByReference(outboxMailReference, false)
		}
	}()
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

func (d *mailSendActionPerformer) signMail(domain string, raw []byte, transaction *sqlx.Tx) ([]byte, error) {
	cert, err := d.certificateManager.GetTLSConfig(domain, false, transaction)
	if err != nil {
		log.Errorf("Failed to get private key for domain [%v]", domain)
		log.Errorf("Refusing to send mail without signing")
		return nil, err
	}

	block, _ := pem.Decode([]byte(cert.PrivatePEMDecrypted))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block for domain [%v]", domain)
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	dkimSelector := "d1"
	if sel, err := d.cruds["mail"].ConfigStore.GetConfigValueFor("mail.dkim_selector", "backend", transaction); err == nil && sel != "" {
		dkimSelector = sel
	}

	options := &dkim.SignOptions{
		Selector:               dkimSelector,
		HeaderCanonicalization: dkim.CanonicalizationRelaxed,
		BodyCanonicalization:   dkim.CanonicalizationRelaxed,
		Domain:                 domain,
		Signer:                 privateKey,
	}

	var b bytes.Buffer
	if err := dkim.Sign(&b, bytes.NewReader(raw), options); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
