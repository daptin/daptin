package resource

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	//"golang.org/x/oauth2"
	"github.com/artpar/api2go"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"time"
)

type OtpLoginBeginActionPerformer struct {
	responseAttrs    map[string]interface{}
	cruds            map[string]*DbResource
	configStore      *ConfigStore
	credentials      *credentials.Credentials
	awsRegion        string
	encryptionSecret []byte
}

func (d *OtpLoginBeginActionPerformer) Name() string {
	return "otp.login.begin"
}

func (d *OtpLoginBeginActionPerformer) DoAction(request ActionRequest, inFieldMap map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	email, ok := inFieldMap["email"]
	var userAccount map[string]interface{}
	var userOtpProfile map[string]interface{}
	var err error
	if !ok || email == nil || email == "" {
		phone, ok := inFieldMap["mobile"]
		if !ok {
			return nil, []ActionResponse{NewActionResponse("client.notify", NewClientNotification("message", "No mobile or email provided", "Failed"))}, []error{errors.New("email or mobile missing")}
		}
		userOtpProfile, err = d.cruds["user_otp_account"].GetObjectByWhereClause("user_otp_account", "mobile_number", phone.(string))
		if err != nil {
			return nil, []ActionResponse{NewActionResponse("client.notify", NewClientNotification("message", "Unregistered mobile number", "Failed"))}, []error{errors.New("unregistered mobile number")}
		}
	} else {
		userAccount, err = d.cruds["user_account"].GetUserAccountRowByEmail(email.(string))
		if err != nil {
			return nil, []ActionResponse{NewActionResponse("client.notify", NewClientNotification("message", "Invalid email", "Failed"))}, []error{errors.New("invalid email")}
		}
		userOtpProfileId, ok := userAccount["user_otp_account_id"]
		if !ok && userOtpProfileId != "" {
			return nil, []ActionResponse{NewActionResponse("client.notify", NewClientNotification("message", "No mobile number registered", "Failed"))}, []error{errors.New("unregistered mobile number")}
		}
		userOtpProfile, err = d.cruds["user_otp_account"].GetObjectByWhereClause("user_otp_account", "reference_id", userOtpProfileId.(string))
	}

	if userOtpProfile == nil || err != nil {
		return nil, []ActionResponse{NewActionResponse("client.notify", NewClientNotification("message", "No mobile number registered", "Failed"))}, []error{errors.New("invalid account")}
	}

	if userOtpProfile["verified"].(int64) != 1 {
		return nil, []ActionResponse{NewActionResponse("client.notify", NewClientNotification("message", "Failed to generate new OTP code", "Failed"))}, []error{errors.New("unverified number cannot be used to login")}
	}

	key, err := Decrypt(d.encryptionSecret, userOtpProfile["otp_secret"].(string))
	if err != nil {
		return nil, []ActionResponse{NewActionResponse("client.notify", NewClientNotification("message", "Failed to generate new OTP code", "Failed"))}, []error{err}
	}

	state, err := totp.GenerateCodeCustom(key, time.Now(), totp.ValidateOpts{
		Period:    300,
		Skew:      1,
		Digits:    4,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		log.Errorf("Failed to generate code: %v", err)
		return nil, []ActionResponse{NewActionResponse("client.notify", NewClientNotification("message", "Failed to generate new OTP code", "Failed"))}, []error{err}
	}

	sess := session.Must(session.NewSession(&aws.Config{
		Region:      &d.awsRegion,
		Credentials: d.credentials,
	}))
	svc := sns.New(sess)

	dataType := "String"
	messageType := "Transactional"
	params := &sns.PublishInput{
		Message:     aws.String(fmt.Sprintf("Your OTP is %s", state)),
		PhoneNumber: aws.String(userOtpProfile["mobile_number"].(string)),
		MessageAttributes: map[string]*sns.MessageAttributeValue{
			"AWS.SNS.SMS.SMSType": {
				DataType:    &dataType,
				StringValue: &messageType,
			},
		},}
	_, err = svc.Publish(params)
	if err != nil {
		return nil, []ActionResponse{NewActionResponse("client.notify", NewClientNotification("message", "Failed to send OTP SMS", "Failed"))}, []error{err}
	}

	return nil, []ActionResponse{NewActionResponse("client.notify", NewClientNotification("message", "OTP sent to registered mobile number", "Success"))}, nil
}

func NewOtpLoginBeginActionPerformer(cruds map[string]*DbResource, configStore *ConfigStore) (ActionPerformerInterface, error) {

	id, _ := configStore.GetConfigValueFor("sms.credentials.aws.id", "backend")
	secret, _ := configStore.GetConfigValueFor("sms.credentials.aws.secret", "backend")
	region, _ := configStore.GetConfigValueFor("sms.credentials.aws.region", "backend")
	encryptionSecret, _ := configStore.GetConfigValueFor("encryption.secret", "backend")

	handler := OtpLoginBeginActionPerformer{
		cruds:            cruds,
		encryptionSecret: []byte(encryptionSecret),
		credentials:      credentials.NewStaticCredentials(id, secret, ""),
		awsRegion:        region,
		configStore:      configStore,
	}

	return &handler, nil

}
