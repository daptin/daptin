package actions

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeMailPartsAndBodies(t *testing.T) {
	raw := strings.Join([]string{
		"From: =?UTF-8?Q?Jos=C3=A9?= <jose@example.com>",
		"To: receiver@example.com",
		"Subject: MIME example",
		"Message-ID: <message-1@example.com>",
		"Content-Type: multipart/mixed; boundary=outer",
		"",
		"--outer",
		"Content-Type: application/octet-stream; name=first.bin",
		"Content-Disposition: attachment; filename=first.bin",
		"Content-Transfer-Encoding: base64",
		"",
		"Zmlyc3Q=",
		"--outer",
		"Content-Type: multipart/alternative; boundary=alternative",
		"",
		"--alternative",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: quoted-printable",
		"",
		"plain=20body",
		"--alternative",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<p>html body</p>",
		"--alternative--",
		"--outer",
		"Content-Type: image/png",
		"Content-Disposition: inline; filename*=UTF-8''caf%C3%A9.png",
		"Content-ID: <image-1>",
		"Content-Transfer-Encoding: base64",
		"",
		"aW1hZ2U=",
		"--outer--",
		"",
	}, "\r\n")

	got, err := normalizeMail([]byte(raw), defaultMailUnpackLimits())
	if err != nil {
		t.Fatalf("normalizeMail returned error: %v", err)
	}
	if got.PreferredTextBody != "plain body" {
		t.Fatalf("plain body = %q", got.PreferredTextBody)
	}
	if got.HTMLBody != "<p>html body</p>" {
		t.Fatalf("html body = %q", got.HTMLBody)
	}
	if len(got.Parts) != 4 {
		t.Fatalf("parts = %d, want 4", len(got.Parts))
	}
	if got.Parts[0].Kind != "attachment" || got.Parts[0].Filename != "first.bin" || string(got.Parts[0].Content) != "first" {
		t.Fatalf("first part = %#v", got.Parts[0])
	}
	if got.Parts[3].Kind != "inline" || got.Parts[3].Filename != "café.png" || got.Parts[3].ContentID != "image-1" {
		t.Fatalf("inline CID part = %#v", got.Parts[3])
	}
	if got.Parts[0].SHA256 == got.Parts[3].SHA256 || len(got.Parts[0].SHA256) != 64 {
		t.Fatalf("unexpected digests: %q %q", got.Parts[0].SHA256, got.Parts[3].SHA256)
	}
}

func TestNormalizeMailHTMLOnlyAndDeterministic(t *testing.T) {
	raw := []byte("Subject: html\r\nContent-Type: text/html; charset=utf-8\r\n\r\n<p>only html</p>")
	first, err := normalizeMail(raw, defaultMailUnpackLimits())
	if err != nil {
		t.Fatalf("first normalizeMail: %v", err)
	}
	second, err := normalizeMail(raw, defaultMailUnpackLimits())
	if err != nil {
		t.Fatalf("second normalizeMail: %v", err)
	}
	if first.PreferredTextBody != "" || first.HTMLBody != "<p>only html</p>" || len(first.Parts) != 1 {
		t.Fatalf("unexpected html-only result: %#v", first)
	}
	if first.Parts[0].Index != second.Parts[0].Index || first.Parts[0].SHA256 != second.Parts[0].SHA256 {
		t.Fatalf("normalization is not deterministic: %#v %#v", first.Parts[0], second.Parts[0])
	}
}

func TestNormalizeMailPlainTextWithoutAttachments(t *testing.T) {
	raw := []byte("From: sender@example.com\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nplain only")
	got, err := normalizeMail(raw, defaultMailUnpackLimits())
	if err != nil {
		t.Fatalf("normalizeMail returned error: %v", err)
	}
	if got.PreferredTextBody != "plain only" || got.HTMLBody != "" || len(got.Parts) != 1 {
		t.Fatalf("unexpected plain-text result: %#v", got)
	}
}

func TestNormalizeMailDecodesCharsetAndImplicitCIDInlinePart(t *testing.T) {
	raw := []byte("Content-Type: multipart/related; boundary=x\r\n\r\n" +
		"--x\r\nContent-Type: text/plain; charset=iso-8859-1\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\ncaf=E9\r\n" +
		"--x\r\nContent-Type: image/png\r\nContent-ID: <logo>\r\nContent-Transfer-Encoding: base64\r\n\r\naW1hZ2U=\r\n--x--\r\n")
	got, err := normalizeMail(raw, defaultMailUnpackLimits())
	if err != nil {
		t.Fatalf("normalizeMail returned error: %v", err)
	}
	if got.PreferredTextBody != "café" {
		t.Fatalf("decoded body = %q, want café", got.PreferredTextBody)
	}
	if len(got.Parts) != 2 || got.Parts[1].Kind != "inline" || got.Parts[1].ContentID != "logo" {
		t.Fatalf("implicit CID part was not classified inline: %#v", got)
	}
}

