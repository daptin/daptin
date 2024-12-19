package resource

import (
	"fmt"
	"github.com/artpar/api2go"
	"github.com/artpar/go-guerrilla"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

type awsMailSendActionPerformer struct {
	cruds              map[string]*DbResource
	mailDaemon         *guerrilla.Daemon
	certificateManager *CertificateManager
	encryptionSecret   []byte
}

func (d *awsMailSendActionPerformer) Name() string {
	return "aws.mail.send"
}

func (d *awsMailSendActionPerformer) DoAction(request Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []ActionResponse, []error) {

	//log.Printf("Sync mail servers")
	responses := make([]ActionResponse, 0)

	toValueInterfaceList, ok := inFields["to"].([]interface{})
	mailTo := make([]string, 0)
	if ok && len(toValueInterfaceList) > 0 {
		for _, toValueInterface := range toValueInterfaceList {
			mailTo = append(mailTo, toValueInterface.(string))
		}
	} else {
		isStrArray, ok := inFields["to"].([]string)
		if ok {
			mailTo = isStrArray
		}
	}
	subject := inFields["subject"].(string)
	mailFrom := inFields["from"].(string)
	credential_name := inFields["credential"].(string)

	credentialRow, err := d.cruds["credential"].GetObjectByWhereClauseWithTransaction(
		"credential", "name", credential_name, transaction)
	if err != nil {
		return nil, nil, []error{err}
	}

	decryptedSpec, err := Decrypt(d.encryptionSecret, credentialRow["content"].(string))

	decryptedSpecMap := make(map[string]interface{})
	err = json.Unmarshal([]byte(decryptedSpec), &decryptedSpecMap)
	if err != nil {
		return nil, nil, []error{err}
	}

	// AWS credentials (IAM Access Key and Secret Key)
	accessKey := decryptedSpecMap["access_key"].(string)
	secretKey := decryptedSpecMap["secret_key"].(string)
	region := decryptedSpecMap["region"].(string)
	token := decryptedSpecMap["token"].(string)
	//provider_name := decryptedSpecMap["provider_name"].(string)

	// AWS Session
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
		Credentials: credentials.NewStaticCredentials(
			accessKey, secretKey, token,
		),
	})
	if err != nil {
		log.Errorf("Failed to create AWS session: %v", err)
		return nil, nil, []error{err}
	}

	// Create SES client
	svc := ses.New(sess)

	// Construct email
	toAd := []*string{}
	for _, mailToI := range mailTo {
		toAd = append(toAd, aws.String(mailToI))
	}
	mailBodyText, isMailBodyText := inFields["text"].(string)
	var awsMailBody *ses.Body = nil
	if isMailBodyText {
		awsMailBody = &ses.Body{
			Text: &ses.Content{
				Data: aws.String(mailBodyText),
			},
		}
	} else {
		mailBodyText, isMailBodyHtml := inFields["html"].(string)
		if isMailBodyHtml {
			awsMailBody = &ses.Body{
				Html: &ses.Content{
					Data: aws.String(mailBodyText),
				},
			}
		}
	}
	if awsMailBody == nil {
		return nil, nil, []error{fmt.Errorf("No valid mail body found")}
	}
	input := &ses.SendEmailInput{
		Destination: &ses.Destination{
			ToAddresses: toAd,
		},
		Message: &ses.Message{
			Body: awsMailBody,
			Subject: &ses.Content{
				Data: aws.String(subject),
			},
		},
		Source: aws.String(mailFrom),
	}

	// Send email
	result, err := svc.SendEmail(input)
	if err != nil {
		return nil, nil, []error{err}
	}

	restartAttrs := make(map[string]interface{})
	restartAttrs["type"] = "success"
	restartAttrs["result"] = result.String()
	restartAttrs["message"] = "email sent successfully"
	restartAttrs["title"] = "Success"
	actionResponse := NewActionResponse("client.notify", restartAttrs)
	responses = append(responses, actionResponse)
	return nil, responses, nil
}

func NewAwsMailSendActionPerformer(cruds map[string]*DbResource, mailDaemon *guerrilla.Daemon, configStore *ConfigStore, transaction *sqlx.Tx) (ActionPerformerInterface, error) {
	encryptionSecret, _ := configStore.GetConfigValueFor("encryption.secret", "backend", transaction)

	handler := awsMailSendActionPerformer{
		cruds:            cruds,
		mailDaemon:       mailDaemon,
		encryptionSecret: []byte(encryptionSecret),
	}

	return &handler, nil

}
