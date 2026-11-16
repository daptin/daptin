package main

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The mail protocol scenarios reuse this database across Daptin restarts.
// When enabled, each scenario creates its own PostgreSQL cluster and socket;
// there is no configurable DSN that could point a write at an installed server.
func mailProtocolDatabase(t *testing.T) transportE2EDaptinOptions {
	t.Helper()
	bin := os.Getenv("DAPTIN_E2E_MAIL_POSTGRES_BIN")
	if bin == "" {
		return transportE2EDaptinOptions{connectionString: filepath.Join(t.TempDir(), "mail.db")}
	}
	if !filepath.IsAbs(bin) {
		t.Fatal("DAPTIN_E2E_MAIL_POSTGRES_BIN must be an absolute path")
	}
	// A short path keeps the PostgreSQL socket below the Unix socket limit,
	// including on macOS where t.TempDir() can have a long prefix.
	socket, err := os.MkdirTemp("/tmp", "daptin-mail-pg-")
	if err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(t.TempDir(), "pgdata")
	run := func(name string, args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
		defer cancel()
		return exec.CommandContext(ctx, filepath.Join(bin, name), args...).CombinedOutput()
	}
	started := false
	t.Cleanup(func() {
		if started {
			if out, err := run("pg_ctl", "-D", data, "-w", "-t", "15", "-m", "fast", "stop"); err != nil {
				t.Errorf("stop disposable PostgreSQL: %v\n%s", err, out)
			}
		}
		if err := os.RemoveAll(socket); err != nil {
			t.Errorf("remove private PostgreSQL socket directory: %v", err)
		}
	})
	const username = "mail_fixture"
	if out, err := run("initdb", "-D", data, "--username="+username, "--auth-local=trust", "--auth-host=reject", "--encoding=UTF8", "--no-locale"); err != nil {
		t.Fatalf("initialize disposable PostgreSQL: %v\n%s", err, out)
	}
	log := filepath.Join(data, "server.log")
	// pg_ctl receives only this fixed option string and our short /tmp path.
	// PostgreSQL has no TCP listener, even while the Daptin listener restarts.
	started = true
	if out, err := run("pg_ctl", "-D", data, "-w", "-t", "30", "-l", log, "-o", "-h '' -p 5432 -k "+socket+" -c unix_socket_permissions=0700", "start"); err != nil {
		detail, _ := os.ReadFile(log)
		t.Fatalf("start disposable PostgreSQL: %v\n%s\n%s", err, out, detail)
	}
	version, err := run("postgres", "--version")
	if err != nil {
		t.Fatalf("identify disposable PostgreSQL executable: %v\n%s", err, version)
	}
	t.Logf("%s on a private Unix socket", strings.TrimSpace(string(version)))
	params := url.Values{"host": {socket}, "port": {"5432"}, "user": {username}, "sslmode": {"disable"}}
	return transportE2EDaptinOptions{databaseType: "postgres", connectionString: "postgres:///postgres?" + params.Encode()}
}
