package server

import (
	"github.com/gin-gonic/gin"
	"github.com/yangxikun/gin-limit-by-key"
	"golang.org/x/time/rate"
	"strings"
	"time"
)

func CreateRateLimiterMiddleware(rateConfig RateConfig) gin.HandlerFunc {
	return limit.NewRateLimiter(func(c *gin.Context) string {
		requestPath := strings.Split(c.Request.RequestURI, "?")[0]
		return c.ClientIP() + requestPath // limit rate by client ip + url
	}, func(c *gin.Context) (*rate.Limiter, time.Duration) {
		requestPath := strings.Split(c.Request.RequestURI, "?")[0]
		ratePerSecond, ok := rateConfig.limits[requestPath]
		if !ok {
			ratePerSecond = 500
		}
		microSecondRateGap := int(1000000 / ratePerSecond)
		return rate.NewLimiter(rate.Every(time.Duration(microSecondRateGap)*time.Microsecond),
			ratePerSecond,
		), time.Minute // limit 10 qps/clientIp and permit bursts of at most 10 tokens, and the limiter liveness time duration is 1 hour
	}, func(c *gin.Context) {
		c.AbortWithStatus(429) // handle exceed rate limit request
	})
}
