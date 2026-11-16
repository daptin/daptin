package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func thunderbirdFixtureCertificate(t *testing.T, hostname string) (map[string]interface{}, string) {
	t.Helper()
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	serial := func() *big.Int {
		number, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
		if err != nil {
			t.Fatal(err)
		}
		return number
	}
	now := time.Now()
	ca := &x509.Certificate{
		SerialNumber: serial(), Subject: pkix.Name{CommonName: "Disposable Daptin IMAP CA"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(24 * time.Hour),
		BasicConstraintsValid: true, IsCA: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	leaf := &x509.Certificate{
		SerialNumber: serial(), Subject: pkix.Name{CommonName: hostname}, DNSNames: []string{hostname},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(12 * time.Hour),
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	encode := func(template, issuer *x509.Certificate, public *rsa.PublicKey, signing *rsa.PrivateKey) string {
		t.Helper()
		der, err := x509.CreateCertificate(rand.Reader, template, issuer, public, signing)
		if err != nil {
			t.Fatal(err)
		}
		return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	}
	root := encode(ca, ca, &caKey.PublicKey, caKey)
	return map[string]interface{}{
		"hostname": hostname, "issuer": "fixture-ca", "certificate_pem": encode(leaf, ca, &leafKey.PublicKey, caKey),
		"root_certificate": root, "private_key_pem": string(pem.EncodeToMemory(&pem.Block{
			Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(leafKey),
		})),
	}, root
}

func exerciseMailWithThunderbird(t *testing.T, driver, address, hostname, root string) uint32 {
	t.Helper()
	ip, port, err := net.SplitHostPort(address)
	if err != nil || ip != "127.0.0.1" || hostname != "localhost" || !filepath.IsAbs(driver) {
		t.Fatal("Thunderbird requires an absolute test driver and a private localhost STARTTLS listener")
	}
	path := filepath.Join(t.TempDir(), "fixture-root.pem")
	if err := os.WriteFile(path, []byte(root), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, driver, hostname, port, path)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Thunderbird fixture: %v\n%s", err, output)
	}
	t.Logf("%s", output)
	for _, line := range strings.Split(string(output), "\n") {
		if value, found := strings.CutPrefix(line, "Thunderbird online HEADER UID: "); found {
			uid, err := strconv.ParseUint(value, 10, 32)
			if err == nil && uid != 0 {
				return uint32(uid)
			}
		}
	}
	t.Fatal("Thunderbird did not report the UID returned by online HEADER search")
	return 0
}
