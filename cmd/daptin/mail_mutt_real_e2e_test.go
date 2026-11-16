package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// An optional independent desktop client against the native Mail E2E fixture.
// Expect waits for curses to paint each screen before supplying the next key.
func exerciseMailWithMutt(t *testing.T, executable, address, hostname, certificate string) {
	t.Helper()
	if runtime.GOOS != "darwin" || !filepath.IsAbs(executable) {
		t.Fatal("DAPTIN_E2E_MUTT_BINARY requires an absolute path to Mutt on macOS")
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil || host != "127.0.0.1" || hostname != "localhost" {
		t.Fatal("Mutt test requires a disposable loopback IMAP listener")
	}
	dir := t.TempDir()
	cert, config := filepath.Join(dir, "fixture.pem"), filepath.Join(dir, "muttrc")
	if err := os.WriteFile(cert, []byte(certificate), 0600); err != nil {
		t.Fatal(err)
	}
	settings := fmt.Sprintf(`set certificate_file=%q
unset ssl_usesystemcerts
set ssl_force_tls ssl_starttls ssl_verify_host ssl_verify_dates
set imap_user="native-alice@native.test" imap_pass="fixture-password"
set imap_authenticators="login"
set imap_passive=no
set mailcap_path=/dev/null
set folder=%q tmpdir=%q
set move=no quit=yes
set charset=utf-8
`, cert, dir, dir)
	if err := os.WriteFile(config, []byte(settings), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/usr/bin/expect", "-f", "-", executable,
		config, "imap://"+net.JoinHostPort(hostname, port)+"/Draft")
	cmd.Stdin = strings.NewReader(`
set timeout 7
set stty_init "rows 28 columns 100"
spawn -noecho [lindex $argv 0] -n -F [lindex $argv 1] -f [lindex $argv 2]
expect {
  -exact "Browser round trip" {}
  -exact "SSL Certificate check" {exit 10}
  timeout {exit 11}
  eof {exit 12}
}
send -- "\r"
expect {
  -exact "こんにちは" {}
  timeout {exit 13}
  eof {exit 14}
}
send -- "qF\$q"
expect {
  eof {}
  timeout {exit 15}
}
exit [lindex [wait] 3]
`)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "TERM=xterm", "LC_ALL=en_US.UTF-8")
	cmd.WaitDelay = 2 * time.Second
	output, err := cmd.CombinedOutput()
	log := strings.ReplaceAll(string(output), "fixture-password", "[redacted]")
	if err != nil || !strings.Contains(log, "Browser round trip") || !strings.Contains(log, "こんにちは") || !strings.Contains(log, "connection using TLS") {
		t.Fatalf("Mutt did not render the expected draft (exit: %v, deadline: %v): %q", err, ctx.Err(), log)
	}
	if strings.Contains(log, "SSL Certificate check") || strings.Contains(log, "Certificate host check failed") {
		t.Fatal("Mutt displayed a trust exception prompt")
	}
	t.Log("Mutt rendered the browser draft and its Japanese MIME text over fixture-only STARTTLS")
}
