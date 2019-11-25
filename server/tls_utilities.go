package server

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"strings"
	"time"
)

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
			Organization: []string{"Daptin Co."},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA: true,
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

	var privateKey = &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}

	privateKeyBytes := pem.EncodeToMemory(privateKey)

	asn1Bytes, err := asn1.Marshal(key.PublicKey)
	if err != nil {
		return nil, nil, key, err
	}

	var pemkey = &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: asn1Bytes,
	}

	publicKeyBytes := pem.EncodeToMemory(pemkey)

	return publicKeyBytes, privateKeyBytes, key, nil
}

func GetTLSConfig(hostnames string) (*tls.Config, []byte, []byte, []byte, error) {

	publicKeyPem, privateKeyPem, key, err := GetPublicPrivateKeyPEMBytes()
	if err != nil {
		log.Printf("Failed to generate key: %v", err)
		return nil, nil, nil, nil, err
	}

	certBytesPEM, err := GenerateCertPEMWithKey(hostnames, key)

	if err != nil {
		log.Printf("Failed to load cert: %v", err)
		return nil, nil, nil, nil, err
	}

	cert, err := tls.X509KeyPair(certBytesPEM, privateKeyPem)
	if err != nil {
		log.Printf("Failed to load cert: %v", err)
		return nil, nil, nil, nil, err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	return tlsConfig, certBytesPEM, privateKeyPem, publicKeyPem, nil
}
