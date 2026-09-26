package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func BenchmarkLLMGatewayHTTPPostgres(b *testing.B) {
	if os.Getenv("DAPTIN_LLM_BENCHMARK") != "1" {
		b.Skip("set DAPTIN_LLM_BENCHMARK=1 with DAPTIN_TEST_POSTGRES_DSN to run the full-path benchmark")
	}
	workload := prepareLLMGatewayHTTPPostgres(b)
	if err := workload.invoke(context.Background()); err != nil {
		b.Fatalf("warm-up request failed: %v", err)
	}
	streamContext, cancelStream := context.WithTimeout(context.Background(), 10*time.Second)
	stream, err := workload.openStream(streamContext)
	if err != nil {
		cancelStream()
		b.Fatalf("stream warm-up failed: %v", err)
	}
	if err := stream.Close(); err != nil {
		cancelStream()
		b.Fatalf("close stream warm-up: %v", err)
	}
	cancelStream()

	b.Run("serial", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := workload.invoke(context.Background()); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("parallel", func(b *testing.B) {
		var failures atomic.Int64
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if workload.invoke(context.Background()) != nil {
					failures.Add(1)
				}
			}
		})
		if count := failures.Load(); count != 0 {
			b.Fatalf("%d parallel gateway requests failed", count)
		}
	})
}

type llmGatewayCapacityResult struct {
	requests     int
	failures     int
	firstFailure error
	elapsed      time.Duration
	achievedRPS  float64
	p95          time.Duration
	p99          time.Duration
}

func TestPacedLLMGatewayCapacityMeasurement(t *testing.T) {
	var calls atomic.Int64
	measured := measureLLMGatewayCapacity(100*time.Millisecond, 100, func(context.Context) error {
		if calls.Add(1) == 3 {
			return errors.New("expected test failure")
		}
		time.Sleep(time.Millisecond)
		return nil
	})
	if measured.requests != 10 || measured.failures != 1 || measured.firstFailure == nil || measured.p95 <= 0 || measured.p99 <= 0 {
		t.Fatalf("paced measurement = %+v", measured)
	}
}

func TestConcurrentLLMGatewayStreamMeasurement(t *testing.T) {
	measured := measureLLMGatewayStreams(context.Background(), 10, time.Millisecond, func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("")), nil
	}, nil)
	if measured.opened != 10 || measured.failures != 0 || measured.openDuration <= 0 {
		t.Fatalf("stream measurement = %+v", measured)
	}
}

func TestLinuxProcessGroupRSS(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux /proc RSS accounting")
	}
	rss, err := linuxProcessGroupRSS(syscall.Getpgrp())
	if err != nil || rss <= 0 {
		t.Fatalf("process-group RSS = %d, %v", rss, err)
	}
}

