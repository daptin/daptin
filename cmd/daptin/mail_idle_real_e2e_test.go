package main

import (
	"bufio"
	"crypto/tls"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func openMailIdleFixture(t *testing.T, address string, tlsConfig *tls.Config) (net.Conn, *bufio.Reader) {
	t.Helper()
	plain, err := net.DialTimeout("tcp", address, 4*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = plain.Close() })
	_ = plain.SetDeadline(time.Now().Add(8 * time.Second))
	r := bufio.NewReader(plain)
	line := func() string {
		value, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	write := func(conn net.Conn, value string) {
		if _, err := io.WriteString(conn, value); err != nil {
			t.Fatal(err)
		}
	}
	if greeting := line(); !strings.HasPrefix(greeting, "* OK ") {
		t.Fatalf("IDLE fixture greeting: %q", greeting)
	}
	write(plain, "t STARTTLS\r\n")
	if response := line(); !strings.HasPrefix(response, "t OK ") {
		t.Fatalf("IDLE fixture STARTTLS: %q", response)
	}
	secure := tls.Client(plain, tlsConfig)
	if err := secure.Handshake(); err != nil {
		t.Fatal(err)
	}
	r = bufio.NewReader(secure)
	write(secure, "a LOGIN native-alice@native.test fixture-password\r\n")
	if response := line(); !strings.HasPrefix(response, "a OK ") {
		t.Fatalf("IDLE fixture login status: %q", response)
	}
	write(secure, "s EXAMINE Native/2026\r\n")
	for {
		if response := line(); strings.HasPrefix(response, "s ") {
			if !strings.HasPrefix(response, "s OK [READ-ONLY] ") {
				t.Fatalf("IDLE fixture EXAMINE: %q", response)
			}
			break
		}
	}
	write(secure, "i IDLE\r\n")
	if response := line(); !strings.HasPrefix(response, "+ ") {
		t.Fatalf("IDLE fixture continuation: %q", response)
	}
	return secure, r
}
