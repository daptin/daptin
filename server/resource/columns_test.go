package resource

import (
	"fmt"
	"testing"
)

func TestAction(t *testing.T) {
	jsonStr, err := json.Marshal(SystemActions)
	if err != nil {
		t.Errorf("Failed to marshal actions: %v", err)
	}
	fmt.Printf("%v", string(jsonStr))
}