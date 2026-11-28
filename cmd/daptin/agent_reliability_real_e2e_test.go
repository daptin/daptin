package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/buraksezer/olric"
	"github.com/go-redis/redis/v8"
	"github.com/shirou/gopsutil/v4/process"
)

// All writes in this suite enter through Daptin's HTTP resource/action paths.
func reliabilityDatabase(t *testing.T, backend string) transportE2EDaptinOptions {
	t.Helper()
	switch backend {
	case "sqlite":
		return transportE2EDaptinOptions{}
	case "postgres":
		return mailProtocolDatabase(t)
	case "mysql":
		container := os.Getenv("DAPTIN_RELIABILITY_MYSQL_CONTAINER")
		if container == "" {
			t.Skip("set DAPTIN_RELIABILITY_MYSQL_CONTAINER for disposable MariaDB coverage")
		}
		name := fmt.Sprintf("rel_%d", time.Now().UnixNano())
		dockerRestartCommand(t, "exec", container, "mariadb", "-uroot", "-ptest-only", "-e", "CREATE DATABASE "+name)
		port := dockerRestartCommand(t, "port", container, "3306")
		return transportE2EDaptinOptions{databaseType: "mysql", connectionString: "root:test-only@tcp(" + strings.TrimSpace(port) + ")/" + name + "?parseTime=true"}
	default:
		t.Fatalf("unknown test backend %s", backend)
		return transportE2EDaptinOptions{}
	}
}

