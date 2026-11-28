package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Deterministic provider responses exercise the real resource, action,
// transaction, process replacement and shared SQL/Olric paths.
func TestAgentSchemaRestartRealE2E(t *testing.T) {
	requireRealE2E(t)
	for _, mode := range []string{"single", "cluster"} {
		for _, schemaPath := range []string{"world", "upload"} {
			t.Run(mode+"/"+schemaPath, func(t *testing.T) {
				var invalidContinuations atomic.Int64
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var request struct {
						Messages []map[string]interface{} `json:"messages"`
					}
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						http.Error(w, err.Error(), 400)
						return
					}
					function := ""
					args := map[string]interface{}{}
					lastName := ""
					lastArgs := map[string]interface{}{}
					for _, message := range request.Messages {
						if calls, ok := message["tool_calls"].([]interface{}); ok && len(calls) > 0 {
							call := calls[len(calls)-1].(map[string]interface{})["function"].(map[string]interface{})
							lastName = call["name"].(string)
							_ = json.Unmarshal([]byte(call["arguments"].(string)), &lastArgs)
						}
						if message["role"] == "tool" && strings.Contains(fmt.Sprint(message["content"]), "unknown entity") {
							invalidContinuations.Add(1)
						}
					}
					content := "done"
					if fmt.Sprint(request.Messages[0]["content"]) != "ready" {
						switch lastName {
						case "":
							function = "daptin_discover"
							args = map[string]interface{}{"entity": "world"}
						case "daptin_discover":
							if lastArgs["entity"] == "agnew" {
								function = "daptin_create"
								args = map[string]interface{}{"entity": "agnew", "attributes": map[string]interface{}{"name": "restart-verified"}}
							} else if schemaPath == "world" {
								function = "daptin_create"
								args = map[string]interface{}{"entity": "world", "attributes": map[string]interface{}{"table_name": "agnew", "world_schema_json": `{"TableName":"agnew","Columns":[{"Name":"name","ColumnName":"name","ColumnType":"label","DataType":"varchar(100)"}]}`}}
							} else {
								function = "daptin_action"
								args = map[string]interface{}{"entity": "world", "action": "upload_system_schema", "attributes": map[string]interface{}{"schema_file": []interface{}{map[string]interface{}{"name": "feature.yaml", "file": base64.StdEncoding.EncodeToString([]byte("Tables:\n  - TableName: agnew\n    Columns:\n      - {Name: name, ColumnType: label, DataType: varchar(100)}\n    Description: " + strings.Repeat("schema description ", 600)))}}}}
							}
						case "daptin_create":
							if lastArgs["entity"] == "world" {
								function = "daptin_action"
								args = map[string]interface{}{"entity": "world", "action": "restart_daptin", "attributes": map[string]interface{}{}}

							} else {
								var result map[string]interface{}
								_ = json.Unmarshal([]byte(request.Messages[len(request.Messages)-1]["content"].(string)), &result)
								function = "daptin_read"
								args = map[string]interface{}{"entity": "agnew", "reference": result["reference_id"]}
							}
						case "daptin_action":
							if lastArgs["action"] == "upload_system_schema" {
								if !strings.Contains(fmt.Sprint(request.Messages[len(request.Messages)-1]["content"]), "client.notify") {
									invalidContinuations.Add(1)
								}
								function = "daptin_action"
								args = map[string]interface{}{"entity": "world", "action": "restart_daptin", "attributes": map[string]interface{}{}}

							} else {
								function = "daptin_discover"
								args = map[string]interface{}{"entity": "agnew"}
							}
						}
					} else {
						content = "ready"
					}
					message := map[string]interface{}{"role": "assistant", "content": content}
					finish := "stop"
					if function != "" {
						data, _ := json.Marshal(args)
						message["content"] = nil
						message["tool_calls"] = []interface{}{map[string]interface{}{"id": "call-" + function + "-" + fmt.Sprint(len(request.Messages)), "type": "function", "function": map[string]interface{}{"name": function, "arguments": string(data)}}}
						finish = "tool_calls"
					}
					transportE2EWriteJSON(w, map[string]interface{}{"id": "restart-response", "object": "chat.completion", "created": 1, "model": "restart-upstream", "choices": []interface{}{map[string]interface{}{"index": 0, "message": message, "finish_reason": finish}}, "usage": map[string]interface{}{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}})
				}))
				defer upstream.Close()
				options := transportE2EDaptinOptions{connectionString: filepath.Join(t.TempDir(), "restart.db"), schema: "Tables:\n  - TableName: agbase\n    Columns:\n      - {Name: name, ColumnType: label, DataType: varchar(100)}\n"}
				if mode == "cluster" {
					schema := options.schema
					options = mailProtocolDatabase(t)
					options.schema = schema
				}
				used := map[int]bool{}
				seed := ""
				var bases []string
				var processes []*transportE2EDaptinProcess
				start := func() {
					port, https := freeTransportE2EPort(t, used), freeTransportE2EPort(t, used)
					node := options
					node.olricPort = freeTransportE2EPortPair(t, used)
					node.olricPeers = seed
					if seed == "" {
						seed = fmt.Sprintf("127.0.0.1:%d", node.olricPort+1)
					}
					base := fmt.Sprintf("http://127.0.0.1:%d", port)
					processes = append(processes, startTransportE2EDaptin(t, port, https, base, node))
					bases = append(bases, base)
				}
				start()
				client := &http.Client{Timeout: 20 * time.Second}
				admin := transportE2ESignupSigninAdmin(t, client, bases[0])
				member := accessGroupsE2ESignupSigninUser(t, client, bases[0], admin, "schema-reader")
				for _, action := range []string{"restart_daptin", "upload_system_schema"} {
					status, body := postLLMActionForStatus(t, client, bases[0]+"/action/world/"+action, member, map[string]interface{}{"attributes": map[string]interface{}{}})
					if status != http.StatusForbidden && status != http.StatusUnauthorized {
						t.Fatalf("non-administrator invoked %s: %d %s", action, status, body)
					}
				}
				if mode == "cluster" {
					start()
					waitForLLME2ECluster(t, client, bases[0], bases[1], admin)
					start()
				}
				defer func() {
					if t.Failed() {
						for i, p := range processes {
							t.Logf("node%d: %s", i, p.logs.String())
						}
					}
				}()
				createLLME2ECatalog(t, client, bases[0], admin, llmE2ECatalog{name: "restart-model", upstreamURL: upstream.URL, apiKey: "test-key", upstreamModel: "restart-upstream", operations: []string{"chat"}, maxConcurrency: 4, requestTimeoutMS: 5000, capabilities: `{"tools":true}`})
				for _, base := range bases {
					waitForLLME2EModel(t, client, base, admin, "restart-model")
					for i := 0; i < 3; i++ {
						status, body := postLLMActionForStatus(t, client, base+"/v1/chat/completions", admin, map[string]interface{}{"model": "restart-model", "max_tokens": 16, "messages": []interface{}{map[string]interface{}{"role": "user", "content": "ready"}}})
						if status != 200 {
							t.Fatalf("member inference protection: %d %s", status, body)
						}
					}
				}
				initialBoots := make([]int, len(processes))
				for i, process := range processes {
					initialBoots[i] = strings.Count(process.logs.String(), "Setting timezone")
				}
				config := map[string]interface{}{"model": "restart-model", "instructions": "Use tools.", "toolset": map[string]interface{}{"resources": []string{"*"}, "operations": []string{"create", "read"}, "actions": []string{"world.restart_daptin", "world.upload_system_schema"}}, "limits": map[string]interface{}{"turns": 10, "tool_calls": 10, "seconds": 180, "output_tokens": 256, "attempts": 3}}
				definition := accessGroupsE2ECreateRecord(t, client, bases[0], admin, "agent", map[string]interface{}{"name": "restart-test", "config": string(mustAgentJSON(config))})
				result := transportE2EPostJSON(t, client, bases[0]+"/action/agent/start", admin, map[string]interface{}{"attributes": map[string]interface{}{"agent_id": definition, "messages": []interface{}{map[string]interface{}{"role": "user", "content": "schema restart"}}}})
				ref, ok := transportE2EPath(result, "0.Attributes.reference_id")
				if !ok {
					t.Fatal(result)
				}
				get := func(base, path string) (int, map[string]interface{}) {
					req, _ := http.NewRequest(http.MethodGet, base+path, nil)
					req.Header.Set("Authorization", "Bearer "+admin)
					response, err := client.Do(req)
					if err != nil {
						return 0, nil
					}
					defer response.Body.Close()
					var body map[string]interface{}
					_ = json.NewDecoder(response.Body).Decode(&body)
					return response.StatusCode, body
				}
				deadline := time.Now().Add(90 * time.Second)
				var last map[string]interface{}
				for time.Now().Before(deadline) {
					_, last = get(bases[0], "/api/agentrun/"+ref.(string))
					status, _ := transportE2EPath(last, "data.attributes.status")
					if status == "completed" {
						break
					}
					if status == "failed" || status == "unknown" {
						t.Fatalf("run ended: %s", mustAgentJSON(last))
					}
					time.Sleep(200 * time.Millisecond)
				}
				if status, _ := transportE2EPath(last, "data.attributes.status"); status != "completed" {
					t.Fatalf("run did not complete: %s", mustAgentJSON(last))
				}
				for _, base := range bases {
					deadline := time.Now().Add(40 * time.Second)
					for {
						status, body := get(base, "/api/agnew")
						name, _ := transportE2EPath(body, "data.0.attributes.name")
						if status == 200 && name == "restart-verified" {
							break
						}
						if time.Now().After(deadline) {
							t.Fatalf("schema not applied on %s: %d %s", base, status, mustAgentJSON(body))
						}
						time.Sleep(200 * time.Millisecond)
					}
				}
				for i, process := range processes {
					boots := strings.Count(process.logs.String(), "Setting timezone")
					if boots != initialBoots[i]+1 {
						t.Fatalf("node %d boot count=%d, expected %d", i, boots, initialBoots[i]+1)
					}
				}
				if invalidContinuations.Load() != 0 {
					t.Fatalf("stale continuations=%d", invalidContinuations.Load())
				}
			})
		}
	}
}