func TestLLMGatewayCapacityPostgres(t *testing.T) {
	if os.Getenv("DAPTIN_LLM_CAPACITY") != "1" {
		t.Skip("set DAPTIN_LLM_CAPACITY=1 with DAPTIN_TEST_POSTGRES_DSN to run the full-path capacity and soak gate")
	}
	if runtime.GOOS != "linux" || runtime.NumCPU() < 8 {
		t.Fatalf("capacity reference requires Linux with at least 8 logical CPUs; got %s/%d", runtime.GOOS, runtime.NumCPU())
	}
	var fileLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &fileLimit); err != nil || fileLimit.Cur < 4096 {
		t.Fatalf("capacity reference requires a file-descriptor limit of at least 4096; limit=%d err=%v", fileLimit.Cur, err)
	}
	memory, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		t.Fatalf("read Linux memory capacity: %v", err)
	}
	var memoryKiB int64
	const minimumUsableMemoryKiB = 16 * 1024 * 1024 * 95 / 100
	if _, err := fmt.Sscanf(string(memory), "MemTotal: %d kB", &memoryKiB); err != nil || memoryKiB < minimumUsableMemoryKiB {
		t.Fatalf("capacity reference requires at least 16 GiB memory; MemTotal=%d KiB err=%v", memoryKiB, err)
	}

	duration := 10 * time.Minute
	if configured := strings.TrimSpace(os.Getenv("DAPTIN_LLM_CAPACITY_DURATION")); configured != "" {
		parsed, err := time.ParseDuration(configured)
		if err != nil || parsed < 10*time.Minute {
			t.Fatalf("DAPTIN_LLM_CAPACITY_DURATION must be at least ten minutes: %q", configured)
		}
		duration = parsed
	}
	targetRPS := 250
	if configured := strings.TrimSpace(os.Getenv("DAPTIN_LLM_CAPACITY_RPS")); configured != "" {
		parsed, err := strconv.Atoi(configured)
		if err != nil || parsed < 250 {
			t.Fatalf("DAPTIN_LLM_CAPACITY_RPS must be at least 250: %q", configured)
		}
		targetRPS = parsed
	}

	t.Setenv("DAPTIN_MAX_OPEN_CONNECTIONS", "20")
	workload := prepareLLMGatewayHTTPPostgres(t)
	if err := workload.invoke(context.Background()); err != nil {
		t.Fatalf("warm-up request failed: %v", err)
	}
	measured := measureLLMGatewayCapacity(duration, targetRPS, workload.invoke)
	if measured.failures != 0 || measured.requests != int(duration.Seconds()*float64(targetRPS)) {
		t.Fatalf("capacity run completed=%d failures=%d first_failure=%v", measured.requests, measured.failures, measured.firstFailure)
	}
	t.Logf("capacity requests=%d duration=%s achieved_rps=%.2f p95=%s p99=%s",
		measured.requests, measured.elapsed, measured.achievedRPS, measured.p95, measured.p99)
	if measured.achievedRPS < float64(targetRPS)*0.98 {
		t.Errorf("capacity throughput %.2f RPS is below 98%% of target %d RPS", measured.achievedRPS, targetRPS)
	}
	if measured.p95 > 15*time.Millisecond || measured.p99 > 40*time.Millisecond {
		t.Errorf("capacity latency exceeds target: p95=%s (limit 15ms), p99=%s (limit 40ms)", measured.p95, measured.p99)
	}
	qualifyLLMGatewayStreams(t, workload.processGroupID, workload.openStream)
}

func measureLLMGatewayCapacity(duration time.Duration, targetRPS int, invoke func(context.Context) error) llmGatewayCapacityResult {
	type result struct {
		latency time.Duration
		err     error
	}
	total := int(duration.Seconds() * float64(targetRPS))
	jobs := make(chan time.Time, targetRPS)
	results := make(chan result, total)
	workers := targetRPS
	if workers > 512 {
		workers = 512
	}
	var wait sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), duration+30*time.Second)
	defer cancel()
	wait.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func() {
			defer wait.Done()
			for scheduled := range jobs {
				err := invoke(ctx)
				results <- result{latency: time.Since(scheduled), err: err}
			}
		}()
	}
	started := time.Now()
	for requestNumber := 0; requestNumber < total; requestNumber++ {
		scheduled := started.Add(time.Duration(requestNumber) * time.Second / time.Duration(targetRPS))
		if delay := time.Until(scheduled); delay > 0 {
			time.Sleep(delay)
		}
		jobs <- scheduled
	}
	close(jobs)
	wait.Wait()
	close(results)
	elapsed := time.Since(started)
	latencies := make([]time.Duration, 0, total)
	failures := 0
	var firstFailure error
	for result := range results {
		if result.err != nil {
			failures++
			if firstFailure == nil {
				firstFailure = result.err
			}
		}
		latencies = append(latencies, result.latency)
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	percentile := func(percent int) time.Duration {
		index := (len(latencies)*percent + 99) / 100
		if index > 0 {
			index--
		}
		return latencies[index]
	}
	p95, p99 := percentile(95), percentile(99)
	achievedRPS := float64(total) / elapsed.Seconds()
	return llmGatewayCapacityResult{requests: len(latencies), failures: failures, firstFailure: firstFailure,
		elapsed: elapsed, achievedRPS: achievedRPS, p95: p95, p99: p99}
}

