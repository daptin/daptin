package resource

import (
	"fmt"
	"github.com/artpar/api2go"
	"github.com/artpar/go-guerrilla"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
	daptinid "github.com/daptin/daptin/server/id"
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

	mailTo := GetValueAsArrayString(inFields, "to")
	mailCc := GetValueAsArrayString(inFields, "cc")
	mailBcc := GetValueAsArrayString(inFields, "bcc")

	subject := inFields["subject"].(string)
	mailFrom := inFields["from"].(string)
	credential_name := inFields["credential"].(string)

	credential, err := d.cruds["credential"].GetCredentialByName(credential_name, transaction)
	if err != nil {
		return nil, nil, []error{err}
	}

	// AWS credentials (IAM Access Key and Secret Key)
	accessKey := credential.DataMap["access_key"].(string)
	secretKey := credential.DataMap["secret_key"].(string)
	region := credential.DataMap["region"].(string)
	token := credential.DataMap["token"].(string)

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
	toAddresses := []*string{}
	for _, mailToI := range mailTo {
		toAddresses = append(toAddresses, &mailToI)
	}
	ccAddresses := []*string{}
	for _, mailToI := range mailCc {
		ccAddresses = append(ccAddresses, &mailToI)
	}

	bccAddresses := []*string{}
	for _, mailToI := range mailBcc {
		bccAddresses = append(bccAddresses, &mailToI)
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
			ToAddresses:  toAddresses,
			CcAddresses:  ccAddresses,
			BccAddresses: bccAddresses,
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

func GetValueAsArrayString(inFields map[string]interface{}, keyName string) []string {
	stringValueList := make([]string, 0)
	valueObject, ok := inFields[keyName]
	if !ok {
		return stringValueList
	}
	toValueInterfaceList, ok := valueObject.([]interface{})
	if ok && len(toValueInterfaceList) > 0 {
		for _, toValueInterface := range toValueInterfaceList {
			stringValueList = append(stringValueList, toValueInterface.(string))
		}
	} else {
		isStrArray, ok := valueObject.([]string)
		if ok {
			stringValueList = isStrArray
		}
	}
	return stringValueList
}

type Credential struct {
	DataMap map[string]interface{}
	Name    string
}

func (d *DbResource) GetCredentialByName(credentialName string, transaction *sqlx.Tx) (*Credential, error) {
	credentialRow, err := d.GetObjectByWhereClauseWithTransaction(
		"credential", "name", credentialName, transaction)
	if err != nil {
		return nil, err
	}

	encryptionSecret, _ := d.configStore.GetConfigValueFor("encryption.secret", "backend", transaction)

	decryptedSpec, err := Decrypt([]byte(encryptionSecret), credentialRow["content"].(string))

	decryptedSpecMap := make(map[string]interface{})
	err = json.Unmarshal([]byte(decryptedSpec), &decryptedSpecMap)
	if err != nil {
		return nil, err
	}
	return &Credential{
		Name:    credentialName,
		DataMap: decryptedSpecMap,
	}, nil
}

func (d *DbResource) GetCredentialByReferenceId(referenceId daptinid.DaptinReferenceId, transaction *sqlx.Tx) (*Credential, error) {
	credentialRow, err := d.GetObjectByWhereClauseWithTransaction(
		"credential", "reference_id", referenceId[:], transaction)
	if err != nil {
		return nil, err
	}

	encryptionSecret, _ := d.configStore.GetConfigValueFor("encryption.secret", "backend", transaction)

	decryptedSpec, err := Decrypt([]byte(encryptionSecret), credentialRow["content"].(string))

	decryptedSpecMap := make(map[string]interface{})
	err = json.Unmarshal([]byte(decryptedSpec), &decryptedSpecMap)
	if err != nil {
		return nil, err
	}
	return &Credential{
		Name:    credentialRow["name"].(string),
		DataMap: decryptedSpecMap,
	}, nil
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