func TestNormalizeMailContinuedFilenameAndDuplicateNames(t *testing.T) {
	raw := strings.Join([]string{
		"Content-Type: multipart/mixed; boundary=files",
		"",
		"--files",
		"Content-Type: application/octet-stream",
		"Content-Disposition: attachment; filename*0*=UTF-8''long%20; filename*1*=name.txt",
		"",
		"one",
		"--files",
		"Content-Type: application/octet-stream",
		"Content-Disposition: attachment; filename=\"long name.txt\"",
		"",
		"two",
		"--files--",
		"",
	}, "\r\n")
	got, err := normalizeMail([]byte(raw), defaultMailUnpackLimits())
	if err != nil {
		t.Fatalf("normalizeMail returned error: %v", err)
	}
	if len(got.Parts) != 2 || got.Parts[0].Filename != "long name.txt" || got.Parts[1].Filename != "long name.txt" {
		t.Fatalf("filenames were not decoded: %#v", got.Parts)
	}
	if got.Parts[0].SHA256 == got.Parts[1].SHA256 {
		t.Fatal("duplicate filenames with distinct contents have the same digest")
	}
}

func TestNormalizeMailLimits(t *testing.T) {
	plain := []byte("Content-Type: text/plain\r\n\r\n0123456789")
	tests := []struct {
		name   string
		raw    []byte
		limits mailUnpackLimitsConfig
		code   string
	}{
		{name: "raw", raw: plain, limits: mailUnpackLimitsConfig{MaxRawBytes: 8}, code: "mail_raw_too_large"},
		{name: "part", raw: plain, limits: mailUnpackLimitsConfig{MaxPartBytes: 5}, code: "mail_part_too_large"},
		{name: "decoded total", raw: plain, limits: mailUnpackLimitsConfig{MaxDecodedBytes: 5}, code: "mail_decoded_too_large"},
		{name: "headers", raw: plain, limits: mailUnpackLimitsConfig{MaxHeaderBytes: 8}, code: "mail_headers_too_large"},
		{
			name: "part count",
			raw: []byte("Content-Type: multipart/mixed; boundary=x\r\n\r\n" +
				"--x\r\nContent-Type: text/plain\r\n\r\na\r\n" +
				"--x\r\nContent-Type: text/plain\r\n\r\nb\r\n--x--\r\n"),
			limits: mailUnpackLimitsConfig{MaxParts: 1},
			code:   "mail_too_many_parts",
		},
		{
			name: "nesting",
			raw: []byte("Content-Type: multipart/mixed; boundary=x\r\n\r\n" +
				"--x\r\nContent-Type: multipart/alternative; boundary=y\r\n\r\n" +
				"--y\r\nContent-Type: text/plain\r\n\r\na\r\n--y--\r\n--x--\r\n"),
			limits: mailUnpackLimitsConfig{MaxNestingDepth: 1},
			code:   "mail_nesting_too_deep",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			limits := defaultMailUnpackLimits()
			if test.limits.MaxRawBytes != 0 {
				limits.MaxRawBytes = test.limits.MaxRawBytes
			}
			if test.limits.MaxDecodedBytes != 0 {
				limits.MaxDecodedBytes = test.limits.MaxDecodedBytes
			}
			if test.limits.MaxPartBytes != 0 {
				limits.MaxPartBytes = test.limits.MaxPartBytes
			}
			if test.limits.MaxHeaderBytes != 0 {
				limits.MaxHeaderBytes = test.limits.MaxHeaderBytes
			}
			if test.limits.MaxParts != 0 {
				limits.MaxParts = test.limits.MaxParts
			}
			if test.limits.MaxNestingDepth != 0 {
				limits.MaxNestingDepth = test.limits.MaxNestingDepth
			}
			_, err := normalizeMail(test.raw, limits)
			assertMailUnpackErrorCode(t, err, test.code)
		})
	}
}

func TestNormalizeMailRejectsMalformedMultipart(t *testing.T) {
	raw := []byte("Content-Type: multipart/mixed\r\n\r\nbody")
	_, err := normalizeMail(raw, defaultMailUnpackLimits())
	assertMailUnpackErrorCode(t, err, "mail_malformed")
}

func TestNormalizeMailAllowsEmptyPartAtDecodedLimit(t *testing.T) {
	raw := []byte("Content-Type: multipart/mixed; boundary=x\r\n\r\n" +
		"--x\r\nContent-Type: text/plain\r\n\r\n12345\r\n" +
		"--x\r\nContent-Type: application/octet-stream\r\n\r\n\r\n--x--\r\n")
	limits := defaultMailUnpackLimits()
	limits.MaxDecodedBytes = 5
	got, err := normalizeMail(raw, limits)
	if err != nil {
		t.Fatalf("normalizeMail returned error: %v", err)
	}
	if len(got.Parts) != 2 || got.Parts[1].DecodedSize != 0 {
		t.Fatalf("parts = %#v, want a trailing empty part", got.Parts)
	}
}

func assertMailUnpackErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	var unpackErr *mailUnpackErrorDetail
	if !errors.As(err, &unpackErr) {
		t.Fatalf("error = %v, want mailUnpackErrorDetail %q", err, code)
	}
	if unpackErr.Code != code {
		t.Fatalf("error code = %q, want %q", unpackErr.Code, code)
	}
}
