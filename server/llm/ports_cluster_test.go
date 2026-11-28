package llm

import (
	"context"
	"errors"
	"fmt"
	gateway "github.com/daptin/llmgateway"
	"io"
	"net"
	"testing"
	"time"

	"github.com/buraksezer/olric"
	olricconfig "github.com/buraksezer/olric/config"
)

func TestOlricCountersAcrossMembers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var addresses []string
	var nodes []*olric.Olric
	var clients []*olric.ClusterClient
	var seed string
	for n := 0; n < 3; n++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		listener.Close()
		member, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		membershipPort := member.Addr().(*net.TCPAddr).Port
		member.Close()
		cfg := olricconfig.New("local")
		cfg.BindAddr = "127.0.0.1"
		cfg.BindPort = port
		cfg.MemberlistConfig.BindAddr = "127.0.0.1"
		cfg.MemberlistConfig.BindPort = membershipPort
		cfg.MemberlistConfig.Name = fmt.Sprintf("127.0.0.1:%d", port)
		cfg.PartitionCount = 23
		cfg.ReplicaCount = 1
		cfg.ReadQuorum = 1
		cfg.WriteQuorum = 1
		cfg.LogOutput = io.Discard
		if seed != "" {
			cfg.Peers = []string{seed}
		} else {
			seed = fmt.Sprintf("127.0.0.1:%d", membershipPort)
		}
		started := make(chan struct{})
		cfg.Started = func() { close(started) }
		node, err := olric.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		nodes = append(nodes, node)
		go func() { _ = node.Start() }()
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = node.Shutdown(ctx)
		})
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		addresses = append(addresses, fmt.Sprintf("127.0.0.1:%d", port))
		if n == 0 {
			seedClient, err := olric.NewClusterClient([]string{addresses[0]}, olric.WithRoutingTableFetchInterval(100*time.Millisecond))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				_ = seedClient.Close(ctx)
			})
			values, err := seedClient.NewDMap("llmgateway-counters")
			if err != nil {
				t.Fatal(err)
			}
			leases, err := seedClient.NewDMap("llmgateway-leases")
			if err != nil {
				t.Fatal(err)
			}
			store := olricCounterStore{values: values, leases: leases}
			for k := 0; k < 100; k++ {
				if _, err := store.Add(ctx, fmt.Sprintf("deployment-%d", k), 1, time.Minute); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	for _, address := range addresses {
		client, err := olric.NewClusterClient([]string{address}, olric.WithRoutingTableFetchInterval(100*time.Millisecond))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = client.Close(ctx)
		})
		clients = append(clients, client)
	}
	for _, client := range clients {
		for {
			members, err := client.Members(ctx)
			if err == nil && len(members) == 3 {
				break
			}
			if ctx.Err() != nil {
				t.Fatal(ctx.Err())
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	stores := make([]olricCounterStore, 3)
	for i, client := range clients {
		values, err := client.NewDMap("llmgateway-counters")
		if err != nil {
			t.Fatal(err)
		}
		leases, err := client.NewDMap("llmgateway-leases")
		if err != nil {
			t.Fatal(err)
		}
		stores[i] = olricCounterStore{values: values, leases: leases}
	}
	// Embedded clients deliberately enter through a non-owner for most keys.
	// That is the same forwarding path a cluster client takes with an older
	// routing table during membership changes.
	for _, node := range nodes {
		embedded := node.NewEmbeddedClient()
		values, err := embedded.NewDMap("llmgateway-counters")
		if err != nil {
			t.Fatal(err)
		}
		leases, err := embedded.NewDMap("llmgateway-leases")
		if err != nil {
			t.Fatal(err)
		}
		stores = append(stores, olricCounterStore{values: values, leases: leases})
	}
	for k := 0; k < 100; k++ {
		key := fmt.Sprintf("deployment-%d", k)
		for i, store := range stores {
			token, err := store.Acquire(ctx, key, 2, time.Minute)
			if err != nil {
				t.Fatalf("key=%s member=%d acquire: %T %v", key, i, err, err)
			}
			if value, found, err := store.Get(ctx, key); err != nil || !found || value != 2 {
				t.Fatalf("member %d counter=%d found=%t err=%v", i, value, found, err)
			}
			if err := store.Release(ctx, token); err != nil {
				t.Fatalf("key=%s member=%d release: %T %v", key, i, err, err)
			}
		}
	}
	type acquired struct {
		member int
		token  string
		err    error
	}
	results := make(chan acquired, len(stores))
	for i, store := range stores {
		go func() {
			token, err := store.Acquire(ctx, "concurrent", 3, time.Minute)
			results <- acquired{i, token, err}
		}()
	}
	var held []acquired
	for range stores {
		result := <-results
		if result.err == nil {
			held = append(held, result)
		} else if !errors.Is(result.err, gateway.ErrCounterLimit) {
			t.Fatal(result.err)
		}
	}
	if len(held) != 3 {
		t.Fatalf("distributed maximum admitted %d, want 3", len(held))
	}
	for _, lease := range held {
		if err := stores[(lease.member+1)%len(stores)].Release(ctx, lease.token); err != nil {
			t.Fatal(err)
		}
	}
	if value, found, err := stores[0].Get(ctx, "concurrent"); err != nil || found || value != 0 {
		t.Fatalf("released counter: %d %t %v", value, found, err)
	}

}
