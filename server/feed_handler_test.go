package server

import (
	"testing"
	"time"
)

func TestFeedDatabaseValues(t *testing.T) {
	for _, tc := range []struct {
		value interface{}
		want  bool
	}{
		{true, true}, {false, false},
		{int64(1), true}, {int64(0), false},
		{"1", true}, {"0", false},
	} {
		got, err := feedEnabled(tc.value)
		if err != nil || got != tc.want {
			t.Errorf("feedEnabled(%v) = %v, %v; want %v", tc.value, got, err, tc.want)
		}
	}
	if _, err := feedEnabled(int64(2)); err == nil {
		t.Error("invalid boolean was accepted")
	}
	if _, err := feedEnabled(nil); err == nil {
		t.Error("missing boolean was accepted")
	}

	created := time.Date(2026, 9, 20, 12, 30, 0, 123456000, time.FixedZone("+0530", 5*60*60+30*60))
	for _, value := range []interface{}{created, created.Format(time.RFC3339Nano), created.String()} {
		got, err := feedCreatedAt(value)
		if err != nil || !got.Equal(created) {
			t.Errorf("feedCreatedAt(%v) = %v, %v; want %v", value, got, err, created)
		}
	}
	if _, err := feedCreatedAt("not-a-date"); err == nil {
		t.Error("invalid date was accepted")
	}
}

func TestFeedItemRejectsMalformedStreamRow(t *testing.T) {
	item := map[string]interface{}{
		"title": "Example", "link": "https://example.test", "description": "Article",
		"author_name": "Author", "author_email": "author@example.test", "created_at": "not-a-date",
	}
	if _, err := feedItem(item); err == nil {
		t.Error("malformed stream date was accepted")
	}
	delete(item, "title")
	if _, err := feedItem(item); err == nil {
		t.Error("missing stream title was accepted")
	}
}