func reliabilityOutput(t *testing.T) string {
	t.Helper()
	root := os.Getenv("DAPTIN_RELIABILITY_RESULTS")
	if root == "" {
		t.Skip("set DAPTIN_RELIABILITY_RESULTS to retain reliability evidence")
	}
	dir := filepath.Join(root, strings.ReplaceAll(t.Name(), "/", "_"))
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}
func reliabilitySave(t *testing.T, dir, name string, data interface{}) {
	t.Helper()
	body, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), body, 0600); err != nil {
		t.Fatal(err)
	}
}
func reliabilityHTTP(client *http.Client, method, url, token string, value interface{}) (int, []byte, error) {
	var body []byte
	if value != nil {
		body, _ = json.Marshal(value)
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err = io.ReadAll(resp.Body)
	return resp.StatusCode, body, err
}
func reliabilityWait(t *testing.T, f func() bool, description string) {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		if f() {
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("timed out: %s", description)
}
func reliabilityUpload(body []byte) map[string]interface{} {
	return map[string]interface{}{"attributes": map[string]interface{}{"schema_file": []interface{}{map[string]interface{}{"name": "reliability.json", "file": base64.StdEncoding.EncodeToString(body)}}}}
}

func TestSchemaUploadTransactionRollbackRealE2E(t *testing.T) {
	requireRealE2E(t)
	for _, backend := range []string{"sqlite", "postgres", "mysql"} {
		t.Run(backend, func(t *testing.T) {
			dir := reliabilityOutput(t)
			f := startDaptinE2E(t, reliabilityDatabase(t, backend))
			token := f.SignupAdmin(t)
			t.Cleanup(func() { _ = os.WriteFile(filepath.Join(dir, "node.log"), []byte(f.Process.logs.String()), 0600) })
			good := mustAgentJSON(map[string]interface{}{"Tables": []interface{}{map[string]interface{}{"TableName": "rlback", "Columns": []interface{}{map[string]interface{}{"Name": "name", "DataType": "varchar(100)", "ColumnType": "label"}}}}, "Actions": []interface{}{map[string]interface{}{"Name": "rollback_probe", "OnType": "world", "InstanceOptional": true, "OutFields": []interface{}{map[string]interface{}{"Type": "probe", "Method": "ACTIONRESPONSE"}}}}})
			files := []interface{}{map[string]interface{}{"name": "good.json", "file": base64.StdEncoding.EncodeToString(good)}, map[string]interface{}{"name": "bad.json", "file": base64.StdEncoding.EncodeToString([]byte(`{"Imports":[{"Entity":"rlback","FilePath":""}]}`))}}
			status, body, err := reliabilityHTTP(f.Client, http.MethodPost, f.URL+"/action/world/upload_system_schema", token, map[string]interface{}{"attributes": map[string]interface{}{"schema_file": files}})
			reliabilitySave(t, dir, "rejected.json", map[string]interface{}{"status": status, "body": string(body), "error": fmt.Sprint(err)})
			if err != nil || status < 400 {
				t.Fatalf("invalid second file not rejected: %d %s %v", status, body, err)
			}
			for _, entity := range []string{"world", "action"} {
				response := transportE2EGetJSON(t, f.Client, f.URL+"/api/"+entity+"?page[size]=1000", token)
				reliabilitySave(t, dir, entity+"-after.json", response)
				if bytes.Contains(mustAgentJSON(response), []byte("rlback")) || bytes.Contains(mustAgentJSON(response), []byte("rollback_probe")) {
					t.Fatalf("partial %s installation escaped rollback", entity)
				}
			}
			// A later valid upload must succeed after the failed transaction.
			transportE2EPostJSON(t, f.Client, f.URL+"/action/world/upload_system_schema", token, reliabilityUpload(good))
			_ = accessGroupsE2EFindResourceID(t, f.Client, f.URL, token, "world", "table_name", "rlback")
		})
	}
}

type reliabilityCluster struct {
	nodes          []*transportE2EDaptinProcess
	bases          []string
	cacheAddresses []string
	client         *http.Client
	token          string
	dir            string
}

func startReliabilityCluster(t *testing.T, schema string) *reliabilityCluster {
	t.Helper()
	c := &reliabilityCluster{dir: reliabilityOutput(t), client: &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{MaxIdleConns: 128, MaxIdleConnsPerHost: 32}}}
	options := mailProtocolDatabase(t)
	options.schema = schema
	ports := map[int]bool{}
	seed := ""
	for i := 0; i < 3; i++ {
		port := freeTransportE2EPort(t, ports)
		node := options
		node.olricPort = freeTransportE2EPortPair(t, ports)
		node.olricPeers = seed
		if seed == "" {
			seed = fmt.Sprintf("127.0.0.1:%d", node.olricPort+1)
		}
		base := fmt.Sprintf("http://127.0.0.1:%d", port)
		c.nodes = append(c.nodes, startTransportE2EDaptin(t, port, freeTransportE2EPort(t, ports), base, node))
		c.bases = append(c.bases, base)
		c.cacheAddresses = append(c.cacheAddresses, fmt.Sprintf("127.0.0.1:%d", node.olricPort))
		if i == 0 {
			c.token = transportE2ESignupSigninAdmin(t, c.client, base)
		}
		if i == 1 {
			waitForLLME2ECluster(t, c.client, c.bases[0], base, c.token)
		}
	}
	t.Cleanup(func() {
		c.client.CloseIdleConnections()
		for i, addr := range c.cacheAddresses {
			client := redis.NewClient(&redis.Options{Addr: addr, ReadTimeout: time.Second, WriteTimeout: time.Second, MaxRetries: -1})
			var metadata []map[string]interface{}
			for _, command := range []string{"cluster.members", "cluster.routingtable"} {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				value, err := client.Do(ctx, command).Result()
				cancel()
				metadata = append(metadata, map[string]interface{}{"command": command, "response": value, "error": fmt.Sprint(err)})
			}
			_ = client.Close()
			reliabilitySave(t, c.dir, fmt.Sprintf("cache-metadata-%d.json", i), metadata)
		}
		for i, node := range c.nodes {
			_ = os.WriteFile(filepath.Join(c.dir, fmt.Sprintf("node%d.log", i)), []byte(node.logs.String()), 0600)
		}
	})
	return c
}
func (c *reliabilityCluster) ready(t *testing.T, expectedBoots []int) {
	t.Helper()
	var last []map[string]interface{}
	defer func() { reliabilitySave(t, c.dir, fmt.Sprintf("readiness-%d.json", time.Now().UnixNano()), last) }()
	reliabilityWait(t, func() bool {
		ready := true
		last = nil
		for i, base := range c.bases {
			status, body, err := reliabilityHTTP(c.client, http.MethodGet, base+"/ready", c.token, nil)
			boots := strings.Count(c.nodes[i].logs.String(), "GitCommit:")
			state := map[string]interface{}{"node": i, "status": status, "body": string(body), "error": fmt.Sprint(err), "boots": boots, "expected_boots": expectedBoots}
			if err != nil || status != 200 {
				ready = false
				last = append(last, state)
				continue
			}
			if expectedBoots != nil && boots != expectedBoots[i] {
				ready = false
			}
			status, body, err = reliabilityHTTP(c.client, http.MethodGet, base+"/statistics", c.token, nil)
			var stats map[string]interface{}
			_ = json.Unmarshal(body, &stats)
			count, _ := transportE2EPath(stats, "olric.member_count")
			state["olric"] = stats["olric"]
			state["statistics_status"] = status
			state["statistics_error"] = fmt.Sprint(err)
			if err != nil || status != 200 || count != float64(3) {
				ready = false
			}
			last = append(last, state)
		}
		return ready
	}, "all three nodes ready with the expected boot counts")
}
func (c *reliabilityCluster) restartProcesses(t *testing.T, node int) float64 {
	t.Helper()
	boots := make([]int, 3)
	for i, p := range c.nodes {
		boots[i] = strings.Count(p.logs.String(), "GitCommit:") + 1
	}
	start := time.Now()
	status, body, err := reliabilityHTTP(c.client, http.MethodPost, c.bases[node]+"/action/world/restart_daptin", c.token, map[string]interface{}{"attributes": map[string]interface{}{}})
	if err != nil || status != 200 {
		t.Fatalf("restart: %d %s %v", status, body, err)
	}
	reliabilityWait(t, func() bool {
		for i, base := range c.bases {
			status, _, err := reliabilityHTTP(c.client, http.MethodGet, base+"/ready", c.token, nil)
			if err != nil || status != 200 || strings.Count(c.nodes[i].logs.String(), "GitCommit:") != boots[i] {
				return false
			}
		}
		return true
	}, "all processes replaced exactly once")
	return time.Since(start).Seconds()
}
func (c *reliabilityCluster) restart(t *testing.T, node int) float64 {
	started := time.Now()
	c.restartProcesses(t, node)
	c.ready(t, nil)
	return time.Since(started).Seconds()
}
func (c *reliabilityCluster) observeMembership(t *testing.T, cycle int) bool {
	deadline := time.Now().Add(10 * time.Second)
	var states []map[string]interface{}
	for {
		states = nil
		healthy := true
		var roster interface{}
		var coordinator interface{}
		for i, base := range c.bases {
			status, body, err := reliabilityHTTP(c.client, http.MethodGet, base+"/statistics", c.token, nil)
			var stats map[string]interface{}
			_ = json.Unmarshal(body, &stats)
			count, _ := transportE2EPath(stats, "olric.member_count")
			members, _ := transportE2EPath(stats, "olric.members")
			leader, _ := transportE2EPath(stats, "olric.coordinator")
			missing, _ := transportE2EPath(stats, "olric.partitions.without_primary")
			routed, _ := transportE2EPath(stats, "olric.routing_available")
			if i == 0 {
				roster = members
				coordinator = leader
			} else if !reflect.DeepEqual(roster, members) || coordinator != leader {
				healthy = false
			}
			if missing != float64(0) || routed != true {
				healthy = false
			}
			states = append(states, map[string]interface{}{"node": i, "status": status, "error": fmt.Sprint(err), "olric": stats["olric"]})
			if err != nil || status != 200 || count != float64(3) {
				healthy = false
			}
		}
		if healthy {
			healthy = c.routingConverged(t, cycle)
		}
		if healthy || time.Now().After(deadline) {
			reliabilitySave(t, c.dir, fmt.Sprintf("membership-%02d.json", cycle), states)
			return healthy
		}
		time.Sleep(150 * time.Millisecond)
	}
}

// Membership can agree before every member has accepted the latest ownership
// table. Keep that convergence window inside the measured restart window.
func (c *reliabilityCluster) routingConverged(t *testing.T, cycle int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	actual := make(map[uint64]string)
	var expected olric.RoutingTable
	var snapshots []interface{}
	defer func() { reliabilitySave(t, c.dir, fmt.Sprintf("routing-%02d.json", cycle), snapshots) }()
	for _, addr := range c.cacheAddresses {
		client, err := olric.NewClusterClientContext(ctx, []string{addr}, olric.WithRoutingTableFetchInterval(time.Hour))
		if err != nil {
			snapshots = append(snapshots, map[string]interface{}{"address": addr, "error": err.Error()})
			return false
		}
		local, statsErr := client.Stats(ctx, addr)
		routing, routingErr := client.RoutingTable(ctx)
		_ = client.Close(context.Background())
		snapshots = append(snapshots, map[string]interface{}{"address": addr, "stats": local, "routing": routing, "stats_error": fmt.Sprint(statsErr), "routing_error": fmt.Sprint(routingErr)})
		if statsErr != nil || routingErr != nil {
			return false
		}
		if expected == nil {
			expected = routing
		} else if !reflect.DeepEqual(expected, routing) {
			return false
		}
		for id, part := range local.Partitions {
			previous := false
			for _, owner := range part.PreviousOwners {
				if owner.ID == local.Member.ID {
					previous = true
					break
				}
			}
			if previous {
				continue
			}
			if _, exists := actual[uint64(id)]; exists {
				return false
			}
			actual[uint64(id)] = local.Member.Name
		}
	}
	if len(actual) != len(expected) {
		return false
	}
	for id, route := range expected {
		if len(route.PrimaryOwners) == 0 || actual[id] != route.PrimaryOwners[len(route.PrimaryOwners)-1] {
			return false
		}
	}
	return true
}
func (c *reliabilityCluster) runStatus(t *testing.T, ref, desired string) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	reliabilityWait(t, func() bool {
		code, body, err := reliabilityHTTP(c.client, http.MethodGet, c.bases[0]+"/api/agentrun/"+ref, c.token, nil)
		if err != nil || code != 200 {
			return false
		}
		_ = json.Unmarshal(body, &result)
		status, _ := transportE2EPath(result, "data.attributes.status")
		if status == "failed" || status == "unknown" || status == "cancelled" {
			t.Fatalf("run %s ended before %s: %s", ref, desired, body)
		}
		return status == desired
	}, "agent "+ref+" status "+desired)
	return result
}
func (c *reliabilityCluster) sample(t *testing.T, cycle int) map[string]interface{} {
	t.Helper()
	nodes := make([]map[string]interface{}, 3)
	for i, node := range c.nodes {
		p, err := process.NewProcess(int32(node.processGroupID))
		if err != nil {
			t.Fatal(err)
		}
		mem, _ := p.MemoryInfo()
		threads, _ := p.NumThreads()
		conns, _ := p.Connections()
		files, _ := p.OpenFiles()
		stats := transportE2EGetJSON(t, c.client, c.bases[i]+"/statistics", c.token).(map[string]interface{})
		nodes[i] = map[string]interface{}{"pid": node.processGroupID, "memory": mem, "threads": threads, "network_connections": len(conns), "open_files": len(files), "db": stats["db"], "olric": stats["olric"]}
	}
	return map[string]interface{}{"cycle": cycle, "time": time.Now().UTC(), "nodes": nodes}
}

