package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const accessGroupsE2ESchema = `
Tables:
  - TableName: public_page
    Permission: 3
    DefaultPermission: 2
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label

  - TableName: private_note
    Permission: 16384
    DefaultPermission: 1
    AccessGroups:
      - Name: users
        Permission: 114688
    DefaultGroups:
      - Name: users
        Permission: 49152
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label

  - TableName: owner_note
    Permission: 16384
    DefaultPermission: 256
    AccessGroups:
      - Name: users
        Permission: 114688
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label

  - TableName: mixed_article
    Permission: 3
    DefaultPermission: 2
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label

  - TableName: workspace_item
    Permission: 16384
    DefaultPermission: 1
    AccessGroups:
      - Name: users
        Permission: 245760
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label

  - TableName: action_doc
    Permission: 524288
    AccessGroups:
      - Name: users
        Permission: 524288
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label

Actions:
  - Name: allowed_action
    Label: Allowed action
    OnType: action_doc
    InstanceOptional: true
    Permission: 0
    AccessGroups:
      - Name: users
        Permission: 524288
    OutFields:
      - Method: ACTIONRESPONSE
        Type: client.notify
        Attributes:
          type: success
          title: Access
          message: allowed

  - Name: denied_action
    Label: Denied action
    OnType: action_doc
    InstanceOptional: true
    Permission: 0
    OutFields:
      - Method: ACTIONRESPONSE
        Type: client.notify
        Attributes:
          type: success
          title: Access
          message: denied
`

