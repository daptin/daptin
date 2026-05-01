package resource

import "testing"

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

func TestCheckMeteringQuota(t *testing.T) {
	allowed, message := checkMeteringQuota(map[string]interface{}{
		"requests_per_period":      int64(2),
		"compute_units_per_period": int64(100),
	}, map[string]interface{}{
		"request_count": int64(2),
		"compute_units": int64(10),
	}, "requests")
	if allowed {
		t.Fatalf("expected quota to be denied")
	}
	if message != "request quota exceeded" {
		t.Fatalf("expected request quota message, got %q", message)
	}
}
