package resource

import (
	"bytes"
	"encoding/base64"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-message/mail"
)

func TestComposeMailMIMEAndEnvelope(t *testing.T) {
	value := MailComposition{
		To: []MailRecipient{{Name: "Alice नमस्ते", Address: "alice@example.test"}}, Cc: []MailRecipient{{Address: "copy@example.test"}}, Bcc: []MailRecipient{{Address: "hidden@example.test"}},
		Subject: "नमस्ते — Invoice", Text: "Plain alternative", HTML: `<p>こんにちは</p><img src="cid:logo@test">`, InReplyTo: "<original@test>", References: []string{"<root@test>", "<original@test>"},
		Parts: []MailComposePart{{Filename: "logo.png", MediaType: "image/png", ContentID: "logo@test", Base64: base64.StdEncoding.EncodeToString([]byte("raster fixture"))}, {Filename: "résumé.txt", MediaType: "text/plain", Base64: base64.StdEncoding.EncodeToString([]byte("attachment bytes"))}},
	}
	raw, envelope, err := ComposeOutgoingMail("owner@example.test", "<test@example.test>", time.Now(), value)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "\r\nBcc:") || len(envelope) != 3 || envelope[2] != "hidden@example.test" {
		t.Fatalf("Bcc wire/envelope separation failed: %s %#v", raw, envelope)
	}
	reader, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if subject, _ := reader.Header.Subject(); subject != value.Subject {
		t.Fatalf("subject = %q", subject)
	}
	if ids, _ := reader.Header.MsgIDList("References"); len(ids) != 2 || ids[1] != "original@test" {
		t.Fatalf("references = %#v", ids)
	}
	seen := map[string]string{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(part.Body)
		if err != nil {
			t.Fatal(err)
		}
		switch header := part.Header.(type) {
		case *mail.InlineHeader:
			media, _, _ := header.ContentType()
			seen[media] = string(data)
		case *mail.AttachmentHeader:
			filename, _ := header.Filename()
			seen[filename] = string(data)
		}
	}
	if seen["text/plain"] != "Plain alternative" || !strings.Contains(seen["text/html"], "こんにちは") || seen["résumé.txt"] != "attachment bytes" {
		t.Fatalf("MIME parts lost: %#v", seen)
	}
	draft, err := ComposeDraftMail("owner@example.test", "<test@example.test>", time.Now(), value)
	if err != nil {
		t.Fatal(err)
	}
	draftReader, err := mail.CreateReader(bytes.NewReader(draft))
	if err != nil {
		t.Fatal(err)
	}
	defer draftReader.Close()
	bcc, err := draftReader.Header.AddressList("Bcc")
	if err != nil || len(bcc) != 1 || bcc[0].Address != "hidden@example.test" {
		t.Fatal("durable draft lost Bcc")
	}
}

func TestComposeMailRejectsInjectionUnresolvedImagesAndUnsupportedAddresses(t *testing.T) {
	for name, value := range map[string]MailComposition{
		"subject injection":    {Subject: "Hi\r\nBcc: other@example.test"},
		"address injection":    {To: []MailRecipient{{Address: "one@example.test\r\nBcc: other@example.test"}}},
		"SMTPUTF8":             {To: []MailRecipient{{Address: "用户@example.test"}}},
		"ephemeral image":      {HTML: `<img src="blob:unpersisted">`},
		"missing CID":          {HTML: `<img src="cid:missing@test">`},
		"SVG inline":           {Parts: []MailComposePart{{MediaType: "image/svg+xml", ContentID: "x@test", Base64: "YWJj"}}},
		"malformed attachment": {Parts: []MailComposePart{{MediaType: "multipart/mixed", Base64: "YWJj"}}},
		"invalid reply header": {InReplyTo: "<one@test>\r\nSubject: injected"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := ComposeOutgoingMail("owner@example.test", "id@test", time.Now(), value); err == nil {
				t.Fatal("unsafe composition accepted")
			}
		})
	}
}

