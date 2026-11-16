package subsite

import (
	"errors"
	"testing"

	"github.com/daptin/daptin/server/dbresourceinterface"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type siteResourceFixture struct {
	dbresourceinterface.DbResourceInterface
	rows  []map[string]interface{}
	err   error
	grant permission.PermissionInstance
	tx    *sqlx.Tx
	calls int
}

func (f *siteResourceFixture) GetAllObjects(table string, tx *sqlx.Tx) ([]map[string]interface{}, error) {
	if table != "site" || tx != f.tx {
		panic("wrong resource or transaction")
	}
	f.calls++
	return f.rows, f.err
}
func (f *siteResourceFixture) GetObjectPermissionByReferenceId(table string, ref daptinid.DaptinReferenceId, tx *sqlx.Tx) permission.PermissionInstance {
	if table != "site" || tx != f.tx || ref != daptinid.InterfaceToDIR(f.rows[0]["reference_id"]) {
		panic("wrong permission context")
	}
	return f.grant
}
func TestSiteLoaderUsesResourceReferencesAndRejectsInvalidPolicy(t *testing.T) {
	ref := daptinid.DaptinReferenceId(uuid.New())
	store := daptinid.DaptinReferenceId(uuid.New())
	owner := daptinid.DaptinReferenceId(uuid.New())
	for _, enabled := range []interface{}{true, int64(1), "1", "true"} {
		f := &siteResourceFixture{tx: &sqlx.Tx{}, grant: permission.PermissionInstance{UserId: owner}, rows: []map[string]interface{}{{
			"id": int64(9), "name": "customer", "hostname": "site.test", "reference_id": ref, "cloud_store_id": store, "user_account_id": owner,
			"enable": enabled, "ftp_enabled": enabled, "headers": `{"X-Content-Type-Options":"nosniff"}`,
		}}}
		sites, err := GetAllSites(f, f.tx)
		if err != nil {
			t.Fatal(err)
		}
		if len(sites) != 1 || !sites[0].Enable || !sites[0].FtpEnabled || sites[0].CloudStoreId != store || sites[0].UserId != owner || sites[0].Permission.UserId != owner || sites[0].Headers["X-Content-Type-Options"] != "nosniff" || f.calls != 1 {
			t.Fatalf("invalid snapshot: %#v", sites)
		}
		f.rows[0]["headers"] = `{"Content-Length":"0"}`
		if _, err := GetAllSites(f, f.tx); err == nil {
			t.Fatal("invalid stored policy silently loaded")
		}
		f.err = errors.New("resource unavailable")
		if _, err := GetAllSites(f, f.tx); !errors.Is(err, f.err) {
			t.Fatalf("resource failure lost: %v", err)
		}
	}
}