type llmGatewayStreamCapacityResult struct {
	opened       int
	failures     int
	firstFailure error
	openDuration time.Duration
}

func qualifyLLMGatewayStreams(t *testing.T, processGroupID int, openStream func(context.Context) (io.ReadCloser, error)) {
	t.Helper()
	const streamCount = 1000
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	baselineRSS, err := linuxProcessGroupRSS(processGroupID)
	if err != nil {
		t.Fatalf("measure Daptin baseline RSS: %v", err)
	}
	var heldRSS int64
	measured := measureLLMGatewayStreams(ctx, streamCount, 30*time.Second, openStream, func() {
		currentRSS, readErr := linuxProcessGroupRSS(processGroupID)
		if readErr != nil && err == nil {
			err = readErr
		}
		if currentRSS > heldRSS {
			heldRSS = currentRSS
		}
	})
	if err != nil {
		t.Fatalf("measure Daptin held-stream RSS: %v", err)
	}
	if measured.failures != 0 {
		t.Fatalf("stream capacity opened=%d failures=%d first_failure=%v", measured.opened, measured.failures, measured.firstFailure)
	}
	streamRSS := heldRSS - baselineRSS
	if streamRSS < 0 {
		streamRSS = 0
	}
	bytesPerStream := streamRSS / streamCount
	t.Logf("stream capacity opened=%d open_duration=%s hold_duration=30s baseline_rss=%d held_rss=%d bytes_per_stream=%d",
		measured.opened, measured.openDuration, baselineRSS, heldRSS, bytesPerStream)
	if bytesPerStream > 96*1024 {
		t.Errorf("steady stream memory %d bytes/stream exceeds the 96 KiB limit", bytesPerStream)
	}
}

func measureLLMGatewayStreams(ctx context.Context, streamCount int, holdDuration time.Duration,
	openStream func(context.Context) (io.ReadCloser, error), observeHeld func()) llmGatewayStreamCapacityResult {
	release := make(chan struct{})
	opened := make(chan error, streamCount)
	closed := make(chan error, streamCount)
	var wait sync.WaitGroup
	wait.Add(streamCount)
	started := time.Now()
	for index := 0; index < streamCount; index++ {
		go func() {
			defer wait.Done()
			body, err := openStream(ctx)
			opened <- err
			if err != nil {
				return
			}
			<-release
			closed <- body.Close()
		}()
	}
	failures := 0
	var firstFailure error
	for index := 0; index < streamCount; index++ {
		if err := <-opened; err != nil {
			failures++
			if firstFailure == nil {
				firstFailure = err
			}
		}
	}
	measured := llmGatewayStreamCapacityResult{opened: streamCount - failures, failures: failures,
		firstFailure: firstFailure, openDuration: time.Since(started)}
	if failures == 0 {
		if observeHeld != nil {
			observeHeld()
		}
		time.Sleep(holdDuration)
		if observeHeld != nil {
			observeHeld()
		}
	}
	close(release)
	wait.Wait()
	for index := 0; index < measured.opened; index++ {
		if err := <-closed; err != nil {
			measured.failures++
			if measured.firstFailure == nil {
				measured.firstFailure = err
			}
		}
	}
	return measured
}

func linuxProcessGroupRSS(processGroupID int) (int64, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, err
	}
	var pages int64
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		stat, err := os.ReadFile("/proc/" + entry.Name() + "/stat")
		if err != nil {
			continue
		}
		closingParenthesis := strings.LastIndexByte(string(stat), ')')
		if closingParenthesis < 0 {
			continue
		}
		fields := strings.Fields(string(stat[closingParenthesis+1:]))
		if len(fields) <= 21 {
			continue
		}
		group, groupErr := strconv.Atoi(fields[2])
		rssPages, rssErr := strconv.ParseInt(fields[21], 10, 64)
		if groupErr == nil && rssErr == nil && group == processGroupID && rssPages > 0 {
			pages += rssPages
		}
	}
	if pages == 0 {
		return 0, fmt.Errorf("process group %d has no observable resident pages", processGroupID)
	}
	return pages * int64(os.Getpagesize()), nil
}

