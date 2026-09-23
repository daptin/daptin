package main

import (
	"bufio"
	"context"
	"crypto/tls"
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

func TestHTTPConnectionDeadlines(t *testing.T) {
	const headerTimeout = 250 * time.Millisecond
	const idleTimeout = 300 * time.Millisecond
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = io.Copy(io.Discard, r.Body)
		}
		_, _ = w.Write([]byte("pong"))
	})

	for _, secure := range []bool{false, true} {
		name := "HTTP"
		if secure {
			name = "HTTPS"
		}
		t.Run(name, func(t *testing.T) {
			testServer := httptest.NewUnstartedServer(handler)
			testServer.Config = newHTTPServer("", handler, nil, headerTimeout, idleTimeout)
			if secure {
				testServer.StartTLS()
			} else {
				testServer.Start()
			}
			defer testServer.Close()

			address := testServer.Listener.Addr().String()
			stalled, err := net.Dial("tcp", address)
			if err != nil {
				t.Fatal(err)
			}
			defer stalled.Close()
			if !secure {
				if _, err := io.WriteString(stalled, "GET /ping HTTP/1.1\r\nHost: example.test\r\nX-Slow: incomplete"); err != nil {
					t.Fatal(err)
				}
			}
			if err := stalled.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, err := io.Copy(io.Discard, stalled); err != nil {
				t.Fatalf("stalled connection did not close: %v", err)
			}

			response, err := testServer.Client().Get(testServer.URL + "/ping")
			if err != nil {
				t.Fatalf("/ping after stalled connection: %v", err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("/ping returned %d", response.StatusCode)
			}
			var keepAlive net.Conn
			if secure {
				keepAlive, err = tls.Dial("tcp", address, &tls.Config{InsecureSkipVerify: true}) // test server certificate
			} else {
				keepAlive, err = net.Dial("tcp", address)
			}
			if err != nil {
				t.Fatal(err)
			}
			defer keepAlive.Close()
			if _, err := io.WriteString(keepAlive, "GET /ping HTTP/1.1\r\nHost: example.test\r\n\r\n"); err != nil {
				t.Fatal(err)
			}
			reader := bufio.NewReader(keepAlive)
			keepAliveResponse, err := http.ReadResponse(reader, nil)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = io.Copy(io.Discard, keepAliveResponse.Body)
			keepAliveResponse.Body.Close()
			if err := keepAlive.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, err := reader.ReadByte(); !errors.Is(err, io.EOF) {
				t.Fatalf("idle connection remained open: %v", err)
			}

			// The header deadline must not become a deadline for request bodies.
			requestBody, bodyWriter := io.Pipe()
			request, err := http.NewRequest(http.MethodPost, testServer.URL+"/upload", requestBody)
			if err != nil {
				t.Fatal(err)
			}
			request.ContentLength = 1
			responseDone := make(chan error, 1)
			go func() {
				response, requestErr := testServer.Client().Do(request)
				if requestErr == nil {
					response.Body.Close()
					if response.StatusCode != http.StatusOK {
						requestErr = errors.New("slow body request was rejected")
					}
				}
				responseDone <- requestErr
			}()
			time.Sleep(headerTimeout + 50*time.Millisecond)
			if _, err := io.WriteString(bodyWriter, "x"); err != nil {
				t.Fatal(err)
			}
			if err := bodyWriter.Close(); err != nil {
				t.Fatal(err)
			}
			if err := <-responseDone; err != nil {
				t.Fatalf("slow body request: %v", err)
			}
		})
	}
}

func TestHTTPTimeoutConfigurationRequiresPositiveIntegerSeconds(t *testing.T) {
	for _, value := range []string{"0", "-1", "1.5", "5s", "9223372036854775807"} {
		if _, err := parseHTTPTimeoutSeconds(value); err == nil {
			t.Errorf("accepted invalid timeout %q", value)
		}
	}
	for _, value := range []string{"1", "5", "30"} {
		if duration, err := parseHTTPTimeoutSeconds(value); err != nil || duration <= 0 {
			t.Errorf("rejected timeout %q: %v", value, err)
		}
	}
}
