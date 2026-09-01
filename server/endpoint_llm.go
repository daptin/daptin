package server

import (
	"net/http"

	"github.com/daptin/daptin/server/llm"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// RegisterLLMEndpoints mounts the reusable gateway's sole HTTP implementation.
// Daptin's authentication middleware has already attached the SessionUser to
// the request context before the gateway authenticates and authorizes it.
func RegisterLLMEndpoints(router *gin.Engine, gateway *llm.Gateway) {
	handler := gin.WrapH(gateway.Handler())
	router.Any("/v1/*path", handler)
	healthHandler := gin.WrapH(http.StripPrefix("/llm", gateway.Handler()))
	router.GET("/llm/healthz", healthHandler)
	router.GET("/llm/readyz", healthHandler)
	log.Info("[llm] registered shared gateway endpoints under /v1 and /llm")
}