func TestAccessGroupsRealAuthorizationScenariosE2E(t *testing.T) {
	if os.Getenv("DAPTIN_REAL_E2E") != "1" {
		t.Skip("set DAPTIN_REAL_E2E=1 to run real access-group authorization e2e")
	}

	port := freeAccessGroupsE2EPort(t)
	httpsPort := freeAccessGroupsE2EPort(t)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	stop := startAccessGroupsE2EDaptin(t, port, httpsPort, baseURL, accessGroupsE2ESchema)
	defer stop()

	client := &http.Client{Timeout: 20 * time.Second}
	adminToken := accessGroupsE2ESignupSigninAdmin(t, client, baseURL)
	userToken := accessGroupsE2ESignupSigninUser(t, client, baseURL, adminToken, "user")
	otherUserToken := accessGroupsE2ESignupSigninUser(t, client, baseURL, adminToken, "other")
	editorToken := accessGroupsE2ESignupSigninUser(t, client, baseURL, adminToken, "editor")
	memberToken := accessGroupsE2ESignupSigninUser(t, client, baseURL, adminToken, "member")

	t.Run("public site", func(t *testing.T) {
		accessGroupsE2ECreateRecord(t, client, baseURL, adminToken, "public_page", map[string]interface{}{"title": "public"})

		accessGroupsE2EAssertListCount(t, client, baseURL, "", "public_page", 1)
		accessGroupsE2EAssertStatus(t, client, http.MethodPost, baseURL+"/api/public_page", "", accessGroupsE2ERecordPayload("public_page", "", map[string]interface{}{"title": "guest write"}), http.StatusForbidden)
	})

	t.Run("private site", func(t *testing.T) {
		accessGroupsE2ECreateRecord(t, client, baseURL, adminToken, "private_note", map[string]interface{}{"title": "private"})

		accessGroupsE2EAssertStatus(t, client, http.MethodGet, baseURL+"/api/private_note", "", nil, http.StatusForbidden)
		accessGroupsE2EAssertListCount(t, client, baseURL, userToken, "private_note", 1)
		accessGroupsE2EAssertStatus(t, client, http.MethodPost, baseURL+"/api/private_note", userToken, accessGroupsE2ERecordPayload("private_note", "", map[string]interface{}{"title": "user private"}), http.StatusCreated)
	})

	t.Run("semi private owner rows", func(t *testing.T) {
		accessGroupsE2ECreateRecord(t, client, baseURL, userToken, "owner_note", map[string]interface{}{"title": "owned by user"})

		accessGroupsE2EAssertListCount(t, client, baseURL, userToken, "owner_note", 1)
		accessGroupsE2EAssertListCount(t, client, baseURL, otherUserToken, "owner_note", 0)
	})

	t.Run("mixed public and private rows", func(t *testing.T) {
		accessGroupsE2ECreateRecord(t, client, baseURL, adminToken, "mixed_article", map[string]interface{}{"title": "public article"})
		privateArticleID := accessGroupsE2ECreateRecord(t, client, baseURL, adminToken, "mixed_article", map[string]interface{}{"title": "private article"})
		accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/mixed_article/"+privateArticleID, adminToken, accessGroupsE2ERecordPayload("mixed_article", privateArticleID, map[string]interface{}{"permission": 0}), http.StatusOK)

		accessGroupsE2EAssertListCount(t, client, baseURL, "", "mixed_article", 1)
		accessGroupsE2EAssertListCount(t, client, baseURL, adminToken, "mixed_article", 2)
	})

	t.Run("shared group workspace", func(t *testing.T) {
		editorsGroupID := accessGroupsE2ECreateUsergroup(t, client, baseURL, adminToken, "e2e_editors")
		membersGroupID := accessGroupsE2ECreateUsergroup(t, client, baseURL, adminToken, "e2e_members")
		editorUserID := accessGroupsE2EFindResourceID(t, client, baseURL, adminToken, "user_account", "email", "editor@test.local")
		memberUserID := accessGroupsE2EFindResourceID(t, client, baseURL, adminToken, "user_account", "email", "member@test.local")
		accessGroupsE2ECreateJoin(t, client, baseURL, adminToken, "user_account_user_account_id_has_usergroup_usergroup_id", map[string]interface{}{"user_account_id": editorUserID, "usergroup_id": editorsGroupID}, 0)
		accessGroupsE2ECreateJoin(t, client, baseURL, adminToken, "user_account_user_account_id_has_usergroup_usergroup_id", map[string]interface{}{"user_account_id": memberUserID, "usergroup_id": membersGroupID}, 0)

		itemID := accessGroupsE2ECreateRecord(t, client, baseURL, adminToken, "workspace_item", map[string]interface{}{"title": "shared"})
		accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/workspace_item/"+itemID, adminToken, accessGroupsE2ERecordPayload("workspace_item", itemID, map[string]interface{}{"permission": 1}), http.StatusOK)
		accessGroupsE2ECreateJoin(t, client, baseURL, adminToken, "workspace_item_workspace_item_id_has_usergroup_usergroup_id", map[string]interface{}{"workspace_item_id": itemID, "usergroup_id": editorsGroupID}, 180224)
		accessGroupsE2ECreateJoin(t, client, baseURL, adminToken, "workspace_item_workspace_item_id_has_usergroup_usergroup_id", map[string]interface{}{"workspace_item_id": itemID, "usergroup_id": membersGroupID}, 49152)

		accessGroupsE2EAssertListCount(t, client, baseURL, editorToken, "workspace_item", 1)
		accessGroupsE2EAssertListCount(t, client, baseURL, memberToken, "workspace_item", 1)
		accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/workspace_item/"+itemID, editorToken, accessGroupsE2ERecordPayload("workspace_item", itemID, map[string]interface{}{"title": "edited"}), http.StatusOK)
		accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/workspace_item/"+itemID, memberToken, accessGroupsE2ERecordPayload("workspace_item", itemID, map[string]interface{}{"title": "member edit"}), http.StatusForbidden)
	})

	t.Run("action two gate", func(t *testing.T) {
		accessGroupsE2EAssertStatus(t, client, http.MethodPost, baseURL+"/action/action_doc/allowed_action", userToken, map[string]interface{}{"attributes": map[string]interface{}{}}, http.StatusOK)
		accessGroupsE2EAssertStatus(t, client, http.MethodPost, baseURL+"/action/action_doc/denied_action", userToken, map[string]interface{}{"attributes": map[string]interface{}{}}, http.StatusForbidden)
		accessGroupsE2EAssertStatus(t, client, http.MethodPost, baseURL+"/action/action_doc/allowed_action", "", map[string]interface{}{"attributes": map[string]interface{}{}}, http.StatusForbidden)
	})
}

