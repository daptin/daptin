package subsite

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

// ErrInvalidTemplateRoutes identifies persisted configuration that cannot be
// mounted. The runtime keeps its built-in routes available for administration.
var ErrInvalidTemplateRoutes = errors.New("invalid template routing configuration")

type templateRoutePlan struct {
	row      map[string]interface{}
	patterns []string
}

// registerTemplateRoutes compiles the whole persisted set before touching any
// live router. Gin itself checks collisions, including wildcard semantics.
func registerTemplateRoutes(rows []map[string]interface{}, routers []*gin.Engine, creator func(map[string]interface{}) func(*gin.Context)) error {
	plans := make([]templateRoutePlan, 0, len(rows))
	for _, row := range rows {
		label := fmt.Sprintf("template %v (%v)", row["name"], row["reference_id"])
		name, valid := row["name"].(string)
		if !valid || name == "" {
			return fmt.Errorf("%w: %s: missing template name", ErrInvalidTemplateRoutes, label)
		}
		encoded, valid := row["url_pattern"].(string)
		var patterns []string
		if !valid {
			return fmt.Errorf("%w: %s: url_pattern must be a JSON array of paths", ErrInvalidTemplateRoutes, label)
		}
		if err := json.Unmarshal([]byte(encoded), &patterns); err != nil || patterns == nil {
			return fmt.Errorf("%w: %s: url_pattern must be a JSON array of paths", ErrInvalidTemplateRoutes, label)
		}
		if len(patterns) == 0 {
			continue
		}
		// These fields are consumed when the routed handler is constructed. Reject
		// malformed stored types before any route is installed.
		for _, field := range []string{"action_config", "cache_config"} {
			if row[field] != nil {
				if _, valid := row[field].(string); !valid {
					return fmt.Errorf("%w: %s: %s must contain JSON text", ErrInvalidTemplateRoutes, label, field)
				}
			}
		}
		for _, path := range patterns {
			if !strings.HasPrefix(path, "/") {
				return fmt.Errorf("%w: %s: pattern %q must start with /", ErrInvalidTemplateRoutes, label, path)
			}
		}
		plans = append(plans, templateRoutePlan{row: row, patterns: patterns})
	}
	// Stable ordering makes diagnostics independent of database/map iteration.
	sort.Slice(plans, func(i, j int) bool { return plans[i].row["name"].(string) < plans[j].row["name"].(string) })
	unique := make([]*gin.Engine, 0, len(routers))
	seen := make(map[*gin.Engine]bool, len(routers))
	layouts := make(map[string]bool)
	placeholder := func(*gin.Context) {}
	for _, router := range routers {
		if seen[router] {
			continue
		}
		seen[router] = true
		unique = append(unique, router)
		routes := router.Routes()
		sort.Slice(routes, func(i, j int) bool {
			if routes[i].Method != routes[j].Method {
				return routes[i].Method < routes[j].Method
			}
			return routes[i].Path < routes[j].Path
		})
		var key strings.Builder
		fmt.Fprintf(&key, "%d\n", len(router.Handlers))
		for _, route := range routes {
			fmt.Fprintf(&key, "%s %q\n", route.Method, route.Path)
		}
		if layouts[key.String()] {
			continue
		}
		layouts[key.String()] = true
		scratch := gin.New()
		scratch.Handlers = router.Handlers
		for _, route := range routes {
			scratch.Handle(route.Method, route.Path, placeholder)
		}
		for _, plan := range plans {
			for _, path := range plan.patterns {
				if err := checkTemplateRoute(scratch, path); err != nil {
					return fmt.Errorf("%w: template %v (%v), pattern %q: %v", ErrInvalidTemplateRoutes, plan.row["name"], plan.row["reference_id"], path, err)
				}
			}
		}
	}
	handlers := make([]gin.HandlerFunc, len(plans))
	for i, plan := range plans {
		handlers[i] = creator(plan.row)
	}
	for i, plan := range plans {
		for _, path := range plan.patterns {
			for _, router := range unique {
				router.Any(path, handlers[i])
			}
		}
	}
	return nil
}

// Gin rejects invalid registrations with panic. Contain that API only on the
// disposable compiler engine, never around a partially modified live router.
func checkTemplateRoute(router *gin.Engine, path string) (err error) {
	defer func() {
		if cause := recover(); cause != nil {
			err = fmt.Errorf("%v", cause)
		}
	}()
	router.Any(path, func(*gin.Context) {})
	return nil
}
