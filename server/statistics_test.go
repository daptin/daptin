package server

import (
	stdjson "encoding/json"
	"strings"
	"testing"
	"time"
)

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