func TestAgentClusterReliabilityRealE2E(t *testing.T) {
	requireRealE2E(t)
	if os.Getenv("DAPTIN_RELIABILITY_SOAK") != "1" {
		t.Skip("set DAPTIN_RELIABILITY_SOAK=1 for the 50-cycle cluster soak")
	}
	// Invariant: each accepted schema/restart request produces one process boot
	// per node and one durable agent effect, while normal service remains usable.
	var providerMu sync.Mutex
	var turns []map[string]interface{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var request struct {
			Messages []map[string]interface{} `json:"messages"`
		}
		_ = json.Unmarshal(raw, &request)
		mode, last, ref := "", "", ""
		for _, m := range request.Messages {
			if m["role"] == "user" {
				if text, ok := m["content"].(string); ok {
					mode = text
				}
			}
			if calls, ok := m["tool_calls"].([]interface{}); ok && len(calls) > 0 {
				last = fmt.Sprint(calls[0].(map[string]interface{})["function"].(map[string]interface{})["name"])
			}
			if m["role"] == "tool" {
				var result map[string]interface{}
				_ = json.Unmarshal([]byte(fmt.Sprint(m["content"])), &result)
				if id, ok := result["reference_id"].(string); ok {
					ref = id
				}
			}
		}
		function := ""
		args := map[string]interface{}{}
		if strings.HasPrefix(mode, "cycle-") {
			switch last {
			case "":
				function = "daptin_create"
				args = map[string]interface{}{"entity": "rlitem", "attributes": map[string]interface{}{"name": mode}}
			case "daptin_create":
				function = "agent_decision"
				args = map[string]interface{}{"question": "Continue after restart?"}
			case "agent_decision":
				function = "daptin_read"
				args = map[string]interface{}{"entity": "rlitem", "reference": ref}
			}
		}
		message := map[string]interface{}{"role": "assistant", "content": "done"}
		finish := "stop"
		if function != "" {
			message["content"] = nil
			message["tool_calls"] = []interface{}{map[string]interface{}{"id": "call-" + function, "type": "function", "function": map[string]interface{}{"name": function, "arguments": string(mustAgentJSON(args))}}}
			finish = "tool_calls"
		}
		response := map[string]interface{}{"id": "reliability", "object": "chat.completion", "model": "reliability", "choices": []interface{}{map[string]interface{}{"index": 0, "message": message, "finish_reason": finish}}, "usage": map[string]interface{}{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}}
		providerMu.Lock()
		turns = append(turns, map[string]interface{}{"request": json.RawMessage(raw), "response": response})
		providerMu.Unlock()
		transportE2EWriteJSON(w, response)
	}))
	defer upstream.Close()
	c := startReliabilityCluster(t, "Tables:\n  - TableName: rlitem\n    Columns:\n      - {Name: name, ColumnType: label, DataType: varchar(100), IsUnique: true}\nActions:\n  - Name: ping\n    OnType: rlitem\n    InstanceOptional: true\n    OutFields:\n      - {Type: pong, Method: ACTIONRESPONSE}\n")
	c.ready(t, nil)
	ftpE2ESetConfig(t, c.client, c.bases[0], c.token, "limit.rate", `{"version":"1","limits":{"/api/rlitem":1000000,"/action/rlitem/ping":1000000,"/v1/chat/completions":1000000}}`)
	c.restartProcesses(t, 0)
	if !c.observeMembership(t, 0) {
		t.Errorf("setup restart split Olric membership")
	}
	createLLME2ECatalog(t, c.client, c.bases[0], c.token, llmE2ECatalog{name: "reliability", upstreamURL: upstream.URL, apiKey: "test-key", upstreamModel: "reliability", operations: []string{"chat"}, maxConcurrency: 16, requestTimeoutMS: 10000, capabilities: `{"tools":true}`})
	config := map[string]interface{}{"model": "reliability", "instructions": "Use tools.", "toolset": map[string]interface{}{"resources": []string{"rlitem"}, "operations": []string{"create", "read"}}, "limits": map[string]interface{}{"turns": 10, "tool_calls": 10, "seconds": 180, "output_tokens": 128, "attempts": 3}}
	definition := accessGroupsE2ECreateRecord(t, c.client, c.bases[0], c.token, "agent", map[string]interface{}{"name": "reliability", "config": string(mustAgentJSON(config))})
	var probes []map[string]interface{}
	t.Cleanup(func() { reliabilitySave(t, c.dir, "model-probes.json", probes) })
	var cycles []map[string]interface{}
	var samples []map[string]interface{}
	t.Cleanup(func() {
		reliabilitySave(t, c.dir, "cycles.json", cycles)
		reliabilitySave(t, c.dir, "resources.json", samples)
		providerMu.Lock()
		defer providerMu.Unlock()
		reliabilitySave(t, c.dir, "provider-turns.json", turns)
	})
	samples = append(samples, c.sample(t, 0))
	c.checkCoordination(t, 0)
	t.Cleanup(func() { c.archiveRuns(t); c.archiveSchema(t) })
	pids := []int{c.nodes[0].processGroupID, c.nodes[1].processGroupID, c.nodes[2].processGroupID}
	var restarting atomic.Bool
	var backgroundOK, backgroundRestartErrors, backgroundUnexpected atomic.Int64
	stopLoad := make(chan struct{})
	var background sync.WaitGroup
	var backgroundMu sync.Mutex
	var backgroundErrors []map[string]interface{}
	for worker := 0; worker < 2; worker++ {
		background.Add(1)
		go func(worker int) {
			defer background.Done()
			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stopLoad:
					return
				case <-ticker.C:
					during := restarting.Load()
					status, body, err := reliabilityHTTP(c.client, http.MethodGet, c.bases[worker%3]+"/api/rlitem", c.token, nil)
					if err != nil || status != 200 {
						backgroundMu.Lock()
						backgroundErrors = append(backgroundErrors, map[string]interface{}{"node": worker % 3, "status": status, "body": string(body), "error": fmt.Sprint(err), "restart_window": during || restarting.Load()})
						backgroundMu.Unlock()
					}
					if err == nil && status == 200 {
						backgroundOK.Add(1)
					} else if during || restarting.Load() {
						backgroundRestartErrors.Add(1)
					} else {
						backgroundUnexpected.Add(1)
					}
				}
			}
		}(worker)
	}
	defer func() {
		close(stopLoad)
		background.Wait()
		reliabilitySave(t, c.dir, "background-load.json", map[string]interface{}{"successes": backgroundOK.Load(), "restart_window_errors": backgroundRestartErrors.Load(), "unexpected_errors": backgroundUnexpected.Load(), "errors": backgroundErrors})
	}()
	for cycle := 1; cycle <= 50; cycle++ {
		base := c.bases[(cycle-1)%3]
		// Wait for model routing after previous process replacement.
		reliabilityWait(t, func() bool {
			status, body, err := reliabilityHTTP(c.client, http.MethodPost, base+"/v1/chat/completions", c.token, map[string]interface{}{"model": "reliability", "max_tokens": 16, "messages": []interface{}{map[string]interface{}{"role": "user", "content": "ready"}}})
			probes = append(probes, map[string]interface{}{"cycle": cycle, "node": (cycle - 1) % 3, "status": status, "body": string(body), "error": fmt.Sprint(err)})
			return err == nil && status == 200
		}, "model ready")
		start := transportE2EPostJSON(t, c.client, base+"/action/agent/start", c.token, map[string]interface{}{"attributes": map[string]interface{}{"agent_id": definition, "messages": []interface{}{map[string]interface{}{"role": "user", "content": fmt.Sprintf("cycle-%02d", cycle)}}}})
		value, _ := transportE2EPath(start, "0.Attributes.reference_id")
		ref := value.(string)
		c.runStatus(t, ref, "waiting_decision")
		column := fmt.Sprintf("c%02d", cycle)
		schema := mustAgentJSON(map[string]interface{}{"Tables": []interface{}{map[string]interface{}{"TableName": "rlitem", "Columns": []interface{}{map[string]interface{}{"Name": column, "ColumnType": "label", "DataType": "varchar(100)", "IsNullable": true}}}}})
		restarting.Store(true)
		transportE2EPostJSON(t, c.client, base+"/action/world/upload_system_schema", c.token, reliabilityUpload(schema))
		elapsed := c.restartProcesses(t, (cycle-1)%3)
		converged := c.observeMembership(t, cycle)
		if !converged {
			t.Errorf("cycle %d: cluster coordination unavailable after restart; continuing observation", cycle)
		}
		restarting.Store(false)
		transportE2EPostJSON(t, c.client, c.bases[cycle%3]+"/action/agentrun/decide", c.token, map[string]interface{}{"attributes": map[string]interface{}{"agentrun_id": ref, "decision": map[string]interface{}{"approved": true}}})
		run := c.runStatus(t, ref, "completed")
		reliabilitySave(t, c.dir, fmt.Sprintf("run-%02d.json", cycle), run)
		calls, _ := transportE2EPath(run, "data.attributes.state.calls")
		accepted, _ := transportE2EPath(run, "data.attributes.state.accepted")
		if calls != float64(3) || accepted != float64(7) {
			t.Fatalf("run repeated or lost logical steps: %s", mustAgentJSON(run))
		}
		c.checkCoordination(t, cycle)
		claim, _ := transportE2EPath(run, "data.attributes.claim")
		if claim != nil {
			t.Fatalf("completed run retained claim: %s", mustAgentJSON(run))
		}
		proof := accessGroupsE2ECreateRecord(t, c.client, base, c.token, "rlitem", map[string]interface{}{"name": fmt.Sprintf("proof-%d", cycle), column: "present"})
		for _, node := range c.bases {
			row := transportE2EGetJSON(t, c.client, node+"/api/rlitem/"+proof, c.token)
			got, _ := transportE2EPath(row, "data.attributes."+column)
			if got != "present" {
				t.Fatalf("node %s missing schema %s", node, column)
			}
		}
		accessGroupsE2EAssertStatus(t, c.client, http.MethodDelete, base+"/api/rlitem/"+proof, c.token, nil, 200)
		rows := transportE2EGetJSON(t, c.client, base+"/api/rlitem?page[size]=100", c.token)
		if len(accessGroupsE2EDataArray(t, rows)) != cycle {
			t.Fatalf("cycle %d: duplicate or missing agent effects: %s", cycle, mustAgentJSON(rows))
		}
		var next int
		var mu sync.Mutex
		load := measurePermissionLoad(t, "mixed", cycle, 4, 2*time.Second, pids, func() error {
			mu.Lock()
			n := next
			next++
			mu.Unlock()
			method, path := http.MethodGet, "/api/rlitem?page[size]=10"
			var payload interface{}
			if n%2 == 1 {
				method = http.MethodPost
				path = "/action/rlitem/ping"
				payload = map[string]interface{}{"attributes": map[string]interface{}{}}
			}
			status, body, err := reliabilityHTTP(c.client, method, c.bases[n%3]+path, c.token, payload)
			if err != nil {
				return err
			}
			if status != 200 {
				return fmt.Errorf("HTTP %d: %.200s", status, body)
			}
			return nil
		})
		cycles = append(cycles, map[string]interface{}{"cycle": cycle, "restart_seconds": elapsed, "membership_converged": converged, "run": ref, "load": load})
		if load.Failures > 0 || backgroundUnexpected.Load() > 0 {
			t.Fatalf("stable mixed load failed: %+v", load)
		}
		samples = append(samples, c.sample(t, cycle))
		reliabilitySave(t, c.dir, "cycles.json", cycles)
		reliabilitySave(t, c.dir, "resources.json", samples)
		t.Logf("cycle=%d/50 restart=%.2fs requests=%d errors=%d cpu=%.1f%% rss=%.1fMiB", cycle, elapsed, load.Requests, load.Failures, load.CPUPercent, load.RSSMeanMiB)
	}
}

