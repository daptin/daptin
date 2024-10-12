package server

import (
	"context"
	"fmt"
	"github.com/buraksezer/olric"
	"github.com/buraksezer/olric/config"
	"github.com/daptin/daptin/server/auth"
	"testing"
	"time"
)

func TestScanGroupPermission(t *testing.T) {
	var permissionList auth.GroupPermissionList
	permissionList = append(permissionList, auth.GroupPermission{})
	permissionList = append(permissionList, auth.GroupPermission{})

	olricInstance, _ := olric.New(config.New("local"))
	go func() {
		fmt.Printf("Start olric server")
		olricInstance.Start()
	}()
	time.Sleep(1 * time.Second)

	olricClient := olricInstance.NewEmbeddedClient()
	err := olricClient.RefreshMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	olricDmap, err := olricClient.NewDMap("test-map-1")
	if err != nil {
		t.Fatal(err)
	}
	err = olricDmap.Put(context.Background(), "test-key", permissionList, olric.EX(30*time.Minute), olric.NX())
	if err != nil {
		t.Fatal(err)
	}

	cachedValue, err := olricDmap.Get(context.Background(), "test-key")
	if err != nil {
		t.Fatal(err)
	}
	var valueFromCache auth.GroupPermissionList
	err = cachedValue.Scan(&valueFromCache)
	if err != nil {
		t.Fatal(err)
	}
	if len(valueFromCache) != 2 {
		t.Errorf("Expected 2 items in the result, instead of [%d]", len(valueFromCache))
	}

}
