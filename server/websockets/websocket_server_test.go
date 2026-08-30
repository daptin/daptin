package websockets

import (
	"context"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestServerShutdownStopsListener(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewServer("/live", nil, nil, nil)
	router := gin.New()
	go server.Listen(router)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown returned an error: %v", err)
	}
}
