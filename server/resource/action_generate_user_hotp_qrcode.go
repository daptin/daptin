package resource

import (
	"bytes"
	"encoding/base64"
	"github.com/artpar/api2go"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"image/png"
	"log"
)

type GenerateHOTPQRCodeActionPerformer struct {
	cruds            map[string]*DbResource
	encryptionSecret []byte
	totpLength       int
	issuerName       string
}

func (d *GenerateHOTPQRCodeActionPerformer) Name() string {
	return "2fa.hotp.qrcode"
}

func (d *GenerateHOTPQRCodeActionPerformer) DoAction(request Outcome, inFieldMap map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	userAccountReferenceId := inFieldMap["reference_id"].(string)
	userAccount, _, err := d.cruds[USER_ACCOUNT_TABLE_NAME].GetSingleRowByReferenceId(USER_ACCOUNT_TABLE_NAME, userAccountReferenceId, nil)
	if err != nil {
		return nil, nil, []error{err}
	}

	userAccountId, _ := userAccount["id"]
	userOtpProfile, err := d.cruds["user_otp_account"].GetObjectByWhereClause("user_otp_account", "otp_of_account", userAccountId)
	if err != nil {
		return nil, nil, []error{err}
	}

	key, _ := Decrypt(d.encryptionSecret, userOtpProfile["otp_secret"].(string))

	k, _ := b32NoPadding.DecodeString(key)
	gotp := totp.GenerateOpts{
		Digits:      otp.Digits(d.totpLength),
		Issuer:      d.issuerName,
		Secret:      k,
		AccountName: userAccount["email"].(string),
		Algorithm:   otp.AlgorithmSHA1,
	}

	totpKey, err := totp.Generate(gotp)
	if err != nil {
		return nil, nil, []error{err}
	}
	log.Printf("TOTP: %v", totpKey.URL())
	qrImage, err := totpKey.Image(300, 300)
	if err != nil {
		return nil, nil, []error{err}
	}

	imageBytes := new(bytes.Buffer)
	err = png.Encode(imageBytes, qrImage)
	if err != nil {
		return nil, nil, []error{err}
	}

	responseAttrs := make(map[string]interface{})
	encoded := base64.StdEncoding.EncodeToString(imageBytes.Bytes())

	responseAttrs["content"] = encoded
	responseAttrs["name"] = "qrcode.png"
	responseAttrs["contentType"] = "image/png"
	responseAttrs["message"] = "Downloading qrcode"

	actionResponse := NewActionResponse("client.file.download", responseAttrs)

	responses = append(responses, actionResponse)

	return nil, responses, nil
}

func NewGenerateHOTPQRCodeActionPerformer(configStore *ConfigStore, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	encryptionSecret, _ := configStore.GetConfigValueFor("encryption.secret", "backend")
	totpLength, err := configStore.GetConfigIntValueFor("totp.length", "backend")
	if err != nil {
		totpLength = 6
		configStore.SetConfigValueFor("totp.length", "6", "backend")
	}
	issuerName, err := configStore.GetConfigValueFor("jwt.token.issuer", "backend")

	handler := GenerateHOTPQRCodeActionPerformer{
		cruds:            cruds,
		encryptionSecret: []byte(encryptionSecret),
		totpLength:       totpLength,
		issuerName:       issuerName,
	}

	return &handler, nil

}
