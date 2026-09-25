package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSchemaEventHandlersRealE2E(t *testing.T) {
	if !strings.EqualFold(os.Getenv("DAPTIN_REAL_E2E"), "1") {
		t.Skip("set DAPTIN_REAL_E2E=1 to run schema event handler e2e")
	}
	t.Setenv("DAPTIN_EVENT_SECRET", "event-e2e-$secret")
	called := make(chan map[string]interface{}, 4)
	release := make(chan struct{})
	asyncRelease := make(chan struct{})
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		payload["__path"] = r.URL.Path
		payload["__authorization"] = r.Header.Get("Authorization")
		called <- payload
		if r.URL.Path == "/sync" {
			<-release
		} else if r.URL.Path == "/async" {
			<-asyncRelease
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer receiver.Close()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	defer func() {
		select {
		case <-asyncRelease:
		default:
			close(asyncRelease)
		}
	}()

	schema := fmt.Sprintf(`
Tables:
  - TableName: event_probe
    IsTopLevel: true
    Permission: 2097151
    DefaultPermission: 2097151
    Columns:
      - Name: status
        DataType: varchar(100)
        ColumnType: label
      - Name: note
        DataType: varchar(100)
        ColumnType: label
      - Name: payload
        DataType: longtext
        ColumnType: content
      - Name: amount
        DataType: int(11)
        ColumnType: measurement
    EventHandlers:
      - Event: before:create
        Handler: conformation
        Attributes:
          status: pending
      - Event: before:create
        Handler: js
        Attributes:
          Script: |
            if (input.note === 'reject') { throw new Error('blocked'); }
            input.note = 'scripted';
            return input;
      - Event: before:update
        Handler: validation
        Attributes:
          Condition: "{{.old.status}} != 'cancelled'"
          Message: Cannot modify cancelled probes
      - Event: before:update
        Handler: validation
        Attributes:
          Condition: "{{.amount}} > 0"
          Message: Amount must be positive
      - Event: after:create
        Handler: action.execute
        Attributes:
          ActionName: record_event
          EntityName: event_probe
      - Event: after:create
        Handler: http.post
        Attributes:
          Url: %s/sync
          Headers:
            Authorization: "Bearer ${env.DAPTIN_EVENT_SECRET}"
          Body:
            status: "{{.status}}"
            reference_id: "{{.reference_id}}"
      - Event: after:update
        Condition: "{{.status}} == 'cancelled'"
        Handler: async.http.post
        Attributes:
          Url: %s/default
      - Event: after:delete
        Handler: async.http.post
        Attributes:
          Url: %s/async
          Body:
            status: "{{.status}}"
            reference_id: "{{.reference_id}}"
      - Event: after:delete
        Handler: action.execute
        Attributes:
          ActionName: record_deleted_event
          EntityName: event_probe
  - TableName: event_receipt
    IsTopLevel: true
    Permission: 2097151
    DefaultPermission: 2097151
    Columns:
      - Name: status
        DataType: varchar(100)
        ColumnType: label
Actions:
  - Name: record_event
    OnType: event_probe
    InstanceOptional: true
    OutFields:
      - Type: event_receipt
        Method: POST
        Attributes:
          status: observed
  - Name: record_deleted_event
    OnType: event_probe
    InstanceOptional: true
    OutFields:
      - Type: event_receipt
        Method: POST
        Attributes:
          status: deleted
`, receiver.URL, receiver.URL, receiver.URL)
	ports := map[int]bool{}
	port := freeTransportE2EPort(t, ports)
	httpsPort := freeTransportE2EPort(t, ports)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	databasePath := filepath.Join(t.TempDir(), "event-handlers.db")
	options := transportE2EDaptinOptions{schema: schema, databaseType: "sqlite3", connectionString: databasePath}
	process := startTransportE2EDaptin(t, port, httpsPort, baseURL, options)
	defer func() { process.stopProcess() }()
	client := &http.Client{Timeout: 45 * time.Second}
	token := accessGroupsE2ESignupSigninAdmin(t, client, baseURL)
	process.stopProcess()
	options.schema = "" // Exercise handlers restored from persisted world metadata.
	process = startTransportE2EDaptin(t, port, httpsPort, baseURL, options)
	accessGroupsE2EAssertStatus(t, client, http.MethodPost, baseURL+"/api/event_probe", token,
		accessGroupsE2ERecordPayload("event_probe", "", map[string]interface{}{"note": "reject"}), http.StatusBadRequest)
	select {
	case payload := <-called:
		t.Fatalf("rejected create emitted webhook: %#v", payload)
	default:
	}

	result := make(chan struct {
		status int
		body   []byte
		err    error
	}, 1)
	headersArrived := make(chan struct{}, 1)
	go func() {
		body, _ := json.Marshal(accessGroupsE2ERecordPayload("event_probe", "", map[string]interface{}{
			"payload": strings.Repeat("x", 16384), "amount": 1,
		}))
		request, _ := http.NewRequest(http.MethodPost, baseURL+"/api/event_probe", bytes.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", "application/vnd.api+json")
		response, err := client.Do(request)
		if err != nil {
			result <- struct {
				status int
				body   []byte
				err    error
			}{err: err}
			return
		}
		headersArrived <- struct{}{}
		data, _ := io.ReadAll(response.Body)
		_ = response.Body.Close()
		result <- struct {
			status int
			body   []byte
			err    error
		}{status: response.StatusCode, body: data}
	}()
	var createPayload map[string]interface{}
	select {
	case createPayload = <-called:
	case response := <-result:
		t.Fatalf("create returned before webhook: %d %v %s\n%s", response.status, response.err, response.body, process.logs.String())
	case <-time.After(10 * time.Second):
		for _, path := range []string{"/api/event_probe", "/api/data_exchange", "/api/exchange_run"} {
			request, _ := http.NewRequest(http.MethodGet, baseURL+path, nil)
			request.Header.Set("Authorization", "Bearer "+token)
			response, err := client.Do(request)
			if err != nil {
				t.Logf("GET %s: %v", path, err)
				continue
			}
			body, _ := io.ReadAll(response.Body)
			_ = response.Body.Close()
			t.Logf("GET %s: %d %s", path, response.StatusCode, body)
		}
		t.Fatalf("create webhook was not called\n%s", process.logs.String())
	}
	if createPayload["status"] != "pending" || createPayload["__path"] != "/sync" ||
		createPayload["__authorization"] != "Bearer event-e2e-$secret" {
		t.Fatalf("create webhook payload: %#v", createPayload)
	}
	select {
	case <-headersArrived:
		t.Fatal("http.post sent response headers before delivery")
	case response := <-result:
		t.Fatalf("http.post returned before delivery: %d %v %s", response.status, response.err, response.body)
	case <-time.After(300 * time.Millisecond):
	}
	close(release)
	select {
	case response := <-result:
		if response.err != nil || response.status != http.StatusCreated {
			t.Fatalf("create response: %d %v %s\n%s", response.status, response.err, response.body, process.logs.String())
		}
		var data map[string]interface{}
		if err := json.Unmarshal(response.body, &data); err != nil {
			t.Fatal(err)
		}
		id := accessGroupsE2EResourceID(t, data)
		created := data["data"].(map[string]interface{})["attributes"].(map[string]interface{})
		if created["note"] != "scripted" {
			t.Fatalf("before:create JavaScript did not conform record: %#v", created)
		}
		receiptList := accessGroupsE2ERequestJSON(t, client, http.MethodGet,
			baseURL+"/api/event_receipt", token, nil, http.StatusOK)
		receipts := accessGroupsE2EDataArray(t, receiptList)
		if len(receipts) != 1 {
			t.Fatalf("action handler created %d receipts, want one", len(receipts))
		}
		receipt := receipts[0].(map[string]interface{})["attributes"].(map[string]interface{})
		if receipt["status"] != "observed" || receipt["user_account_id"] != created["user_account_id"] {
			t.Fatalf("action handler did not use writer identity: %#v", receipt)
		}
		accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/event_probe/"+id, token,
			accessGroupsE2ERecordPayload("event_probe", id, map[string]interface{}{"amount": -1}), http.StatusBadRequest)
		accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/event_probe/"+id, token,
			accessGroupsE2ERecordPayload("event_probe", id, map[string]interface{}{"status": "cancelled"}), http.StatusOK)
		select {
		case updatePayload := <-called:
			user, _ := updatePayload["user"].(map[string]interface{})
			record, _ := updatePayload["record"].(map[string]interface{})
			if updatePayload["__path"] != "/default" || updatePayload["event"] != "after:update" ||
				updatePayload["table"] != "event_probe" || updatePayload["timestamp"] == nil ||
				record["status"] != "cancelled" || user["email"] != "admin@test.local" ||
				user["id"] != created["user_account_id"] {
				t.Fatalf("default webhook payload: %#v", updatePayload)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("update webhook was not called\n%s", process.logs.String())
		}
		accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/event_probe/"+id, token,
			accessGroupsE2ERecordPayload("event_probe", id, map[string]interface{}{"status": "changed"}), http.StatusBadRequest)
		accessGroupsE2EAssertStatus(t, client, http.MethodDelete, baseURL+"/api/event_probe/"+id, token, nil, http.StatusOK)
		select {
		case deletePayload := <-called:
			if deletePayload["__path"] != "/async" || deletePayload["status"] != "cancelled" || deletePayload["reference_id"] != id {
				t.Fatalf("delete webhook payload: %#v\n%s", deletePayload, process.logs.String())
			}
			close(asyncRelease)
			deadline := time.Now().Add(10 * time.Second)
			for time.Now().Before(deadline) {
				list := accessGroupsE2ERequestJSON(t, client, http.MethodGet,
					baseURL+"/api/event_receipt", token, nil, http.StatusOK)
				if len(accessGroupsE2EDataArray(t, list)) == 2 {
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
			t.Fatalf("delete action did not create receipt\n%s", process.logs.String())
		case <-time.After(10 * time.Second):
			t.Fatalf("delete webhook was not called\n%s", process.logs.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("http.post did not finish after delivery\n%s", process.logs.String())
	}
}
