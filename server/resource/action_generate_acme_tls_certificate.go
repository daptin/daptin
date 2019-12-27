package resource

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/auth"
	"github.com/gin-gonic/gin"
	"github.com/go-acme/lego/v3/registration"
	"log"
	"net/http"
)

// You'll need a user or account type that implements acme.User
type MyUser struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *MyUser) GetEmail() string {
	return u.Email
}
func (u MyUser) GetRegistration() *registration.Resource {
	return u.Registration
}
func (u *MyUser) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

type AcmeTlsCertificateGenerateActionPerformer struct {
	responseAttrs    map[string]interface{}
	cruds            map[string]*DbResource
	configStore      *ConfigStore
	encryptionSecret []byte
	hostSwitch       *gin.Engine
}

func (d *AcmeTlsCertificateGenerateActionPerformer) Name() string {
	return "acme.tls.generate"
}

func (d *AcmeTlsCertificateGenerateActionPerformer) DoAction(request Outcome, inFieldMap map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	email, emailOk := inFieldMap["email"]
	emailString, isEmailStr := email.(string)
	var userAccount map[string]interface{}
	var err error

	if !emailOk || !isEmailStr || len(emailString) < 4 {
		return nil, nil, []error{errors.New("email or mobile missing")}
	} else {
		userAccount, err = d.cruds["user_account"].GetUserAccountRowByEmail(emailString)
		if err != nil || userAccount == nil {
			return nil, nil, []error{errors.New("invalid email")}
		}
		i := userAccount["id"]
		if i == nil {
			return nil, nil, []error{errors.New("invalid account")}
		}
	}
	email = userAccount["email"].(string)
	httpReq := &http.Request{}
	user := &auth.SessionUser{
		UserId:          userAccount["id"].(int64),
		UserReferenceId: userAccount["reference_id"].(string),
	}
	httpReq = httpReq.WithContext(context.WithValue(context.Background(), "user", user))
	//req := api2go.Request{
	//	PlainRequest: httpReq,
	//}

	userPrivateKeyEncrypted, err := d.configStore.GetConfigValueFor("encryption.private_key."+email.(string), "backend")

	var myUser MyUser

	certificateSubject := inFieldMap["certificate"]
	log.Printf("Generate certificate for: %v", certificateSubject)

	if err != nil {
		log.Printf("No existing private key for [%v]", email)
		// no existing key, create one

		// Create a user. New accounts need an email and private key to start.
		publicKeyPem, privateKeyPem, privateKey, err := GetPublicPrivateKeyPEMBytes()
		if err != nil {
			return nil, []ActionResponse{}, []error{err}
		}

		myUser = MyUser{
			Email: email.(string),
			key:   privateKey,
		}

		encryptedPem, err := Encrypt(d.encryptionSecret, string(privateKeyPem))
		if err != nil {
			return nil, []ActionResponse{}, []error{err}
		}

		err = d.configStore.SetConfigValueFor("encryption.private_key."+email.(string), encryptedPem, "backend")
		if err != nil {
			return nil, []ActionResponse{}, []error{err}
		}
		err = d.configStore.SetConfigValueFor("encryption.public_key."+email.(string), string(publicKeyPem), "backend")
		if err != nil {
			return nil, []ActionResponse{}, []error{err}
		}

	} else {

		privateKeyPem, err := Decrypt(d.encryptionSecret, userPrivateKeyEncrypted)
		if err != nil {
			return nil, []ActionResponse{}, []error{err}
		}

		key, err := ParseRsaPrivateKeyFromPemStr(privateKeyPem)
		if err != nil {
			return nil, []ActionResponse{}, []error{err}
		}

		myUser = MyUser{
			Email: email.(string),
			key:   key,
		}

	}

	log.Printf("User loaded: %v ", myUser.Email)

	return nil, []ActionResponse{}, nil
}

func ParseRsaPrivateKeyFromPemStr(privPEM string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privPEM))
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing the key")
	}

	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return priv, nil
}

func NewAcmeTlsCertificateGenerateActionPerformer(cruds map[string]*DbResource, configStore *ConfigStore, hostSwitch *gin.Engine) (ActionPerformerInterface, error) {

	encryptionSecret, _ := configStore.GetConfigValueFor("encryption.secret", "backend")

	handler := AcmeTlsCertificateGenerateActionPerformer{
		cruds:            cruds,
		encryptionSecret: []byte(encryptionSecret),
		configStore:      configStore,
		hostSwitch:       hostSwitch,
	}

	return &handler, nil

}
