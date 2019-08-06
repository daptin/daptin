package resource

import (
	"github.com/artpar/api2go"
	"github.com/pkg/errors"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	log "github.com/sirupsen/logrus"
	"time"
)

type OtpGenerateActionPerformer struct {
	responseAttrs    map[string]interface{}
	cruds            map[string]*DbResource
	configStore      *ConfigStore
	encryptionSecret []byte
}

func (d *OtpGenerateActionPerformer) Name() string {
	return "otp.generate"
}

func (d *OtpGenerateActionPerformer) DoAction(request Outcome, inFieldMap map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

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
		return nil, []ActionResponse{NewActionResponse("client.notify", NewClientNotification("message", "Unverified account", "Failed"))}, []error{errors.New("unverified number cannot be used to login")}
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

	responder := api2go.NewApi2GoModelWithData("otp", nil, 0, nil, map[string]interface{}{
		"otp": state,
	})

	resp := &api2go.Response{
		Res: responder,
	}

	return resp, []ActionResponse{}, nil
}

func NewOtpGenerateActionPerformer(cruds map[string]*DbResource, configStore *ConfigStore) (ActionPerformerInterface, error) {

	encryptionSecret, _ := configStore.GetConfigValueFor("encryption.secret", "backend")

	handler := OtpGenerateActionPerformer{
		cruds:            cruds,
		encryptionSecret: []byte(encryptionSecret),
		configStore:      configStore,
	}

	return &handler, nil

}
