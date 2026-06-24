package actions

import (
	"strings"
	"testing"

	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestMailSendRequiresMailServerHostnameOrBackendDefault(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	configStore, err := resource.NewConfigStore(db)
	if err != nil {
		t.Fatalf("create config store: %v", err)
	}
	transaction, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	defer transaction.Rollback()

	performer := &mailSendActionPerformer{
		cruds: map[string]*resource.DbResource{
			"mail": {ConfigStore: configStore},
		},
	}
	_, _, errs := performer.DoAction(actionresponse.Outcome{}, map[string]interface{}{
		"from":    "login@example.com",
		"to":      "user@example.net",
		"subject": "subject",
		"body":    "body",
	}, transaction)
	if len(errs) != 1 {
		t.Fatalf("expected one error, got %#v", errs)
	}
	if !strings.Contains(errs[0].Error(), "mail_server_hostname") ||
		!strings.Contains(errs[0].Error(), "mail.default_server_hostname") {
		t.Fatalf("unexpected error: %v", errs[0])
	}
}
