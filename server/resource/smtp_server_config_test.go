package resource

import (
	"bytes"
	"crypto/tls"
	"encoding/pem"
	"testing"
)

func TestCertificateChainPEMAppendsRootCertificatesWithoutDuplicates(t *testing.T) {
	leaf := testCertificatePEM([]byte{1, 2, 3})
	intermediate := testCertificatePEM([]byte{4, 5, 6})
	root := testCertificatePEM([]byte{7, 8, 9})

	chain := certificateChainPEM(leaf, append(intermediate, root...))
	if got := bytes.Count(chain, []byte("BEGIN CERTIFICATE")); got != 3 {
		t.Fatalf("expected 3 certificate blocks, got %d:\n%s", got, string(chain))
	}

	chain = certificateChainPEM(append(leaf, intermediate...), append(intermediate, root...))
	if got := bytes.Count(chain, []byte("BEGIN CERTIFICATE")); got != 3 {
		t.Fatalf("expected duplicate intermediate to be removed, got %d:\n%s", got, string(chain))
	}
}

func TestCertificateChainPEMIgnoresNonCertificatePEMBlocks(t *testing.T) {
	publicKey := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte{1, 2, 3}})
	leaf := testCertificatePEM([]byte{4, 5, 6})

	chain := certificateChainPEM(append(publicKey, leaf...), nil)
	if bytes.Contains(chain, []byte("BEGIN PUBLIC KEY")) {
		t.Fatalf("expected public key PEM block to be omitted from certificate chain:\n%s", string(chain))
	}
	if got := bytes.Count(chain, []byte("BEGIN CERTIFICATE")); got != 1 {
		t.Fatalf("expected 1 certificate block, got %d:\n%s", got, string(chain))
	}
}

func TestCertificateChainPEMLoadsAsTLSCertificateChain(t *testing.T) {
	_, leafPrivatePEM, leafKey, err := CreateNewPublicPrivateKeyPEMBytes()
	if err != nil {
		t.Fatalf("failed to generate leaf key: %v", err)
	}
	leafPEM, err := GenerateCertPEMWithKey("leaf.example.test", leafKey)
	if err != nil {
		t.Fatalf("failed to generate leaf certificate: %v", err)
	}
	_, _, rootKey, err := CreateNewPublicPrivateKeyPEMBytes()
	if err != nil {
		t.Fatalf("failed to generate root key: %v", err)
	}
	rootPEM, err := GenerateCertPEMWithKey("root.example.test", rootKey)
	if err != nil {
		t.Fatalf("failed to generate root certificate: %v", err)
	}

	cert, err := tls.X509KeyPair(certificateChainPEM(leafPEM, rootPEM), leafPrivatePEM)
	if err != nil {
		t.Fatalf("expected combined certificate chain to load: %v", err)
	}
	if got := len(cert.Certificate); got != 2 {
		t.Fatalf("expected TLS certificate chain to include 2 certificates, got %d", got)
	}
}

func TestSMTPConfigBoolAcceptsDatabaseBooleanRepresentations(t *testing.T) {
	trueValues := []interface{}{true, "true", "TRUE", "1", 1, int64(1), uint(1), "yes", "on"}
	for _, value := range trueValues {
		if !smtpConfigBool(value) {
			t.Fatalf("expected %v (%T) to parse as true", value, value)
		}
	}

	falseValues := []interface{}{false, "false", "0", 0, int64(0), uint(0), "", nil}
	for _, value := range falseValues {
		if smtpConfigBool(value) {
			t.Fatalf("expected %v (%T) to parse as false", value, value)
		}
	}
}

func testCertificatePEM(bytes []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: bytes})
}
