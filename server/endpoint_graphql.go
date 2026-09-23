package server

import (
	"bytes"
	"io"
	"net/http"

	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/parser"
	"github.com/graphql-go/handler"
)

const maxGraphQLOperationSelections = 256

// graphQLOperationWithinSelectionLimit counts fragment expansions as often as
// they are selected, so aliases and repeated fragments cannot hide work.
func graphQLOperationWithinSelectionLimit(request *http.Request, body []byte) bool {
	clone := request.Clone(request.Context())
	if body != nil {
		clone.Body = io.NopCloser(bytes.NewReader(body))
	}
	options := handler.NewRequestOptions(clone)
	if options.Query == "" {
		return true
	}
	document, err := parser.Parse(parser.ParseParams{Source: options.Query})
	if err != nil {
		return true // The GraphQL handler reports malformed queries.
	}
	fragments := make(map[string]*ast.FragmentDefinition)
	for _, definition := range document.Definitions {
		if fragment, ok := definition.(*ast.FragmentDefinition); ok && fragment.Name != nil {
			fragments[fragment.Name.Value] = fragment
		}
	}
	for _, definition := range document.Definitions {
		operation, ok := definition.(*ast.OperationDefinition)
		if !ok {
			continue
		}
		if options.OperationName != "" && (operation.Name == nil || operation.Name.Value != options.OperationName) {
			continue
		}
		count := 0
		if !countGraphQLSelections(operation.SelectionSet, fragments, make(map[string]bool), &count) {
			return false
		}
	}
	return true
}

func countGraphQLSelections(set *ast.SelectionSet, fragments map[string]*ast.FragmentDefinition, visiting map[string]bool, count *int) bool {
	if set == nil {
		return true
	}
	for _, selection := range set.Selections {
		(*count)++
		if *count > maxGraphQLOperationSelections {
			return false
		}
		switch selected := selection.(type) {
		case *ast.Field:
			if !countGraphQLSelections(selected.SelectionSet, fragments, visiting, count) {
				return false
			}
		case *ast.InlineFragment:
			if !countGraphQLSelections(selected.SelectionSet, fragments, visiting, count) {
				return false
			}
		case *ast.FragmentSpread:
			if selected.Name == nil {
				continue
			}
			name := selected.Name.Value
			if visiting[name] {
				return false
			}
			fragment := fragments[name]
			if fragment == nil {
				continue // The GraphQL handler reports unknown fragments.
			}
			visiting[name] = true
			valid := countGraphQLSelections(fragment.SelectionSet, fragments, visiting, count)
			delete(visiting, name)
			if !valid {
				return false
			}
		}
	}
	return true
}

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