func TestConcurrentSchemaUploadsRealE2E(t *testing.T) {
	requireRealE2E(t)
	c := startReliabilityCluster(t, "Tables:\n  - TableName: rlmerge\n    Columns:\n      - {Name: name, ColumnType: label, DataType: varchar(100)}\n")
	c.ready(t, nil)
	var results [3]map[string]interface{}
	gate := make(chan struct{})
	var workers sync.WaitGroup
	for i := 0; i < 3; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			<-gate
			table := fmt.Sprintf("rlnew%d", i)
			column := fmt.Sprintf("col%d", i)
			schema := mustAgentJSON(map[string]interface{}{"Tables": []interface{}{map[string]interface{}{"TableName": table, "Columns": []interface{}{map[string]interface{}{"Name": "name", "ColumnType": "label", "DataType": "varchar(100)"}}}, map[string]interface{}{"TableName": "rlmerge", "Columns": []interface{}{map[string]interface{}{"Name": column, "ColumnType": "label", "DataType": "varchar(100)", "IsNullable": true}}}}, "Relations": []interface{}{map[string]interface{}{"Subject": table, "SubjectName": table + "_id", "Object": "rlmerge", "ObjectName": "rlmerge_id", "Relation": "belongs_to"}}, "Actions": []interface{}{map[string]interface{}{"Name": fmt.Sprintf("parallel_%d", i), "OnType": "rlmerge", "InstanceOptional": true, "OutFields": []interface{}{map[string]interface{}{"Type": "parallel.result", "Method": "ACTIONRESPONSE", "Attributes": map[string]interface{}{"node": i}}}}}})
			status, body, err := reliabilityHTTP(c.client, http.MethodPost, c.bases[i]+"/action/world/upload_system_schema", c.token, reliabilityUpload(schema))
			results[i] = map[string]interface{}{"status": status, "body": string(body), "error": fmt.Sprint(err)}
		}(i)
	}
	close(gate)
	workers.Wait()
	reliabilitySave(t, c.dir, "concurrent-responses.json", results)
	for _, r := range results {
		if r["status"] != 200 {
			t.Fatalf("concurrent upload failed: %s", mustAgentJSON(results))
		}
	}
	id := accessGroupsE2EFindResourceID(t, c.client, c.bases[0], c.token, "world", "table_name", "rlmerge")
	world := transportE2EGetJSON(t, c.client, c.bases[0]+"/api/world/"+id, c.token)
	reliabilitySave(t, c.dir, "merged-world.json", world)
	for i := 0; i < 3; i++ {
		if !bytes.Contains(mustAgentJSON(world), []byte(fmt.Sprintf(`"ColumnName":"col%d"`, i))) {
			t.Errorf("concurrent upload lost column col%d", i)
		}
	}
	if t.Failed() {
		return
	}
	before := mustAgentJSON(world)
	conflict := []byte(`{"Tables":[{"TableName":"rlmerge","Columns":[{"Name":"col0"},{"Name":"col0"}]}]}`)
	status, body, err := reliabilityHTTP(c.client, http.MethodPost, c.bases[1]+"/action/world/upload_system_schema", c.token, reliabilityUpload(conflict))
	reliabilitySave(t, c.dir, "conflict.json", map[string]interface{}{"status": status, "body": string(body), "error": fmt.Sprint(err)})
	if err != nil || status < 400 {
		t.Fatalf("conflicting columns accepted: %d %s %v", status, body, err)
	}
	after := transportE2EGetJSON(t, c.client, c.bases[0]+"/api/world/"+id, c.token)
	if !bytes.Equal(before, mustAgentJSON(after)) {
		t.Fatal("rejected upload mutated the merged definition")
	}
	c.restartProcesses(t, 2)
	if !c.observeMembership(t, 1) {
		t.Errorf("Olric membership split after concurrent schema restart; checking persisted results independently")
	}
	ref := accessGroupsE2ECreateRecord(t, c.client, c.bases[0], c.token, "rlmerge", map[string]interface{}{"name": "merged", "col0": "a", "col1": "b", "col2": "c"})
	for n := 0; n < 3; n++ {
		accessGroupsE2ECreateRecord(t, c.client, c.bases[n], c.token, fmt.Sprintf("rlnew%d", n), map[string]interface{}{"name": "related", "rlmerge_id": ref})
	}
	for i, base := range c.bases {
		row := transportE2EGetJSON(t, c.client, base+"/api/rlmerge/"+ref, c.token)
		for col, want := range map[string]string{"col0": "a", "col1": "b", "col2": "c"} {
			v, _ := transportE2EPath(row, "data.attributes."+col)
			if v != want {
				t.Errorf("node %d lost %s", i, col)
			}
		}
		for n := 0; n < 3; n++ {
			transportE2EPostJSON(t, c.client, base+fmt.Sprintf("/action/rlmerge/parallel_%d", n), c.token, map[string]interface{}{"attributes": map[string]interface{}{}})
			related := transportE2EGetJSON(t, c.client, base+fmt.Sprintf("/api/rlnew%d", n), c.token)
			got, _ := transportE2EPath(related, "data.0.attributes.rlmerge_id")
			if got != ref {
				t.Errorf("node %d lost relation for table %d: %s", i, n, mustAgentJSON(related))
			}
		}
	}
}

