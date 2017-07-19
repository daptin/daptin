package gingonic

import (
  "net/http"
  gin "gopkg.in/gin-gonic/gin.v1"
  "github.com/artpar/api2go/routing"
)

type ginRouter struct {
  router *gin.Engine
}

func (g ginRouter) Handler() http.Handler {
  return g.router
}

func (g ginRouter) Handle(protocol, route string, handler routing.HandlerFunc) {
  wrappedCallback := func(c *gin.Context) {
    params := map[string]string{}
    for _, p := range c.Params {
      params[p.Key] = p.Value
    }

    handler(c.Writer, c.Request, params)
  }

  g.router.Handle(protocol, route, wrappedCallback)
}

//New creates a new api2go router to use with the gin framework
func New(g *gin.Engine) routing.Routeable {
  return &ginRouter{router: g}
}
