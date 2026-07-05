package resource

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/table_info"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestMailColumnBytesAcceptsDBBackedStringAfterCloudStoreConfiguration(t *testing.T) {
	message := []byte("Subject: migrated\r\n\r\nbody")
	encoded := base64.StdEncoding.EncodeToString(message)
	mailResource := &DbResource{
		tableInfo: cloudStoreMailTableInfo("mail"),
	}
	root := &DbResource{
		Cruds: map[string]*DbResource{"mail": mailResource},
	}

	got, err := root.MailColumnBytes("mail", "mail", encoded)
	if err != nil {
		t.Fatalf("MailColumnBytes returned error: %v", err)
	}
	if string(got) != string(message) {
		t.Fatalf("MailColumnBytes = %q, want %q", string(got), string(message))
	}
}

func TestMailColumnBytesCloudStoreRequiresHydratedContents(t *testing.T) {
	message := []byte("Subject: cloud\r\n\r\nbody")
	encoded := base64.StdEncoding.EncodeToString(message)
	mailResource := &DbResource{
		tableInfo: cloudStoreMailTableInfo("outbox"),
	}
	root := &DbResource{
		Cruds: map[string]*DbResource{"outbox": mailResource},
	}

	storedMetadata := []map[string]interface{}{{
		"name": "queued-message.eml",
		"path": "mail-storage/mail-messages/queued-message.eml",
		"type": mailMessageFileType,
	}}
	if _, err := root.MailColumnBytes("outbox", "mail", storedMetadata); err == nil {
		t.Fatalf("expected stored cloud-store metadata without contents to fail")
	}

	hydratedMetadata := []map[string]interface{}{{
		"name":     "queued-message.eml",
		"path":     "mail-storage/mail-messages/queued-message.eml",
		"type":     mailMessageFileType,
		"contents": encoded,
	}}
	got, err := root.MailColumnBytes("outbox", "mail", hydratedMetadata)
	if err != nil {
		t.Fatalf("MailColumnBytes returned error for hydrated cloud-store mail: %v", err)
	}
	if string(got) != string(message) {
		t.Fatalf("MailColumnBytes = %q, want %q", string(got), string(message))
	}
}

