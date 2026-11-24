package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"testing"
	"time"
)

// Both LOGIN forms must reach the same database-backed mail_account verifier
// with the complete credential bytes, including an initial response >160 bytes.
func TestMailSMTPLOGINRealE2E(t *testing.T) {
	requireRealE2E(t)
	t.Setenv("DAPTIN_DISABLE_SMTP", "false")
	address := fmt.Sprintf("127.0.0.1:%d", freeTransportE2EPort(t, make(map[int]bool)))
	options := mailProtocolDatabase(t)
	fixture := startDaptinE2E(t, options)
	admin := fixture.SignupAdmin(t)
	certificate, root := thunderbirdFixtureCertificate(t, "localhost")
	accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "certificate", certificate)
	server := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "mail_server", map[string]interface{}{
		"hostname": "localhost", "is_enabled": true, "listen_interface": address, "max_size": 30000000,
		"max_clients": 4, "xclient_on": false, "always_on_tls": false, "authentication_required": true,
	})
	usernames := []string{"owner@localhost", strings.Repeat("a", 60) + "@" + strings.Repeat("b", 60) + ".test"}
	for _, username := range usernames {
		accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "mail_account", map[string]interface{}{
			"username": username, "password": "fixture-password", "password_md5": "fixture-password", "mail_server_id": server,
		})
	}
	fixture.Process.stopProcess()
	fixture = startDaptinE2E(t, options)
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(root)) {
		t.Fatal("invalid fixture CA")
	}
	for index, username := range usernames {
		for _, initial := range []bool{false, true} {
			for _, password := range []string{"fixture-password", "wrong-password"} {
				t.Run(fmt.Sprintf("account-%d/initial-%t/%s", index, initial, password), func(t *testing.T) {
					conn, err := net.DialTimeout("tcp", address, 5*time.Second)
					if err != nil {
						t.Fatal(err)
					}
					defer conn.Close()
					if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
						t.Fatal(err)
					}
					client, err := smtp.NewClient(conn, "localhost")
					if err != nil {
						t.Fatal(err)
					}
					defer client.Close()
					if err := client.StartTLS(&tls.Config{RootCAs: roots, ServerName: "localhost", MinVersion: tls.VersionTLS12}); err != nil {
						t.Fatal(err)
					}
					command := "aUtH lOgIn"
					if initial {
						command += " " + base64.StdEncoding.EncodeToString([]byte(username))
					}
					send := func(command string, code int) {
						t.Helper()
						id, err := client.Text.Cmd("%s", command)
						if err != nil {
							t.Fatal(err)
						}
						client.Text.StartResponse(id)
						_, _, err = client.Text.ReadResponse(code)
						client.Text.EndResponse(id)
						if err != nil {
							t.Fatal(err)
						}
					}
					send(command, 334)
					if !initial {
						send(base64.StdEncoding.EncodeToString([]byte(username)), 334)
					}
					code := 235
					if password == "wrong-password" {
						code = 535
					}
					send(base64.StdEncoding.EncodeToString([]byte(password)), code)
					if password == "wrong-password" {
						send("MAIL FROM:<"+username+">", 250)
						send("RCPT TO:<"+username+">", 554)
					}
					if err := client.Quit(); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
