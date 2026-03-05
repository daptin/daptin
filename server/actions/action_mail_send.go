package actions

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/go-guerrilla"
	"github.com/artpar/go-guerrilla/mail"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/emersion/go-msgauth/dkim"
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
	mailServer, useMailServer := inFields["mail_server_hostname"]

	outboxUrl, _ := url.Parse("/api/outbox")
	outboxReq := api2go.Request{
		PlainRequest: &http.Request{
			Method: "POST",
			URL:    outboxUrl,
		},
	}

	if !useMailServer {

		var body bytes.Buffer

		mimeHeaders := "MIME-version: 1.0;\r\nContent-Type: text/plain; charset=\"UTF-8\";\r\n"
		body.Write([]byte(fmt.Sprintf("Subject: %v\r\n%s\r\n", subject, mimeHeaders)))

		body.Write([]byte(mailBody))

		_, err := mail.NewAddress(mailFrom)
		if err != nil {
			log.Errorf("Mail from value is not a valid address [%v]: %v", mailFrom, err)
			return nil, nil, []error{err}
		}
		bodyBytes := body.Bytes()

		for _, to := range mailTo {
			toAddr, toErr := mail.NewAddress(to)
			toHost := ""
			if toErr == nil {
				toHost = toAddr.Host
			}
			outboxModel := api2go.NewApi2GoModelWithData("outbox", nil, 0, nil, map[string]interface{}{
				"from_address":  mailFrom,
				"to_address":    to,
				"to_host":       toHost,
				"mail":          base64.StdEncoding.EncodeToString(bodyBytes),
				"sent":          false,
				"retry_count":   0,
				"next_retry_at": time.Now(),
			})
			_, err = d.cruds["outbox"].CreateWithoutFilter(outboxModel, outboxReq, transaction)
			if err != nil {
				log.Errorf("Failed to queue mail to outbox for [%v]: %v", to, err)
				return nil, nil, []error{err}
			}
		}

	} else {

		mailServerObj, err := d.cruds["mail_server"].GetObjectByWhereClause("mail_server", "hostname", mailServer, transaction)
		if err != nil {
			log.Errorf("Failed to get mail server details for sending as: %v", mailServer)
			return nil, nil, []error{fmt.Errorf("failed to get mail server details for sending as: %v", mailServer)}
		}

		var emailEnvelope *mail.Envelope
		mailFromAddress, err := mail.NewAddress(mailFrom)
		if err != nil {
			log.Errorf("Invalid mail-to mailToAddress [%v]: %v", mailTo, err)
			return nil, nil, []error{err}
		}
		toAddresses := make([]mail.Address, 0)
		for _, adr := range mailTo {
			mailToAddress, err := mail.NewAddress(adr)
			resource.CheckErr(err, "Failed to parse address: %v", adr)
			toAddresses = append(toAddresses, *mailToAddress)

		}
		if err != nil {
			log.Errorf("Invalid mail-to mailToAddress [%v]: %v", mailTo, err)
			return nil, nil, []error{err}
		}

		emailEnvelope = &mail.Envelope{
			MailFrom:       *mailFromAddress,
			RcptTo:         toAddresses,
			Subject:        subject,
			DeliveryHeader: "Return-PATH: admin@" + mailServerObj["hostname"].(string) + "\n",
		}

		cert, err := d.certificateManager.GetTLSConfig(emailEnvelope.MailFrom.Host, false, transaction)
		if err != nil {
			log.Errorf("Failed to get private key for domain [%v]", emailEnvelope.MailFrom.Host)
			log.Errorf("Refusing to send mail without signing")
			return nil, nil, []error{err}
		}

		block, _ := pem.Decode([]byte(cert.PrivatePEMDecrypted))
		if block == nil {
			return nil, nil, []error{fmt.Errorf("failed to decode PEM block for domain [%v]", emailEnvelope.MailFrom.Host)}
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
			Domain:                 emailEnvelope.MailFrom.Host,
			Signer:                 privateKey,
		}

		newMailString := fmt.Sprintf("From: %s\r\nSubject: %s\r\nTo: %s\r\nDate: %s\r\n",
			emailEnvelope.MailFrom.String(), emailEnvelope.Subject, strings.Join(mailTo, ","), time.Now().Format(time.RFC822Z))

		for headerName, headerValue := range emailEnvelope.Header {
			headerNameSmall := strings.ToLower(headerName)

			if headerNameSmall == "date" || headerNameSmall == "to" || headerNameSmall == "from" || headerNameSmall == "subject" {
				continue
			}
			for _, val := range headerValue {
				newMailString = newMailString + headerName + ": " + val + "\r\n"
			}
		}

		newMailString = newMailString + "\r\n" + mailBody

		var b bytes.Buffer
		if err := dkim.Sign(&b, bytes.NewReader([]byte(newMailString)), options); err != nil {
			log.Errorf("Failed to sign outgoing mail via dkim, not sending it ahead [%v]", err)
			return nil, nil, []error{err}
		}

		finalMail := b.Bytes()
		log.Printf("Final Mail: From [%v] to [%v]", emailEnvelope.MailFrom.String(), strings.Join(mailTo, ","))

		for _, to := range mailTo {
			rcptAddr, rcptErr := mail.NewAddress(to)
			rcptHost := ""
			if rcptErr == nil {
				rcptHost = rcptAddr.Host
			}
			outboxModel := api2go.NewApi2GoModelWithData("outbox", nil, 0, nil, map[string]interface{}{
				"from_address":  emailEnvelope.MailFrom.String(),
				"to_address":    to,
				"to_host":       rcptHost,
				"mail":          base64.StdEncoding.EncodeToString(finalMail),
				"sent":          false,
				"retry_count":   0,
				"next_retry_at": time.Now(),
			})
			_, err = d.cruds["outbox"].CreateWithoutFilter(outboxModel, outboxReq, transaction)
			if err != nil {
				log.Errorf("Failed to queue mail to outbox for [%v]: %v", to, err)
				return nil, nil, []error{err}
			}
		}

	}

	return nil, responses, nil
}

func NewMailSendActionPerformer(cruds map[string]*resource.DbResource, mailDaemon *guerrilla.Daemon, certificateManager *resource.CertificateManager) (actionresponse.ActionPerformerInterface, error) {

	handler := mailSendActionPerformer{
		cruds:              cruds,
		mailDaemon:         mailDaemon,
		certificateManager: certificateManager,
	}

	return &handler, nil

}
