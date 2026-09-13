package actions

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
)

func TestMailUnpackConsumesResolvedMailValue(t *testing.T) {
	raw := strings.Join([]string{
		"Content-Type: multipart/mixed; boundary=parts",
		"",
		"--parts",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"message body",
		"--parts",
		"Content-Type: application/octet-stream",
		"Content-Disposition: attachment; filename=document.bin",
		"Content-Transfer-Encoding: base64",
		"",
		"YXR0YWNobWVudA==",
		"--parts--",
		"",
	}, "\r\n")
	performer := &mailUnpackActionPerformer{cruds: map[string]*resource.DbResource{
		"mail": {},
	}}

	responder, responses, errs := performer.DoAction(actionresponse.Outcome{}, map[string]interface{}{
		"mail": base64.StdEncoding.EncodeToString([]byte(raw)),
	}, nil)
	if len(errs) != 0 {
		t.Fatalf("mail.unpack returned errors: %v", errs)
	}
	if len(responses) != 0 || responder == nil {
		t.Fatalf("unexpected response contract: responder=%v responses=%v", responder, responses)
	}
	model, ok := responder.Result().(api2go.Api2GoModel)
	if !ok {
		t.Fatalf("mail.unpack result type = %T", responder.Result())
	}
	attributes := model.GetAttributes()
	if attributes["preferred_text_body"] != "message body" {
		t.Fatalf("preferred_text_body = %#v", attributes["preferred_text_body"])
	}
	parts, ok := attributes["parts"].([]map[string]interface{})
	if !ok || len(parts) != 2 {
		t.Fatalf("parts = %#v", attributes["parts"])
	}
	content := parts[1]["content"].([]interface{})[0].(map[string]interface{})
	decoded, err := base64.StdEncoding.DecodeString(content["contents"].(string))
	if err != nil || string(decoded) != "attachment" {
		t.Fatalf("attachment content = %q, %v", decoded, err)
	}
}

func TestMailUnpackRequiresResolvedMailValue(t *testing.T) {
	performer := &mailUnpackActionPerformer{cruds: map[string]*resource.DbResource{"mail": {}}}
	responder, responses, errs := performer.DoAction(actionresponse.Outcome{}, map[string]interface{}{}, nil)
	if responder != nil || len(responses) != 0 || len(errs) != 1 {
		t.Fatalf("unexpected missing-mail response: responder=%v responses=%v errors=%v", responder, responses, errs)
	}
}

func TestMailUnpackLimitsUseDefaultsOrExplicitReductions(t *testing.T) {
	defaults := defaultMailUnpackLimits()
	limits, err := mailUnpackLimits(map[string]interface{}{})
	if err != nil {
		t.Fatalf("mailUnpackLimits defaults: %v", err)
	}
	if limits != defaults {
		t.Fatalf("limits = %#v, want %#v", limits, defaults)
	}

	limits, err = mailUnpackLimits(map[string]interface{}{
		"max_raw_size":      "1024",
		"max_decoded_size":  int64(2048),
		"max_part_size":     512,
		"max_header_size":   float64(256),
		"max_parts":         4,
		"max_nesting_depth": 2,
	})
	if err != nil {
		t.Fatalf("mailUnpackLimits reductions: %v", err)
	}
	if limits.MaxRawBytes != 1024 || limits.MaxDecodedBytes != 2048 || limits.MaxPartBytes != 512 ||
		limits.MaxHeaderBytes != 256 || limits.MaxParts != 4 || limits.MaxNestingDepth != 2 {
		t.Fatalf("unexpected reduced limits: %#v", limits)
	}
}

func TestMailUnpackLimitsRejectInvalidValues(t *testing.T) {
	maximum := defaultMailUnpackLimits().MaxRawBytes
	tests := []interface{}{-1, 0, maximum + 1, 1.5, "", "not-a-number", true}
	for _, value := range tests {
		if _, err := mailUnpackLimits(map[string]interface{}{"max_raw_size": value}); err == nil {
			t.Errorf("max_raw_size %#v was accepted", value)
		}
	}
}
