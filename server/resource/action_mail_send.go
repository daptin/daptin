package resource

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/artpar/api2go"
	"github.com/artpar/go-guerrilla"
	"github.com/artpar/go-guerrilla/mail"
	mta "github.com/artpar/go-smtp-mta"
	"github.com/emersion/go-msgauth/dkim"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

type mailSendActionPerformer struct {
	cruds              map[string]*DbResource
	mailDaemon         *guerrilla.Daemon
	certificateManager *CertificateManager
}

func (d *mailSendActionPerformer) Name() string {
	return "mail.send"
}

func (d *mailSendActionPerformer) DoAction(request Outcome, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	//log.Printf("Sync mail servers")
	responses := make([]ActionResponse, 0)

	mailTo := inFields["to"].(string)
	subject := inFields["subject"].(string)
	mailFrom := inFields["from"].(string)
	mailBody := inFields["body"].(string)
	mailServer, useMailServer := inFields["mail_server_hostname"]


	if !useMailServer {

		var body bytes.Buffer

		mimeHeaders := "MIME-version: 1.0;\nContent-Type: text/plain; charset=\"UTF-8\";\n\n"
		body.Write([]byte(fmt.Sprintf("Subject: %v \n%s\n\n", subject, mimeHeaders)))

		body.Write([]byte(mailBody))


		mailFromAddress, err := mail.NewAddress(mailFrom)
		if err != nil {
			log.Errorf("Mail from value is not a valid address [%v]: %v", mailFrom, err)
			return nil, nil, []error{err}
		}
		i2 := mta.Sender{
			Hostname: mailFromAddress.Host,
		}
		bodyBytes := body.Bytes()
		err = (&i2).Send(mailFrom, []string{mailTo}, bytes.NewReader(bodyBytes))
		if err != nil {
			log.Errorf("Failed to send mail to [%v]: %v", mailTo, err)
			log.Errorf("Mail: %v", string(bodyBytes))
			return nil, nil, []error{err}
		}

	} else {

		mailServerObj, err := d.cruds["mail_server"].GetObjectByWhereClause("mail_server", "hostname", mailServer)
		if err != nil {
			log.Errorf("Failed to get mail server details for sending as: %v", mailServer)
			return nil, nil, []error{fmt.Errorf("failed to get mail server details for sending as: %v", mailServer)}
		}

		var emailEnvelope *mail.Envelope
		mailFromAddress, err := mail.NewAddress(mailTo)
		if err != nil {
			log.Errorf("Invalid mail-to mailToAddress [%v]: %v", mailTo, err)
			return nil, nil, []error{err}
		}
		mailToAddress, err := mail.NewAddress(mailTo)
		if err != nil {
			log.Errorf("Invalid mail-to mailToAddress [%v]: %v", mailTo, err)
			return nil, nil, []error{err}
		}

		emailEnvelope = &mail.Envelope{
			MailFrom:       *mailFromAddress,
			RcptTo:         []mail.Address{*mailToAddress},
			Subject:        subject,
			DeliveryHeader: "Return-PATH: admin@" + mailServerObj["hostname"].(string) + "\n",
		}

		fmt.Printf("Original Mail: \n%s\n", string(mailBody))

		_, _, privateKeyPemByte, _, _, err := d.certificateManager.GetTLSConfig(emailEnvelope.MailFrom.Host, false)
		if err != nil {
			log.Errorf("Failed to get private key for domain [%v]", emailEnvelope.MailFrom.Host)
			log.Errorf("Refusing to send mail without signing")
			return nil, nil, []error{err}
		}

		//log.Printf("Private key [%v] %v", emailEnvelope.MailFrom.Host, string(privateKeyPemByte))
		//log.Printf("Public key [%v] %v", emailEnvelope.MailFrom.Host, string(publicKeyBytes))

		block, _ := pem.Decode([]byte(privateKeyPemByte))

		privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)

		if err != nil {
			return nil, nil, []error{err}
		}

		options := &dkim.SignOptions{
			Selector:               "d1",
			HeaderCanonicalization: dkim.CanonicalizationRelaxed,
			BodyCanonicalization:   dkim.CanonicalizationRelaxed,
			Domain:                 emailEnvelope.MailFrom.Host,
			Signer:                 privateKey,
		}

		newMailString := fmt.Sprintf("From: %s\r\nSubject: %s\r\nTo: %s\r\nDate: %s\r\n", emailEnvelope.MailFrom.String(), emailEnvelope.Subject, mailToAddress.String(), time.Now().Format(time.RFC822Z))

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
		fmt.Printf("Mail\n%s", string(finalMail))
		log.Printf("Final Mail: From [%v] to [%v] [%v]", emailEnvelope.MailFrom.String(), mailToAddress.String(), string(finalMail))

		i2 := mta.Sender{
			Hostname: emailEnvelope.MailFrom.Host,
		}
		err = (&i2).Send(emailEnvelope.MailFrom.String(), []string{mailToAddress.String()}, bytes.NewReader(finalMail))

		if err != nil {
			log.Errorf("Failed to send mail: %v", err)
			return nil, nil, []error{err}
		}

	}

	return nil, responses, nil
}

func NewMailSendActionPerformer(cruds map[string]*DbResource, mailDaemon *guerrilla.Daemon, certificateManager *CertificateManager) (ActionPerformerInterface, error) {

	handler := mailSendActionPerformer{
		cruds:              cruds,
		mailDaemon:         mailDaemon,
		certificateManager: certificateManager,
	}

	return &handler, nil

}
