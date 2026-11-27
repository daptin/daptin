package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
)

// Opt-in, disposable full-HTTP measurement. No capacity thresholds are asserted.
// DAPTIN_LOAD_VARIANT labels the binary supplied through DAPTIN_E2E_BINARY.
func TestPermissionLoadRealE2E(t *testing.T) {
	requireRealE2E(t)
	if os.Getenv("DAPTIN_PERMISSION_LOAD") != "1" {
		t.Skip("set DAPTIN_PERMISSION_LOAD=1 and DAPTIN_LOAD_RESULTS")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("this measurement uses macOS ps CPU-time and RSS accounting")
	}
	output := os.Getenv("DAPTIN_LOAD_RESULTS")
	if output == "" || os.Getenv("DAPTIN_E2E_BINARY") == "" {
		t.Fatal("DAPTIN_LOAD_RESULTS and a prebuilt DAPTIN_E2E_BINARY are required")
	}
	seconds := 10
	if value := os.Getenv("DAPTIN_LOAD_SECONDS"); value != "" {
		var err error
		seconds, err = strconv.Atoi(value)
		if err != nil || seconds < 1 {
			t.Fatal("DAPTIN_LOAD_SECONDS must be a positive integer")
		}
	}
	nodeCount := 1
	if os.Getenv("DAPTIN_LOAD_CLUSTER") == "1" {
		nodeCount = 3
		if os.Getenv("DAPTIN_E2E_MAIL_POSTGRES_BIN") == "" {
			t.Fatal("the cluster measurement requires private PostgreSQL")
		}
	}
	options := mailProtocolDatabase(t)
	options.schema = fmt.Sprintf(`Tables:
  - TableName: lditem
    DefaultPermission: %d
    AccessGroups:
      - Name: users
        Permission: %d
    DefaultGroups:
      - Name: users
        Permission: %d
    Columns:
      - Name: name
        ColumnName: name
        ColumnType: label
        DataType: varchar(100)
      - Name: payload
        ColumnName: payload
        ColumnType: content
        DataType: text
`, auth.UserCRUD, auth.GroupCRUD, auth.GroupPeek|auth.GroupRead|auth.GroupRefer)
	used := map[int]bool{}
	seed := ""
	client := &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{MaxIdleConns: 256, MaxIdleConnsPerHost: 128}}
	t.Cleanup(client.CloseIdleConnections)
	start := func(peer string) *daptinE2EFixture {
		port := freeTransportE2EPort(t, used)
		base := fmt.Sprintf("http://127.0.0.1:%d", port)
		configuration := options
		configuration.olricPort = freeTransportE2EPortPair(t, used)
		configuration.olricPeers = peer
		if seed == "" {
			seed = fmt.Sprintf("127.0.0.1:%d", configuration.olricPort+1)
		}
		process := startTransportE2EDaptin(t, port, freeTransportE2EPort(t, used), base, configuration)
		return &daptinE2EFixture{URL: base, Client: client, Process: process}
	}
	first := start("")
	admin := first.SignupAdmin(t)
	tokens := make([]string, 4)
	for i := range tokens {
		tokens[i] = accessGroupsE2ESignupSigninUser(t, client, first.URL, admin, fmt.Sprintf("load-reader-%d", i))
	}
	refs := make([]string, 50)
	limits := map[string]int{"/api/lditem": 1000000, "/action/agent/start": 1000000}
	for i := range refs {
		refs[i] = accessGroupsE2ECreateRecord(t, client, first.URL, admin, "lditem", map[string]interface{}{"name": fmt.Sprintf("item-%03d", i), "payload": strings.Repeat("x", 512)})
		limits["/api/lditem/"+refs[i]] = 1000000
	}
	rateJSON, _ := json.Marshal(map[string]interface{}{"version": "1", "limits": limits})
	ftpE2ESetConfig(t, client, first.URL, admin, "limit.rate", string(rateJSON))
	first.Process.stopProcess()
	seed = ""
	nodes := []*daptinE2EFixture{start("")}
	for len(nodes) < nodeCount {
		nodes = append(nodes, start(seed))
	}
	if nodeCount > 1 {
		deadline := time.Now().Add(20 * time.Second)
		for {
			ready := true
			for _, node := range nodes {
				stats := transportE2EGetJSON(t, client, node.URL+"/statistics", admin)
				count, _ := transportE2EPath(stats, "olric.member_count")
				ready = ready && count == float64(nodeCount)
			}
			if ready {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("Olric cluster did not converge")
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
	pids := make([]int, len(nodes))
	for i, node := range nodes {
		pids[i] = node.Process.processGroupID
	}
	results := []permissionLoadResult{}
	write := func(result permissionLoadResult) {
		results = append(results, result)
		data, err := json.MarshalIndent(map[string]interface{}{"variant": os.Getenv("DAPTIN_LOAD_VARIANT"), "nodes": nodeCount, "database": options.databaseType, "logical_cpus": runtime.NumCPU(), "os": runtime.GOOS, "arch": runtime.GOARCH, "go": runtime.Version(), "duration_seconds": seconds, "results": results}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(output, data, 0600); err != nil {
			t.Fatal(err)
		}
		t.Logf("LOAD %s trial=%d c=%d requests=%d failures=%d rps=%.1f p95=%.2fms cpu=%.1f%% rss_avg=%.1fMiB rss_peak=%.1fMiB first_error=%s", result.Workload, result.Trial, result.Concurrency, result.Requests, result.Failures, result.SuccessRPS, result.P95ms, result.CPUPercent, result.RSSMeanMiB, result.RSSPeakMiB, result.FirstError)
	}
	write(measurePermissionLoad(t, "idle", 0, 0, 5*time.Second, pids, nil))
	var next atomic.Uint64
	read := func(list bool) error {
		i := int(next.Add(1) - 1)
		path := "/api/lditem/" + refs[(i/len(tokens))%len(refs)]
		if list {
			path = "/api/lditem?page[size]=25&page[number]=1"
		}
		req, _ := http.NewRequest(http.MethodGet, nodes[i%len(nodes)].URL+path, nil)
		req.Header.Set("Authorization", "Bearer "+tokens[i%len(tokens)])
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if resp.StatusCode != 200 {
			return fmt.Errorf("HTTP %d: %.200s", resp.StatusCode, body)
		}
		var decoded struct{ Data json.RawMessage }
		if err := json.Unmarshal(body, &decoded); err != nil {
			return err
		}
		if list {
			var rows []json.RawMessage
			if err := json.Unmarshal(decoded.Data, &rows); err != nil || len(rows) != 25 {
				return fmt.Errorf("list returned %d rows: %v", len(rows), err)
			}
		} else if !bytes.Contains(decoded.Data, []byte(`"payload"`)) {
			return fmt.Errorf("record response missing payload")
		}
		return nil
	}
	if os.Getenv("DAPTIN_LOAD_ONLY_AGENT") != "1" {
		// Warm every measured row and node before the timed runs.
		for i := 0; i < len(refs)*len(tokens)*nodeCount; i++ {
			if err := read(false); err != nil {
				t.Fatal(err)
			}
		}
		trials := 2
		concurrencies := []int{1, 16, 64}
		if nodeCount > 1 {
			trials, concurrencies = 1, []int{16, 64}
		}
		for trial := 1; trial <= trials; trial++ {
			for _, concurrency := range concurrencies {
				for _, list := range []bool{false, true} {
					name := "record"
					if list {
						name = "list25"
					}
					result := measurePermissionLoad(t, name, trial, concurrency, time.Duration(seconds)*time.Second, pids, func() error { return read(list) })
					write(result)
					if result.Failures > 0 {
						t.Errorf("%s c=%d: %d failures, first=%s", name, concurrency, result.Failures, result.FirstError)
					}
				}
			}
		}
	}
	if os.Getenv("DAPTIN_LOAD_AGENT") == "1" {
		permissionLoadAgents(t, nodes, client, admin, refs[0], pids, write)
	}
}

type permissionLoadResult struct {
	Workload                                 string
	Trial, Concurrency, Requests, Failures   int
	Seconds, SuccessRPS, P50ms, P95ms, P99ms float64
	CPUSeconds, CPUPercent, CPUMsPerSuccess  float64
	RSSMeanMiB, RSSPeakMiB, RSSEndMiB        float64
	Samples                                  int
	FirstError                               string `json:",omitempty"`
}

func permissionLoadProcessSample(pids []int) (cpu, rss float64, err error) {
	ids := make([]string, len(pids))
	for i, pid := range pids {
		ids[i] = strconv.Itoa(pid)
	}
	out, err := exec.Command("ps", "-p", strings.Join(ids, ","), "-o", "pid=,time=,rss=").Output()
	if err != nil {
		return 0, 0, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != len(pids) {
		return 0, 0, fmt.Errorf("missing measured processes: %s", out)
	}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return 0, 0, fmt.Errorf("unexpected ps output: %q", line)
		}
		var seconds float64
		for _, component := range strings.Split(fields[1], ":") {
			value, e := strconv.ParseFloat(component, 64)
			if e != nil {
				return 0, 0, e
			}
			seconds = seconds*60 + value
		}
		memory, e := strconv.ParseFloat(fields[2], 64)
		if e != nil {
			return 0, 0, e
		}
		cpu += seconds
		rss += memory / 1024
	}
	return cpu, rss, nil
}

func measurePermissionLoad(t *testing.T, name string, trial, concurrency int, duration time.Duration, pids []int, invoke func() error) permissionLoadResult {
	t.Helper()
	cpuStart, rssStart, err := permissionLoadProcessSample(pids)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	deadline := start.Add(duration)
	var mu sync.Mutex
	var latencies []float64
	result := permissionLoadResult{Workload: name, Trial: trial, Concurrency: concurrency, RSSPeakMiB: rssStart}
	stop := make(chan struct{})
	sampled := make(chan struct{})
	var sampleErr error
	var sumRSS float64
	go func() {
		defer close(sampled)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				_, rss, err := permissionLoadProcessSample(pids)
				if err != nil {
					sampleErr = err
					return
				}
				sumRSS += rss
				result.Samples++
				if rss > result.RSSPeakMiB {
					result.RSSPeakMiB = rss
				}
			}
		}
	}()
	var workers sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			var local []float64
			failures := 0
			firstError := ""
			for time.Now().Before(deadline) {
				started := time.Now()
				err := invoke()
				local = append(local, float64(time.Since(started))/float64(time.Millisecond))
				if err != nil {
					failures++
					if firstError == "" {
						firstError = err.Error()
					}
				}
			}
			mu.Lock()
			latencies = append(latencies, local...)
			result.Failures += failures
			if result.FirstError == "" {
				result.FirstError = firstError
			}
			mu.Unlock()
		}()
	}
	if concurrency == 0 {
		time.Sleep(duration)
	}
	workers.Wait()
	result.Seconds = time.Since(start).Seconds()
	close(stop)
	<-sampled
	if sampleErr != nil {
		t.Fatal(sampleErr)
	}
	cpuEnd, rssEnd, err := permissionLoadProcessSample(pids)
	if err != nil {
		t.Fatal(err)
	}
	result.CPUSeconds = cpuEnd - cpuStart
	result.CPUPercent = 100 * result.CPUSeconds / result.Seconds
	result.RSSEndMiB = rssEnd
	if rssEnd > result.RSSPeakMiB {
		result.RSSPeakMiB = rssEnd
	}
	if result.Samples > 0 {
		result.RSSMeanMiB = sumRSS / float64(result.Samples)
	}
	result.Requests = len(latencies)
	if success := result.Requests - result.Failures; success > 0 {
		result.SuccessRPS = float64(success) / result.Seconds
		result.CPUMsPerSuccess = result.CPUSeconds * 1000 / float64(success)
	}
	if len(latencies) > 0 {
		sort.Float64s(latencies)
		result.P50ms = latencies[(len(latencies)-1)*50/100]
		result.P95ms = latencies[(len(latencies)-1)*95/100]
		result.P99ms = latencies[(len(latencies)-1)*99/100]
	}
	return result
}

