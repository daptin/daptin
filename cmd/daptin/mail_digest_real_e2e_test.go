package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/artpar/go-imap"
	imapclient "github.com/artpar/go-imap/client"
)

func checkNativeDigestFetch(t *testing.T, native *imapclient.Client, conn io.Writer, rawReader *bufio.Reader) {
	t.Helper()
	const mailbox = "DigestFetch"
	const enclosedHeader = "From: sender@example.test\r\nSubject: Enclosed message\r\nContent-Type: multipart/mixed; boundary=inner\r\n\r\n"
	const enclosedText = "--inner\r\nContent-Type: text/plain\r\n\r\nAttached body.\r\n--inner--\r\n"
	const enclosed = enclosedHeader + enclosedText
	const raw = "From: owner@example.test\r\nSubject: Digest fixture\r\nContent-Type: multipart/digest; boundary=digest\r\n\r\n" +
		"--digest\r\nContent-Description: Enclosure\r\n\r\n" + enclosed + "\r\n--digest--\r\n"
	if err := native.Create(mailbox); err != nil {
		t.Fatal(err)
	}
	if err := native.Append(mailbox, nil, time.Now(), strings.NewReader(raw)); err != nil {
		t.Fatal(err)
	}
	reader := imap.NewReader(rawReader)
	command := func(tag, text string) map[string]interface{} {
		t.Helper()
		if _, err := fmt.Fprintf(conn, "%s %s\r\n", tag, text); err != nil {
			t.Fatal(err)
		}
		items := map[string]interface{}{}
		for {
			response, err := imap.ReadResp(reader)
			if err != nil {
				t.Fatal(err)
			}
			if status, ok := response.(*imap.StatusResp); ok && status.Tag == tag {
				if status.Type != imap.StatusRespOk {
					t.Errorf("%s: %s %s", text, status.Type, status.Info)
				}
				return items
			}
			name, fields, ok := imap.ParseNamedResp(response)
			if ok && name == "FETCH" && len(fields) == 2 {
				list := fields[1].([]interface{})
				for index := 0; index < len(list); index += 2 {
					items[list[index].(string)] = list[index+1]
				}
			}
		}
	}
	command("di0", "EXAMINE "+mailbox)
	items := command("di1", "UID FETCH 1 (BODY BODYSTRUCTURE)")
	for _, key := range []string{"BODY", "BODYSTRUCTURE"} {
		value, ok := items[key].([]interface{})
		if !ok {
			t.Errorf("missing %s", key)
			continue
		}
		structure := new(imap.BodyStructure)
		if err := structure.Parse(value); err != nil || len(structure.Parts) != 1 || !strings.EqualFold(structure.MIMESubType, "digest") {
			t.Errorf("%s: invalid digest: %v %v", key, value, err)
			continue
		}
		part := structure.Parts[0]
		if !strings.EqualFold(part.MIMEType+"/"+part.MIMESubType, "message/rfc822") || part.Size != uint32(len(enclosed)) ||
			part.Envelope == nil || part.Envelope.Subject != "Enclosed message" || part.BodyStructure == nil || len(part.BodyStructure.Parts) != 1 {
			t.Errorf("%s: lost enclosed message envelope, size or structure: %#v", key, part)
			continue
		}
		if part.Lines != uint32(strings.Count(enclosed, "\n")) || !strings.EqualFold(part.Encoding, "7bit") ||
			part.Extended != (key == "BODYSTRUCTURE") || part.BodyStructure.Parts[0].Extended != (key == "BODYSTRUCTURE") {
			t.Errorf("%s: incorrect encoded line count, encoding or extension fields", key)
		}
	}
	for i, test := range []struct{ request, response, want string }{
		{"BODY.PEEK[]", "BODY[]", raw},
		{"BODY.PEEK[1]", "BODY[1]", enclosed},
		{"BODY.PEEK[1.MIME]", "BODY[1.MIME]", "Content-Description: Enclosure\r\n\r\n"},
		{"BODY.PEEK[1.HEADER]", "BODY[1.HEADER]", enclosedHeader},
		{"BODY.PEEK[1.HEADER.FIELDS (SUBJECT)]", "BODY[1.HEADER.FIELDS (SUBJECT)]", "Subject: Enclosed message\r\n\r\n"},
		{"BODY.PEEK[1.TEXT]", "BODY[1.TEXT]", enclosedText},
		{"BODY.PEEK[1.1]", "BODY[1.1]", "Attached body."},
		{"BODY.PEEK[1.1]<2.4>", "BODY[1.1]<2>", "tach"},
	} {
		got := command(fmt.Sprintf("di%d", i+2), "UID FETCH 1 "+test.request)
		literal, ok := got[test.response].(imap.Literal)
		if !ok {
			t.Errorf("missing %s literal: %v", test.response, got)
			continue
		}
		data, err := io.ReadAll(literal)
		if err != nil || string(data) != test.want {
			t.Errorf("%s = %q, want %q (%v)", test.response, data, test.want, err)
		}
	}
	items = command("di10", "UID FETCH 1 (UID BODY.PEEK[1.2] BODY.PEEK[1.1.HEADER])")
	for _, key := range []string{"BODY[1.2]", "BODY[1.1.HEADER]"} {
		if value, present := items[key]; !present || value != nil {
			t.Errorf("missing section must return %s NIL: %#v", key, items)
		}
	}
	t.Log("Digest FETCH checked nested structure, exact wire sections, a partial literal and missing sections")
}
