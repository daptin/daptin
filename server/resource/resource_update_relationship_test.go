package resource

import "testing"

func TestRelationshipReplacementReferenceIDsAcceptsResourceLinkageShapes(t *testing.T) {
	referenceIDs := relationshipReplacementReferenceIDs([]interface{}{
		map[string]interface{}{"id": "019d5df0-a8b7-7c84-9015-d26bb74ae86a"},
		map[string]interface{}{"reference_id": "019d5df0-a8b7-7c84-9015-d26bb74ae86b"},
	})
	if !referenceIDs["019d5df0-a8b7-7c84-9015-d26bb74ae86a"] ||
		!referenceIDs["019d5df0-a8b7-7c84-9015-d26bb74ae86b"] {
		t.Fatalf("relationship linkage reference IDs were not retained: %#v", referenceIDs)
	}
}
