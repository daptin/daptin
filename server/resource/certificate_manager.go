package resource

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/auth"
	"log"
	"math/big"
	"net"
	"net/http"
	"strings"
	"time"
)

type CertificateManager struct {
	cruds            map[string]*DbResource
	configStore      *ConfigStore
	encryptionSecret string
}

func NewCertificateManager(cruds map[string]*DbResource, configStore *ConfigStore) (*CertificateManager, error) {

	secret, err := configStore.GetConfigValueFor("encryption.secret", "backend")
	if err != nil {
		return nil, errors.New("no secret to decrypt key certificate")
	}

	return &CertificateManager{
		cruds:            cruds,
		configStore:      configStore,
		encryptionSecret: secret,
	}, nil
}

func GenerateCertPEMWithKey(hostname string, privateKey *rsa.PrivateKey) ([]byte, error) {

	var notBefore time.Time
	notBefore = time.Now()

	validFor := time.Duration(365 * 24 * time.Hour)

	notAfter := notBefore.Add(validFor)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)

	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)

	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Country:      []string{"IN"},
			Organization: []string{"Daptin Co."},
			CommonName:   hostname,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	hosts := strings.Split(hostname, ",")

	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}

	}

	template.IsCA = true

	template.KeyUsage |= x509.KeyUsageCertSign

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		log.Printf("Failed to create certificate: %s", err)
		return nil, err
	}

	certBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	return certBytes, err
}

func GetPublicPrivateKeyPEMBytes() ([]byte, []byte, *rsa.PrivateKey, error) {

	reader := rand.Reader
	bitSize := 2048

	key, err := rsa.GenerateKey(reader, bitSize)
	CheckErr(err, "Failed to generate key of size [%v]", bitSize)

	var privateKey = &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}

	privateKeyBytes := pem.EncodeToMemory(privateKey)

	asn1Bytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	CheckErr(err, "Failed to marshal as PKIX public key")

	var pemkey = &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: asn1Bytes,
	}

	publicKeyBytes := pem.EncodeToMemory(pemkey)

	return publicKeyBytes, privateKeyBytes, key, nil
}

func (cm *CertificateManager) GetTLSConfig(hostname string, createIfNotFound bool) (*tls.Config, []byte, []byte, []byte, []byte, error) {

	log.Printf("Get certificate for [%v]: %v", hostname, createIfNotFound)
	hostname = strings.Split(hostname, ":")[0]
	certMap, err := cm.cruds["certificate"].GetObjectByWhereClause("certificate", "hostname", hostname)

	if createIfNotFound && (err != nil || certMap == nil || certMap["certificate_pem"] == nil || certMap["certificate_pem"].(string) == "") {

		publicKeyPem, privateKeyPem, key, err := GetPublicPrivateKeyPEMBytes()
		if err != nil {
			log.Printf("Failed to generate key: %v", err)
			return nil, nil, nil, nil, nil, err
		}

		certBytesPEM, err := GenerateCertPEMWithKey(hostname, key)

		if err != nil {
			log.Printf("Failed to load cert bytes pem: %v", err)
			return nil, nil, nil, nil, nil, err
		}

		cert, err := tls.X509KeyPair(certBytesPEM, privateKeyPem)
		if err != nil {
			log.Printf("Failed to load cert pair: %v", err)
			return nil, nil, nil, nil, nil, err
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			ServerName:   hostname,
			ClientAuth:   tls.NoClientCert,
		}

		adminUserReferenceId := cm.cruds["certificate"].GetAdminReferenceId()
		adminId, err := cm.cruds[USER_ACCOUNT_TABLE_NAME].GetReferenceIdToId("user_account", adminUserReferenceId)
		if err != nil {
			log.Printf("Failed to get admin id for user: %v == %v", adminUserReferenceId, err)
		}

		newCertificate := map[string]interface{}{
			"hostname":         hostname,
			"issuer":           "self",
			"generated_at":     time.Now().Format(time.RFC3339),
			"certificate_pem":  string(certBytesPEM),
			"private_key_pem":  string(privateKeyPem),
			"root_certificate": string(certBytesPEM),
			"public_key_pem":   string(publicKeyPem),
		}
		request := &http.Request{
			Method: "PUT",
		}

		request = request.WithContext(context.WithValue(context.Background(), "user", &auth.SessionUser{
			UserReferenceId: adminUserReferenceId,
			UserId:          adminId,
		}))
		req := api2go.Request{
			PlainRequest: request,
		}

		data := api2go.NewApi2GoModelWithData("certificate", nil, 0, nil, newCertificate)

		if certMap != nil && certMap["reference_id"] != nil {
			data.Data["reference_id"] = certMap["reference_id"]
			_, err = cm.cruds["certificate"].UpdateWithoutFilters(data, req)
			if err != nil {
				log.Printf("Failed to store locally generated certificate: %v", err)
			}
		} else {
			request.Method = "POST"
			_, err = cm.cruds["certificate"].CreateWithoutFilter(data, req)

			if err != nil {
				log.Printf("Failed to store locally generated certificate: %v", err)
			}
		}

		return tlsConfig, certBytesPEM, privateKeyPem, publicKeyPem, certBytesPEM, nil
	} else if certMap != nil && err == nil {

		certPEM := certMap["certificate_pem"].(string)

		privatePEM := AsStringOrEmpty(certMap["private_key_pem"])

		publicPEM := AsStringOrEmpty(certMap["public_key_pem"])
		rootCert := AsStringOrEmpty(certMap["root_certificate"])

		privatePEMDecrypted, err := Decrypt([]byte(cm.encryptionSecret), privatePEM)
		publicPEMDecrypted := publicPEM

		if err != nil {
			log.Printf("Failed to load cert: %v", err)
			return nil, nil, nil, nil, nil, err
		}

		cert, err := tls.X509KeyPair([]byte(certPEM), []byte(privatePEMDecrypted))
		rootCaCert := x509.NewCertPool()
		rootCaCert.AppendCertsFromPEM([]byte(rootCert))

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			ServerName:   hostname,
			RootCAs:      rootCaCert,
			ClientAuth:   tls.VerifyClientCertIfGiven,
		}

		return tlsConfig, []byte(certPEM), []byte(privatePEMDecrypted), []byte(publicPEMDecrypted), []byte(rootCert), nil

	}
	return nil, nil, nil, nil, nil, errors.New("certificate not found")
}
func AsStringOrEmpty(i interface{}) string {
	if i == nil {
		return ""
	}
	return i.(string)
}
