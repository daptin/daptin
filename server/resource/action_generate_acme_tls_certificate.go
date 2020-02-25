package resource

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/auth"
	"github.com/gin-gonic/gin"
	"github.com/go-acme/lego/v3/certcrypto"
	"github.com/go-acme/lego/v3/certificate"
	"github.com/go-acme/lego/v3/lego"
	"github.com/go-acme/lego/v3/registration"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
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
	challenge        map[string]string
}

func (d *AcmeTlsCertificateGenerateActionPerformer) Name() string {
	return "acme.tls.generate"
}

func (d *AcmeTlsCertificateGenerateActionPerformer) Present(domain, token, keyAuth string) error {
	log.Printf("Infof Present lego %v %v %v", domain, token, keyAuth)
	d.challenge[token] = keyAuth
	return nil
}
func (d *AcmeTlsCertificateGenerateActionPerformer) CleanUp(domain, token, keyAuth string) error {
	log.Printf("Infof CleanUp lego %v %v %v", domain, token, keyAuth)
	delete(d.challenge, token)
	return nil
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
	httpReq := &http.Request{
		Method: "PUT",
	}
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

	certificateSubject := inFieldMap["certificate"].(map[string]interface{})
	hostname := certificateSubject["hostname"].(string)
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

	config := lego.NewConfig(&myUser)

	// This CA URL is configured for a local dev instance of Boulder running in Docker in a VM.
	//config.CADirURL = "https://localhost:14000/dir"
	config.CADirURL = lego.LEDirectoryProduction
	config.Certificate.KeyType = certcrypto.RSA2048
	config.HTTPClient = &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	// A client facilitates communication with the CA server.
	client, err := lego.NewClient(config)
	if err != nil {
		log.Printf("Failed to create client: %v", err)
	}

	// We specify an http port of 5002 and an tls port of 5001 on all interfaces
	// because we aren't running as root and can't bind a listener to port 80 and 443
	// (used later when we attempt to pass challenges). Keep in mind that you still
	// need to proxy challenge traffic to port 5002 and 5001.
	err = client.Challenge.SetHTTP01Provider(d)

	if err != nil {
		log.Printf("Failed to create client: %v", err)
		return nil, []ActionResponse{}, []error{err}
	}

	// New users will need to register
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		log.Printf("Failed to create client: %v", err)
		return nil, []ActionResponse{}, []error{err}
	}
	myUser.Registration = reg

	certificateRequest := certificate.ObtainRequest{
		Domains: []string{hostname},
		Bundle:  true,
	}

	certificates, err := client.Certificate.Obtain(certificateRequest)
	if err != nil {
		log.Printf("Failed to create client: %v", err)
		return nil, []ActionResponse{}, []error{err}

	}

	certificateString := string(certificates.Certificate)

	certificateString = strings.Split(certificateString, "-----END CERTIFICATE-----")[0] + "-----END CERTIFICATE-----"

	rootCert := string(certificates.IssuerCertificate)

	publicKeyBytes := ""
	privateKey, err := ParseRsaPrivateKeyFromPemStr(string(certificates.PrivateKey))
	if err != nil {
		log.Printf("Failed to parse value as private key: %v", err)
	} else {

		asn1Bytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
		CheckErr(err, "Failed to marshal key as pkix public key")

		var pemkey = &pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: asn1Bytes,
		}

		publicKeyBytes = string(pem.EncodeToMemory(pemkey))
	}

	newCertificate := map[string]interface{}{
		"hostname":         hostname,
		"issuer":           "acme",
		"generated_at":     time.Now().Format(time.RFC3339),
		"certificate_pem":  certificateString,
		"private_key_pem":  string(certificates.PrivateKey),
		"public_key_pem":   publicKeyBytes,
		"root_certificate": rootCert,
		"reference_id":     certificateSubject["reference_id"].(string),
	}

	data := api2go.NewApi2GoModelWithData("certificate", nil, 0, nil, newCertificate)
	_, err = d.cruds["certificate"].UpdateWithoutFilters(data, api2go.Request{
		PlainRequest: httpReq,
	})

	// Each certificate comes back with the cert bytes, the bytes of the client's
	// private key, and a certificate URL. SAVE THESE TO DISK.
	//fmt.Printf("%#v\n", certificates)

	if err != nil {
		return nil, []ActionResponse{}, []error{err}
	}

	return nil, []ActionResponse{}, nil
}

func ParseRsaPrivateKeyFromPemStr(privPEM string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privPEM))
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing the key")
	}

	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)

	return priv, err
}

func NewAcmeTlsCertificateGenerateActionPerformer(cruds map[string]*DbResource, configStore *ConfigStore, hostSwitch *gin.Engine) (ActionPerformerInterface, error) {

	encryptionSecret, _ := configStore.GetConfigValueFor("encryption.secret", "backend")

	handler := AcmeTlsCertificateGenerateActionPerformer{
		cruds:            cruds,
		encryptionSecret: []byte(encryptionSecret),
		configStore:      configStore,
		hostSwitch:       hostSwitch,
		challenge:        make(map[string]string),
	}

	challengeResponse := func(c *gin.Context) {
		token := c.Param("token")
		log.Printf("Get challenge response: %v", token)
		c.String(200, handler.challenge[token])
	}
	hostSwitch.GET("/.well-known/acme-challenge/:token", challengeResponse)

	return &handler, nil

}