func TestActiveAgentActionBatchDrainRealE2E(t *testing.T) {
	requireRealE2E(t)
	started := map[string]chan struct{}{"agent-slow": make(chan struct{}, 10), "batch-slow": make(chan struct{}, 10), "effect": make(chan struct{}, 10)}
	release := make(chan struct{})
	var releaseOnce sync.Once
	var mu sync.Mutex
	counts := map[string]int{}
	var exchanges []map[string]interface{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/effect" {
			mu.Lock()
			counts["effect"]++
			mu.Unlock()
			started["effect"] <- struct{}{}
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
			transportE2EWriteJSON(w, map[string]interface{}{"sent": true})
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var request struct {
			Messages []map[string]interface{} `json:"messages"`
		}
		_ = json.Unmarshal(raw, &request)
		mode := ""
		hasTool := false
		for _, m := range request.Messages {
			if m["role"] == "user" {
				mode = fmt.Sprint(m["content"])
			}
			if m["role"] == "tool" {
				hasTool = true
			}
		}
		mu.Lock()
		counts[mode]++
		mu.Unlock()
		if signal := started[mode]; signal != nil {
			signal <- struct{}{}
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		message := map[string]interface{}{"role": "assistant", "content": "drained"}
		finish := "stop"
		if mode == "action-slow" && !hasTool {
			message["content"] = nil
			message["tool_calls"] = []interface{}{map[string]interface{}{"id": "effect-call", "type": "function", "function": map[string]interface{}{"name": "daptin_action", "arguments": `{"entity":"rlwork","action":"effect","attributes":{}}`}}}
			finish = "tool_calls"
		}
		response := map[string]interface{}{"id": "active-response", "object": "chat.completion", "model": "rlactive", "choices": []interface{}{map[string]interface{}{"index": 0, "message": message, "finish_reason": finish}}, "usage": map[string]interface{}{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}}
		mu.Lock()
		exchanges = append(exchanges, map[string]interface{}{"request": json.RawMessage(raw), "response": response})
		mu.Unlock()
		transportE2EWriteJSON(w, response)
	}))
	defer upstream.Close()
	defer releaseOnce.Do(func() { close(release) })
	schema := "Tables:\n  - TableName: rlwork\n    Columns:\n      - {Name: name, ColumnType: label, DataType: varchar(100)}\n  - TableName: document\n    Columns:\n      - Name: document_content\n        IsForeignKey: true\n        ForeignKeyData: {DataSource: cloud_store, Namespace: reliability-files, KeyName: files}\nActions:\n  - Name: effect\n    OnType: rlwork\n    InstanceOptional: true\n    OutFields:\n      - Type: $network.request\n        Method: EXECUTE\n        Attributes:\n          Url: " + upstream.URL + "/effect\n          Method: GET\n"
	c := startReliabilityCluster(t, schema)
	c.ready(t, nil)
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		reliabilitySave(t, c.dir, "provider-exchanges.json", exchanges)
		reliabilitySave(t, c.dir, "provider-counts.json", counts)
	})
	accessGroupsE2ECreateRecord(t, c.client, c.bases[0], c.token, "cloud_store", map[string]interface{}{"name": "reliability-files", "store_type": "local", "store_provider": "local", "root_path": t.TempDir(), "store_parameters": "{}"})
	c.restart(t, 0)
	createLLME2ECatalog(t, c.client, c.bases[0], c.token, llmE2ECatalog{name: "rlactive", upstreamURL: upstream.URL, apiKey: "test-key", upstreamModel: "rlactive", operations: []string{"chat"}, maxConcurrency: 8, requestTimeoutMS: 60000, capabilities: `{"tools":true}`})
	for _, base := range c.bases {
		waitForLLME2EModel(t, c.client, base, c.token, "rlactive")
	}
	config := map[string]interface{}{"model": "rlactive", "instructions": "Use tools.", "toolset": map[string]interface{}{"resources": []string{"rlwork"}, "operations": []string{"create", "read"}, "actions": []string{"rlwork.effect"}}, "limits": map[string]interface{}{"turns": 6, "tool_calls": 6, "seconds": 180, "output_tokens": 128, "attempts": 3}}
	definition := accessGroupsE2ECreateRecord(t, c.client, c.bases[0], c.token, "agent", map[string]interface{}{"name": "active-drain", "config": string(mustAgentJSON(config))})
	var runs []string
	for i, mode := range []string{"agent-slow", "action-slow"} {
		result := transportE2EPostJSON(t, c.client, c.bases[i]+"/action/agent/start", c.token, map[string]interface{}{"attributes": map[string]interface{}{"agent_id": definition, "messages": []interface{}{map[string]interface{}{"role": "user", "content": mode}}}})
		ref, _ := transportE2EPath(result, "0.Attributes.reference_id")
		runs = append(runs, ref.(string))
	}
	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	_ = writer.WriteField("purpose", "batch")
	part, _ := writer.CreateFormFile("file", "requests.jsonl")
	_, _ = io.WriteString(part, `{"custom_id":"r1","method":"POST","url":"/v1/chat/completions","body":{"model":"rlactive","max_tokens":16,"messages":[{"role":"user","content":"batch-slow"}]}}`+"\n")
	_ = writer.Close()
	fileBody := requestAssetE2E(t, c.client, http.MethodPost, c.bases[0]+"/v1/files", c.token, &form, writer.FormDataContentType(), http.StatusOK)
	var file map[string]interface{}
	_ = json.Unmarshal(fileBody, &file)
	batch := transportE2EPostJSON(t, c.client, c.bases[1]+"/v1/batches", c.token, map[string]interface{}{"input_file_id": file["id"], "endpoint": "/v1/chat/completions", "completion_window": "24h"}).(map[string]interface{})
	ref := batch["id"].(string)
	for _, name := range []string{"agent-slow", "batch-slow", "effect"} {
		signal := started[name]
		reliabilityWait(t, func() bool {
			select {
			case <-signal:
				return true
			default:
				return false
			}
		}, name+" active")
	}
	boots := make([]int, 3)
	for i, node := range c.nodes {
		boots[i] = strings.Count(node.logs.String(), "GitCommit:") + 1
	}
	transportE2EPostJSON(t, c.client, c.bases[2]+"/action/world/restart_daptin", c.token, map[string]interface{}{"attributes": map[string]interface{}{}})
	reliabilityWait(t, func() bool {
		status, _, err := reliabilityHTTP(c.client, http.MethodGet, c.bases[2]+"/ready", c.token, nil)
		return err != nil || status == 503
	}, "quiesce while three operations are active")
	releaseOnce.Do(func() { close(release) })
	c.ready(t, boots)
	for i, run := range runs {
		row := c.runStatus(t, run, "completed")
		reliabilitySave(t, c.dir, fmt.Sprintf("run%d.json", i), row)
	}
	var completed map[string]interface{}
	reliabilityWait(t, func() bool {
		status, body, err := reliabilityHTTP(c.client, http.MethodGet, c.bases[0]+"/v1/batches/"+ref, c.token, nil)
		_ = json.Unmarshal(body, &completed)
		return err == nil && status == 200 && completed["status"] == "completed"
	}, "active batch completes after restart")
	reliabilitySave(t, c.dir, "batch.json", completed)
	if value, _ := transportE2EPath(completed, "request_counts.completed"); value != float64(1) {
		t.Fatalf("batch counts: %s", mustAgentJSON(completed))
	}
	mu.Lock()
	snapshot := map[string]int{}
	for name, n := range counts {
		snapshot[name] = n
	}
	mu.Unlock()
	for _, name := range []string{"agent-slow", "batch-slow", "effect"} {
		if snapshot[name] != 1 {
			t.Errorf("%s executed %d times", name, snapshot[name])
		}
	}
	output := requestAssetE2E(t, c.client, http.MethodGet, c.bases[0]+"/v1/files/"+completed["output_file_id"].(string)+"/content", c.token, nil, "", http.StatusOK)
	if !bytes.Contains(output, []byte(`"custom_id":"r1"`)) || !bytes.Contains(output, []byte("drained")) {
		t.Fatalf("batch lost result: %s", output)
	}
	_ = os.WriteFile(filepath.Join(c.dir, "batch-output.jsonl"), output, 0600)
}

