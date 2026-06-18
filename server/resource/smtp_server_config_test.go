package resource

import (
	"bytes"
	"encoding/pem"
	"testing"
)

func TestSMTPCertificateChainPEMAppendsRootCertificatesWithoutDuplicates(t *testing.T) {
	leaf := testCertificatePEM([]byte{1, 2, 3})
	intermediate := testCertificatePEM([]byte{4, 5, 6})
	root := testCertificatePEM([]byte{7, 8, 9})

	chain := smtpCertificateChainPEM(leaf, append(intermediate, root...))
	if got := bytes.Count(chain, []byte("BEGIN CERTIFICATE")); got != 3 {
		t.Fatalf("expected 3 certificate blocks, got %d:\n%s", got, string(chain))
	}

	chain = smtpCertificateChainPEM(append(leaf, intermediate...), append(intermediate, root...))
	if got := bytes.Count(chain, []byte("BEGIN CERTIFICATE")); got != 3 {
		t.Fatalf("expected duplicate intermediate to be removed, got %d:\n%s", got, string(chain))
	}
}

func TestSMTPCertificateChainPEMIgnoresNonCertificatePEMBlocks(t *testing.T) {
	publicKey := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte{1, 2, 3}})
	leaf := testCertificatePEM([]byte{4, 5, 6})

	chain := smtpCertificateChainPEM(append(publicKey, leaf...), nil)
	if bytes.Contains(chain, []byte("BEGIN PUBLIC KEY")) {
		t.Fatalf("expected public key PEM block to be omitted from certificate chain:\n%s", string(chain))
	}
	if got := bytes.Count(chain, []byte("BEGIN CERTIFICATE")); got != 1 {
		t.Fatalf("expected 1 certificate block, got %d:\n%s", got, string(chain))
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
