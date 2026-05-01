package table_info

import (
	"encoding/json"
	"testing"

	"github.com/daptin/daptin/server/auth"
)

func TestDefaultGroupListUnmarshalStringAndObjectForms(t *testing.T) {
	var groups DefaultGroupList
	err := json.Unmarshal([]byte(`["users", {"Name": "agents", "Permission": 524288}]`), &groups)
	if err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].Name != "users" {
		t.Fatalf("expected first group to be users, got %q", groups[0].Name)
	}
	if groups[0].Permission != nil {
		t.Fatalf("expected string-form group permission to be nil")
	}
	if groups[1].Name != "agents" {
		t.Fatalf("expected second group to be agents, got %q", groups[1].Name)
	}
	if groups[1].Permission == nil || *groups[1].Permission != auth.AuthPermission(524288) {
		t.Fatalf("expected agents permission 524288, got %v", groups[1].Permission)
	}
}

func TestDefaultGroupListMarshalPreservesStringFormWithoutPermissions(t *testing.T) {
	groups := DefaultGroups("users", "administrators")

	encoded, err := json.Marshal(groups)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	if string(encoded) != `["users","administrators"]` {
		t.Fatalf("expected string-form groups, got %s", encoded)
	}
}

func TestDefaultGroupListMarshalPreservesNilAsNull(t *testing.T) {
	var groups DefaultGroupList

	encoded, err := json.Marshal(groups)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}

	if string(encoded) != `null` {
		t.Fatalf("expected nil groups to marshal as null, got %s", encoded)
	}
}

func TestDefaultGroupListNames(t *testing.T) {
	permission := auth.AuthPermission(524288)
	groups := DefaultGroupList{
		{Name: "users"},
		{Name: ""},
		{Name: "agents", Permission: &permission},
	}

	names := groups.Names()
	if len(names) != 2 || names[0] != "users" || names[1] != "agents" {
		t.Fatalf("unexpected names: %#v", names)
	}
}