func TestComposeMailPlainOnlyDoesNotAdvertiseAnEmptyHTMLAlternative(t *testing.T) {
	raw, _, err := ComposeOutgoingMail("owner@example.test", "plain@test", time.Now(), MailComposition{Text: "Visible plain message"})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	parts := 0
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		parts++
		header := part.Header.(*mail.InlineHeader)
		mediaType, _, _ := header.ContentType()
		if mediaType != "text/plain" {
			t.Fatalf("plain-only composition advertises %s", mediaType)
		}
		body, err := io.ReadAll(part.Body)
		if err != nil || string(body) != "Visible plain message" {
			t.Fatalf("plain body lost: %q %v", body, err)
		}
	}
	if parts != 1 {
		t.Fatalf("plain-only message parts = %d", parts)
	}
}

func TestComposeMailPreservesOpaqueInlineCIDIdentifiers(t *testing.T) {
	value := MailComposition{Text: "Inline image", HTML: `<img src="CID:ii_inline_image">`, Parts: []MailComposePart{{Filename: "image.png", MediaType: "image/png", ContentID: "ii_inline_image", Base64: "YWJj"}}}
	raw, _, err := ComposeOutgoingMail("owner@example.test", "message@test", time.Now(), value)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Content-Id: <ii_inline_image>") && !strings.Contains(string(raw), "Content-ID: <ii_inline_image>") {
		t.Fatalf("inline CID not preserved: %s", raw)
	}
}

func TestMailOutgoingSavedDraftPreservesMIME(t *testing.T) {
	value := MailComposition{
		To: []MailRecipient{{Name: "こんにちは", Address: "recipient@example.test"}}, Cc: []MailRecipient{{Address: "copy@example.test"}}, Bcc: []MailRecipient{{Address: "hidden@example.test"}, {Address: "recipient@example.test"}},
		Subject: "Signed reply नमस्ते", Text: "plain", HTML: `<p>HTML</p><img src="cid:ii_logo">`, InReplyTo: "<original@test>", References: []string{"<root@test>", "<original@test>"},
		Parts: []MailComposePart{{Filename: "logo.png", MediaType: "image/png", ContentID: "ii_logo", Base64: "cG5n"}, {Filename: "résumé.txt", MediaType: "text/plain", Base64: "cHJvb2Y="}},
	}
	raw, err := ComposeDraftMail("owner@example.test", "draft@example.test", time.Now(), value)
	if err != nil {
		t.Fatal(err)
	}
	outgoing, envelope, err := outgoingDraft("owner@example.test", raw, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(envelope) != 3 || envelope[2] != "hidden@example.test" {
		t.Fatalf("envelope = %#v", envelope)
	}
	split := func(raw []byte) []byte { return bytes.SplitN(raw, []byte("\r\n\r\n"), 2)[1] }
	if !bytes.Equal(split(raw), split(outgoing)) {
		t.Fatal("saved MIME bytes changed")
	}
	message, err := mail.CreateReader(bytes.NewReader(outgoing))
	if err != nil {
		t.Fatal(err)
	}
	defer message.Close()
	if message.Header.Get("Bcc") != "" || message.Header.Get("From") != "<owner@example.test>" {
		t.Fatalf("transport headers = %#v", message.Header)
	}
	if message.Header.Get("In-Reply-To") != "<original@test>" {
		t.Fatal("reply context lost")
	}
	if subject, _ := message.Header.Subject(); subject != value.Subject {
		t.Fatal("unicode subject lost")
	}
}

func TestMailOutgoingDraftRejectsMissingAndDuplicateRecipients(t *testing.T) {
	for _, header := range []string{"", "To: first@example.test\r\nTo: second@example.test\r\n", "To: user@\r\n", "To: नमस्ते@example.test\r\n"} {
		if _, _, err := outgoingDraft("owner@example.test", []byte("From: spoof@example.test\r\n"+header+"\r\nbody"), time.Now()); err == nil {
			t.Fatalf("accepted recipients %q", header)
		}
	}
	raw := []byte("From: spoof@example.test\r\nTo: recipient@example.test\r\nBcc: hidden@example.test\r\nResent-Bcc: leaked@example.test\r\nSender: spoof@example.test\r\nReturn-Path: spoof@example.test\r\nDKIM-Signature: fake\r\n\r\nbody")
	outgoing, _, err := outgoingDraft("owner@example.test", raw, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"spoof@example.test", "Bcc:", "Sender:", "DKIM-Signature:", "Return-Path:"} {
		if strings.Contains(string(outgoing), forbidden) {
			t.Fatalf("retained transport header %q", forbidden)
		}
	}
}
