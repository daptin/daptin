package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
	"github.com/emersion/go-message"
	"github.com/google/uuid"
)

type loopbackSMTPReceipt struct {
	from, to string
	data     []byte
}

// Exercise the production sender against an ephemeral loopback SMTP sink.
// The normal administrator-only backend setting supplies its unprivileged port.
func TestMailLoopbackSMTPAndCrashRealE2E(t *testing.T) {
	requireRealE2E(t)
	// PostgreSQL TIMESTAMP has no timezone; keep this test sensitive to a
	// worker accidentally reinterpreting the scheduled wall time as UTC.
	t.Setenv("TZ", "Asia/Kolkata")
	smtpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	receipts := make(chan loopbackSMTPReceipt, 10)
	var holdAck atomic.Bool
	releaseAck := make(chan struct{})
	stopped := make(chan struct{})
	var connections sync.Map
	var workers sync.WaitGroup
	t.Cleanup(func() {
		close(stopped)
		_ = smtpListener.Close()
		connections.Range(func(key, value interface{}) bool { _ = key.(net.Conn).Close(); return true })
		workers.Wait()
	})
	workers.Add(1)
	go func() {
		defer workers.Done()
		for {
			connection, err := smtpListener.Accept()
			if err != nil {
				return
			}
			connections.Store(connection, true)
			workers.Add(1)
			go func() {
				defer workers.Done()
				defer connection.Close()
				defer connections.Delete(connection)
				_ = connection.SetDeadline(time.Now().Add(30 * time.Second))
				reader := bufio.NewReader(connection)
				reply := func(line string) bool { _, err := fmt.Fprintf(connection, "%s\r\n", line); return err == nil }
				if !reply("220 localhost disposable SMTP sink") {
					return
				}
				receipt := loopbackSMTPReceipt{}
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					command := strings.TrimSpace(line)
					upper := strings.ToUpper(command)
					switch {
					case strings.HasPrefix(upper, "EHLO "), strings.HasPrefix(upper, "HELO "):
						if !reply("250 localhost") {
							return
						}
					case strings.HasPrefix(upper, "MAIL FROM:"):
						receipt.from = strings.Trim(command[len("MAIL FROM:"):], "<>")
						if !reply("250 sender accepted") {
							return
						}
					case strings.HasPrefix(upper, "RCPT TO:"):
						receipt.to = strings.Trim(command[len("RCPT TO:"):], "<>")
						if !reply("250 recipient accepted") {
							return
						}
					case upper == "DATA":
						if !reply("354 send message") {
							return
						}
						var body bytes.Buffer
						for {
							part, err := reader.ReadString('\n')
							if err != nil {
								return
							}
							if part == ".\r\n" || part == ".\n" {
								break
							}
							if strings.HasPrefix(part, "..") {
								part = part[1:]
							}
							body.WriteString(part)
						}
						receipt.data = append([]byte(nil), body.Bytes()...)
						select {
						case receipts <- receipt:
						case <-stopped:
							return
						}
						if holdAck.Load() {
							select {
							case <-releaseAck:
							case <-stopped:
								return
							}
						}
						if !reply("250 queued in disposable sink") {
							return
						}
					case upper == "QUIT":
						reply("221 bye")
						return
					case upper == "RSET":
						receipt = loopbackSMTPReceipt{}
						if !reply("250 reset") {
							return
						}
					default:
						if !reply("502 unsupported") {
							return
						}
					}
				}
			}()
		}
	}()
	options := mailProtocolDatabase(t)
	fixture := startDaptinE2E(t, options)
	admin := fixture.SignupAdmin(t)
	configurePort := func(value string) {
		request, err := http.NewRequest(http.MethodPost, fixture.URL+"/_config/backend/outbox.smtp_port", strings.NewReader(value))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+admin)
		response, err := fixture.Client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("configure SMTP port: HTTP %d", response.StatusCode)
		}
	}
	_, smtpPort, err := net.SplitHostPort(smtpListener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	configurePort(smtpPort)

	alice := accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, admin, "smtp-alice")
	aliceRef := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "user_account", "email", "smtp-alice@test.local")
	private := int64(auth.UserCRUD | auth.UserExecute)
	seed := func(table string, attributes map[string]interface{}) string {
		ref := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, table, attributes)
		accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/"+table+"/"+ref, admin, accessGroupsE2ERecordPayload(table, ref, map[string]interface{}{"user_account_id": aliceRef, "permission": private}), http.StatusOK)
		return ref
	}
	serverRef := seed("mail_server", map[string]interface{}{"hostname": "smtp.test", "is_enabled": false, "listen_interface": "127.0.0.1:0", "max_size": 30000000, "max_clients": 1, "xclient_on": false, "always_on_tls": false, "authentication_required": true})
	accountRef := seed("mail_account", map[string]interface{}{"username": "smtp-alice@smtp.test", "password": "fixture-password", "password_md5": "fixture-password", "mail_server_id": serverRef})
	certificate := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "certificate", map[string]interface{}{"hostname": "smtp.test", "issuer": "self"})
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, fixture.URL+"/action/certificate/generate_self_certificate", admin, map[string]interface{}{"attributes": map[string]interface{}{"certificate_id": certificate}}, http.StatusOK)
	action := func(name, family string, input map[string]interface{}) map[string]interface{} {
		response := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail_account/"+name, alice, map[string]interface{}{"attributes": map[string]interface{}{family: input}}, http.StatusOK)
		return response.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})
	}
	const attachedWire = "Subject: Attached original\r\nFrom: original@example.test\r\n\r\n.Dot-start line\r\nExact attached bytes.\r\n"
	prepare := func(subject string, includeBCC bool, extraParts ...map[string]interface{}) (map[string]interface{}, string, []byte) {
		key := uuid.NewString()
		content := map[string]interface{}{"subject": subject, "text": "Local SMTP नमस्ते", "html": "<p>Local SMTP नमस्ते</p><img src=\"cid:proof@smtp\">", "to": []map[string]interface{}{{"address": "recipient@localhost"}},
			"in_reply_to": "<original@test>", "references": []string{"<original@test>"}, "parts": []map[string]interface{}{{"filename": "proof.png", "media_type": "image/png", "content_id": "proof@smtp", "base64": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aH2kAAAAASUVORK5CYII="}, {"filename": "résumé.txt", "media_type": "text/plain", "base64": "cHJvb2Y="}}}
		if includeBCC {
			content["cc"] = []map[string]interface{}{{"address": "Recipient@LOCALHOST"}, {"address": "recipient@LOCALHOST"}}
			content["bcc"] = []map[string]interface{}{{"address": "hidden@localhost"}}
			content["parts"] = append(content["parts"].([]map[string]interface{}), map[string]interface{}{
				"filename": "original.eml", "media_type": "message/rfc822", "base64": base64.StdEncoding.EncodeToString([]byte(attachedWire)),
			})
		}
		content["parts"] = append(content["parts"].([]map[string]interface{}), extraParts...)
		saved := action("draft_save", "draft", map[string]interface{}{"account_ref": accountRef, "compose_key": key, "write_key": uuid.NewString(), "content": content})
		submission := map[string]interface{}{"account_ref": accountRef, "compose_key": key, "submission_key": uuid.NewString(), "expected_revision": saved["revision"]}
		queued := action("submit", "submission", submission)
		ref := queued["reference_id"].(string)
		if ref != saved["reference_id"] {
			t.Fatal("submission did not preserve the canonical draft reference")
		}
		source := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail/source", alice, map[string]interface{}{"attributes": map[string]interface{}{"message_ref": ref}}, http.StatusOK)
		encoded, _ := accessGroupsE2EFindString(source, "raw_base64")
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		return submission, ref, raw
	}
	process := func() {
		// One case deliberately waits for the production 30-second attempt deadline.
		client := *fixture.Client
		client.Timeout = 45 * time.Second
		accessGroupsE2EAssertStatus(t, &client, http.MethodPost, fixture.URL+"/action/outbox/process_outbox", admin, map[string]interface{}{"attributes": map[string]interface{}{}}, http.StatusOK)
	}
	receive := func() loopbackSMTPReceipt {
		select {
		case receipt := <-receipts:
			return receipt
		case <-time.After(15 * time.Second):
			t.Fatalf("no loopback SMTP DATA received\n%s", fixture.Process.logs.String())
			return loopbackSMTPReceipt{}
		}
	}
	submission, _, wire := prepare("Accepted loopback submission", true)
	configurePort("65536")
	invalid := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/outbox/process_outbox", admin, map[string]interface{}{"attributes": map[string]interface{}{}}, http.StatusInternalServerError)
	if !strings.Contains(fmt.Sprint(invalid), "outbox.smtp_port") {
		t.Fatalf("invalid SMTP configuration not rejected: %#v", invalid)
	}
	queuedStatus := action("submission_status", "submission", submission)
	queuedRecipients := queuedStatus["recipients"].([]interface{})
	if len(queuedRecipients) != 3 {
		t.Fatalf("expected three distinct queued recipients: %#v", queuedStatus)
	}
	for _, value := range queuedRecipients {
		if value.(map[string]interface{})["state"] != "queued" {
			t.Fatalf("invalid port claimed queued mail: %#v", queuedStatus)
		}
	}
	select {
	case <-receipts:
		t.Fatal("invalid port dispatched SMTP")
	default:
	}
	configurePort(smtpPort)
	process()
	recipients := map[string]bool{}
	for i := 0; i < 3; i++ {
		receipt := receive()
		if recipients[receipt.to] {
			t.Fatalf("duplicate loopback recipient %q", receipt.to)
		}
		recipients[receipt.to] = true
		if receipt.from != "smtp-alice@smtp.test" || !bytes.Equal(receipt.data, wire) || bytes.Contains(receipt.data, []byte("\r\nBcc:")) || !bytes.Contains(receipt.data, []byte("DKIM-Signature:")) {
			t.Fatal("SMTP envelope or signed multipart MIME changed")
		}
		outer, err := message.Read(bytes.NewReader(receipt.data))
		if err != nil {
			t.Fatal(err)
		}
		attached := false
		if err := outer.Walk(func(_ []int, part *message.Entity, err error) error {
			if err != nil {
				return err
			}
			media, _, _ := part.Header.ContentType()
			if media != "message/rfc822" {
				return nil
			}
			disposition, params, _ := part.Header.ContentDisposition()
			payload, err := io.ReadAll(part.Body)
			if err != nil || attached || disposition != "attachment" || params["filename"] != "original.eml" ||
				part.Header.Get("Content-Transfer-Encoding") != "7bit" || string(payload) != attachedWire {
				return fmt.Errorf("SMTP attached RFC822 bytes, disposition or transfer encoding changed")
			}
			attached = true
			return nil
		}); err != nil || !attached {
			t.Fatalf("SMTP receiver did not recover the exact 7bit .eml: %v", err)
		}
	}
	if !recipients["recipient@localhost"] || !recipients["Recipient@localhost"] || !recipients["hidden@localhost"] {
		t.Fatalf("case-sensitive To/Cc or Bcc envelope recipients missing: %v", recipients)
	}
	select {
	case extra := <-receipts:
		t.Fatalf("unexpected additional loopback recipient %q", extra.to)
	default:
	}
	t.Log("Loopback SMTP received both case-distinct local parts and the hidden recipient, with exact 7bit .eml bytes")
	status := action("submission_status", "submission", submission)
	for _, value := range status["recipients"].([]interface{}) {
		if value.(map[string]interface{})["state"] != "smtp_accepted" {
			t.Fatalf("accepted SMTP not attributed: %#v", status)
		}
	}
	// Match the file from the Office browser size-recovery qualification and
	// verify both the signed wire source and the independently decoded file.
	const largeFilename = "retained.bin"
	largeAttachment := bytes.Repeat([]byte{'A'}, 16*1024*1024)
	largeSubmission, largeRef, largeWire := prepare("Large attachment submission", false, map[string]interface{}{
		"filename": largeFilename, "media_type": "application/octet-stream", "base64": base64.StdEncoding.EncodeToString(largeAttachment),
	})
	process()
	largeReceipt := receive()
	if largeReceipt.from != "smtp-alice@smtp.test" || largeReceipt.to != "recipient@localhost" || !bytes.Equal(largeReceipt.data, largeWire) {
		t.Fatal("large submission changed the SMTP recipient or signed MIME bytes")
	}
	outer, err := message.Read(bytes.NewReader(largeReceipt.data))
	if err != nil {
		t.Fatal(err)
	}
	if outer.Header.Get("DKIM-Signature") == "" {
		t.Fatal("large SMTP submission is missing its DKIM signature")
	}
	expectedHash := sha256.Sum256(largeAttachment)
	var largePartCount int
	fileHash := sha256.New()
	if err := outer.Walk(func(_ []int, part *message.Entity, err error) error {
		if err != nil {
			return err
		}
		disposition, params, _ := part.Header.ContentDisposition()
		if params["filename"] != largeFilename {
			return nil
		}
		media, _, _ := part.Header.ContentType()
		if disposition != "attachment" || media != "application/octet-stream" {
			return fmt.Errorf("wrong disposition or content type for %s", largeFilename)
		}
		largePartCount++
		length, err := io.Copy(fileHash, part.Body)
		if err != nil || length != int64(len(largeAttachment)) {
			return fmt.Errorf("decoded %d attachment bytes, expected %d: %v", length, len(largeAttachment), err)
		}
		return nil
	}); err != nil || largePartCount != 1 || !bytes.Equal(fileHash.Sum(nil), expectedHash[:]) {
		t.Fatalf("SMTP receiver did not recover exactly one intact large attachment: count %d, error %v", largePartCount, err)
	}
	largeStatus := action("submission_status", "submission", largeSubmission)
	largeStates := largeStatus["recipients"].([]interface{})
	if largeStatus["reference_id"] != largeRef || len(largeStates) != 1 || largeStates[0].(map[string]interface{})["state"] != "smtp_accepted" {
		t.Fatalf("large SMTP submission lost canonical acceptance: %#v", largeStatus)
	}
	process()
	select {
	case <-receipts:
		t.Fatal("accepted large attachment was redelivered")
	default:
	}
	t.Logf("Loopback SMTP accepted %d signed MIME bytes and recovered %d attachment bytes (SHA-256 %x), without replay", len(largeWire), len(largeAttachment), fileHash.Sum(nil))
	// A stalled final acknowledgment must expire through the actual action's
	// deadline, retain uncertainty and stay ineligible for another attempt.
	holdAck.Store(true)
	timedOut, timeoutRef, timeoutWire := prepare("SMTP acknowledgment deadline", false)
	started := time.Now()
	process()
	elapsed := time.Since(started)
	if elapsed < 25*time.Second || elapsed > 40*time.Second {
		t.Fatalf("SMTP deadline took %s", elapsed)
	}
	if receipt := receive(); !bytes.Equal(receipt.data, timeoutWire) {
		t.Fatal("timeout changed SMTP MIME")
	}
	timeoutStatus := action("submission_status", "submission", timedOut)
	timeoutStates := timeoutStatus["recipients"].([]interface{})
	if timeoutStatus["reference_id"] != timeoutRef || len(timeoutStates) != 1 || timeoutStates[0].(map[string]interface{})["state"] != "uncertain" {
		t.Fatalf("deadline lost canonical uncertainty: %#v", timeoutStatus)
	}
	process()
	select {
	case <-receipts:
		t.Fatal("deadline uncertainty was redelivered")
	default:
	}
	t.Logf("Production SMTP attempt deadline qualified in %s", elapsed)

	// Hold the final SMTP acknowledgment after the entire body is received,
	// then abruptly kill the worker's process. The sink has the bytes; sender
	// acceptance is unknown. A fresh Olric lease must not cause redelivery.
	holdAck.Store(true)
	uncertain, uncertainRef, _ := prepare("Crash during SMTP acknowledgment", false)
	processURL := fixture.URL + "/action/outbox/process_outbox"
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		request, _ := http.NewRequest(http.MethodPost, processURL, strings.NewReader(`{"attributes":{}}`))
		request.Header.Set("Authorization", "Bearer "+admin)
		request.Header.Set("Content-Type", "application/json")
		response, err := fixture.Client.Do(request)
		if err == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
		}
	}()
	receive()
	if err := syscall.Kill(-fixture.Process.processGroupID, syscall.SIGKILL); err != nil {
		t.Fatalf("crash disposable worker: %v", err)
	}
	fixture.Process.stopProcess()
	close(releaseAck)
	select {
	case <-finished:
	case <-time.After(20 * time.Second):
		t.Fatal("crashed worker request did not terminate")
	}
	fixture = startDaptinE2E(t, options)
	recovered := action("submission_status", "submission", uncertain)
	if recovered["reference_id"] != uncertainRef {
		t.Fatal("crash lost canonical Sent attribution")
	}
	states := recovered["recipients"].([]interface{})
	if len(states) != 1 || states[0].(map[string]interface{})["state"] != "uncertain" {
		t.Fatalf("crash uncertainty not durable: %#v", recovered)
	}
	process()
	select {
	case <-receipts:
		t.Fatal("restarted worker redelivered an uncertain SMTP attempt")
	case <-time.After(300 * time.Millisecond):
	}
}
