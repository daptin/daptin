package resource

import (
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
)

func TestEvaluateMeteringCost(t *testing.T) {
	cost, err := EvaluateMeteringCost("response.usage.total_tokens + request.rows", map[string]interface{}{
		"request": map[string]interface{}{
			"rows": 2,
		},
		"response": map[string]interface{}{
			"usage": map[string]interface{}{
				"total_tokens": 9,
			},
		},
	})
	if err != nil {
		t.Fatalf("EvaluateMeteringCost returned error: %v", err)
	}
	if cost != 11 {
		t.Fatalf("expected cost 11, got %d", cost)
	}
}

func TestEvaluateMeteringCostRoundsUpFractions(t *testing.T) {
	cost, err := EvaluateMeteringCost("2.2", map[string]interface{}{})
	if err != nil {
		t.Fatalf("EvaluateMeteringCost returned error: %v", err)
	}
	if cost != 3 {
		t.Fatalf("expected cost 3, got %d", cost)
	}
}

func TestMeteringLimitsAreGenericValidatedAndDeterministic(t *testing.T) {
	limits, err := meteringLimits(map[string]interface{}{
		"limits": `[{"metric":"total_tokens","window":"month","maximum":100,"mode":"hard"},{"metric":"requests","window":"month","maximum":20,"mode":"hard"},{"metric":"requests","window":"minute","maximum":2,"mode":"soft"}]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(limits) != 3 || limits[0].Metric != "requests" || limits[0].Window != "minute" ||
		limits[1].Metric != "requests" || limits[1].Window != "month" || limits[2].Metric != "total_tokens" {
		t.Fatalf("limits were not normalized in deterministic lock order: %#v", limits)
	}
	if _, err := meteringLimits(map[string]interface{}{
		"limits": `[{"metric":"requests","window":"minute","maximum":1},{"metric":"requests","window":"minute","maximum":2}]`,
	}); err == nil {
		t.Fatal("expected duplicate metric limits to be rejected")
	}
}

func TestHydrateMeteringContextRestoresPersistedRequest(t *testing.T) {
	user := &auth.SessionUser{UserId: 42}
	ctx := hydrateMeteringContext(MeteringContext{User: user, StatusCode: 200}, map[string]interface{}{
		"endpoint": "/v1/chat", "method": "POST", "entity_type": "llm_model",
		"action_name": "invoke", "request_type": "llm_chat",
	})
	if ctx.Endpoint != "/v1/chat" || ctx.Method != "POST" || ctx.EntityType != "llm_model" ||
		ctx.ActionName != "invoke" || ctx.RequestType != "llm_chat" {
		t.Fatalf("hydrated context = %#v", ctx)
	}
	if ctx.Request == nil || ctx.Request.Method != "POST" || ctx.Request.URL.Path != "/v1/chat" ||
		ctx.Request.Context().Value("user") != user {
		t.Fatalf("hydrated request = %#v", ctx.Request)
	}
}

func TestMeteringWindowBoundaries(t *testing.T) {
	now := time.Date(2026, time.September, 1, 23, 59, 59, 0, time.FixedZone("test", 5*60*60+30*60))
	for _, test := range []struct {
		window string
		start  time.Time
		end    time.Time
	}{
		{window: "minute", start: time.Date(2026, 9, 1, 18, 29, 0, 0, time.UTC), end: time.Date(2026, 9, 1, 18, 30, 0, 0, time.UTC)},
		{window: "hour", start: time.Date(2026, 9, 1, 18, 0, 0, 0, time.UTC), end: time.Date(2026, 9, 1, 19, 0, 0, 0, time.UTC)},
		{window: "day", start: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), end: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)},
		{window: "month", start: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), end: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
	} {
		t.Run(test.window, func(t *testing.T) {
			start, end, err := meteringWindow(test.window, nil, now)
			if err != nil {
				t.Fatal(err)
			}
			if !start.Equal(test.start) || !end.Equal(test.end) {
				t.Fatalf("%s window = %s..%s, want %s..%s", test.window, start, end, test.start, test.end)
			}
		})
	}
	memberStart := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	memberEnd := memberStart.AddDate(0, 1, 0)
	start, end, err := meteringWindow("member_period", map[string]interface{}{
		"period_start": memberStart, "period_end": memberEnd,
	}, now)
	if err != nil || !start.Equal(memberStart) || !end.Equal(memberEnd) {
		t.Fatalf("member period = %s..%s, %v", start, end, err)
	}
}
