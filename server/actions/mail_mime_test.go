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

func TestNormalizeFlowedPlainTextUsesDisplayTextOnly(t *testing.T) {
	for _, tc := range []struct{ name, params, wire, want string }{
		{"defaults to keeping the join space", "; format=flowed", "One two \r\nthree.\r\n", "One two three.\n"},
		{"DelSp yes with UTF-8", "; FORMAT=Flowed; DELSP=Yes", "日本 \r\n語\r\nkeep  \r\nspace", "日本語\nkeep space"},
		{"quotes and stuffing", "; format=flowed", "> one \r\n> two\r\n>> nested\r\n > literal greater\r\n From stuffed\r\n   indentation", "> one two\n>> nested\n> literal greater\nFrom stuffed\n  indentation"},
		{"quote depth and signatures break a flow", "; format=flowed; delsp=yes", "> one \r\n>> two \r\n>> -- \r\n>> signature\r\n-- \r\nplain", "> one\n>> two\n>> -- \n>> signature\n-- \nplain"},
		{"unterminated final line", "; format=flowed; delsp=yes", "One \r\nlast ", "Onelast "},
		{"fixed MIME remains literal", "; format=fixed; delsp=yes", "One \r\n From fixed\r\n", "One \r\n From fixed\r\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte("Content-Type: text/plain; charset=utf-8" + tc.params + "\r\n\r\n" + tc.wire)
			got, err := normalizeMail(raw, defaultMailUnpackLimits())
			if err != nil {
				t.Fatal(err)
			}
			if got.PreferredTextBody != tc.want || len(got.Parts) != 1 || string(got.Parts[0].Content) != tc.wire {
				t.Fatalf("display = %q, want %q; parts = %#v", got.PreferredTextBody, tc.want, got.Parts)
			}
		})
	}
}

func TestNormalizeMailKeepsSupportedAlternativesAndOpaqueLeaves(t *testing.T) {
	raw := []byte("Content-Type: multipart/alternative; boundary=x\r\n\r\n" +
		"--x\r\nContent-Type: text/plain\r\n\r\nplain fallback\r\n" +
		"--x\r\nContent-Type: text/html\r\n\r\n<p>supported</p>\r\n" +
		"--x\r\nContent-Type: text/html; charset=x-unknown-charset\r\nContent-Transfer-Encoding: base64\r\n\r\nPHNjcmlwdD4=\r\n" +
		"--x\r\nContent-Type: text/html\r\nContent-Transfer-Encoding: x-unknown-encoding\r\n\r\nraw bytes\r\n--x--\r\n")
	got, err := normalizeMail(raw, defaultMailUnpackLimits())
	if err != nil {
		t.Fatal(err)
	}
	if got.PreferredTextBody != "plain fallback" || got.HTMLBody != "<p>supported</p>" || len(got.Parts) != 4 {
		t.Fatalf("wrong bodies or part count: %q %q %d", got.PreferredTextBody, got.HTMLBody, len(got.Parts))
	}
	for i, expected := range []string{"<script>", "raw bytes"} {
		part := got.Parts[i+2]
		if part.Index != i+2 || part.MediaType != "application/octet-stream" || part.Presentation != "unselected" || string(part.Content) != expected {
			t.Errorf("unsupported leaf = %#v, want preserved opaque bytes", part)
		}
	}
}

func TestNormalizeMailKeepsOpaqueRootWithoutPresentingIt(t *testing.T) {
	raw := []byte("Subject: opaque\r\nContent-Type: text/html; charset=x-unknown-charset\r\n\r\n<p>do not show</p>")
	got, err := normalizeMail(raw, defaultMailUnpackLimits())
	if err != nil {
		t.Fatal(err)
	}
	if got.HTMLBody != "" || got.PreferredTextBody != "" || len(got.Parts) != 1 || got.Parts[0].Presentation != "unselected" {
		t.Fatal("unsupported root became message content")
	}
}

func TestNormalizeMailKeepsGoodSiblingsAfterDamagedBase64(t *testing.T) {
	raw := []byte("Content-Type: multipart/mixed; boundary=m\r\n\r\n" +
		"--m\r\nContent-Type: text/plain\r\n\r\nreadable body\r\n" +
		"--m\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=damaged.bin\r\nContent-Transfer-Encoding: base64\r\n\r\ncHJlZml4!!!!\r\n" +
		"--m\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=good.bin\r\nContent-Transfer-Encoding: base64\r\n\r\nAP8=\r\n--m--\r\n")
	got, err := normalizeMail(raw, defaultMailUnpackLimits())
	if err != nil {
		t.Fatal(err)
	}
	if got.PreferredTextBody != "readable body" || len(got.Parts) != 3 {
		t.Fatalf("sibling lost: %#v", got)
	}
	damaged, good := got.Parts[1], got.Parts[2]
	if damaged.Index != 1 || damaged.Presentation != "damaged" || damaged.DecodedSize != 0 || len(damaged.Content) != 0 {
		t.Fatalf("damaged part leaked a decoded prefix: %#v", damaged)
	}
	if good.Index != 2 || good.Presentation != "attachment" || string(good.Content) != "\x00\xff" {
		t.Fatalf("later attachment lost: %#v", good)
	}
	limits := defaultMailUnpackLimits()
	limits.MaxDecodedBytes = int64(len("readable bodyprefix"))
	_, err = normalizeMail(raw, limits)
	assertMailUnpackErrorCode(t, err, "mail_decoded_too_large")
}

