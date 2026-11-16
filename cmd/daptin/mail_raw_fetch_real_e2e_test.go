package main

import (
	"encoding/base64"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/artpar/go-imap"
	imapclient "github.com/artpar/go-imap/client"
)

func checkNativeRawSourceFetch(t *testing.T, native *imapclient.Client, fixture *daptinE2EFixture, admin string, source func(string) []byte) {
	t.Helper()
	const folder = "RawSourceFetch"
	const lf = "From: native-alice@native.test\nSubject: Raw FETCH LF fixture\n\nStored line\n"
	const subject = "Raw FETCH unparseable fixture"
	const malformed = "From: native-alice@native.test\r\nSubject: " + subject + "\r\nMissing colon in stored header\r\n\r\nStored body.\r\n"
	if err := native.Create(folder); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{lf, "From: native-alice@native.test\r\nSubject: " + subject + "\r\n\r\nSeed.\r\n"} {
		if err := native.Append(folder, nil, time.Now(), strings.NewReader(raw)); err != nil {
			t.Fatal(err)
		}
	}
	// APPEND validates the message. Model damage already present in a stored
	// row without weakening the production append or unpack parser.
	ref := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "mail", "subject", subject)
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/mail/"+ref, admin,
		accessGroupsE2ERecordPayload("mail", ref, map[string]interface{}{"mail": base64.StdEncoding.EncodeToString([]byte(malformed)), "size": len(malformed)}), http.StatusOK)
	lfRef := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "mail", "subject", "Raw FETCH LF fixture")
	if string(source(ref)) != malformed || string(source(lfRef)) != lf {
		t.Fatal("the Office source action did not return the exact stored fixture bytes")
	}
	if _, err := native.Select(folder, false); err != nil {
		t.Fatal(err)
	}
	fetch := func(uid uint32, item imap.FetchItem, want string, size int, seen bool) {
		t.Helper()
		seq := new(imap.SeqSet)
		seq.AddNum(uid)
		messages := make(chan *imap.Message, 1)
		if err := native.UidFetch(seq, []imap.FetchItem{item, imap.FetchUid, imap.FetchRFC822Size, imap.FetchFlags}, messages); err != nil {
			t.Errorf("UID %d %s failed: %v", uid, item, err)
			return
		}
		entry := <-messages
		if entry == nil || entry.Uid != uid || entry.Size != uint32(size) || slices.Contains(entry.Flags, imap.SeenFlag) != seen {
			t.Errorf("UID %d %s: unexpected metadata %+v (size %d, Seen %t)", uid, item, entry, size, seen)
			return
		}
		section, err := imap.ParseBodySectionName(item)
		if err != nil {
			t.Fatal(err)
		}
		literal := entry.GetBody(section)
		if literal == nil {
			t.Errorf("UID %d %s: missing source literal", uid, item)
			return
		}
		got, err := io.ReadAll(literal)
		if err != nil || string(got) != want {
			t.Errorf("UID %d %s = %q, want %q (%v)", uid, item, got, want, err)
		}
	}
	fetch(1, "BODY.PEEK[]", lf, len(lf), false)
	start := strings.Index(lf, "\n\n")
	fetch(1, imap.FetchItem("BODY.PEEK[]<"+strconv.Itoa(start)+".6>"), lf[start:start+6], len(lf), false)
	fetch(2, "BODY.PEEK[]", malformed, len(malformed), false)
	fetch(2, "BODY.PEEK[]<999.10>", "", len(malformed), false)
	broken := new(imap.SeqSet)
	broken.AddNum(2)
	parsed := make(chan *imap.Message, 1)
	if err := native.UidFetch(broken, []imap.FetchItem{"BODY[HEADER]"}, parsed); err == nil || !strings.Contains(err.Error(), "could not fetch MIME section for UID 2") || len(parsed) != 0 {
		t.Errorf("malformed parsed header should fail explicitly without returning data: %v", err)
	}
	fetch(2, "BODY.PEEK[]", malformed, len(malformed), false)
	fetch(2, imap.FetchRFC822, malformed, len(malformed), true)
	fetch(2, "BODY.PEEK[]", malformed, len(malformed), true)
	if !t.Failed() {
		t.Log("BODY.PEEK[] and RFC822 matched Office source for LF and malformed stored messages; partials and Seen semantics passed")
	}
}
