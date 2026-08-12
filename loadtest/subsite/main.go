package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/daptin/daptin/server"
	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/rootpojo"
	"github.com/daptin/daptin/server/subsite"
	"github.com/gin-gonic/gin"
)

type result struct {
	latency time.Duration
	status  int
	err     error
}

type summary struct {
	URL             string  `json:"url"`
	DurationSeconds float64 `json:"duration_seconds"`
	Concurrency     int     `json:"concurrency"`
	Requests        int64   `json:"requests"`
	RequestsPerSec  float64 `json:"requests_per_second"`
	Successes       int64   `json:"successes"`
	Errors          int64   `json:"errors"`
	LatencyP50Ms    float64 `json:"latency_p50_ms"`
	LatencyP95Ms    float64 `json:"latency_p95_ms"`
	LatencyP99Ms    float64 `json:"latency_p99_ms"`
	LatencyMaxMs    float64 `json:"latency_max_ms"`
	BytesRead       int64   `json:"bytes_read"`
	MiBPerSec       float64 `json:"mib_per_second"`
	GoVersion       string  `json:"go_version"`
	GOOS            string  `json:"goos"`
	GOARCH          string  `json:"goarch"`
	GOMAXPROCS      int     `json:"gomaxprocs"`
}

func main() {
	mode := flag.String("mode", "load", "load or serve")
	listen := flag.String("listen", "127.0.0.1:18080", "listen address in serve mode")
	directory := flag.String("dir", "", "directory to serve in serve mode")
	url := flag.String("url", "http://127.0.0.1:18080/app.js", "URL to load")
	duration := flag.Duration("duration", 10*time.Second, "measurement duration")
	concurrency := flag.Int("concurrency", 100, "number of workers")
	header := flag.String("header", "", "optional request header as Name: value")
	output := flag.String("output", "", "optional JSON output file")
	flag.Parse()

	if *mode == "serve" {
		serve(*listen, *directory)
		return
	}
	if err := load(*url, *duration, *concurrency, *header, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serve(address, directory string) {
	if directory == "" {
		fmt.Fprintln(os.Stderr, "-dir is required in serve mode")
		os.Exit(2)
	}
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard
	cache := &assetcachepojo.AssetFolderCache{
		LocalSyncPath: directory,
		CloudStore: rootpojo.CloudStore{
			StoreProvider: "loadtest-cache",
		},
	}
	engine := server.CreateSubsiteEngine(subsite.SubSite{}, cache, nil, true)
	httpServer := &http.Server{
		Addr:              address,
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	fmt.Printf("serving %s on http://%s\n", directory, address)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func load(url string, duration time.Duration, concurrency int, header, output string) error {
	if concurrency < 1 || duration <= 0 {
		return fmt.Errorf("concurrency and duration must be positive")
	}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns:          concurrency,
		MaxIdleConnsPerHost:   concurrency,
		MaxConnsPerHost:       concurrency,
		IdleConnTimeout:       30 * time.Second,
		DisableCompression:    true,
		ResponseHeaderTimeout: 5 * time.Second,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	atomic.StoreInt64(&totalBytes, 0)

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	results := make(chan result, concurrency*2)
	start := time.Now()
	var workers sync.WaitGroup
	for range concurrency {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for ctx.Err() == nil {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
				if err != nil {
					results <- result{err: err}
					return
				}
				if header != "" {
					for i := 0; i < len(header); i++ {
						if header[i] == ':' {
							req.Header.Set(header[:i], trimSpace(header[i+1:]))
							break
						}
					}
				}
				began := time.Now()
				response, err := client.Do(req)
				latency := time.Since(began)
				if err != nil {
					if ctx.Err() == nil {
						results <- result{latency: latency, err: err}
					}
					continue
				}
				bytesRead, readErr := io.Copy(io.Discard, response.Body)
				_ = response.Body.Close()
				if readErr != nil && ctx.Err() != nil {
					continue
				}
				results <- result{latency: latency, status: response.StatusCode, err: readErr}
				atomic.AddInt64(&totalBytes, bytesRead)
			}
		}()
	}
	go func() {
		workers.Wait()
		close(results)
	}()

	var latencies []time.Duration
	var successes, errors int64
	for item := range results {
		if item.err == nil && item.status >= 200 && item.status < 400 {
			successes++
		} else {
			errors++
		}
		latencies = append(latencies, item.latency)
	}
	elapsed := time.Since(start)
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	total := successes + errors
	report := summary{
		URL:             url,
		DurationSeconds: elapsed.Seconds(),
		Concurrency:     concurrency,
		Requests:        total,
		RequestsPerSec:  float64(total) / elapsed.Seconds(),
		Successes:       successes,
		Errors:          errors,
		LatencyP50Ms:    milliseconds(percentile(latencies, 0.50)),
		LatencyP95Ms:    milliseconds(percentile(latencies, 0.95)),
		LatencyP99Ms:    milliseconds(percentile(latencies, 0.99)),
		LatencyMaxMs:    milliseconds(percentile(latencies, 1)),
		BytesRead:       atomic.LoadInt64(&totalBytes),
		MiBPerSec:       float64(atomic.LoadInt64(&totalBytes)) / elapsed.Seconds() / (1024 * 1024),
		GoVersion:       runtime.Version(),
		GOOS:            runtime.GOOS,
		GOARCH:          runtime.GOARCH,
		GOMAXPROCS:      runtime.GOMAXPROCS(0),
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))
	if output != "" {
		return os.WriteFile(output, append(encoded, '\n'), 0o644)
	}
	return nil
}

var totalBytes int64

func trimSpace(value string) string {
	for len(value) > 0 && (value[0] == ' ' || value[0] == '\t') {
		value = value[1:]
	}
	return value
}

func percentile(values []time.Duration, fraction float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1) * fraction)
	return values[index]
}

func milliseconds(value time.Duration) float64 {
	return float64(value) / float64(time.Millisecond)
}
