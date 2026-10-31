package actions

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestOutboxSMTPCancellationClosesStalledConnection(t *testing.T) {
	for _, stage := range []string{"greeting", "ehlo", "data-ack"} {
		t.Run(stage, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			stalled, peerClosed, serverDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
			go func() {
				defer close(serverDone)
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				reader := bufio.NewReader(conn)
				waitForClose := func() {
					close(stalled)
					_, err := io.Copy(io.Discard, reader)
					if err == nil {
						close(peerClosed)
					}
				}
				if stage == "greeting" {
					waitForClose()
					return
				}
				_, _ = fmt.Fprint(conn, "220 localhost sink\r\n")
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					command := strings.ToUpper(strings.TrimSpace(line))
					switch {
					case strings.HasPrefix(command, "EHLO"), strings.HasPrefix(command, "HELO"):
						if stage == "ehlo" {
							waitForClose()
							return
						}
						_, _ = fmt.Fprint(conn, "250 localhost\r\n")
					case strings.HasPrefix(command, "MAIL"), strings.HasPrefix(command, "RCPT"):
						_, _ = fmt.Fprint(conn, "250 ok\r\n")
					case command == "DATA":
						_, _ = fmt.Fprint(conn, "354 continue\r\n")
						for {
							line, err = reader.ReadString('\n')
							if err != nil {
								return
							}
							if line == ".\r\n" {
								break
							}
						}
						waitForClose()
						return
					default:
						return
					}
				}
			}()
			// Greeting exercises deadline expiry; later stages exercise explicit cancel.
			var ctx context.Context
			var cancel context.CancelFunc
			if stage == "greeting" {
				ctx, cancel = context.WithTimeout(context.Background(), time.Second)
			} else {
				ctx, cancel = context.WithCancel(context.Background())
			}
			defer cancel()
			result := make(chan error, 1)
			go func() {
				result <- sendOutboxSMTPData(ctx, "127.0.0.1", "sender.example.test", "owner@example.test", []string{"guest@localhost"}, []byte("Subject: local\r\n\r\nbody\r\n"), listener.Addr().(*net.TCPAddr).Port)
			}()
			select {
			case <-stalled:
			case <-time.After(3 * time.Second):
				t.Fatal("SMTP did not reach stalled stage")
			}
			if stage != "greeting" {
				cancel()
			}
			select {
			case err := <-result:
				if err == nil {
					t.Fatal("cancelled SMTP unexpectedly succeeded")
				}
				var uncertain *outboxUncertainError
				if errors.As(err, &uncertain) != (stage == "data-ack") {
					t.Fatalf("wrong DATA uncertainty at %s: %v", stage, err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("cancelled SMTP sender kept waiting")
			}
			select {
			case <-peerClosed:
			case <-time.After(time.Second):
				t.Fatal("SMTP peer did not observe socket close")
			}
			select {
			case <-serverDone:
			case <-time.After(time.Second):
				t.Fatal("loopback sink did not terminate")
			}
		})
	}
}
