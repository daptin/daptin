package actions

import (
	"strings"
	"testing"
)

func TestMailSendServerHostnameUsesSenderRelationship(t *testing.T) {
	hostname, err := mailSendServerHostname(map[string]interface{}{}, map[string]interface{}{
		"hostname": "mail.example.test",
	})
	if err != nil {
		t.Fatalf("mailSendServerHostname returned error: %v", err)
	}
	if hostname != "mail.example.test" {
		t.Fatalf("hostname = %q, want mail.example.test", hostname)
	}
}

func TestMailSendServerHostnameRejectsDifferentAssertion(t *testing.T) {
	_, err := mailSendServerHostname(map[string]interface{}{
		"mail_server_hostname": "other.example.test",
	}, map[string]interface{}{
		"hostname": "mail.example.test",
	})
	if err == nil || !strings.Contains(err.Error(), "does not match sender mail server") {
		t.Fatalf("expected hostname mismatch, got %v", err)
	}
}
