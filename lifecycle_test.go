package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReadinessGateLifecycle(t *testing.T) {
	applicationCalls := 0
	checkErr := error(nil)
	gate := newReadinessGate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		applicationCalls++
		w.WriteHeader(http.StatusNoContent)
	}), func(context.Context) error { return checkErr })

	assertStatus := func(path string, expected int) {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		gate.ServeHTTP(response, request)
		if response.Code != expected {
			t.Fatalf("%s returned %d, expected %d", path, response.Code, expected)
		}
	}

	assertStatus("/ping", http.StatusOK)
	assertStatus("/ready", http.StatusServiceUnavailable)
	assertStatus("/api/world", http.StatusServiceUnavailable)

	gate.SetReady(true)
	assertStatus("/ready", http.StatusOK)
	assertStatus("/api/world", http.StatusNoContent)
	if applicationCalls != 1 {
		t.Fatalf("application handler called %d times, expected once", applicationCalls)
	}

	checkErr = errors.New("database unavailable")
	assertStatus("/ready", http.StatusServiceUnavailable)
	gate.SetReady(false)
	assertStatus("/ping", http.StatusOK)
	assertStatus("/api/world", http.StatusServiceUnavailable)
}

func TestHTTPShutdownCompletesAcceptedRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	gate := newReadinessGate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("complete"))
	}), nil)
	gate.SetReady(true)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: gate}
	go func() { _ = server.Serve(listener) }()

	responseDone := make(chan error, 1)
	go func() {
		response, requestErr := http.Get("http://" + listener.Addr().String() + "/slow")
		if requestErr != nil {
			responseDone <- requestErr
			return
		}
		defer response.Body.Close()
		body, requestErr := io.ReadAll(response.Body)
		if requestErr == nil && string(body) != "complete" {
			requestErr = errors.New("unexpected response body")
		}
		responseDone <- requestErr
	}()
	<-started

	gate.SetReady(false)
	shutdownDone := make(chan error, 1)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() { shutdownDone <- server.Shutdown(shutdownCtx) }()

	select {
	case shutdownErr := <-shutdownDone:
		t.Fatalf("shutdown returned before the accepted request completed: %v", shutdownErr)
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	if requestErr := <-responseDone; requestErr != nil {
		t.Fatalf("accepted request failed during shutdown: %v", requestErr)
	}
	if shutdownErr := <-shutdownDone; shutdownErr != nil {
		t.Fatalf("shutdown failed: %v", shutdownErr)
	}
}
