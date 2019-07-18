package resource

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/daptin/daptin/server/auth"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"net/http"

	//"golang.org/x/oauth2"
	"github.com/artpar/api2go"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"time"
)

type OtpRegisterBeginActionPerformer struct {
	responseAttrs    map[string]interface{}
	cruds            map[string]*DbResource
	configStore      *ConfigStore
	credentials      *credentials.Credentials
	encryptionSecret []byte
	awsRegion        string
}

func (d *OtpRegisterBeginActionPerformer) Name() string {
	return "otp.register.begin"
}

func (d *OtpRegisterBeginActionPerformer) DoAction(request ActionRequest, inFieldMap map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	email, emailOk := inFieldMap["email"]
	phone, phoneOk := inFieldMap["mobile"]
	var userAccount map[string]interface{}
	var userOtpProfile map[string]interface{}
	var err error
	if !emailOk || !phoneOk {
		return nil, nil, []error{errors.New("email or mobile missing")}
	} else {
		userAccount, err = d.cruds["user_account"].GetUserAccountRowByEmail(email.(string))
		if err != nil {
			return nil, nil, []error{errors.New("invalid email")}
		}
		userOtpProfileId, ok := userAccount["primary_user_otp"]
		if ok && userOtpProfileId != nil {
			userOtpProfile, err = d.cruds["user_otp_account"].GetObjectByWhereClause("user_otp_account", "reference_id", userOtpProfileId.(string))
		}
	}

	httpReq := &http.Request{}
	user := &auth.SessionUser{
		UserId:          userAccount["id"].(int64),
		UserReferenceId: userAccount["reference_id"].(string),
	}
	httpReq = httpReq.WithContext(context.WithValue(context.Background(), "user", user))

	if userAccount == nil {
		return nil, nil, []error{errors.New("no such account")}
	}

	if userOtpProfile == nil {

		key, err := totp.Generate(totp.GenerateOpts{
			Issuer:      "site.daptin.com",
			AccountName: "dummy@site.daptin.com",
			Period:      300,
			Digits:      4,
			SecretSize:  10,
		})

		if err != nil {
			log.Errorf("Failed to generate code: %v", err)
			return nil, nil, []error{err}
		}

		userOtpProfile = map[string]interface{}{
			"otp_secret":     key.Secret(),
			"verified":       0,
			"mobile_number":  phone,
			"otp_of_account": userAccount["reference_id"],
		}

		req := api2go.Request{
			PlainRequest: httpReq,
		}

		createdOtpProfile, err := d.cruds["user_otp_account"].CreateWithoutFilter(api2go.NewApi2GoModelWithData("user_otp_account", nil, 0, nil, userOtpProfile), req)
		if err != nil {
			return nil, nil, []error{errors.New("failed to create otp profile")}
		}

		userOtpProfile = createdOtpProfile

	}

	if userOtpProfile["mobile_number"] != phone {

		req := api2go.Request{
			PlainRequest: httpReq,
		}

		model := api2go.NewApi2GoModelWithData("user_otp_account", nil, 0, nil, userOtpProfile)
		model.SetAttributes(map[string]interface{}{
			"mobile_number": phone,
			"verified":      0,
		})
		_, err := d.cruds["user_otp_account"].UpdateWithoutFilters(model, req)

		if err != nil {
			log.Printf("Failed to update mobile number for the account: %v", err)
			return nil, nil, []error{errors.New("failed to update number")}
		}
	}

	key, _ := Decrypt(d.encryptionSecret, userOtpProfile["otp_secret"].(string))
	state, err := totp.GenerateCodeCustom(key, time.Now(), totp.ValidateOpts{
		Period:    300,
		Skew:      1,
		Digits:    4,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		log.Errorf("Failed to generate code: %v", err)
		return nil, nil, []error{err}
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
		},
	}
	_, err = svc.Publish(params)
	if err != nil {
		return nil, nil, []error{err}
	}

	return nil, []ActionResponse{NewActionResponse("client.notify", NewClientNotification("message", "OTP sent to registered mobile number", "Success"))}, nil
}

func NewOtpRegisterBeginActionPerformer(cruds map[string]*DbResource, configStore *ConfigStore) (ActionPerformerInterface, error) {

	id, _ := configStore.GetConfigValueFor("sms.credentials.aws.id", "backend")
	secret, _ := configStore.GetConfigValueFor("sms.credentials.aws.secret", "backend")
	region, _ := configStore.GetConfigValueFor("sms.credentials.aws.region", "backend")
	encryptionSecret, _ := configStore.GetConfigValueFor("encryption.secret", "backend")

	//token, _ := uuid.NewV1()

	handler := OtpRegisterBeginActionPerformer{
		cruds:            cruds,
		credentials:      credentials.NewStaticCredentials(id, secret, ""),
		configStore:      configStore,
		awsRegion:        region,
		encryptionSecret: []byte(encryptionSecret),
	}

	return &handler, nil

}
