package main

import (
	"context"
	"net/http"
	"sync/atomic"
)

type readinessGate struct {
	handler http.Handler
	check   func(context.Context) error
	ready   atomic.Bool
}

func newReadinessGate(handler http.Handler, check func(context.Context) error) *readinessGate {
	return &readinessGate{handler: handler, check: check}
}

func (g *readinessGate) SetReady(ready bool) {
	g.ready.Store(ready)
}

func (g *readinessGate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/ping":
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pong"))
		return
	case "/ready":
		if !g.ready.Load() || (g.check != nil && g.check(r.Context()) != nil) {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
		return
	}

	if !g.ready.Load() {
		http.Error(w, "server is draining", http.StatusServiceUnavailable)
		return
	}
	g.handler.ServeHTTP(w, r)
}