type llmGatewayHTTPWorkload struct {
	processGroupID int
	invoke         func(context.Context) error
	openStream     func(context.Context) (io.ReadCloser, error)
}

func prepareLLMGatewayHTTPPostgres(t testing.TB) llmGatewayHTTPWorkload {
	t.Helper()
	dsn := os.Getenv("DAPTIN_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("DAPTIN_TEST_POSTGRES_DSN must identify an empty disposable PostgreSQL database")
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			http.NotFound(response, request)
			return
		}
		if request.Header.Get("Authorization") != "Bearer benchmark-provider-key" {
			http.Error(response, "invalid provider credential", http.StatusUnauthorized)
			return
		}
		var body struct {
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			http.Error(response, "invalid request", http.StatusBadRequest)
			return
		}
		if body.Stream {
			response.Header().Set("Content-Type", "text/event-stream")
			_, _ = response.Write([]byte("data: {\"id\":\"benchmark-stream\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"benchmark-upstream\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"ok\"},\"finish_reason\":null}]}\n\n"))
			if flusher, ok := response.(http.Flusher); ok {
				flusher.Flush()
			}
			<-request.Context().Done()
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"id":"benchmark-response","object":"chat.completion","created":1,"model":"benchmark-upstream","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":8,"completion_tokens":1,"total_tokens":9}}`))
	}))
	t.Cleanup(upstream.Close)

	usedPorts := make(map[int]bool, 4)
	port := freeTransportE2EPort(t, usedPorts)
	httpsPort := freeTransportE2EPort(t, usedPorts)
	olricPort := freeTransportE2EPortPair(t, usedPorts)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	daptinProcess := startTransportE2EDaptin(t, port, httpsPort, baseURL, transportE2EDaptinOptions{
		databaseType: "postgres", connectionString: dsn, olricPort: olricPort,
	})
	t.Cleanup(daptinProcess.stopProcess)
	client := &http.Client{Timeout: 20 * time.Second}
	t.Cleanup(client.CloseIdleConnections)
	token := transportE2ESignupSigninAdmin(t, client, baseURL)
	modelName := fmt.Sprintf("llm-benchmark-%d", time.Now().UnixNano())
	createLLME2ECatalog(t, client, baseURL, token, llmE2ECatalog{
		name: modelName, upstreamURL: upstream.URL, apiKey: "benchmark-provider-key",
		upstreamModel: "benchmark-upstream", operations: []string{"chat"}, maxConcurrency: 1200,
	})
	waitForLLME2EModel(t, client, baseURL, token, modelName)
	payload, err := json.Marshal(map[string]interface{}{
		"model": modelName, "messages": []map[string]string{{"role": "user", "content": "benchmark request"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	invoke := func(ctx context.Context) error {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/chat/completions", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		_, readErr := io.Copy(io.Discard, response.Body)
		closeErr := response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("gateway returned %s", response.Status)
		}
		return errors.Join(readErr, closeErr)
	}
	streamPayload, err := json.Marshal(map[string]interface{}{
		"model": modelName, "messages": []map[string]string{{"role": "user", "content": "stream qualification"}}, "stream": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	streamClient := &http.Client{}
	t.Cleanup(streamClient.CloseIdleConnections)
	openStream := func(ctx context.Context) (io.ReadCloser, error) {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/chat/completions", bytes.NewReader(streamPayload))
		if err != nil {
			return nil, err
		}
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", "application/json")
		response, err := streamClient.Do(request)
		if err != nil {
			return nil, err
		}
		if response.StatusCode != http.StatusOK {
			_ = response.Body.Close()
			return nil, fmt.Errorf("stream gateway returned %s", response.Status)
		}
		reader := bufio.NewReader(response.Body)
		frame, err := reader.ReadString('\n')
		if err != nil || !strings.Contains(frame, "chat.completion.chunk") {
			_ = response.Body.Close()
			return nil, fmt.Errorf("stream gateway returned no semantic event: %w", err)
		}
		return response.Body, nil
	}
	return llmGatewayHTTPWorkload{processGroupID: daptinProcess.processGroupID, invoke: invoke, openStream: openStream}
}
