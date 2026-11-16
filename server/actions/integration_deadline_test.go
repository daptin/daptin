package actions

import (
	"context"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/reflection/grpc_testing"
)

func TestIntegrationRejectsTimeoutDurationOverflow(t *testing.T) {
	op := &openapi3.Operation{}
	op.Extensions = map[string]interface{}{"x-daptin-timeout-ms": int64(math.MaxInt64)}
	if _, err := integrationTransportConfigFromOperation(op, "overflow"); err == nil {
		t.Fatal("overflowing provider timeout accepted")
	}
}

func TestIntegrationWebSocketCancellationClosesConnection(t *testing.T) {
	reached := make(chan struct{})
	stopped := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer connection.Close()
		if _, _, err = connection.ReadMessage(); err != nil {
			return
		}
		close(reached)
		_, _, _ = connection.ReadMessage()
		close(stopped)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, _, err := executeWebSocketIntegrationTransport(ctx, server.URL, &openapi3.Operation{}, integrationTransportConfig{Timeout: time.Minute}, nil, integrationTransportAuth{})
		done <- err
	}()
	select {
	case <-reached:
	case <-ctx.Done():
		t.Fatal("websocket request not reached")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled WebSocket succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("WebSocket ignored parent cancellation")
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("provider connection remained open")
	}
}

type cancelledIntegrationSearch struct {
	grpc_testing.UnimplementedSearchServiceServer
	reached chan struct{}
	stopped chan struct{}
}

func (s *cancelledIntegrationSearch) Search(ctx context.Context, _ *grpc_testing.SearchRequest) (*grpc_testing.SearchResponse, error) {
	close(s.reached)
	<-ctx.Done()
	close(s.stopped)
	return nil, ctx.Err()
}

func TestIntegrationGRPCCancellationReachesProvider(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	defer server.Stop()
	service := &cancelledIntegrationSearch{reached: make(chan struct{}), stopped: make(chan struct{})}
	grpc_testing.RegisterSearchServiceServer(server, service)
	reflection.Register(server)
	go server.Serve(listener)
	op := grpcSearchOperation()
	config, err := integrationTransportConfigFromOperation(op, "Search")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, _, err := executeGRPCIntegrationTransport(ctx, "http://"+listener.Addr().String(), op, config, map[string]interface{}{"query": "cancel"}, integrationTransportAuth{})
		done <- err
	}()
	select {
	case <-service.reached:
	case <-ctx.Done():
		t.Fatal("gRPC request not reached")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled gRPC succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("gRPC ignored parent cancellation")
	}
	select {
	case <-service.stopped:
	case <-time.After(time.Second):
		t.Fatal("provider did not stop")
	}
}
