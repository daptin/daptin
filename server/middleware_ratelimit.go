package server

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/buraksezer/olric"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

const (
	defaultRateConfigJSON = `{"version":"1","limits":{}}`
	defaultRatePerSecond  = 500
	maxRatePerSecond      = 1_000_000
	rateLimitDMapName     = "http-rate-limit"
)

type RateConfig struct {
	Version string         `json:"version"`
	Limits  map[string]int `json:"limits"`
}

var defaultRateConfig = RateConfig{
	Version: "1",
	Limits:  map[string]int{},
}

type localRateWindow struct {
	window int64
	count  int
}

type localRateCounter struct {
	mu          sync.Mutex
	windows     map[string]localRateWindow
	lastCleanup int64
}

func ParseRateConfig(value string) (RateConfig, error) {
	var config RateConfig
	if err := json.Unmarshal([]byte(value), &config); err != nil {
		return RateConfig{}, fmt.Errorf("parse JSON: %w", err)
	}
	if config.Version != "1" && config.Version != "default" {
		return RateConfig{}, fmt.Errorf("unsupported version %q", config.Version)
	}
	if config.Limits == nil {
		config.Limits = map[string]int{}
	}
	for route, limitValue := range config.Limits {
		if err := validateRateLimitRoute(route); err != nil {
			return RateConfig{}, err
		}
		if limitValue < 1 || limitValue > maxRatePerSecond {
			return RateConfig{}, fmt.Errorf("rate for route %q must be between 1 and %d requests per second", route, maxRatePerSecond)
		}
	}
	return config, nil
}

func validateRateLimitRoute(route string) error {
	if route == "" || route != strings.TrimSpace(route) || strings.ContainsAny(route, "?#") {
		return fmt.Errorf("invalid rate-limit route %q", route)
	}
	if strings.HasPrefix(route, "/") {
		return nil
	}
	if separator := strings.IndexByte(route, '/'); separator > 0 {
		return nil
	}
	return fmt.Errorf("rate-limit route %q must be /path or host/path", route)
}

func CreateRateLimiterMiddleware(rateConfig RateConfig, olricClient *olric.EmbeddedClient) gin.HandlerFunc {
	return createRateLimiterMiddleware(rateConfig, olricClient, func(c *gin.Context) string {
		return strings.Split(c.Request.RequestURI, "?")[0]
	}, time.Now)
}

func CreateSubsiteRateLimiterMiddleware(rateConfig RateConfig, olricClient *olric.EmbeddedClient) gin.HandlerFunc {
	return createRateLimiterMiddleware(rateConfig, olricClient, func(c *gin.Context) string {
		return c.Request.Host + strings.Split(c.Request.RequestURI, "?")[0]
	}, time.Now)
}

func createRateLimiterMiddleware(rateConfig RateConfig, olricClient *olric.EmbeddedClient, routeKey func(*gin.Context) string, now func() time.Time) gin.HandlerFunc {
	var distributedCounter olric.DMap
	if olricClient != nil {
		var err error
		distributedCounter, err = olricClient.NewDMap(rateLimitDMapName)
		if err != nil {
			log.Errorf("Failed to initialize distributed HTTP rate limiter; using process-local fallback: %v", err)
		}
	}
	localCounter := &localRateCounter{windows: make(map[string]localRateWindow)}

	return func(c *gin.Context) {
		route := routeKey(c)
		limitValue, ok := rateConfig.Limits[route]
		if !ok {
			limitValue = defaultRatePerSecond
		}
		window := now().UTC().Unix()
		clientKey := c.ClientIP() + "\x00" + route
		count, err := incrementDistributedRateCounter(c.Request.Context(), distributedCounter, clientKey, window)
		if err != nil {
			if distributedCounter != nil {
				log.Warnf("Distributed HTTP rate limiter failed; using process-local fallback: %v", err)
			}
			count = localCounter.increment(clientKey, window)
		}

		remaining := limitValue - count
		if remaining < 0 {
			remaining = 0
		}
		c.Header("X-RateLimit-Limit", strconv.Itoa(limitValue))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(window+1, 10))
		if count > limitValue {
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"errors": []gin.H{{
					"status": "429",
					"title":  "Rate limit exceeded",
					"detail": "Too many requests. Please retry after the current one-second window.",
				}},
			})
			return
		}
		c.Next()
	}
}

func incrementDistributedRateCounter(ctx context.Context, counter olric.DMap, clientKey string, window int64) (int, error) {
	if counter == nil {
		return 0, errors.New("Olric rate-limit map is unavailable")
	}
	digest := sha256.Sum256([]byte(clientKey))
	key := fmt.Sprintf("%x:%d", digest, window)
	err := counter.Put(ctx, key, 0, olric.NX(), olric.EX(2*time.Second))
	if err != nil && !errors.Is(err, olric.ErrKeyFound) {
		return 0, err
	}
	return counter.Incr(ctx, key, 1)
}

func (counter *localRateCounter) increment(key string, window int64) int {
	counter.mu.Lock()
	defer counter.mu.Unlock()
	if counter.lastCleanup != window {
		for existingKey, existing := range counter.windows {
			if existing.window < window {
				delete(counter.windows, existingKey)
			}
		}
		counter.lastCleanup = window
	}
	current := counter.windows[key]
	if current.window != window {
		current = localRateWindow{window: window}
	}
	current.count++
	counter.windows[key] = current
	return current.count
}
