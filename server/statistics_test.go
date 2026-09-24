package server

import (
	"context"
	stdjson "encoding/json"
	"strings"
	"testing"
	"time"

	olricstats "github.com/buraksezer/olric/stats"
)

func TestOlricStatisticsExposeLocalClusterMetricsWithoutCommandLine(t *testing.T) {
	address, _, client := startRateLimitTestNode(t, "")
	before, err := client.Stats(context.Background(), address)
	if err != nil {
		t.Fatal(err)
	}
	dmap, err := client.NewDMap("statistics-test")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = dmap.Get(context.Background(), "missing")
	if err := dmap.Put(context.Background(), "present", "value"); err != nil {
		t.Fatal(err)
	}
	if _, err := dmap.Get(context.Background(), "present"); err != nil {
		t.Fatal(err)
	}

	stats := olricStatistics(context.Background(), client, address)
	if stats["available"] != true || stats["member_count"] != 1 || stats["routing_available"] != true {
		t.Fatalf("Olric statistics unavailable: %#v", stats)
	}
	if stats["member"] != address || stats["members"].([]string)[0] != address {
		t.Fatalf("wrong Olric member: %#v", stats)
	}
	partitions := stats["partitions"].(map[string]int)
	if partitions["total"] == 0 || partitions["without_primary"] != 0 {
		t.Fatalf("unexpected partition ownership: %#v", partitions)
	}
	dmaps := stats["dmaps"].(olricstats.DMaps)
	if dmaps.GetHits <= before.DMaps.GetHits || dmaps.GetMisses <= before.DMaps.GetMisses {
		t.Fatalf("Olric cache reads were not counted: %#v", dmaps)
	}
	serialized, err := stdjson.Marshal(stats)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(serialized)), "cmdline") {
		t.Fatalf("Olric statistics expose process command line: %s", serialized)
	}
}

func TestProcessStatisticsNeverExposeCommandLines(t *testing.T) {
	stats, err := NewHostStats(time.Second).GetProcessInfo()
	if err != nil {
		t.Fatalf("GetProcessInfo: %v", err)
	}

	processStats, ok := stats.(map[string]interface{})
	if !ok {
		t.Fatalf("process statistics type = %T", stats)
	}
	topProcesses, ok := processStats["top_processes"].([]map[string]interface{})
	if !ok {
		t.Fatalf("top_processes type = %T", processStats["top_processes"])
	}
	for i, process := range topProcesses {
		if _, exists := process["cmdline"]; exists {
			t.Fatalf("process %d exposes cmdline: %#v", i, process)
		}
	}

	serialized, err := stdjson.Marshal(stats)
	if err != nil {
		t.Fatalf("marshal process statistics: %v", err)
	}
	if strings.Contains(strings.ToLower(string(serialized)), "cmdline") {
		t.Fatalf("serialized process statistics expose command lines: %s", serialized)
	}
}