func TestNormalizeMailChoosesThePreferredRelatedRepresentation(t *testing.T) {
	raw := []byte(strings.Join([]string{
		"Content-Type: multipart/mixed; boundary=outer", "", "--outer",
		"Content-Type: multipart/alternative; boundary=alt", "", "--alt",
		"Content-Type: text/plain", "", "readable fallback", "--alt",
		"Content-Type: multipart/related; boundary=low; type=\"text/html\"", "", "--low",
		"Content-Type: text/html", "", "<p>earlier rendition</p>", "--low",
		"Content-Type: image/png", "Content-ID: <figure@test>", "", "wrong image", "--low--", "--alt",
		"Content-Type: multipart/related; boundary=rich; type=\"text/html\"; start=\"<root@test>\"", "", "--rich",
		"Content-Type: text/html", "", "<p>non-root HTML</p>", "--rich",
		"Content-Type: image/png", "Content-ID: <figure@test>", "Content-Disposition: attachment; filename=right.png", "", "right image", "--rich",
		"Content-Type: text/html", "Content-ID: <root@test>", "Content-Disposition: attachment", "", "<p>preferred rendition</p><img src=\"cid:figure@test\">", "--rich--", "--alt--", "--outer",
		"Content-Type: image/png", "Content-ID: <figure@test>", "Content-Disposition: attachment; filename=independent.png", "", "independent attachment", "--outer--", "",
	}, "\r\n"))
	got, err := normalizeMail(raw, defaultMailUnpackLimits())
	if err != nil {
		t.Fatal(err)
	}
	if got.PreferredTextBody != "readable fallback" || !strings.Contains(got.HTMLBody, "preferred rendition") {
		t.Fatalf("selected bodies: plain=%q HTML=%q", got.PreferredTextBody, got.HTMLBody)
	}
	want := []string{"body", "unselected", "unselected", "unselected", "html-resource", "body", "attachment"}
	if len(got.Parts) != len(want) {
		t.Fatalf("wire parts changed: got %d, want %d", len(got.Parts), len(want))
	}
	for index, purpose := range want {
		if part := got.Parts[index]; part.Presentation != purpose || part.Index != index {
			t.Errorf("part %d = %q (index %d), want %q", index, part.Presentation, part.Index, purpose)
		}
	}
}

func TestNormalizeMailDoesNotBindAmbiguousRelatedResources(t *testing.T) {
	for _, test := range []struct{ start, rootType, html string }{
		{"root@test", "text/html", "<p>HTML root</p>"},
		{"missing@test", "text/html", ""},
		{"root@test", "text/plain", ""},
	} {
		raw := []byte(strings.Join([]string{
			"Content-Type: multipart/alternative; boundary=alt", "", "--alt", "Content-Type: text/plain", "", "plain fallback", "--alt",
			`Content-Type: multipart/related; boundary=rel; type="` + test.rootType + `"; start="<` + test.start + `>"`, "", "--rel",
			"Content-Type: text/html", "Content-ID: <root@test>", "", "<p>HTML root</p>", "--rel",
			"Content-Type: image/png", "Content-ID: <asset@test>", "", "first image", "--rel",
			"Content-Type: image/png", "Content-ID: <asset@test>", "", "second image", "--rel--", "--alt--", "",
		}, "\r\n"))
		got, err := normalizeMail(raw, defaultMailUnpackLimits())
		if err != nil {
			t.Fatal(err)
		}
		if got.PreferredTextBody != "plain fallback" || got.HTMLBody != test.html {
			t.Fatalf("start=%q, type=%q: plain=%q HTML=%q", test.start, test.rootType, got.PreferredTextBody, got.HTMLBody)
		}
		for _, part := range got.Parts {
			if part.Presentation == "html-resource" {
				t.Fatal("ambiguous attachment was bound to HTML")
			}
		}
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

func TestNormalizeMailExposesCalendarWithoutPresentingItAsBody(t *testing.T) {
	for _, fixture := range []struct{ header, filename string }{
		{"Content-Type: text/calendar; method=REQUEST", "event.ics"},
		{"Content-Type: text/calendar\r\nContent-Disposition: attachment; filename=meeting.ics", "meeting.ics"},
	} {
		const payload = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nEND:VCALENDAR\r\n"
		result, err := normalizeMail([]byte(fixture.header+"\r\n\r\n"+payload), defaultMailUnpackLimits())
		if err != nil {
			t.Fatal(err)
		}
		if result.PreferredTextBody != "" || result.HTMLBody != "" || len(result.Parts) != 1 {
			t.Fatalf("calendar was treated as the reader body: %#v", result)
		}
		part := result.Parts[0]
		if part.Presentation != "attachment" || part.Filename != fixture.filename || string(part.Content) != payload {
			t.Fatalf("unexpected calendar artifact: %#v", part)
		}
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
