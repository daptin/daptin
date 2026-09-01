package resource

import "testing"

func TestOAuthHashTokenIsStableSHA256Hex(t *testing.T) {
	got := OAuthHashToken("test-token")
	want := "4c5dc9b7708905f77f5e5d16316b5dfb425e68cb326dcd55a860e90a7707031e"
	if got != want {
		t.Fatalf("unexpected token hash: got %s want %s", got, want)
	}
}

func TestOAuthPKCES256(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	want := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if got := OAuthPKCES256(verifier); got != want {
		t.Fatalf("unexpected PKCE challenge: got %s want %s", got, want)
	}
}

func TestValidatePKCES256(t *testing.T) {
	row := map[string]interface{}{
		"code_challenge":        "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM",
		"code_challenge_method": "S256",
	}
	if err := validatePKCE(row, "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"); err != nil {
		t.Fatalf("expected verifier to pass: %v", err)
	}
	if err := validatePKCE(row, "wrong"); err == nil {
		t.Fatalf("expected wrong verifier to fail")
	}
}

func TestOAuthRowBelongsToAppFailsClosedOnInvalidNumericIdentity(t *testing.T) {
	provider := &OAuthProvider{}
	tests := []struct {
		name string
		row  map[string]interface{}
		app  map[string]interface{}
		want bool
	}{
		{name: "equal", row: map[string]interface{}{"oauth_app_id": int64(7)}, app: map[string]interface{}{"id": int64(7)}, want: true},
		{name: "equal string", row: map[string]interface{}{"oauth_app_id": "7"}, app: map[string]interface{}{"id": int64(7)}, want: true},
		{name: "different", row: map[string]interface{}{"oauth_app_id": int64(7)}, app: map[string]interface{}{"id": int64(8)}},
		{name: "missing", row: map[string]interface{}{}, app: map[string]interface{}{}},
		{name: "zero", row: map[string]interface{}{"oauth_app_id": int64(0)}, app: map[string]interface{}{"id": int64(0)}},
		{name: "malformed row", row: map[string]interface{}{"oauth_app_id": "invalid"}, app: map[string]interface{}{"id": int64(7)}},
		{name: "malformed app", row: map[string]interface{}{"oauth_app_id": int64(7)}, app: map[string]interface{}{"id": "invalid"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := provider.RowBelongsToApp(test.row, test.app); got != test.want {
				t.Fatalf("RowBelongsToApp() = %v, want %v", got, test.want)
			}
		})
	}
}
