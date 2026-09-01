package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func BenchmarkLLMGatewayHTTPPostgres(b *testing.B) {
	if os.Getenv("DAPTIN_LLM_BENCHMARK") != "1" {
		b.Skip("set DAPTIN_LLM_BENCHMARK=1 with DAPTIN_TEST_POSTGRES_DSN to run the full-path benchmark")
	}
	dsn := os.Getenv("DAPTIN_TEST_POSTGRES_DSN")
	if dsn == "" {
		b.Fatal("DAPTIN_TEST_POSTGRES_DSN must identify an empty disposable PostgreSQL database")
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
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"id":"benchmark-response","object":"chat.completion","created":1,"model":"benchmark-upstream","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":8,"completion_tokens":1,"total_tokens":9}}`))
	}))
	defer upstream.Close()

	usedPorts := make(map[int]bool, 4)
	port := freeLLMMultinodePort(b, usedPorts)
	httpsPort := freeLLMMultinodePort(b, usedPorts)
	olricPort := freeLLMMultinodePortPair(b, usedPorts)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	stopDaptin := startTransportE2EDaptin(b, port, httpsPort, baseURL, transportE2EDaptinOptions{
		databaseType: "postgres", connectionString: dsn, olricPort: olricPort,
	})
	defer stopDaptin()

	client := &http.Client{Timeout: 20 * time.Second}
	token, _ := transportE2ESignupSigninAdmin(b, client, baseURL)
	modelName := fmt.Sprintf("llm-benchmark-%d", time.Now().UnixNano())
	createLLME2ECatalog(b, client, baseURL, token, llmE2ECatalog{
		name: modelName, upstreamURL: upstream.URL, apiKey: "benchmark-provider-key",
		upstreamModel: "benchmark-upstream", operations: []string{"chat"}, maxConcurrency: 256,
	})
	waitForLLME2EModel(b, client, baseURL, token, modelName)
	payload, err := json.Marshal(map[string]interface{}{
		"model":    modelName,
		"messages": []map[string]string{{"role": "user", "content": "benchmark request"}},
	})
	if err != nil {
		b.Fatal(err)
	}

	invoke := func() error {
		request, err := http.NewRequest(http.MethodPost, baseURL+"/v1/chat/completions", bytes.NewReader(payload))
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
	if err := invoke(); err != nil {
		b.Fatalf("warm-up request failed: %v", err)
	}

	b.Run("serial", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := invoke(); err != nil {
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
				if invoke() != nil {
					failures.Add(1)
				}
			}
		})
		if count := failures.Load(); count != 0 {
			b.Fatalf("%d parallel gateway requests failed", count)
		}
	})
}
