package server

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	imapserver "github.com/artpar/go-imap/server"
)

func TestMailServiceRuntimeStatusLifecycle(t *testing.T) {
	status := newMailServiceRuntimeStatus("runtime-a")
	before := status.snapshot()
	if before.RuntimeID != "runtime-a" || before.IMAP.State != "disabled" || before.SMTP.State != "disabled" {
		t.Fatalf("initial: %#v", before)
	}
	observation := mailProtocolStatus{State: "running", ConfiguredEnabled: true, Enabled: true, CertificateLoaded: true,
		ListenInterface: "127.0.0.1:0", BoundAddress: "127.0.0.1:1234", AuthenticationMechanisms: []string{"PLAIN"}}
	status.setIMAP(observation)
	observation.AuthenticationMechanisms[0] = "changed"
	copy := status.snapshot()
	if copy.IMAP.AuthenticationMechanisms[0] != "PLAIN" {
		t.Fatal("observation aliases snapshot")
	}
	copy.IMAP.AuthenticationMechanisms[0] = "changed-again"
	if status.snapshot().IMAP.AuthenticationMechanisms[0] != "PLAIN" {
		t.Fatal("snapshot aliases runtime")
	}
	status.imapState("failed", "listen_failed")
	failed := status.snapshot()
	if failed.IMAP.State != "failed" || failed.IMAP.BoundAddress != "" || !failed.IMAP.CertificateLoaded || failed.IMAP.ErrorMessage == "" {
		t.Fatalf("failed: %#v", failed)
	}
	status.setIMAP(mailProtocolStatus{State: "running"})
	status.imapStopped()
	if got := status.snapshot().IMAP; got.State != "failed" || got.ErrorCode != "serve_failed" {
		t.Fatalf("unexpected stop: %#v", got)
	}
	status.imapState("stopping", "")
	status.imapState("stopped", "")
	if got := status.snapshot().IMAP; got.State != "failed" || got.ErrorCode != "serve_failed" {
		t.Fatalf("quiesce lost failure: %#v", got)
	}
	status.setIMAP(mailProtocolStatus{State: "running"})
	status.imapState("stopping", "")
	status.imapStopped()
	if got := status.snapshot().IMAP; got.State != "stopped" || got.ErrorCode != "" {
		t.Fatalf("intentional stop: %#v", got)
	}
	if fresh := newMailServiceRuntimeStatus("runtime-b").snapshot(); fresh.IMAP.ErrorCode != "" || fresh.IMAP.State != "disabled" {
		t.Fatalf("fresh runtime: %#v", fresh)
	}
}

func TestMailServiceRuntimeStatusConcurrentReads(t *testing.T) {
	status := newMailServiceRuntimeStatus("race-test")
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		for i := 0; i < 100; i++ {
			status.setIMAP(mailProtocolStatus{State: "running", AuthenticationMechanisms: []string{"PLAIN"}})
			status.imapState("stopping", "")
			status.imapStopped()
		}
	}()
	go func() {
		defer workers.Done()
		for i := 0; i < 100; i++ {
			got := status.snapshot()
			if len(got.IMAP.AuthenticationMechanisms) > 0 {
				got.IMAP.AuthenticationMechanisms[0] = "copy"
			}
		}
	}()
	workers.Wait()
}

func TestMailServiceStatusQuiesce(t *testing.T) {
	status := newMailServiceRuntimeStatus("quiesce-test")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	imap := imapserver.New(nil)
	// Serve registers its listener before sending the greeting. Reading a
	// greeting establishes that Close below owns the real serving listener.
	done := make(chan struct{})
	go func() { defer close(done); _ = imap.Serve(listener); status.imapStopped() }()
	connection, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = connection.SetDeadline(time.Now().Add(time.Second))
	greeting := make([]byte, 512)
	if _, err := connection.Read(greeting); err != nil {
		t.Fatal(err)
	}
	connection.Close()
	status.setIMAP(mailProtocolStatus{State: "running", BoundAddress: listener.Addr().String()})
	runtime := &Runtime{imapServer: imap, mailStatus: status}
	if err := runtime.Quiesce(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("IMAP did not stop")
	}
	if got := status.snapshot().IMAP; got.State != "stopped" || got.BoundAddress != "" || got.ErrorCode != "" {
		t.Fatalf("quiesced: %#v", got)
	}
	if err := runtime.Quiesce(context.Background()); err != nil {
		t.Fatal(err)
	}
}