func TestResultToArrayCloudStoreMailColumnAcceptsDBBackedString(t *testing.T) {
	message := []byte("Subject: migrated\r\n\r\nbody")
	encoded := base64.StdEncoding.EncodeToString(message)
	dbResource := &DbResource{
		tableInfo: cloudStoreMailTableInfo("mail"),
	}

	rows, includes, err := dbResource.ResultToArrayOfMapWithTransaction(
		[]map[string]interface{}{{
			"__type":  "mail",
			"mail":    encoded,
			"mail_id": "migrated-message",
		}},
		map[string]api2go.ColumnInfo{
			"mail": cloudStoreMailColumn(),
		},
		map[string]bool{"mail": true},
		nil,
	)
	if err != nil {
		t.Fatalf("ResultToArrayOfMapWithTransaction returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	files, ok := rows[0]["mail"].([]map[string]interface{})
	if !ok || len(files) != 1 {
		t.Fatalf("expected mail file list, got %#v", rows[0]["mail"])
	}
	if files[0]["contents"] != encoded {
		t.Fatalf("expected inline contents to be preserved, got %#v", files[0])
	}
	if files[0]["name"] != "migrated-message.eml" {
		t.Fatalf("expected stable file name, got %#v", files[0]["name"])
	}
	if len(includes) != 1 || len(includes[0]) != 1 {
		t.Fatalf("expected one local include, got %#v", includes)
	}
	if includes[0][0]["__type"] != "gzip" {
		t.Fatalf("expected include type gzip, got %#v", includes[0][0]["__type"])
	}
}

func TestAppendSentMailForSenderCreatesSentMailboxAndMailRow(t *testing.T) {
	env := newSentMailTestEnv(t, false)
	tx := env.db.MustBegin()

	_, err := env.root.AppendSentMailForSender("sender@example.test", sentMailTestMessage(), tx)
	if err != nil {
		t.Fatalf("AppendSentMailForSender returned error: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var sentCount int
	if err := env.db.QueryRowx(`select count(*) from mail_box where name = 'Sent' and mail_account_id = 1`).Scan(&sentCount); err != nil {
		t.Fatalf("count sent boxes: %v", err)
	}
	if sentCount != 1 {
		t.Fatalf("sent mailbox count = %d, want 1", sentCount)
	}

	var uid int64
	var seen, recent bool
	var flags, subject string
	if err := env.db.QueryRowx(`select uid, seen, recent, flags, subject from mail`).Scan(&uid, &seen, &recent, &flags, &subject); err != nil {
		t.Fatalf("select sent mail: %v", err)
	}
	if uid != 1 {
		t.Fatalf("uid = %d, want 1", uid)
	}
	if !seen || recent {
		t.Fatalf("seen/recent = %v/%v, want true/false", seen, recent)
	}
	if flags != "\\Seen" {
		t.Fatalf("flags = %q, want \\Seen", flags)
	}
	if subject != "Sent copy" {
		t.Fatalf("subject = %q, want Sent copy", subject)
	}
}

func TestAppendSentMailForSenderReusesExistingSentMailbox(t *testing.T) {
	env := newSentMailTestEnv(t, true)
	tx := env.db.MustBegin()

	_, err := env.root.AppendSentMailForSender("Sender <sender@example.test>", sentMailTestMessage(), tx)
	if err != nil {
		t.Fatalf("AppendSentMailForSender returned error: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var sentCount, nextUid, uid int64
	if err := env.db.QueryRowx(`select count(*), max(nextuid) from mail_box where name = 'Sent' and mail_account_id = 1`).Scan(&sentCount, &nextUid); err != nil {
		t.Fatalf("select sent mailbox state: %v", err)
	}
	if sentCount != 1 {
		t.Fatalf("sent mailbox count = %d, want 1", sentCount)
	}
	if nextUid != 6 {
		t.Fatalf("nextuid = %d, want 6", nextUid)
	}
	if err := env.db.QueryRowx(`select uid from mail`).Scan(&uid); err != nil {
		t.Fatalf("select sent uid: %v", err)
	}
	if uid != 5 {
		t.Fatalf("uid = %d, want 5", uid)
	}
}

func TestAppendSentMailForSenderRequiresSenderMailAccount(t *testing.T) {
	env := newSentMailTestEnv(t, false)
	tx := env.db.MustBegin()
	defer tx.Rollback()

	if _, err := env.root.AppendSentMailForSender("missing@example.test", sentMailTestMessage(), tx); err == nil {
		t.Fatalf("expected missing sender account error")
	}

	var mailCount int
	if err := tx.QueryRowx(`select count(*) from mail`).Scan(&mailCount); err != nil {
		t.Fatalf("count mail: %v", err)
	}
	if mailCount != 0 {
		t.Fatalf("mail count = %d, want 0", mailCount)
	}
}

type sentMailTestEnv struct {
	db   *sqlx.DB
	root *DbResource
}

func newSentMailTestEnv(t *testing.T, existingSent bool) sentMailTestEnv {
	t.Helper()

	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	for _, statement := range []string{
		`create table usergroup (id integer primary key, name text, reference_id blob, permission integer)`,
		`create table user_account (
			id integer primary key,
			reference_id blob,
			user_account_id integer,
			permission integer,
			created_at timestamp,
			updated_at timestamp
		)`,
		`create table user_account_user_account_id_has_usergroup_usergroup_id (
			id integer primary key,
			user_account_id integer,
			usergroup_id integer,
			reference_id blob,
			permission integer,
			created_at timestamp
		)`,
		`create table mail_account (
			id integer primary key,
			username text,
			password text,
			password_md5 text,
			user_account_id integer,
			reference_id blob,
			permission integer,
			created_at timestamp,
			updated_at timestamp
		)`,
		`create table mail_account_mail_account_id_has_usergroup_usergroup_id (
			id integer primary key,
			mail_account_id integer,
			usergroup_id integer,
			reference_id blob,
			permission integer,
			created_at timestamp
		)`,
		`create table mail_box (
			id integer primary key,
			name text,
			mail_account_id integer,
			uidvalidity integer,
			nextuid integer,
			subscribed bool,
			attributes text,
			flags text,
			permanent_flags text,
			user_account_id integer,
			reference_id blob,
			permission integer,
			created_at timestamp,
			updated_at timestamp
		)`,
		`create table mail_box_mail_box_id_has_usergroup_usergroup_id (
			id integer primary key,
			mail_box_id integer,
			usergroup_id integer,
			reference_id blob,
			permission integer,
			created_at timestamp
		)`,
		`create table mail (
			id integer primary key,
			message_id text,
			mail_id text,
			from_address text,
			to_address text,
			sender_address text,
			subject text,
			body text,
			mail text,
			spam_score float,
			spam bool,
			hash text,
			internal_date timestamp,
			content_type text,
			reply_to_address text,
			recipient text,
			has_attachment bool,
			ip_addr text,
			return_path text,
			is_tls bool,
			mail_box_id integer,
			user_account_id integer,
			uid integer,
			seen bool,
			recent bool,
			deleted bool,
			size integer,
			flags text,
			reference_id blob,
			permission integer,
			created_at timestamp,
			updated_at timestamp
		)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("setup statement failed: %v", err)
		}
	}

	adminGroupRef := daptinid.DaptinReferenceId(uuid.New())
	userRef := daptinid.DaptinReferenceId(uuid.New())
	mailAccountRef := daptinid.DaptinReferenceId(uuid.New())
	if _, err := db.Exec(`insert into usergroup (id, name, reference_id, permission) values (?, ?, ?, ?)`,
		2, "administrators", adminGroupRef[:], int64(auth.DEFAULT_PERMISSION)); err != nil {
		t.Fatalf("insert admin group: %v", err)
	}
	if _, err := db.Exec(`insert into user_account (id, reference_id, permission, created_at, updated_at) values (?, ?, ?, ?, ?)`,
		1, userRef[:], int64(auth.DEFAULT_PERMISSION), time.Now(), time.Now()); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.Exec(`insert into mail_account (id, username, user_account_id, reference_id, permission, created_at, updated_at) values (?, ?, ?, ?, ?, ?, ?)`,
		1, "sender@example.test", 1, mailAccountRef[:], int64(16256), time.Now(), time.Now()); err != nil {
		t.Fatalf("insert mail account: %v", err)
	}
	if existingSent {
		sentRef := daptinid.DaptinReferenceId(uuid.New())
		if _, err := db.Exec(`insert into mail_box (id, name, mail_account_id, uidvalidity, nextuid, subscribed, attributes, flags, permanent_flags, user_account_id, reference_id, permission, created_at, updated_at) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			7, "Sent", 1, 1, 5, true, "", "\\*", "\\*", 1, sentRef[:], int64(16256), time.Now(), time.Now()); err != nil {
			t.Fatalf("insert sent mailbox: %v", err)
		}
	}

	root := newSentMailTestCrudGraph(t, db, adminGroupRef)
	return sentMailTestEnv{db: db, root: root}
}

func newSentMailTestCrudGraph(t *testing.T, db *sqlx.DB, adminGroupRef daptinid.DaptinReferenceId) *DbResource {
	t.Helper()

	cruds := make(map[string]*DbResource)
	userAccountCrud := sentMailTestCrud(db, "user_account", sentMailUserAccountColumns(), int64(auth.DEFAULT_PERMISSION))
	mailAccountCrud := sentMailTestCrud(db, "mail_account", sentMailAccountColumns(), int64(auth.DEFAULT_PERMISSION))
	mailBoxCrud := sentMailTestCrud(db, "mail_box", sentMailBoxColumns(), 16256)
	mailCrud := sentMailTestCrud(db, "mail", sentMailColumns(), 768)

	cruds[USER_ACCOUNT_TABLE_NAME] = userAccountCrud
	cruds["mail_account"] = mailAccountCrud
	cruds["mail_box"] = mailBoxCrud
	cruds["mail"] = mailCrud
	for _, crud := range cruds {
		crud.Cruds = cruds
		crud.AdministratorGroupId = adminGroupRef
	}
	oldUserAccountCrud := CRUD_MAP[USER_ACCOUNT_TABLE_NAME]
	CRUD_MAP[USER_ACCOUNT_TABLE_NAME] = userAccountCrud
	t.Cleanup(func() {
		if oldUserAccountCrud == nil {
			delete(CRUD_MAP, USER_ACCOUNT_TABLE_NAME)
			return
		}
		CRUD_MAP[USER_ACCOUNT_TABLE_NAME] = oldUserAccountCrud
	})
	return mailCrud
}

func sentMailTestCrud(db *sqlx.DB, tableName string, columns []api2go.ColumnInfo, defaultPermission int64) *DbResource {
	return &DbResource{
		model: api2go.NewApi2GoModel(tableName, columns, defaultPermission, nil),
		tableInfo: &table_info.TableInfo{
			TableName:         tableName,
			Columns:           columns,
			DefaultPermission: auth.AuthPermission(defaultPermission),
		},
		db:         db,
		connection: db,
		ms:         &MiddlewareSet{},
	}
}

func sentMailUserAccountColumns() []api2go.ColumnInfo {
	return []api2go.ColumnInfo{
		{Name: "id", ColumnName: "id", DataType: "INTEGER", ColumnType: "id", IsAutoIncrement: true},
		{Name: "reference_id", ColumnName: "reference_id", DataType: "blob", ColumnType: "alias"},
		{Name: "user_account_id", ColumnName: "user_account_id", DataType: "int(11)", ColumnType: "value", IsNullable: true},
		{Name: "permission", ColumnName: "permission", DataType: "integer"},
		{Name: "created_at", ColumnName: "created_at", DataType: "timestamp", ColumnType: "datetime"},
		{Name: "updated_at", ColumnName: "updated_at", DataType: "timestamp", ColumnType: "datetime", IsNullable: true},
	}
}

func sentMailAccountColumns() []api2go.ColumnInfo {
	return []api2go.ColumnInfo{
		{Name: "id", ColumnName: "id", DataType: "INTEGER", ColumnType: "id", IsAutoIncrement: true},
		{Name: "username", ColumnName: "username", DataType: "varchar(100)", ColumnType: "label"},
		{Name: "password", ColumnName: "password", DataType: "varchar(100)", ColumnType: "password", IsNullable: true},
		{Name: "password_md5", ColumnName: "password_md5", DataType: "varchar(100)", ColumnType: "md5-bcrypt", IsNullable: true},
		{Name: "user_account_id", ColumnName: "user_account_id", DataType: "int(11)", ColumnType: "value"},
		{Name: "reference_id", ColumnName: "reference_id", DataType: "blob", ColumnType: "alias"},
		{Name: "permission", ColumnName: "permission", DataType: "integer"},
		{Name: "created_at", ColumnName: "created_at", DataType: "timestamp", ColumnType: "datetime"},
		{Name: "updated_at", ColumnName: "updated_at", DataType: "timestamp", ColumnType: "datetime", IsNullable: true},
	}
}

func sentMailBoxColumns() []api2go.ColumnInfo {
	return []api2go.ColumnInfo{
		{Name: "id", ColumnName: "id", DataType: "INTEGER", ColumnType: "id", IsAutoIncrement: true},
		{Name: "name", ColumnName: "name", DataType: "varchar(100)", ColumnType: "label"},
		{
			Name:         "mail_account",
			ColumnName:   "mail_account_id",
			DataType:     "int(11)",
			ColumnType:   "alias",
			IsForeignKey: true,
			ForeignKeyData: api2go.ForeignKeyData{
				Namespace:  "mail_account",
				KeyName:    "id",
				DataSource: "self",
			},
		},
		{Name: "uidvalidity", ColumnName: "uidvalidity", DataType: "int(11)", ColumnType: "value"},
		{Name: "nextuid", ColumnName: "nextuid", DataType: "int(11)", ColumnType: "value"},
		{Name: "subscribed", ColumnName: "subscribed", DataType: "bool", ColumnType: "truefalse"},
		{Name: "attributes", ColumnName: "attributes", DataType: "varchar(100)", ColumnType: "label"},
		{Name: "flags", ColumnName: "flags", DataType: "varchar(100)", ColumnType: "label"},
		{Name: "permanent_flags", ColumnName: "permanent_flags", DataType: "varchar(100)", ColumnType: "label"},
		{Name: "user_account_id", ColumnName: "user_account_id", DataType: "int(11)", ColumnType: "value"},
		{Name: "reference_id", ColumnName: "reference_id", DataType: "blob", ColumnType: "alias"},
		{Name: "permission", ColumnName: "permission", DataType: "integer"},
		{Name: "created_at", ColumnName: "created_at", DataType: "timestamp", ColumnType: "datetime"},
		{Name: "updated_at", ColumnName: "updated_at", DataType: "timestamp", ColumnType: "datetime", IsNullable: true},
	}
}

func sentMailColumns() []api2go.ColumnInfo {
	return []api2go.ColumnInfo{
		{Name: "id", ColumnName: "id", DataType: "INTEGER", ColumnType: "id", IsAutoIncrement: true},
		{Name: "message_id", ColumnName: "message_id", DataType: "varchar(200)", ColumnType: "label"},
		{Name: "mail_id", ColumnName: "mail_id", DataType: "varchar(100)", ColumnType: "label"},
		{Name: "from_address", ColumnName: "from_address", DataType: "varchar(200)", ColumnType: "label"},
		{Name: "to_address", ColumnName: "to_address", DataType: "varchar(200)", ColumnType: "label"},
		{Name: "sender_address", ColumnName: "sender_address", DataType: "varchar(200)", ColumnType: "label"},
		{Name: "subject", ColumnName: "subject", DataType: "varchar(500)", ColumnType: "label"},
		{Name: "body", ColumnName: "body", DataType: "text", ColumnType: "label"},
		{Name: "mail", ColumnName: "mail", DataType: "blob", ColumnType: "gzip"},
		{Name: "spam_score", ColumnName: "spam_score", DataType: "float", ColumnType: "measurement"},
		{Name: "spam", ColumnName: "spam", DataType: "bool", ColumnType: "truefalse"},
		{Name: "hash", ColumnName: "hash", DataType: "varchar(100)", ColumnType: "label"},
		{Name: "internal_date", ColumnName: "internal_date", DataType: "timestamp", ColumnType: "datetime"},
		{Name: "content_type", ColumnName: "content_type", DataType: "text", ColumnType: "content"},
		{Name: "reply_to_address", ColumnName: "reply_to_address", DataType: "varchar(200)", ColumnType: "label"},
		{Name: "recipient", ColumnName: "recipient", DataType: "varchar(200)", ColumnType: "label"},
		{Name: "has_attachment", ColumnName: "has_attachment", DataType: "bool", ColumnType: "truefalse"},
		{Name: "ip_addr", ColumnName: "ip_addr", DataType: "varchar(30)", ColumnType: "label"},
		{Name: "return_path", ColumnName: "return_path", DataType: "varchar(255)", ColumnType: "content"},
		{Name: "is_tls", ColumnName: "is_tls", DataType: "bool", ColumnType: "truefalse"},
		{
			Name:         "mail_box",
			ColumnName:   "mail_box_id",
			DataType:     "int(11)",
			ColumnType:   "alias",
			IsForeignKey: true,
			ForeignKeyData: api2go.ForeignKeyData{
				Namespace:  "mail_box",
				KeyName:    "id",
				DataSource: "self",
			},
		},
		{Name: "user_account_id", ColumnName: "user_account_id", DataType: "int(11)", ColumnType: "value"},
		{Name: "uid", ColumnName: "uid", DataType: "int(11)", ColumnType: "value"},
		{Name: "seen", ColumnName: "seen", DataType: "bool", ColumnType: "truefalse"},
		{Name: "recent", ColumnName: "recent", DataType: "bool", ColumnType: "truefalse"},
		{Name: "deleted", ColumnName: "deleted", DataType: "bool", ColumnType: "truefalse"},
		{Name: "size", ColumnName: "size", DataType: "int(11)", ColumnType: "value"},
		{Name: "flags", ColumnName: "flags", DataType: "varchar(500)", ColumnType: "label"},
		{Name: "reference_id", ColumnName: "reference_id", DataType: "blob", ColumnType: "alias"},
		{Name: "permission", ColumnName: "permission", DataType: "integer"},
		{Name: "created_at", ColumnName: "created_at", DataType: "timestamp", ColumnType: "datetime"},
		{Name: "updated_at", ColumnName: "updated_at", DataType: "timestamp", ColumnType: "datetime", IsNullable: true},
	}
}

func sentMailTestMessage() []byte {
	return []byte("From: sender@example.test\r\nTo: recipient@example.test\r\nSubject: Sent copy\r\nDate: Tue, 02 Jan 2024 03:04:05 +0000\r\nMessage-ID: <sent-copy@example.test>\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nhello")
}

func cloudStoreMailTableInfo(tableName string) *table_info.TableInfo {
	return &table_info.TableInfo{
		TableName: tableName,
		Columns: []api2go.ColumnInfo{
			cloudStoreMailColumn(),
		},
	}
}

func cloudStoreMailColumn() api2go.ColumnInfo {
	return api2go.ColumnInfo{
		Name:         "mail",
		ColumnName:   "mail",
		ColumnType:   "gzip",
		DataType:     "blob",
		IsForeignKey: true,
		ForeignKeyData: api2go.ForeignKeyData{
			DataSource: "cloud_store",
			Namespace:  "mail-storage",
			KeyName:    "mail-messages",
		},
	}
}
