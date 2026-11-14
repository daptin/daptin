package server

import (
	"context"
	stdjson "encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	olricstats "github.com/buraksezer/olric/stats"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

func TestStatisticsRejectsGuestsBeforeCollectingMetrics(t *testing.T) {
	for name, user := range map[string]interface{}{
		"absent":         nil,
		"wrong type":     "administrator",
		"empty identity": &auth.SessionUser{},
		"typed nil":      (*auth.SessionUser)(nil),
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(response)
			c.Request = httptest.NewRequest(http.MethodGet, "/statistics", nil)
			if user != nil {
				c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "user", user))
			}
			// A nil database would panic if collection or authorization queries ran.
			CreateStatisticsHandler(nil, nil, "", nil)(c)
			if response.Code != http.StatusForbidden || response.Body.Len() != 0 || !c.IsAborted() {
				t.Fatalf("guest statistics: status=%d body=%s", response.Code, response.Body)
			}
		})
	}
}

func TestStatisticsDatabaseFailureDoesNotExposeMetrics(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest(http.MethodGet, "/statistics", nil)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "user", &auth.SessionUser{
		UserReferenceId: daptinid.DaptinReferenceId(uuid.New()),
	}))
	CreateStatisticsHandler(db, nil, "", nil)(c)
	if response.Code != http.StatusInternalServerError || response.Body.Len() != 0 || !c.IsAborted() {
		t.Fatalf("unavailable authorization: status=%d body=%s", response.Code, response.Body)
	}
}

func TestStatisticsUnavailableMembershipDoesNotExposeMetrics(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest(http.MethodGet, "/statistics", nil)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "user", &auth.SessionUser{
		UserReferenceId: daptinid.DaptinReferenceId(uuid.New()),
	}))
	// No membership schema is available; its lookup must fail closed.
	CreateStatisticsHandler(db, nil, "", &resource.DbResource{
		AdministratorGroupId: daptinid.DaptinReferenceId(uuid.New()),
	})(c)
	if response.Code != http.StatusForbidden || response.Body.Len() != 0 || !c.IsAborted() {
		t.Fatalf("unavailable membership: status=%d body=%s", response.Code, response.Body)
	}
}

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
