package resource

import (
	"strings"
	"testing"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/sqlite3"
)

// TestResolveDefaultSortOrder covers the JSON:API read regression reported in
// daptin#213, where a system table with an empty/malformed world.default_order
// produced an empty backtick identifier and a 500 ("unrecognized token: \"`\"").
func TestResolveDefaultSortOrder(t *testing.T) {
	tests := []struct {
		name         string
		defaultOrder string
		hasCreatedAt bool
		want         []string
	}{
		{
			name:         "empty default with created_at falls back to -created_at",
			defaultOrder: "",
			hasCreatedAt: true,
			want:         []string{"-created_at"},
		},
		{
			name:         "whitespace default with created_at falls back to -created_at",
			defaultOrder: "   ",
			hasCreatedAt: true,
			want:         []string{"-created_at"},
		},
		{
			name:         "empty default without created_at omits order",
			defaultOrder: "",
			hasCreatedAt: false,
			want:         nil,
		},
		{
			name:         "bare-dash default with created_at falls back to -created_at",
			defaultOrder: "'-'",
			hasCreatedAt: true,
			want:         []string{"-created_at"},
		},
		{
			name:         "bare-dash default without created_at omits order",
			defaultOrder: "-",
			hasCreatedAt: false,
			want:         nil,
		},
		{
			name:         "quoted default order is honored",
			defaultOrder: "'-created_at'",
			hasCreatedAt: true,
			want:         []string{"-created_at"},
		},
		{
			name:         "comma-separated default order drops empty segments",
			defaultOrder: "-created_at,, +name",
			hasCreatedAt: true,
			want:         []string{"-created_at", "+name"},
		},
		{
			name:         "plain default order is honored",
			defaultOrder: "name",
			hasCreatedAt: false,
			want:         []string{"name"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveDefaultSortOrder(tc.defaultOrder, tc.hasCreatedAt)
			if len(got) != len(tc.want) {
				t.Fatalf("resolveDefaultSortOrder(%q, %v) = %v, want %v", tc.defaultOrder, tc.hasCreatedAt, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("resolveDefaultSortOrder(%q, %v)[%d] = %q, want %q", tc.defaultOrder, tc.hasCreatedAt, i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestBuildOrderExpressionsNeverEmptyIdentifier asserts that no combination of
// empty, whitespace, or bare-sign tokens ever yields an empty backtick-quoted
// identifier in the generated SQL - the exact failure from daptin#213.
func TestBuildOrderExpressionsNeverEmptyIdentifier(t *testing.T) {
	dialect := goqu.Dialect("sqlite3")
	prefix := "llm_usage."

	// Tokens that previously produced an empty identifier must yield no ORDER BY.
	for _, badTokens := range [][]string{
		{"-"},
		{"+"},
		{""},
		{"   "},
		{"-", "+", ""},
	} {
		orders := buildOrderExpressions(badTokens, prefix)
		if len(orders) != 0 {
			t.Fatalf("buildOrderExpressions(%v) produced %d orders, want 0", badTokens, len(orders))
		}
		sql, _, err := dialect.From("llm_usage").Order(orders...).ToSQL()
		if err != nil {
			t.Fatalf("ToSQL for %v returned error: %v", badTokens, err)
		}
		if strings.Contains(sql, "``") {
			t.Fatalf("buildOrderExpressions(%v) produced empty backtick identifier in SQL: %s", badTokens, sql)
		}
		if strings.Contains(sql, "ORDER BY") {
			t.Fatalf("buildOrderExpressions(%v) unexpectedly produced ORDER BY: %s", badTokens, sql)
		}
	}

	// A valid fallback token must produce a well-formed ORDER BY with no empty identifier.
	orders := buildOrderExpressions([]string{"-created_at"}, prefix)
	if len(orders) != 1 {
		t.Fatalf("buildOrderExpressions([-created_at]) produced %d orders, want 1", len(orders))
	}
	sql, _, err := dialect.From("llm_usage").Order(orders...).ToSQL()
	if err != nil {
		t.Fatalf("ToSQL for valid order returned error: %v", err)
	}
	if strings.Contains(sql, "``") {
		t.Fatalf("valid order produced empty backtick identifier: %s", sql)
	}
	if !strings.Contains(sql, "`llm_usage`.`created_at`") {
		t.Fatalf("expected qualified created_at column in SQL, got: %s", sql)
	}
	if !strings.Contains(sql, "ORDER BY") {
		t.Fatalf("expected ORDER BY for valid order, got: %s", sql)
	}
}
