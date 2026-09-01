package llm

import (
	"context"
	"errors"
	"testing"
	"time"

	gateway "github.com/daptin/llmgateway"
	"github.com/google/uuid"
)

func TestDaptinOlricPortsImplementGatewayContract(t *testing.T) {
	_, _, client, _ := newCatalogTestResources(t)
	values, err := client.NewDMap("llmgateway-port-values-" + uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	leases, err := client.NewDMap("llmgateway-port-leases-" + uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	cacheValues, err := client.NewDMap("llmgateway-port-cache-" + uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	counters := olricCounterStore{values: values, leases: leases}
	if value, err := counters.Add(ctx, "rpm", 1, time.Second); err != nil || value != 1 {
		t.Fatalf("first fixed-window add = %d, %v", value, err)
	}
	time.Sleep(600 * time.Millisecond)
	if value, err := counters.Add(ctx, "rpm", 1, time.Second); err != nil || value != 2 {
		t.Fatalf("second fixed-window add = %d, %v", value, err)
	}
	time.Sleep(600 * time.Millisecond)
	if value, found, err := counters.Get(ctx, "rpm"); err != nil || found || value != 0 {
		t.Fatalf("fixed-window expiry was extended: value=%d found=%v err=%v", value, found, err)
	}

	lease, err := counters.Acquire(ctx, "deployment", 1, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := counters.Acquire(ctx, "deployment", 1, time.Minute); !errors.Is(err, gateway.ErrCounterLimit) {
		t.Fatalf("second acquire = %v, want counter limit", err)
	}
	if err := counters.Release(ctx, lease); err != nil {
		t.Fatal(err)
	}
	if err := counters.Release(ctx, lease); err != nil {
		t.Fatalf("duplicate release must be idempotent: %v", err)
	}
	if _, err := counters.Acquire(ctx, "deployment", 1, time.Minute); err != nil {
		t.Fatalf("released capacity was not reusable: %v", err)
	}

	first, err := counters.Acquire(ctx, "staggered", 2, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(600 * time.Millisecond)
	second, err := counters.Acquire(ctx, "staggered", 2, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(600 * time.Millisecond)
	if _, err := counters.Acquire(ctx, "staggered", 2, time.Second); !errors.Is(err, gateway.ErrCounterLimit) {
		t.Fatalf("staggered lease counter expired before its newest lease: %v", err)
	}
	if err := counters.Release(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := counters.Release(ctx, second); err != nil {
		t.Fatal(err)
	}

	cache := olricResponseCache{values: cacheValues}
	if err := cache.Set(ctx, "response", []byte("payload"), time.Minute); err != nil {
		t.Fatal(err)
	}
	payload, found, err := cache.Get(ctx, "response")
	if err != nil || !found || string(payload) != "payload" {
		t.Fatalf("cache get = %q, %v, %v", payload, found, err)
	}
	if err := cache.Delete(ctx, "response"); err != nil {
		t.Fatal(err)
	}
	if _, found, err := cache.Get(ctx, "response"); err != nil || found {
		t.Fatalf("cache delete left entry: found=%v err=%v", found, err)
	}
}