func permissionLoadAgents(t *testing.T, nodes []*daptinE2EFixture, client *http.Client, admin, reference string, pids []int, write func(permissionLoadResult)) {
	var providerCalls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Messages []struct{ Role string } }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		providerCalls.Add(1)
		hasResult := false
		for _, message := range body.Messages {
			hasResult = hasResult || message.Role == "tool"
		}
		message := map[string]interface{}{"role": "assistant", "content": "done"}
		finish := "stop"
		if !hasResult {
			args, _ := json.Marshal(map[string]interface{}{"entity": "lditem", "reference": reference})
			message = map[string]interface{}{"role": "assistant", "tool_calls": []interface{}{map[string]interface{}{"id": "read1", "type": "function", "function": map[string]interface{}{"name": "daptin_read", "arguments": string(args)}}}}
			finish = "tool_calls"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "load-response", "object": "chat.completion", "created": time.Now().Unix(), "model": "load-upstream", "choices": []interface{}{map[string]interface{}{"index": 0, "message": message, "finish_reason": finish}}, "usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}})
	}))
	defer upstream.Close()
	base := nodes[0].URL
	createLLME2ECatalog(t, client, base, admin, llmE2ECatalog{name: "load-model", upstreamURL: upstream.URL, apiKey: "local-load-key", upstreamModel: "load-upstream", operations: []string{"chat"}, maxConcurrency: 32, requestTimeoutMS: 10000})
	model := accessGroupsE2EFindResourceID(t, client, base, admin, "llm_model", "name", "load-model")
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, base+"/api/llm_model/"+model, admin, accessGroupsE2ERecordPayload("llm_model", model, map[string]interface{}{"capabilities": `{"tools":true}`}), 200)
	for _, node := range nodes {
		waitForLLME2EModel(t, client, node.URL, admin, "load-model")
	}
	config, _ := json.Marshal(map[string]interface{}{"instructions": "Read the record and finish.", "model": "load-model", "toolset": map[string]interface{}{"resources": []string{"lditem"}, "operations": []string{"read"}}, "limits": map[string]interface{}{"turns": 3, "tool_calls": 3, "seconds": 120, "output_tokens": 64}})
	definition := accessGroupsE2ECreateRecord(t, client, base, admin, "agent", map[string]interface{}{"name": "load-agent", "config": string(config)})
	var serial atomic.Uint64
	invoke := func() error {
		i := serial.Add(1)
		node := nodes[int(i)%len(nodes)]
		payload, _ := json.Marshal(map[string]interface{}{"attributes": map[string]interface{}{"agent_id": definition, "messages": []interface{}{map[string]interface{}{"role": "user", "content": fmt.Sprintf("read-%d", i)}}}})
		req, _ := http.NewRequest("POST", node.URL+"/action/agent/start", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+admin)
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		var response interface{}
		err = json.NewDecoder(resp.Body).Decode(&response)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 {
			return fmt.Errorf("agent start HTTP %d: %v %v", resp.StatusCode, err, response)
		}
		value, ok := transportE2EPath(response, "0.Attributes.reference_id")
		if !ok {
			return fmt.Errorf("missing run reference: %v", response)
		}
		for deadline := time.Now().Add(130 * time.Second); time.Now().Before(deadline); {
			request, _ := http.NewRequest("GET", node.URL+"/api/agentrun/"+fmt.Sprint(value), nil)
			request.Header.Set("Authorization", "Bearer "+admin)
			state, err := client.Do(request)
			if err != nil {
				return err
			}
			var result interface{}
			err = json.NewDecoder(state.Body).Decode(&result)
			state.Body.Close()
			if err != nil || state.StatusCode != 200 {
				return fmt.Errorf("run poll HTTP %d: %v", state.StatusCode, err)
			}
			status, _ := transportE2EPath(result, "data.attributes.status")
			if status == "completed" {
				return nil
			}
			if status == "failed" || status == "unknown" {
				return fmt.Errorf("run ended %s: %v", status, result)
			}
			time.Sleep(100 * time.Millisecond)
		}
		return fmt.Errorf("agent deadline expired")
	}
	// Include admission, durable history, tool dispatch, gateway and polling.
	for _, concurrency := range []int{1, 8} {
		before := providerCalls.Load()
		result := measurePermissionLoad(t, "agent_read", 1, concurrency, 20*time.Second, pids, invoke)
		write(result)
		t.Logf("AGENT provider_calls=%d completed=%d", providerCalls.Load()-before, result.Requests-result.Failures)
		if result.Failures > 0 {
			t.Errorf("agent load: %d failures, first=%s", result.Failures, result.FirstError)
		}
	}
	write(measurePermissionLoad(t, "cooldown", 0, 0, 10*time.Second, pids, nil))
}
