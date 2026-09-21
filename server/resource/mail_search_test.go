package resource

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/artpar/go-imap"
	"github.com/daptin/daptin/server/statementbuilder"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

func TestSearchMailBoxUsesMailboxSequenceAndUIDs(t *testing.T) {
	db := newMailSearchTestDB(t)
	crud := &DbResource{connection: db}

	for index := 1; index <= 12; index++ {
		subject := fmt.Sprintf("message-%02d", index)
		from := "alice@example.test"
		if index == 2 {
			subject = "unique second message"
			from = "bob@example.test"
		}
		_, err := db.Exec(`INSERT INTO mail
			(id, reference_id, mail_box_id, uid, subject, from_address, to_address, cc_address, bcc_address,
             sender_address, reply_to_address, message_id, body, internal_date, sent_date,
             size, seen, recent, deleted, flags)
			VALUES (?, ?, 7, ?, ?, ?, 'recipient@example.test', '', '', '', '', ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			index, mailSearchReferenceID(index), 100+index, subject, from, fmt.Sprintf("<message-%02d@example.test>", index),
			"body "+subject, time.Date(2026, time.January, index, 12, 0, 0, 0, time.UTC),
			time.Date(2025, time.December, index, 12, 0, 0, 0, time.UTC), 100+index,
			index == 1, index == 12, index == 3, mailSearchTestFlags(index))
		if err != nil {
			t.Fatal(err)
		}
	}

	assertMailSearch(t, crud, false, searchHeader("Subject", "unique second"), []uint32{2})
	assertMailSearch(t, crud, false, searchHeader("From", "nobody@example.test"), nil)
	assertMailSearch(t, crud, false, imap.NewSearchCriteria(), []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12})
	assertMailSearch(t, crud, true, searchHeader("Message-ID", "message-12"), []uint32{112})

	body := imap.NewSearchCriteria()
	body.Body = []string{"unique second"}
	assertMailSearch(t, crud, false, body, []uint32{2})
	text := imap.NewSearchCriteria()
	text.Text = []string{"bob@example.test"}
	assertMailSearch(t, crud, false, text, []uint32{2})
	since := imap.NewSearchCriteria()
	since.Since = time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC)
	assertMailSearch(t, crud, false, since, []uint32{5, 6, 7, 8, 9, 10, 11, 12})
	sentSince := imap.NewSearchCriteria()
	sentSince.SentSince = time.Date(2025, time.December, 10, 0, 0, 0, 0, time.UTC)
	assertMailSearch(t, crud, false, sentSince, []uint32{10, 11, 12})
	larger := imap.NewSearchCriteria()
	larger.Larger = 110
	assertMailSearch(t, crud, false, larger, []uint32{11, 12})

	unseen := imap.NewSearchCriteria()
	unseen.WithoutFlags = []string{imap.SeenFlag}
	assertMailSearch(t, crud, false, unseen, []uint32{2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12})

	deleted := imap.NewSearchCriteria()
	deleted.WithFlags = []string{imap.DeletedFlag}
	assertMailSearch(t, crud, false, deleted, []uint32{3})
	keyword := imap.NewSearchCriteria()
	keyword.WithFlags = []string{"project"}
	assertMailSearch(t, crud, false, keyword, []uint32{4})

	uidSet, err := imap.ParseSeqSet("102,105:106,112")
	if err != nil {
		t.Fatal(err)
	}
	uidCriteria := imap.NewSearchCriteria()
	uidCriteria.Uid = uidSet
	assertMailSearch(t, crud, true, uidCriteria, []uint32{102, 105, 106, 112})
	uidCriteria.Uid, err = imap.ParseSeqSet("*")
	if err != nil {
		t.Fatal(err)
	}
	assertMailSearch(t, crud, true, uidCriteria, []uint32{112})
	uidCriteria.Uid, err = imap.ParseSeqSet("999:*")
	if err != nil {
		t.Fatal(err)
	}
	assertMailSearch(t, crud, true, uidCriteria, []uint32{112})
	uidCriteria.Uid, err = imap.ParseSeqSet("4294967294")
	if err != nil {
		t.Fatal(err)
	}
	assertMailSearch(t, crud, true, uidCriteria, nil)

	sequenceCriteria := imap.NewSearchCriteria()
	sequenceCriteria.SeqNum, err = imap.ParseSeqSet("2,12")
	if err != nil {
		t.Fatal(err)
	}
	assertMailSearch(t, crud, false, sequenceCriteria, []uint32{2, 12})

	if _, err := db.Exec(`DELETE FROM mail WHERE id = 2`); err != nil {
		t.Fatal(err)
	}
	assertMailSearch(t, crud, false, searchHeader("Subject", "message-03"), []uint32{2})

	emptyBody := imap.NewSearchCriteria()
	emptyBody.Body = []string{""}
	assertMailSearch(t, crud, false, emptyBody, []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})
	emptySubject := searchHeader("Subject", "")
	assertMailSearch(t, crud, false, emptySubject, []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})
}

func TestSelectMailBoxCandidates(t *testing.T) {
	db := newMailSearchTestDB(t)
	crud := &DbResource{connection: db}
	for index, uid := range []int{31, 35, 40} {
		id := index + 1
		db.MustExec(`insert into mail
			(id, reference_id, mail_box_id, uid, subject, internal_date, sent_date, size, seen, recent, deleted, flags)
			values (?, ?, 7, ?, ?, ?, ?, 1, false, false, false, '')`,
			id, mailSearchReferenceID(id), uid, fmt.Sprintf("mail-%d", id), time.Now(), time.Now())
	}
	tx := db.MustBegin()
	defer tx.Rollback()

	sequenceSet, err := imap.ParseSeqSet("2:3")
	if err != nil {
		t.Fatal(err)
	}
	selections, err := crud.SelectMailBoxCandidates(7, false, sequenceSet, tx)
	if err != nil {
		t.Fatal(err)
	}
	if len(selections) != 2 || selections[0].SequenceNumber != 2 || selections[0].UID != 35 ||
		selections[1].SequenceNumber != 3 || selections[1].UID != 40 {
		t.Fatalf("unexpected sequence selection: %#v", selections)
	}

	uidSet, err := imap.ParseSeqSet("35:40")
	if err != nil {
		t.Fatal(err)
	}
	selections, err = crud.SelectMailBoxCandidates(7, true, uidSet, tx)
	if err != nil {
		t.Fatal(err)
	}
	if len(selections) != 2 || selections[0].SequenceNumber != 2 || selections[1].SequenceNumber != 3 {
		t.Fatalf("unexpected UID selection: %#v", selections)
	}
}

func TestExtractMailSearchMetadataKeepsAllHeaderAddresses(t *testing.T) {
	metadata, err := ExtractMailSearchMetadata([]byte("From: Alice <alice@example.test>, Bob <bob@example.test>\r\n" +
		"To: Carol <carol@example.test>, Dan <dan@example.test>\r\n" +
		"Cc: Eve <eve@example.test>\r\nBcc: Frank <frank@example.test>\r\n" +
		"Sender: Sender <sender@example.test>\r\nReply-To: replies@example.test\r\n" +
		"Subject: Searchable subject\r\nMessage-ID: <search@example.test>\r\n" +
		"Date: Tue, 06 Jan 2026 12:00:00 +0000\r\nContent-Type: text/plain\r\n\r\nSearchable body"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(metadata.From, "alice@example.test") || !strings.Contains(metadata.From, "bob@example.test") ||
		!strings.Contains(metadata.To, "carol@example.test") || !strings.Contains(metadata.To, "dan@example.test") ||
		!strings.Contains(metadata.Cc, "eve@example.test") || !strings.Contains(metadata.Bcc, "frank@example.test") ||
		!strings.Contains(metadata.Sender, "sender@example.test") || metadata.ReplyTo != "<replies@example.test>" ||
		metadata.Subject != "Searchable subject" || metadata.MessageID != "search@example.test" ||
		metadata.Body != "Searchable body" || metadata.SentDate.IsZero() {
		t.Fatalf("unexpected metadata: %#v", metadata)
	}
}

func TestSearchMailBoxComposesCriteriaInSQL(t *testing.T) {
	db := newMailSearchTestDB(t)
	crud := &DbResource{connection: db}
	rows := []struct {
		id, uid int
		subject string
		seen    bool
	}{
		{1, 21, "release alpha", false},
		{2, 22, "release beta", true},
		{3, 23, "unrelated", false},
	}
	for _, row := range rows {
		_, err := db.Exec(`INSERT INTO mail
			(id, reference_id, mail_box_id, uid, subject, from_address, to_address, cc_address, bcc_address,
             sender_address, reply_to_address, message_id, body, internal_date, sent_date,
             size, seen, recent, deleted, flags)
			VALUES (?, ?, 7, ?, ?, 'sender@example.test', '', '', '', '', '', '', 'body', ?, ?, 100, ?, 0, 0, ?)`,
			row.id, mailSearchReferenceID(row.id), row.uid, row.subject, time.Now(), time.Now(), row.seen, mailSearchTestFlagsForSeen(row.seen))
		if err != nil {
			t.Fatal(err)
		}
	}

	release := searchHeader("Subject", "release")
	release.WithoutFlags = []string{imap.SeenFlag}
	assertMailSearch(t, crud, false, release, []uint32{1})

	criteria := imap.NewSearchCriteria()
	criteria.Or = append(criteria.Or, [2]*imap.SearchCriteria{
		searchHeader("Subject", "beta"), searchHeader("Subject", "unrelated"),
	})
	criteria.Not = append(criteria.Not, searchHeader("Subject", "beta"))
	assertMailSearch(t, crud, false, criteria, []uint32{3})

	unsupported := searchHeader("X-Trace-ID", "abc")
	tx, err := db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := crud.SearchMailBoxCandidates(7, unsupported, tx); err == nil {
		t.Fatal("unsupported header search should fail instead of matching unrelated messages")
	}
}

func TestSearchMailBoxDatabaseContracts(t *testing.T) {
	t.Cleanup(func() { statementbuilder.InitialiseStatementBuilder("sqlite3") })
	tests := []struct {
		name, dialect, driver, dsn, temporary, referenceType string
	}{
		{"postgres", "postgres", "postgres", os.Getenv("DAPTIN_TEST_POSTGRES_DSN"), "temporary ", "BYTEA"},
		{"mysql", "mysql", "mysql", os.Getenv("DAPTIN_TEST_MYSQL_DSN"), "temporary ", "BINARY(16)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.dsn == "" {
				t.Skip("set the corresponding DAPTIN_TEST database DSN to run this contract")
			}
			db, err := sqlx.Open(test.driver, test.dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(1)
			if err := db.Ping(); err != nil {
				t.Fatal(err)
			}
			statementbuilder.InitialiseStatementBuilder(test.dialect)
			_, err = db.Exec(`CREATE ` + test.temporary + `TABLE mail (
				id BIGINT PRIMARY KEY, reference_id ` + test.referenceType + ` NOT NULL, mail_box_id BIGINT NOT NULL, uid BIGINT NOT NULL,
                subject TEXT, from_address TEXT, to_address TEXT, cc_address TEXT, bcc_address TEXT,
                sender_address TEXT, reply_to_address TEXT, message_id TEXT, body TEXT,
                internal_date TIMESTAMP NULL, sent_date TIMESTAMP NULL, size BIGINT,
                seen BOOLEAN, recent BOOLEAN, deleted BOOLEAN, flags TEXT
            )`)
			if err != nil {
				t.Fatal(err)
			}
			insert := db.Rebind(`INSERT INTO mail
				(id, reference_id, mail_box_id, uid, subject, from_address, body, internal_date, sent_date,
                 size, seen, recent, deleted, flags)
				VALUES (?, ?, 7, ?, ?, ?, ?, ?, ?, 100, ?, false, false, ?)`)
			for index, subject := range []string{"first", "database target", "third"} {
				seen := index == 0
				flags := mailSearchTestFlagsForSeen(seen)
				if index == 0 {
					flags += "," + imap.FlaggedFlag
				}
				if _, err := db.Exec(insert, index+1, mailSearchReferenceID(index+1), index+41, subject, "sender@example.test", "body "+subject,
					time.Now(), time.Now(), seen, flags); err != nil {
					t.Fatal(err)
				}
			}
			crud := &DbResource{connection: db}
			assertMailSearch(t, crud, false, searchHeader("Subject", "target"), []uint32{2})
			assertMailSearch(t, crud, true, searchHeader("Subject", "target"), []uint32{42})
			flagged := imap.NewSearchCriteria()
			flagged.WithFlags = []string{imap.FlaggedFlag}
			assertMailSearch(t, crud, false, flagged, []uint32{1})
			highUID, err := imap.ParseSeqSet("4294967294:*")
			if err != nil {
				t.Fatal(err)
			}
			highUIDCriteria := imap.NewSearchCriteria()
			highUIDCriteria.Uid = highUID
			assertMailSearch(t, crud, true, highUIDCriteria, []uint32{43})
		})
	}
}

func newMailSearchTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	statementbuilder.InitialiseStatementBuilder("sqlite3")
	db := sqlx.MustOpen("sqlite3", ":memory:")
	t.Cleanup(func() { db.Close() })
	db.MustExec(`CREATE TABLE mail (
		id INTEGER PRIMARY KEY, reference_id BLOB NOT NULL,
        mail_box_id INTEGER NOT NULL,
        uid INTEGER NOT NULL,
        subject TEXT, from_address TEXT, to_address TEXT, cc_address TEXT, bcc_address TEXT,
        sender_address TEXT, reply_to_address TEXT, message_id TEXT, body TEXT,
        internal_date TIMESTAMP, sent_date TIMESTAMP, size INTEGER,
        seen BOOLEAN, recent BOOLEAN, deleted BOOLEAN, flags TEXT
    )`)
	return db
}

func assertMailSearch(t *testing.T, crud *DbResource, uid bool, criteria *imap.SearchCriteria, want []uint32) {
	t.Helper()
	tx, err := crud.Connection().Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	selections, err := crud.SearchMailBoxCandidates(7, criteria, tx)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]uint32, 0, len(selections))
	for _, selection := range selections {
		if uid {
			got = append(got, selection.UID)
		} else {
			got = append(got, selection.SequenceNumber)
		}
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("SearchMailBox(uid=%v) = %v, want %v", uid, got, want)
	}
}

func mailSearchReferenceID(index int) []byte {
	referenceID := uuid.NewMD5(uuid.Nil, []byte(fmt.Sprintf("mail-%d", index)))
	return referenceID[:]
}

func searchHeader(name, value string) *imap.SearchCriteria {
	criteria := imap.NewSearchCriteria()
	criteria.Header.Add(name, value)
	return criteria
}

func mailSearchTestFlags(index int) string {
	flags := ""
	if index == 1 {
		flags = imap.SeenFlag
	}
	if index == 3 {
		flags = imap.DeletedFlag
	}
	if index == 4 {
		flags = "project"
	}
	if index == 12 {
		flags = imap.RecentFlag
	}
	return flags
}

func mailSearchTestFlagsForSeen(seen bool) string {
	if seen {
		return imap.SeenFlag
	}
	return ""
}
