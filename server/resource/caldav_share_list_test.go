package resource

import (
	"testing"

	daptinid "github.com/daptin/daptin/server/id"
)

func TestCalendarShareCursorIsBoundToCallerAndCollection(t *testing.T) {
	action := &calendarListSharesAction{share: &calendarShareAction{cruds: map[string]*DbResource{
		calendarCollectionTable: {EncryptionSecret: []byte("0123456789abcdef0123456789abcdef")},
	}}}
	caller := daptinid.InterfaceToDIR("11111111-1111-4111-8111-111111111111")
	collection := daptinid.InterfaceToDIR("22222222-2222-4222-8222-222222222222")
	last := daptinid.InterfaceToDIR("33333333-3333-4333-8333-333333333333")
	cursor, err := action.encodeCursor(last, caller, collection)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := action.decodeCursor(cursor, caller, collection); err != nil || got != last {
		t.Fatalf("cursor round trip = %s, %v", got, err)
	}
	for _, mismatch := range []struct {
		name       string
		cursor     interface{}
		caller     daptinid.DaptinReferenceId
		collection daptinid.DaptinReferenceId
	}{
		{"another caller", cursor, last, collection},
		{"another collection", cursor, caller, last},
		{"malformed", "invalid", caller, collection},
	} {
		t.Run(mismatch.name, func(t *testing.T) {
			if _, err := action.decodeCursor(mismatch.cursor, mismatch.caller, mismatch.collection); err == nil {
				t.Fatal("share cursor crossed its permission scope")
			}
		})
	}
}

func TestAccountShareTargetRequiresItsPersistedMembership(t *testing.T) {
	collection := daptinid.InterfaceToDIR("22222222-2222-4222-8222-222222222222")
	account := daptinid.InterfaceToDIR("33333333-3333-4333-8333-333333333333")
	other := daptinid.InterfaceToDIR("44444444-4444-4444-8444-444444444444")
	name := davAccountShareGroupName("calshare", collection, account)
	if got := davAccountShareTarget("calshare", collection, name, []daptinid.DaptinReferenceId{account}); got != account {
		t.Fatalf("valid account share target = %s", got)
	}
	for _, members := range [][]daptinid.DaptinReferenceId{{}, {other}, {account, other}} {
		if got := davAccountShareTarget("calshare", collection, name, members); got != daptinid.NullReferenceId {
			t.Fatalf("changed membership was accepted: %v -> %s", members, got)
		}
	}
	if got := davAccountShareTarget("calshare", other, name, []daptinid.DaptinReferenceId{account}); got != daptinid.NullReferenceId {
		t.Fatalf("another calendar's share was accepted: %s", got)
	}
}
