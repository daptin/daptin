package server

import (
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	"github.com/graphql-go/handler"
)

func InitializeGraphqlResource(initConfig resource.CmsConfig, cruds map[string]*resource.DbResource, defaultRouter *gin.Engine) {
	graphqlSchema := MakeGraphqlSchema(&initConfig, cruds)

	graphqlHttpHandler := handler.New(&handler.Config{
		Schema:     graphqlSchema,
		Pretty:     true,
		Playground: true,
		GraphiQL:   true,
	})

	// serve HTTP
	defaultRouter.Handle("GET", "/graphql", func(c *gin.Context) {
		graphqlHttpHandler.ServeHTTP(c.Writer, c.Request)
	})
	// serve HTTP
	defaultRouter.Handle("POST", "/graphql", func(c *gin.Context) {
		graphqlHttpHandler.ServeHTTP(c.Writer, c.Request)
	})
	// serve HTTP
	defaultRouter.Handle("PUT", "/graphql", func(c *gin.Context) {
		graphqlHttpHandler.ServeHTTP(c.Writer, c.Request)
	})
	// serve HTTP
	defaultRouter.Handle("PATCH", "/graphql", func(c *gin.Context) {
		graphqlHttpHandler.ServeHTTP(c.Writer, c.Request)
	})
	// serve HTTP
	defaultRouter.Handle("DELETE", "/graphql", func(c *gin.Context) {
		graphqlHttpHandler.ServeHTTP(c.Writer, c.Request)
	})
}
