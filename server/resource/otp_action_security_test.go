package resource

import (
	"testing"

	"github.com/daptin/daptin/server/auth"
)

func TestSensitiveOTPActionsExcludeGuestExecution(t *testing.T) {
	sensitive := map[string]bool{
		"send_otp":             true,
		"verify_otp":           true,
		"verify_mobile_number": true,
	}
	found := map[string]bool{}
	for _, action := range SystemActions {
		if !sensitive[action.Name] {
			continue
		}
		found[action.Name] = true
		if action.Permission == nil {
			t.Fatalf("%s must have an explicit permission", action.Name)
		}
		if *action.Permission&auth.GuestExecute != 0 {
			t.Fatalf("%s must not grant GuestExecute", action.Name)
		}
		if *action.Permission&auth.AuthenticatedExecute == 0 {
			t.Fatalf("%s must grant AuthenticatedExecute", action.Name)
		}
	}
	for name := range sensitive {
		if !found[name] {
			t.Fatalf("sensitive OTP action %s not found", name)
		}
	}
}

func TestSignupDoesNotProvisionOTPProfile(t *testing.T) {
	for _, action := range SystemActions {
		if action.Name != "signup" {
			continue
		}
		for _, outcome := range action.OutFields {
			if outcome.Type == "otp.generate" {
				t.Fatal("signup must not provision an OTP profile before authenticated enrollment")
			}
		}
		return
	}
	t.Fatal("signup action not found")
}
