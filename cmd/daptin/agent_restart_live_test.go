//go:build live

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"gopkg.in/yaml.v2"
)

// Runs real inference and real process restarts. Reports contain all provider
// bodies, but never authentication headers or provider secrets.
func TestAgentSchemaRestartLive(t *testing.T) {
	requireRealE2E(t)
	path := os.Getenv("DAPTIN_AGENT_CREDENTIALS")
	if path == "" {
		t.Skip("set DAPTIN_AGENT_CREDENTIALS")
	}
	var credentials struct {
		Providers map[string]struct {
			APIKey string `yaml:"api_key"`
		} `yaml:"providers"`
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(body, &credentials); err != nil {
		t.Fatal("decode credentials")
	}
	key := credentials.Providers["openrouter"].APIKey
	if key == "" {
		t.Fatal("openrouter credential required")
	}
	for _, mode := range []string{"single", "cluster"} {
		t.Run(mode, func(t *testing.T) {
			if only := os.Getenv("DAPTIN_AGENT_RESTART_MODE"); only != "" && mode != only {
				t.Skip("other mode")
			}
			report := map[string]interface{}{"mode": mode, "started": time.Now().UTC()}
			// Keep evidence across repetitions and failures, independently of the
			// fixture directories removed by testing.T cleanup.
			evidence, err := os.MkdirTemp("", "daptin-schema-restart-"+mode+"-")
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("retaining experiment evidence in %s", evidence)
			journal, err := os.OpenFile(filepath.Join(evidence, "http.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				t.Fatal(err)
			}
			defer journal.Close()
			recorder := &agentLiveRecorder{transport: http.DefaultTransport, secret: key, journal: journal}
			defer func() {
				if err := recorder.report(report); err != nil {
					t.Error(err)
				}
				data, _ := json.MarshalIndent(report, "", "  ")
				if err := os.WriteFile(filepath.Join(evidence, "report.json"), data, 0600); err != nil {
					t.Error(err)
				}
			}()
			target, _ := url.Parse("https://openrouter.ai/api/v1")
			proxy := httputil.NewSingleHostReverseProxy(target)
			direct := proxy.Director
			proxy.Director = func(r *http.Request) {
				r.URL.Path = strings.TrimPrefix(r.URL.Path, "/v1")
				direct(r)
				r.Host = target.Host
				r.Header.Set("Accept-Encoding", "identity")
			}
			proxy.Transport = recorder
			upstream := httptest.NewServer(proxy)
			defer upstream.Close()
			options := transportE2EDaptinOptions{connectionString: filepath.Join(t.TempDir(), "restart.db"), schema: "Tables:\n  - TableName: agbase\n    Columns:\n      - {Name: name, ColumnType: label, DataType: varchar(100)}\n"}
			if mode == "cluster" {
				options = mailProtocolDatabase(t)
				options.schema = "Tables:\n  - TableName: agbase\n    Columns:\n      - {Name: name, ColumnType: label, DataType: varchar(100)}\n"
			}
			used := map[int]bool{}
			var bases []string
			var olricAddresses []string
			var processes []*transportE2EDaptinProcess
			seed := ""
			start := func() {
				port, https := freeTransportE2EPort(t, used), freeTransportE2EPort(t, used)
				node := options
				node.olricPort = freeTransportE2EPortPair(t, used)
				node.olricPeers = seed
				olricAddresses = append(olricAddresses, fmt.Sprintf("127.0.0.1:%d", node.olricPort))
				if seed == "" {
					seed = fmt.Sprintf("127.0.0.1:%d", node.olricPort+1)
				}
				base := fmt.Sprintf("http://127.0.0.1:%d", port)
				processes = append(processes, startTransportE2EDaptin(t, port, https, base, node))
				bases = append(bases, base)
			}
			start()
			client := &http.Client{Timeout: 5 * time.Second}
			admin := transportE2ESignupSigninAdmin(t, client, bases[0])
			if mode == "cluster" {
				start()
				waitForLLME2ECluster(t, client, bases[0], bases[1], admin)
				start()
				deadline := time.Now().Add(20 * time.Second)
				for {
					var roster interface{}
					converged := true
					for _, base := range bases {
						stats := transportE2EGetJSON(t, client, base+"/statistics", admin)
						members, _ := transportE2EPath(stats, "olric.members")
						count, _ := transportE2EPath(stats, "olric.member_count")
						if count != float64(3) {
							converged = false
						}
						if roster == nil {
							roster = members
						} else if !reflect.DeepEqual(roster, members) {
							converged = false
						}
					}
					if converged {
						report["olric_members_before"] = roster
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("three Olric nodes did not converge")
					}
					time.Sleep(100 * time.Millisecond)
				}
			}
			report["nodes"] = bases
			// Prove a publication through one embedded Olric reaches subscribers
			// connected independently to each member. This does not restart nodes.
			if mode == "cluster" {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				var clients []*redis.Client
				var subscriptions []*redis.PubSub
				for _, address := range olricAddresses {
					client := redis.NewClient(&redis.Options{Addr: address})
					defer client.Close()
					sub := client.Subscribe(ctx, "agent-restart-experiment")
					defer sub.Close()
					if _, err := sub.Receive(ctx); err != nil {
						t.Fatal(err)
					}
					clients = append(clients, client)
					subscriptions = append(subscriptions, sub)
				}
				count, err := clients[0].Publish(ctx, "agent-restart-experiment", "restart-probe-only").Result()
				if err != nil {
					t.Fatal(err)
				}
				deliveries := []string{}
				for _, sub := range subscriptions {
					msg, err := sub.ReceiveMessage(ctx)
					if err != nil {
						t.Fatal(err)
					}
					deliveries = append(deliveries, msg.Payload)
				}
				report["olric_broadcast_probe"] = map[string]interface{}{"subscriber_count": count, "deliveries": deliveries}
				if count != 3 {
					t.Fatalf("broadcast reached %d subscribers, want 3", count)
				}
			}
			defer func() {
				for i, p := range processes {
					if err := os.WriteFile(filepath.Join(evidence, fmt.Sprintf("node%d.log", i)), []byte(p.logs.String()), 0600); err != nil {
						t.Error(err)
					}
				}
			}()
			createLLME2ECatalog(t, client, bases[0], admin, llmE2ECatalog{name: "restart-model", providerType: "openrouter", upstreamURL: upstream.URL, apiKey: key, upstreamModel: "openai/gpt-4.1-mini", operations: []string{"chat"}, maxConcurrency: 2, requestTimeoutMS: 90000, connectTimeoutMS: 10000, capabilities: `{"tools":true}`})
			for _, base := range bases {
				waitForLLME2EModel(t, client, base, admin, "restart-model")
			}
			// Membership and catalog visibility do not establish that distributed
			// inference protection is ready. Use the existing E2E admission probe.
			var readiness []interface{}
			for _, base := range bases {
				deadline := time.Now().Add(30 * time.Second)
				for {
					status, body := postLLMActionForStatus(t, client, base+"/v1/chat/completions", admin, map[string]interface{}{"model": "restart-model", "max_tokens": 8, "messages": []interface{}{map[string]interface{}{"role": "user", "content": "Reply READY"}}})
					readiness = append(readiness, map[string]interface{}{"node": base, "status": status, "body": string(body)})
					report["inference_readiness"] = readiness
					if status == 200 {
						break
					}
					if time.Now().After(deadline) {
						t.Fatalf("inference protection unavailable at %s: %d %s", base, status, body)
					}
					time.Sleep(250 * time.Millisecond)
				}
			}
			config := map[string]interface{}{"model": "restart-model", "instructions": "You administer this Daptin instance. Use tools to make and verify the requested change. All entity names must be at most 8 characters. Discover schemas before writing. Schema definitions persist via the world resource: create a world record with table_name and world_schema_json (a JSON string containing TableName and Columns). Example column: {\"Name\":\"name\",\"ColumnName\":\"name\",\"ColumnType\":\"label\",\"DataType\":\"varchar(100)\"}. After creating the definition, discover and call world.restart_daptin. After restart, discover the new entity and create/read a record. If a tool returns entity missing immediately after restart, use agent_wait for 10 seconds before rediscovering; do not repeat restart or recreate the schema. Keep output brief. Never claim success without read verification.", "toolset": map[string]interface{}{"resources": []string{"*"}, "operations": []string{"list", "read", "create", "update", "delete", "relations"}, "actions": []string{"world.restart_daptin"}}, "limits": map[string]interface{}{"turns": 25, "tool_calls": 40, "seconds": 300, "output_tokens": 2048, "attempts": 3}}
			definition := accessGroupsE2ECreateRecord(t, client, bases[0], admin, "agent", map[string]interface{}{"name": "restart-test", "config": string(mustAgentJSON(config))})
			response := transportE2EPostJSON(t, client, bases[0]+"/action/agent/start", admin, map[string]interface{}{"attributes": map[string]interface{}{"agent_id": definition, "messages": []interface{}{map[string]interface{}{"role": "user", "content": "Create a new entity named agnew with a name text column by persisting its schema through world, restart Daptin using its restart action, then create a record in agnew with name restart-verified and read it back. Complete this in the same agent run."}}}})
			ref, _ := transportE2EPath(response, "0.Attributes.reference_id")
			report["run"] = ref
			request := func(base, path string) (int, interface{}) {
				req, _ := http.NewRequest(http.MethodGet, base+path, nil)
				req.Header.Set("Authorization", "Bearer "+admin)
				resp, err := client.Do(req)
				if err != nil {
					return 0, err.Error()
				}
				defer resp.Body.Close()
				data, _ := io.ReadAll(resp.Body)
				var decoded interface{}
				if json.Unmarshal(data, &decoded) != nil {
					decoded = string(data)
				}
				return resp.StatusCode, decoded
			}
			var last interface{}
			deadline := time.Now().Add(320 * time.Second)
			for time.Now().Before(deadline) {
				_, last = request(bases[0], "/api/agentrun/"+ref.(string))
				status, _ := transportE2EPath(last, "data.attributes.status")
				if status == "completed" || status == "failed" || status == "unknown" || status == "waiting_decision" {
					break
				}
				time.Sleep(250 * time.Millisecond)
			}
			report["final_run"] = last
			observations := []interface{}{}
			verified := false
			var stale []string
			for _, base := range bases {
				status, body := request(base, "/api/agnew")
				observations = append(observations, map[string]interface{}{"node": base, "status": status, "body": body})
				if status == 404 {
					stale = append(stale, base)
				}
				name, _ := transportE2EPath(body, "data.0.attributes.name")
				if status == 200 && name == "restart-verified" {
					verified = true
				}
			}
			report["entity_observations"] = observations
			for _, entity := range []string{"agentstp", "agenttry"} {
				_, body := request(bases[0], "/api/"+entity+"?page%5Bsize%5D=100")
				report[entity] = body
			}
			status, _ := transportE2EPath(last, "data.attributes.status")
			state, _ := transportE2EPath(last, "data.attributes.state")
			t.Logf("mode=%s run=%s status=%v entity observations=%s", mode, ref, status, mustAgentJSON(observations))
			if status != "completed" {
				t.Errorf("agent did not complete: %s; inspect %s", mustAgentJSON(state), filepath.Join(evidence, "report.json"))
			}
			if !verified {
				t.Error("no node returned the verified record")
				return
			}
			// A separate operator intervention documents what the local-only
			// restart action leaves outstanding; it is not part of agent success.
			var rolled []interface{}
			for _, base := range stale {
				result := transportE2EPostJSON(t, client, base+"/action/world/restart_daptin", admin, map[string]interface{}{"attributes": map[string]interface{}{}})
				deadline := time.Now().Add(40 * time.Second)
				var status int
				var body interface{}
				for time.Now().Before(deadline) {
					status, body = request(base, "/api/agnew")
					if status == 200 {
						break
					}
					time.Sleep(200 * time.Millisecond)
				}
				rolled = append(rolled, map[string]interface{}{"node": base, "restart_response": result, "status": status, "body": body})
				name, _ := transportE2EPath(body, "data.0.attributes.name")
				if status != 200 || name != "restart-verified" {
					t.Errorf("node %s did not apply persisted schema after restart", base)
				}
			}
			report["operator_restarted_stale_nodes"] = rolled
		})
	}
}