// Query the existing Olric diagnostics to measure the server's Go runtime;
// no application instrumentation or additional listener is required.
func (c *reliabilityCluster) checkCoordination(t *testing.T, cycle int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var clients []*olric.ClusterClient
	var subscriptions []*redis.PubSub
	defer func() {
		for _, sub := range subscriptions {
			_ = sub.Close()
		}
		for _, client := range clients {
			_ = client.Close(context.Background())
		}
	}()
	var publishers []*olric.PubSub
	var snapshots []interface{}
	topic := fmt.Sprintf("reliability-%d-%d", time.Now().UnixNano(), cycle)
	key := topic
	for _, base := range c.bases {
		stats := transportE2EGetJSON(t, c.client, base+"/statistics", c.token)
		addr, _ := transportE2EPath(stats, "olric.member")
		client, err := olric.NewClusterClientContext(ctx, []string{addr.(string)}, olric.WithRoutingTableFetchInterval(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		clients = append(clients, client)
		runtime, err := client.Stats(ctx, addr.(string), olric.CollectRuntime())
		if err != nil {
			t.Fatal(err)
		}
		snapshots = append(snapshots, runtime.Runtime)
		ps, err := client.NewPubSub(olric.ToAddress(addr.(string)))
		if err != nil {
			t.Fatal(err)
		}
		publishers = append(publishers, ps)
		sub := ps.Subscribe(ctx, topic)
		subscriptions = append(subscriptions, sub)
		if _, err := sub.Receive(ctx); err != nil {
			t.Fatal(err)
		}
	}
	for i, client := range clients {
		counter, err := client.NewDMap("reliability-counters")
		if err != nil {
			t.Fatal(err)
		}
		value, err := counter.Incr(ctx, key, 1)
		if err != nil || value != i+1 {
			t.Fatalf("counter not shared: node=%d value=%d err=%v", i, value, err)
		}
	}
	counter, _ := clients[0].NewDMap("reliability-counters")
	_, _ = counter.Delete(ctx, key)
	if _, err := publishers[0].Publish(ctx, topic, "verified"); err != nil {
		t.Fatal(err)
	}
	for i, sub := range subscriptions {
		select {
		case message := <-sub.Channel():
			if message.Payload != "verified" {
				t.Fatalf("unexpected PubSub payload on node %d", i)
			}
		case <-ctx.Done():
			t.Fatalf("PubSub did not reach node %d: %v", i, ctx.Err())
		}
	}
	reliabilitySave(t, c.dir, fmt.Sprintf("go-runtime-%02d.json", cycle), snapshots)
}

func (c *reliabilityCluster) archiveRuns(t *testing.T) {
	for _, entity := range []string{"agentrun", "agentstp", "agenttry", "rlitem"} {
		for page := 1; page <= 50; page++ {
			status, body, err := reliabilityHTTP(c.client, http.MethodGet, c.bases[0]+fmt.Sprintf("/api/%s?page[size]=100&page[number]=%d", entity, page), c.token, nil)
			reliabilitySave(t, c.dir, fmt.Sprintf("%s-page-%02d.json", entity, page), map[string]interface{}{"status": status, "body": string(body), "error": fmt.Sprint(err)})
			if err != nil || status != 200 {
				break
			}
			var decoded map[string]interface{}
			_ = json.Unmarshal(body, &decoded)
			rows, _ := decoded["data"].([]interface{})
			if len(rows) < 100 {
				break
			}
		}
	}
}

func (c *reliabilityCluster) archiveSchema(t *testing.T) {
	for i, base := range c.bases {
		for _, entity := range []string{"api_usage", "user_account", "agentrun", "agentstp", "agenttry", "rlitem"} {
			status, body, err := reliabilityHTTP(c.client, http.MethodGet, base+"/jsmodel/"+entity, c.token, nil)
			reliabilitySave(t, c.dir, fmt.Sprintf("runtime-%d-%s.json", i, entity), map[string]interface{}{"status": status, "body": string(body), "error": fmt.Sprint(err)})
			if status == 200 && (bytes.Contains(body, []byte(`\u0000`)) || bytes.ContainsRune(body, 0)) {
				t.Errorf("node %d %s runtime metadata contains NUL bytes", i, entity)
			}
		}
	}
	for page := 1; page <= 10; page++ {
		status, body, err := reliabilityHTTP(c.client, http.MethodGet, c.bases[0]+fmt.Sprintf("/api/world?page[size]=100&page[number]=%d", page), c.token, nil)
		reliabilitySave(t, c.dir, fmt.Sprintf("world-page-%02d.json", page), map[string]interface{}{"status": status, "body": string(body), "error": fmt.Sprint(err)})
		var decoded map[string]interface{}
		_ = json.Unmarshal(body, &decoded)
		rows, _ := decoded["data"].([]interface{})
		if err != nil || status != 200 || len(rows) < 100 {
			break
		}
	}
}
