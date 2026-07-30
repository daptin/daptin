package actions

import (
	"context"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/buraksezer/olric"
	olricConfig "github.com/buraksezer/olric/config"
	"github.com/daptin/daptin/server/resource"
)

var otpSecurityTestMu sync.Mutex

func startOTPTestOlric(t *testing.T) (*olric.EmbeddedClient, olric.DMap) {
	t.Helper()
	// OlricCache is a package-global framework cache, so tests using it cannot run in parallel.
	otpSecurityTestMu.Lock()
	t.Cleanup(otpSecurityTestMu.Unlock)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate Olric test port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	errCh := make(chan error, 1)
	cfg := olricConfig.New("local")
	cfg.BindAddr = "127.0.0.1"
	cfg.BindPort = port
	cfg.MemberlistConfig.BindAddr = "127.0.0.1"
	cfg.MemberlistConfig.BindPort = 0
	cfg.MemberlistConfig.Name = net.JoinHostPort(cfg.BindAddr, strconv.Itoa(port))
	cfg.LogOutput = nil
	emb, err := olric.New(cfg)
	if err != nil {
		t.Fatalf("create Olric: %v", err)
	}
	go func() { errCh <- emb.Start() }()
	client := emb.NewEmbeddedClient()
	var dm olric.DMap
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		dm, err = client.NewDMap("otp-security-test")
		if err == nil {
			break
		}
		select {
		case startErr := <-errCh:
			t.Fatalf("start Olric: %v", startErr)
		case <-time.After(25 * time.Millisecond):
		}
	}
	if dm == nil || err != nil {
		t.Fatalf("create OTP DMap after starting Olric: %v", err)
	}
	oldCache := resource.OlricCache
	resource.OlricCache = dm
	t.Cleanup(func() {
		resource.OlricCache = oldCache
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = emb.Shutdown(ctx)
		select {
		case <-errCh:
		case <-time.After(5 * time.Second):
		}
	})
	return client, dm
}

func TestOTPAttemptLimitIsSharedByAccount(t *testing.T) {
	_, _ = startOTPTestOlric(t)
	for attempt := int64(1); attempt <= otpAccountAttemptMax; attempt++ {
		if err := consumeOTPAttempt(42, "192.0.2.10"); err != nil {
			t.Fatalf("attempt %d unexpectedly rejected: %v", attempt, err)
		}
	}
	if err := consumeOTPAttempt(42, "198.51.100.20"); err != errOTPTooManyAttempts {
		t.Fatalf("distributed-source attempt should hit account limit, got %v", err)
	}
}

func TestOTPAttemptLimitIsAtomicUnderConcurrency(t *testing.T) {
	_, _ = startOTPTestOlric(t)
	const attempts = 20
	results := make(chan error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(source int) {
			defer wg.Done()
			results <- consumeOTPAttempt(84, "192.0.2."+strconv.Itoa(source+1))
		}(i)
	}
	wg.Wait()
	close(results)
	allowed := 0
	for err := range results {
		if err == nil {
			allowed++
			continue
		}
		if err != errOTPTooManyAttempts {
			t.Fatalf("unexpected concurrent attempt error: %v", err)
		}
	}
	if allowed != int(otpAccountAttemptMax) {
		t.Fatalf("expected exactly %d admitted attempts, got %d", otpAccountAttemptMax, allowed)
	}
}

func TestOTPProtectionFailsClosedWithoutOlric(t *testing.T) {
	oldCache := resource.OlricCache
	resource.OlricCache = nil
	t.Cleanup(func() { resource.OlricCache = oldCache })
	if err := consumeOTPAttempt(42, "192.0.2.10"); err != errOTPProtectionUnavailable {
		t.Fatalf("expected fail-closed error, got %v", err)
	}
}

func TestOTPCodeCanOnlyBeConsumedOnce(t *testing.T) {
	_, _ = startOTPTestOlric(t)
	now := time.Now().UTC()
	if err := consumeOTPCode(42, now); err != nil {
		t.Fatalf("first consume failed: %v", err)
	}
	if err := consumeOTPCode(42, now); err != errOTPAlreadyUsed {
		t.Fatalf("expected replay rejection, got %v", err)
	}
}
