package resource

import (
	"fmt"
	"github.com/artpar/api2go"
	"github.com/artpar/go-guerrilla"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/jmoiron/sqlx"
)

type awsMailSendActionPerformer struct {
	cruds              map[string]*DbResource
	mailDaemon         *guerrilla.Daemon
	certificateManager *CertificateManager
}

func (d *awsMailSendActionPerformer) Name() string {
	return "aws.mail.send"
}

func (d *awsMailSendActionPerformer) DoAction(request Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []ActionResponse, []error) {

	//log.Printf("Sync mail servers")
	responses := make([]ActionResponse, 0)

	mailTo := inFields["to"].([]string)
	subject := inFields["subject"].(string)
	mailFrom := inFields["from"].(string)
	mailBody := inFields["body"].(string)

	// AWS credentials
	accessKey := "your-access-key"
	secretKey := "your-secret-key"
	region := "us-east-1" // Replace with your AWS region

	// Create AWS config
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		),
	)
	if err != nil {
		fmt.Printf("failed to load configuration, %v\n", err)
		return
	}

	// Create SES client
	client := ses.NewFromConfig(cfg)

	// Define email details
	from := "sender@example.com"
	to := "recipient@example.com"
	subject := "Test Email from AWS SES"
	bodyText := "This is a test email sent from AWS SES using the Go SDK."
	bodyHTML := "<html><body><h1>Test Email</h1><p>This is a test email sent from AWS SES using the Go SDK.</p></body></html>"

	// Send email
	input := &ses.SendEmailInput{
		Destination: &types.Destination{
			ToAddresses: []string{to},
		},
		Message: &types.Message{
			Body: &types.Body{
				Text: &types.Content{
					Charset: aws.String("UTF-8"),
					Data:    aws.String(bodyText),
				},
				Html: &types.Content{
					Charset: aws.String("UTF-8"),
					Data:    aws.String(bodyHTML),
				},
			},
			Subject: &types.Content{
				Charset: aws.String("UTF-8"),
				Data:    aws.String(subject),
			},
		},
		Source: aws.String(from),
	}

	// Make the SendEmail API call
	output, err := client.SendEmail(context.TODO(), input)
	if err != nil {
		fmt.Printf("failed to send email, %v\n", err)
		return
	}

	fmt.Printf("Email sent successfully! Message ID: %s\n", *output.MessageId)

	return nil, responses, nil
}

func NewAwsMailSendActionPerformer(cruds map[string]*DbResource, mailDaemon *guerrilla.Daemon, certificateManager *CertificateManager) (ActionPerformerInterface, error) {

	handler := awsMailSendActionPerformer{
		cruds:              cruds,
		mailDaemon:         mailDaemon,
		certificateManager: certificateManager,
	}

	return &handler, nil

}
