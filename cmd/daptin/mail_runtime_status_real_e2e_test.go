package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"strings"
	"testing"
	"time"

	imapclient "github.com/artpar/go-imap/client"
)

const mailRuntimeStatusSchema = `
Actions:
  - Name: mail_runtime_wrapper
    OnType: world
    InstanceOptional: true
    Permission: 32
    OutFields:
      - Type: mail.service.status
        Method: EXECUTE
  - Name: mail_runtime_as_user
    OnType: world
    InstanceOptional: true
    Permission: 0
    InFields:
      - Name: user_reference_id
        ColumnType: label
    OutFields:
      - Method: SWITCH_USER
        Type: __as_user
        Attributes:
          user_reference_id: ~user_reference_id
      - Type: mail.service.status
        Method: EXECUTE
`

func TestMailRuntimeStatusRealE2E(t *testing.T) {
	requireRealE2E(t)
	t.Setenv("DAPTIN_DISABLE_SMTP", "false")
	options := mailProtocolDatabase(t)
	options.schema = mailRuntimeStatusSchema
	fixture := startDaptinE2E(t, options)
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, fixture.URL+"/action/world/mail_service_status", "",
		map[string]interface{}{"attributes": map[string]interface{}{}}, http.StatusForbidden)
	admin := fixture.SignupAdmin(t)
	// Load the schema again after bootstrap so wrapper permissions are final.
	fixture.Process.stopProcess()
	fixture = startDaptinE2E(t, options)
	user := accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, admin, "runtime-reader")
	userRef := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "user_account", "email", "runtime-reader@test.local")
	invoke := func(name, token string, attributes map[string]interface{}, want int) interface{} {
		t.Helper()
		return accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/world/"+name, token, map[string]interface{}{"attributes": attributes}, want)
	}
	status := func() map[string]interface{} {
		t.Helper()
		result := invoke("mail_service_status", admin, map[string]interface{}{}, http.StatusOK).([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})
		raw, _ := json.Marshal(result)
		for _, secret := range []string{"BEGIN CERTIFICATE", "PRIVATE KEY", "address already in use", "password"} {
			if strings.Contains(string(raw), secret) {
				t.Fatalf("diagnostic leaked %q: %s", secret, raw)
			}
		}
		return result
	}
	for _, token := range []string{"", user} {
		invoke("mail_service_status", token, map[string]interface{}{}, http.StatusForbidden)
	}
	invoke("mail_runtime_wrapper", user, map[string]interface{}{"sessionUser": map[string]interface{}{"Groups": []string{"administrators"}}}, http.StatusForbidden)
	invoke("mail_runtime_as_user", admin, map[string]interface{}{"user_reference_id": userRef}, http.StatusForbidden)
	initial := status()
	if initial["smtp"].(map[string]interface{})["state"] != "disabled" || initial["imap"].(map[string]interface{})["state"] != "disabled" {
		t.Fatalf("initial services: %#v", initial)
	}
	group := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "usergroup", "name", "administrators")
	link := map[string]interface{}{"data": []interface{}{map[string]interface{}{"type": "usergroup", "id": group}}}
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/user_account/"+userRef, admin,
		map[string]interface{}{"data": map[string]interface{}{"type": "user_account", "id": userRef,
			"relationships": map[string]interface{}{"usergroup_id": link}}}, http.StatusOK)
	invoke("mail_runtime_wrapper", user, map[string]interface{}{}, http.StatusOK)
	invoke("mail_service_status", user, map[string]interface{}{}, http.StatusOK)
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodDelete, fixture.URL+"/api/user_account/"+userRef+"/relationships/usergroup_id", admin, link, http.StatusNoContent)
	invoke("mail_runtime_wrapper", user, map[string]interface{}{}, http.StatusForbidden)
	invoke("mail_service_status", user, map[string]interface{}{}, http.StatusForbidden)

	certificate, root := thunderbirdFixtureCertificate(t, "localhost")
	accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "certificate", certificate)
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	imapAddress := occupied.Addr().String()
	smtpAddress := fmt.Sprintf("127.0.0.1:%d", freeTransportE2EPort(t, make(map[int]bool)))
	accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "mail_server", map[string]interface{}{
		"hostname": "localhost", "is_enabled": true, "listen_interface": smtpAddress, "max_size": 30000000,
		"max_clients": 4, "xclient_on": false, "always_on_tls": false, "authentication_required": true,
	})
	configure := func(key, value string) {
		t.Helper()
		request, err := http.NewRequest(http.MethodPost, fixture.URL+"/_config/backend/"+key, strings.NewReader(value))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+admin)
		response, err := fixture.Client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("configure %s: %d", key, response.StatusCode)
		}
	}
	configure("imap.enabled", "true")
	configure("imap.hostname", "localhost")
	configure("imap.listen_interface", imapAddress)
	if status()["imap"].(map[string]interface{})["state"] != "disabled" {
		t.Fatal("saved settings became runtime observations before restart")
	}
	t.Setenv("DAPTIN_DISABLE_SMTP", "true")
	fixture.Process.stopProcess()
	fixture = startDaptinE2E(t, options)
	failed := status()
	imap := failed["imap"].(map[string]interface{})
	smtpStatus := failed["smtp"].(map[string]interface{})
	if imap["state"] != "failed" || imap["error_code"] != "listen_failed" || imap["certificate_loaded"] != true || imap["configured_enabled"] != true || imap["bound_address"] != nil {
		t.Fatalf("occupied IMAP: %#v", imap)
	}
	if smtpStatus["state"] != "disabled" || smtpStatus["reason"] != "environment_disabled" || smtpStatus["configured_enabled"] != true {
		t.Fatalf("SMTP suppression: %#v", smtpStatus)
	}
	ready, err := fixture.Client.Get(fixture.URL + "/ready")
	if err != nil {
		t.Fatal(err)
	}
	ready.Body.Close()
	if ready.StatusCode != http.StatusOK {
		t.Fatalf("mail failure changed HTTP readiness: %d", ready.StatusCode)
	}
	// A configured certificate without its private key must report the
	// initializer's certificate failure, independently of HTTP availability.
	accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "certificate", map[string]interface{}{
		"hostname": "missing-key.test", "issuer": "fixture", "certificate_pem": root,
	})
	configure("imap.hostname", "missing-key.test")
	fixture.Process.stopProcess()
	fixture = startDaptinE2E(t, options)
	certificateFailure := status()["imap"].(map[string]interface{})
	if certificateFailure["state"] != "failed" || certificateFailure["error_code"] != "certificate_failed" || certificateFailure["certificate_loaded"] != false {
		t.Fatalf("certificate failure: %#v", certificateFailure)
	}
	configure("imap.hostname", "localhost")
	occupied.Close()
	t.Setenv("DAPTIN_DISABLE_SMTP", "false")
	fixture.Process.stopProcess()
	fixture = startDaptinE2E(t, options)
	healthy := status()
	imap = healthy["imap"].(map[string]interface{})
	smtpStatus = healthy["smtp"].(map[string]interface{})
	if healthy["runtime_id"] == failed["runtime_id"] || imap["state"] != "running" || imap["error_code"] != nil || imap["bound_address"] != imapAddress {
		t.Fatalf("IMAP recovery: %#v", healthy)
	}
	listeners := smtpStatus["listeners"].([]interface{})
	if smtpStatus["state"] != "running" || len(listeners) != 1 || listeners[0].(map[string]interface{})["bound_address"] != smtpAddress {
		t.Fatalf("SMTP running: %#v", smtpStatus)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(root)) {
		t.Fatal("invalid fixture CA")
	}
	tlsConfig := &tls.Config{RootCAs: roots, ServerName: "localhost", MinVersion: tls.VersionTLS12}
	native, err := imapclient.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, imapAddress)
	if err != nil {
		t.Fatal(err)
	}
	native.Timeout = 5 * time.Second
	defer native.Terminate()
	if err := native.StartTLS(tlsConfig); err != nil {
		t.Fatal(err)
	}
	if err := native.Logout(); err != nil {
		t.Fatal(err)
	}
	connection, err := net.DialTimeout("tcp", smtpAddress, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(5 * time.Second))
	submission, err := smtp.NewClient(connection, "localhost")
	if err != nil {
		t.Fatal(err)
	}
	defer submission.Close()
	if err := submission.StartTLS(tlsConfig); err != nil {
		t.Fatal(err)
	}
	if err := submission.Quit(); err != nil {
		t.Fatal(err)
	}

	// Reload success is a configuration event; inspect the actual new listener.
	blocked, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer blocked.Close()
	reloadCertificate, _ := thunderbirdFixtureCertificate(t, "reload.test")
	accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "certificate", reloadCertificate)
	accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "mail_server", map[string]interface{}{
		"hostname": "reload.test", "is_enabled": true, "listen_interface": blocked.Addr().String(), "max_size": 30000000,
		"max_clients": 4, "xclient_on": false, "always_on_tls": false, "authentication_required": true,
	})
	if len(status()["smtp"].(map[string]interface{})["listeners"].([]interface{})) != 1 {
		t.Fatal("saved SMTP settings replaced active listener observations")
	}
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail_server/sync_mail_servers", admin, map[string]interface{}{"attributes": map[string]interface{}{}}, http.StatusOK)
	reloaded := status()["smtp"].(map[string]interface{})
	if reloaded["state"] != "failed" || len(reloaded["listeners"].([]interface{})) != 2 {
		t.Fatalf("SMTP reload: %#v", reloaded)
	}
	states := make(map[string]string)
	for _, value := range reloaded["listeners"].([]interface{}) {
		listener := value.(map[string]interface{})
		states[listener["listen_interface"].(string)] = listener["state"].(string)
		if listener["listen_interface"] == blocked.Addr().String() && (listener["state"] != "failed" || listener["error_code"] != "listen_failed" || listener["bound_address"] != nil) {
			t.Fatalf("occupied SMTP: %#v", listener)
		}
	}
	if states[smtpAddress] != "running" || states[blocked.Addr().String()] != "failed" {
		t.Fatalf("SMTP listener isolation: %#v", states)
	}

}