func startAccessGroupsE2EDaptin(t *testing.T, port int, httpsPort int, baseURL string, schema string) func() {
	t.Helper()

	tmpDir := t.TempDir()
	storageDir := filepath.Join(tmpDir, "storage")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		t.Fatalf("create storage dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "schema_access_groups_e2e.yaml"), []byte(schema), 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	logs := &lockedAccessGroupsE2EBuffer{}
	cmd := exec.CommandContext(ctx, "go", "run", ".",
		"-port", fmt.Sprintf(":%d", port),
		"-https_port", fmt.Sprintf(":%d", httpsPort),
		"-db_type", "sqlite3",
		"-db_connection_string", filepath.Join(tmpDir, "daptin.db"),
		"-local_storage_path", storageDir,
		"-runtime", "test",
		"-log_level", "error",
	)
	cmd.Env = append(os.Environ(), "DAPTIN_SCHEMA_FOLDER="+tmpDir)
	cmd.Stdout = logs
	cmd.Stderr = logs
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start daptin: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			cancel()
			t.Fatalf("daptin exited before readiness: %v\n%s", err, logs.String())
		default:
		}
		resp, err := http.Get(baseURL + "/api/world")
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode < 500 {
				return func() {
					if cmd.Process != nil {
						_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
					}
					select {
					case <-done:
					case <-time.After(10 * time.Second):
						if cmd.Process != nil {
							_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
						}
						<-done
					}
					cancel()
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	cancel()
	t.Fatalf("daptin did not become ready\n%s", logs.String())
	return func() {}
}

func accessGroupsE2ESignupSigninAdmin(t *testing.T, client *http.Client, baseURL string) string {
	t.Helper()

	token := accessGroupsE2ESignupSigninUser(t, client, baseURL, "", "admin")
	accessGroupsE2EAssertStatus(t, client, http.MethodPost, baseURL+"/action/world/become_an_administrator", token, map[string]interface{}{"attributes": map[string]interface{}{}}, http.StatusOK)
	return token
}

func accessGroupsE2ESignupSigninUser(t *testing.T, client *http.Client, baseURL string, adminToken string, localPart string) string {
	t.Helper()

	email := localPart + "@test.local"
	password := "testpass123"
	accessGroupsE2EAssertStatus(t, client, http.MethodPost, baseURL+"/action/user_account/signup", adminToken, map[string]interface{}{
		"attributes": map[string]interface{}{
			"email":           email,
			"password":        password,
			"passwordConfirm": password,
			"name":            localPart,
		},
	}, http.StatusOK)

	response := accessGroupsE2ERequestJSON(t, client, http.MethodPost, baseURL+"/action/user_account/signin", "", map[string]interface{}{
		"attributes": map[string]interface{}{
			"email":    email,
			"password": password,
		},
	}, http.StatusOK)
	token, ok := accessGroupsE2EFindString(response, "value")
	if !ok || token == "" {
		t.Fatalf("signin response did not include token: %#v", response)
	}
	return token
}

func accessGroupsE2ECreateRecord(t *testing.T, client *http.Client, baseURL string, token string, entity string, attributes map[string]interface{}) string {
	t.Helper()

	response := accessGroupsE2ERequestJSON(t, client, http.MethodPost, baseURL+"/api/"+entity, token, accessGroupsE2ERecordPayload(entity, "", attributes), http.StatusCreated)
	id := accessGroupsE2EResourceID(t, response)
	if id == "" {
		t.Fatalf("create %s response did not include id: %#v", entity, response)
	}
	return id
}

func accessGroupsE2ECreateUsergroup(t *testing.T, client *http.Client, baseURL string, token string, name string) string {
	t.Helper()

	response := accessGroupsE2ERequestJSON(t, client, http.MethodPost, baseURL+"/api/usergroup", token, accessGroupsE2ERecordPayload("usergroup", "", map[string]interface{}{"name": name}), http.StatusCreated)
	id := accessGroupsE2EResourceID(t, response)
	if id == "" {
		t.Fatalf("create usergroup response did not include id: %#v", response)
	}
	return id
}

func accessGroupsE2ECreateJoin(t *testing.T, client *http.Client, baseURL string, token string, joinEntity string, attributes map[string]interface{}, permission int64) string {
	t.Helper()

	response := accessGroupsE2ERequestJSON(t, client, http.MethodPost, baseURL+"/api/"+joinEntity, token, accessGroupsE2ERecordPayload(joinEntity, "", attributes), http.StatusCreated)
	id := accessGroupsE2EResourceID(t, response)
	if id == "" {
		t.Fatalf("create %s response did not include id: %#v", joinEntity, response)
	}
	if permission != 0 {
		accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/"+joinEntity+"/"+id, token, accessGroupsE2ERecordPayload(joinEntity, id, map[string]interface{}{"permission": permission}), http.StatusOK)
	}
	return id
}

func accessGroupsE2EFindResourceID(t *testing.T, client *http.Client, baseURL string, token string, entity string, attr string, value string) string {
	t.Helper()

	query := url.Values{}
	query.Set("page[size]", "200")
	response := accessGroupsE2ERequestJSON(t, client, http.MethodGet, baseURL+"/api/"+entity+"?"+query.Encode(), token, nil, http.StatusOK)
	for _, item := range accessGroupsE2EDataArray(t, response) {
		itemMap, _ := item.(map[string]interface{})
		attributes, _ := itemMap["attributes"].(map[string]interface{})
		if attributes[attr] == value {
			if id, ok := itemMap["id"].(string); ok && id != "" {
				return id
			}
		}
	}
	t.Fatalf("%s with %s=%s not found in %#v", entity, attr, value, response)
	return ""
}

func accessGroupsE2EAssertListCount(t *testing.T, client *http.Client, baseURL string, token string, entity string, want int) {
	t.Helper()

	response := accessGroupsE2ERequestJSON(t, client, http.MethodGet, baseURL+"/api/"+entity+"?page%5Bsize%5D=100", token, nil, http.StatusOK)
	if got := len(accessGroupsE2EDataArray(t, response)); got != want {
		t.Fatalf("expected %s list count %d, got %d: %#v", entity, want, got, response)
	}
}

func accessGroupsE2EAssertStatus(t *testing.T, client *http.Client, method string, requestURL string, token string, payload interface{}, want int) {
	t.Helper()
	_ = accessGroupsE2ERequestJSON(t, client, method, requestURL, token, payload, want)
}

func accessGroupsE2ERequestJSON(t *testing.T, client *http.Client, method string, requestURL string, token string, payload interface{}, want int) interface{} {
	t.Helper()

	var body io.Reader
	if payload != nil {
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = bytes.NewReader(payloadBytes)
	}
	req, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if payload != nil {
		if strings.Contains(requestURL, "/api/") {
			req.Header.Set("Content-Type", "application/vnd.api+json")
		} else {
			req.Header.Set("Content-Type", "application/json")
		}
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, requestURL, err)
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != want {
		t.Fatalf("%s %s returned %d, want %d: %s", method, requestURL, resp.StatusCode, want, string(responseBody))
	}
	if len(bytes.TrimSpace(responseBody)) == 0 {
		return nil
	}
	var decoded interface{}
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		t.Fatalf("%s %s returned invalid JSON: %v\n%s", method, requestURL, err, string(responseBody))
	}
	return decoded
}

func accessGroupsE2ERecordPayload(entity string, id string, attributes map[string]interface{}) map[string]interface{} {
	data := map[string]interface{}{
		"type":       entity,
		"attributes": attributes,
	}
	if id != "" {
		data["id"] = id
	}
	return map[string]interface{}{"data": data}
}

func accessGroupsE2EDataArray(t *testing.T, response interface{}) []interface{} {
	t.Helper()
	responseMap, ok := response.(map[string]interface{})
	if !ok {
		t.Fatalf("response is not an object: %#v", response)
	}
	data, ok := responseMap["data"].([]interface{})
	if !ok {
		t.Fatalf("response data is not an array: %#v", response)
	}
	return data
}

func accessGroupsE2EResourceID(t *testing.T, response interface{}) string {
	t.Helper()
	responseMap, ok := response.(map[string]interface{})
	if !ok {
		t.Fatalf("response is not an object: %#v", response)
	}
	data, ok := responseMap["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("response data is not an object: %#v", response)
	}
	id, ok := data["id"].(string)
	if !ok {
		t.Fatalf("response data id is not a string: %#v", response)
	}
	return id
}

func accessGroupsE2EFindString(value interface{}, key string) (string, bool) {
	switch typed := value.(type) {
	case map[string]interface{}:
		for k, v := range typed {
			if strings.EqualFold(k, key) {
				if str, ok := v.(string); ok {
					return str, true
				}
			}
			if str, ok := accessGroupsE2EFindString(v, key); ok {
				return str, true
			}
		}
	case []interface{}:
		for _, item := range typed {
			if str, ok := accessGroupsE2EFindString(item, key); ok {
				return str, true
			}
		}
	}
	return "", false
}

func freeAccessGroupsE2EPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

type lockedAccessGroupsE2EBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedAccessGroupsE2EBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedAccessGroupsE2EBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
