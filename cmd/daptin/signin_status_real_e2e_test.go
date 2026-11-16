package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/daptin/daptin/server/auth"
)

func TestSigninRejectionStatusRealE2E(t *testing.T) {
	requireRealE2E(t)
	for _, backend := range []struct{ name, driver, dsn string }{{"sqlite", "sqlite3", ""}, {"postgres", "postgres", os.Getenv("DAPTIN_TEST_POSTGRES_DSN")}} {
		t.Run(backend.name, func(t *testing.T) {
			if backend.driver == "postgres" && backend.dsn == "" {
				t.Skip("set DAPTIN_TEST_POSTGRES_DSN to an empty disposable database")
			}
			fixture := startDaptinE2E(t, transportE2EDaptinOptions{databaseType: backend.driver, connectionString: backend.dsn, schema: "EnableGraphQL: true\n"})
			base, client := fixture.URL, fixture.Client
			admin := accessGroupsE2ESignupSigninAdmin(t, client, base)
			accessGroupsE2ESignupSigninUser(t, client, base, admin, "signin-status-user")
			email := "signin-status-user@test.local"
			var firstRejection interface{}
			for _, test := range []struct {
				name       string
				attributes map[string]interface{}
				status     int
			}{
				{"wrong password", map[string]interface{}{"email": email, "password": "wrong-password"}, 401},
				{"unknown account", map[string]interface{}{"email": "missing-signin-user@test.local", "password": "wrong-password"}, 401},
				{"empty password", map[string]interface{}{"email": email, "password": ""}, 401},
				{"null password", map[string]interface{}{"email": email, "password": nil}, 401},
				{"missing password", map[string]interface{}{"email": email}, 401},
				{"empty email", map[string]interface{}{"email": "", "password": "wrong-password"}, 401},
				{"missing email", map[string]interface{}{"password": "wrong-password"}, 401},
				{"numeric password", map[string]interface{}{"email": email, "password": 123}, 400},
				{"object password", map[string]interface{}{"email": email, "password": map[string]interface{}{"value": "wrong-password"}}, 400},
				{"numeric email", map[string]interface{}{"email": 123, "password": "wrong-password"}, 400},
				{"caller bypass flag", map[string]interface{}{"email": email, "password": "wrong-password", "skipPasswordCheck": true}, 401},
			} {
				encoded, err := json.Marshal(map[string]interface{}{"attributes": test.attributes})
				if err != nil {
					t.Fatal(err)
				}
				request, err := http.NewRequest(http.MethodPost, base+"/action/user_account/signin", strings.NewReader(string(encoded)))
				if err != nil {
					t.Fatal(err)
				}
				request.Header.Set("Content-Type", "application/json")
				response, err := client.Do(request)
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(response.Body)
				response.Body.Close()
				if err != nil {
					t.Fatal(err)
				}
				if response.StatusCode != test.status {
					t.Fatalf("%s: HTTP %d, want %d", test.name, response.StatusCode, test.status)
				}
				var decoded interface{}
				if err = json.Unmarshal(data, &decoded); err != nil {
					t.Fatalf("%s: rejection was not JSON", test.name)
				}
				if token, ok := transportE2EFindString(decoded, "value"); ok && token != "" {
					t.Fatalf("%s issued a token", test.name)
				}
				for _, cookie := range response.Header.Values("Set-Cookie") {
					if strings.Contains(cookie, "token=") {
						t.Fatalf("%s set a token cookie", test.name)
					}
				}
				if test.status == 401 {
					message, ok := transportE2EFindString(decoded, "message")
					if !ok || message != "Invalid username or password" {
						t.Fatalf("%s: unexpected rejection message", test.name)
					}
					if firstRejection == nil {
						firstRejection = decoded
					} else if !reflect.DeepEqual(firstRejection, decoded) {
						t.Fatalf("%s: credential rejection disclosed different account state", test.name)
					}
				}
				t.Logf("%s: HTTP %d, no token or cookie", test.name, response.StatusCode)
			}
			var graphQLMessage string
			for _, account := range []string{email, "missing-signin-user@test.local"} {
				response := transportE2EPostJSON(t, client, base+"/graphql", "", map[string]interface{}{"query": fmt.Sprintf(`mutation { executeSigninOnUserAccount(email: %q, password: "wrong-password") { ResponseType } }`, account)})
				if !transportE2EGraphQLHasErrors(response) {
					t.Fatal("GraphQL accepted invalid credentials")
				}
				errors := response.(map[string]interface{})["errors"].([]interface{})
				message := errors[0].(map[string]interface{})["message"].(string)
				if !strings.Contains(message, "Invalid username or password") {
					t.Fatal("GraphQL did not return the generic credential rejection")
				}
				if graphQLMessage != "" && message != graphQLMessage {
					t.Fatal("GraphQL disclosed different account state")
				}
				graphQLMessage = message
				data := response.(map[string]interface{})["data"].(map[string]interface{})
				if data["executeSigninOnUserAccount"] != nil {
					t.Fatal("GraphQL returned token directives for rejected credentials")
				}
			}
			assertUsableToken := func(response interface{}) {
				t.Helper()
				token, ok := transportE2EFindString(response, "value")
				if !ok || token == "" {
					t.Fatal("successful login produced no token")
				}
				mine := accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/user_account/mine", token, nil, http.StatusOK)
				got, ok := transportE2EFindString(mine, "email")
				if !ok || got != email {
					t.Fatal("token authenticated the wrong account")
				}
			}
			// Correct credentials still work after all rejected attempts.
			assertUsableToken(accessGroupsE2ERequestJSON(t, client, http.MethodPost, base+"/action/user_account/signin", "", map[string]interface{}{"attributes": map[string]interface{}{"email": email, "password": "testpass123"}}, http.StatusOK))
			// The OAuth composition explicitly enables password-free issuance in a
			// trusted backend outcome. Keep that capability behind action permissions.
			world := accessGroupsE2EFindResourceID(t, client, base, admin, "world", "table_name", "user_account")
			definition := map[string]interface{}{"Name": "trusted_signin_status", "OnType": "user_account", "InstanceOptional": true, "InFields": []interface{}{}, "OutFields": []interface{}{map[string]interface{}{"Type": "jwt.token", "Method": "EXECUTE", "Attributes": map[string]interface{}{"email": email, "skipPasswordCheck": true}}}}
			schema, err := json.Marshal(definition)
			if err != nil {
				t.Fatal(err)
			}
			action := accessGroupsE2ECreateRecord(t, client, base, admin, "action", map[string]interface{}{"action_name": "trusted_signin_status", "label": "Trusted sign-in fixture", "world_id": world, "instance_optional": true, "action_schema": string(schema)})
			accessGroupsE2EAssertStatus(t, client, http.MethodPatch, base+"/api/action/"+action, admin, accessGroupsE2ERecordPayload("action", action, map[string]interface{}{"permission": int64(auth.None)}), http.StatusOK)
			accessGroupsE2EAssertStatus(t, client, http.MethodPost, base+"/action/user_account/trusted_signin_status", "", map[string]interface{}{"attributes": map[string]interface{}{}}, http.StatusForbidden)
			assertUsableToken(accessGroupsE2ERequestJSON(t, client, http.MethodPost, base+"/action/user_account/trusted_signin_status", admin, map[string]interface{}{"attributes": map[string]interface{}{"email": "missing-signin-user@test.local", "skipPasswordCheck": false}}, http.StatusOK))
			t.Logf("%s: identical known/unknown rejection through HTTP and GraphQL; correct-password and trusted backend issuance produced usable account-scoped tokens", backend.name)
		})
	}
}
