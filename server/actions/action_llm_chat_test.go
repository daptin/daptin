package actions

import (
	"strings"
	"testing"

	"github.com/daptin/daptin/server/actionresponse"
)

func TestLLMChatActionRejectsUnsupportedActionShapesBeforeInvocation(t *testing.T) {
	performer := &llmChatActionPerformer{}
	for _, test := range []struct {
		name  string
		input map[string]interface{}
		want  string
	}{
		{name: "stream", input: map[string]interface{}{"model": "model", "stream": true}, want: "does not support streaming"},
		{name: "multiple choices", input: map[string]interface{}{
			"model": "model", "n": 2.0, "messages": []interface{}{map[string]interface{}{"role": "user", "content": "hello"}},
		}, want: "exactly one choice"},
		{name: "arbitrary parameters", input: map[string]interface{}{"model": "model", "extra_params": map[string]interface{}{}}, want: "extra_params is not supported"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, failures := performer.DoAction(actionresponse.Outcome{}, test.input, nil)
			if len(failures) != 1 || !strings.Contains(failures[0].Error(), test.want) {
				t.Fatalf("failures = %v, want %q", failures, test.want)
			}
		})
	}
}