func TestAgentRestartCancelsInferenceAtDrainDeadlineRealE2E(t *testing.T) {
	requireRealE2E(t)
	var calls atomic.Int64
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if calls.Add(1) == 1 {
			started <- struct{}{}
			select {
			case <-r.Context().Done():
			case <-release:
			}
			return
		}
		transportE2EWriteJSON(w, map[string]interface{}{"id": "resumed", "object": "chat.completion", "created": 1, "model": "drain-upstream", "choices": []interface{}{map[string]interface{}{"index": 0, "message": map[string]interface{}{"role": "assistant", "content": "resumed"}, "finish_reason": "stop"}}, "usage": map[string]interface{}{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}})
	}))
	defer func() { close(release); upstream.Close() }()
	f := startDaptinE2E(t, transportE2EDaptinOptions{connectionString: filepath.Join(t.TempDir(), "drain.db"), schema: "Tables:\n  - TableName: agbase\n    Columns:\n      - {Name: name, ColumnType: label, DataType: varchar(100)}\n"})
	admin := f.SignupAdmin(t)
	createLLME2ECatalog(t, f.Client, f.URL, admin, llmE2ECatalog{name: "drain-model", upstreamURL: upstream.URL, apiKey: "test-key", upstreamModel: "drain-upstream", operations: []string{"chat"}, maxConcurrency: 1, requestTimeoutMS: 120000, capabilities: `{"tools":true}`})
	waitForLLME2EModel(t, f.Client, f.URL, admin, "drain-model")
	config := map[string]interface{}{"model": "drain-model", "limits": map[string]interface{}{"seconds": 180, "output_tokens": 16, "attempts": 3}}
	definition := accessGroupsE2ECreateRecord(t, f.Client, f.URL, admin, "agent", map[string]interface{}{"name": "drain-test", "config": string(mustAgentJSON(config))})
	response := transportE2EPostJSON(t, f.Client, f.URL+"/action/agent/start", admin, map[string]interface{}{"attributes": map[string]interface{}{"agent_id": definition, "messages": []interface{}{map[string]interface{}{"role": "user", "content": "Finish when ready"}}}})
	ref, ok := transportE2EPath(response, "0.Attributes.reference_id")
	if !ok {
		t.Fatal(response)
	}
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatalf("inference did not start: %s", mustAgentJSON(transportE2EGetJSON(t, f.Client, f.URL+"/api/agentrun/"+ref.(string), admin)))
	}
	transportE2EPostJSON(t, f.Client, f.URL+"/action/world/restart_daptin", admin, map[string]interface{}{"attributes": map[string]interface{}{}})
	deadline := time.Now().Add(100 * time.Second)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest(http.MethodGet, f.URL+"/api/agentrun/"+ref.(string), nil)
		req.Header.Set("Authorization", "Bearer "+admin)
		response, err := f.Client.Do(req)
		if err == nil {
			var body map[string]interface{}
			_ = json.NewDecoder(response.Body).Decode(&body)
			response.Body.Close()
			status, _ := transportE2EPath(body, "data.attributes.status")
			if status == "completed" {
				if calls.Load() != 2 {
					t.Fatalf("inference attempts=%d, want 2", calls.Load())
				}
				return
			}
			if status == "failed" || status == "unknown" {
				t.Fatalf("run did not recover: %s\n%s", mustAgentJSON(body), f.Process.logs.String())
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("process did not restart and resume after drain deadline\n%s", f.Process.logs.String())
}
