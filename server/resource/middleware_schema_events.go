package resource

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/dop251/goja"
	"github.com/jmoiron/sqlx"
)

// Schema event handlers run inside the resource lifecycle. External after
// effects are handled by the durable exchange path, not by this interceptor.
type schemaEventMiddleware struct{}

func NewSchemaEventMiddleware() DatabaseRequestInterceptor { return &schemaEventMiddleware{} }

func (*schemaEventMiddleware) String() string { return "SchemaEventHandlers" }

var schemaEventTemplate = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)
var schemaEventEnvTemplate = regexp.MustCompile(`\$\{env\.([A-Za-z_][A-Za-z0-9_]*)\}`)

func (*schemaEventMiddleware) InterceptBefore(dr *DbResource, req *api2go.Request,
	objects []map[string]interface{}, transaction *sqlx.Tx) ([]map[string]interface{}, error) {
	operation := schemaEventOperation(req.PlainRequest.Method)
	if operation == "" || dr.tableInfo == nil || len(dr.tableInfo.EventHandlers) == 0 {
		return objects, nil
	}
	hasBeforeHandler := false
	for _, handler := range dr.tableInfo.EventHandlers {
		if handler.Event == "before:"+operation {
			hasBeforeHandler = true
			break
		}
	}
	if !hasBeforeHandler {
		return objects, nil
	}
	for _, input := range objects {
		var oldRecord map[string]interface{}
		current := make(map[string]interface{}, len(input))
		if operation != "create" {
			reference := daptinid.InterfaceToDIR(input["reference_id"])
			if reference == daptinid.NullReferenceId {
				return nil, fmt.Errorf("event handler requires a record reference_id")
			}
			row, err := dr.GetReferenceIdToObjectWithTransaction(dr.model.GetTableName(), reference, transaction)
			if err != nil {
				return nil, err
			}
			oldRecord = schemaEventPublicRecord(row)
			for key, value := range row {
				current[key] = value
			}
		}
		for key, value := range input {
			current[key] = value
		}
		for _, handler := range dr.tableInfo.EventHandlers {
			if handler.Event != "before:"+operation {
				continue
			}
			context := schemaEventContext(dr, req, current)
			context["old"] = oldRecord
			allowed, err := schemaEventCondition(handler.Condition, context)
			if err != nil {
				return nil, api2go.NewHTTPError(err, "event condition failed", 400)
			}
			if !allowed {
				continue
			}
			switch handler.Handler {
			case "validation":
				valid, err := schemaEventCondition(fmt.Sprint(handler.Attributes["Condition"]), context)
				if err != nil {
					return nil, api2go.NewHTTPError(err, "event validation failed", 400)
				}
				if !valid {
					return nil, api2go.NewHTTPError(fmt.Errorf("%v", handler.Attributes["Message"]),
						"event validation failed", 400)
				}
			case "conformation":
				for name, value := range handler.Attributes {
					resolved, err := schemaEventValue(value, context)
					if err != nil {
						return nil, api2go.NewHTTPError(err, "event conformation failed", 400)
					}
					input[name] = resolved
					current[name] = resolved
				}
			case "js":
				changed, err := runSchemaEventScript(fmt.Sprint(handler.Attributes["Script"]), input, context)
				if err != nil {
					return nil, api2go.NewHTTPError(err, "event script failed", 400)
				}
				for name, value := range changed {
					input[name] = value
					current[name] = value
				}
			}
		}
	}
	return objects, nil
}

func (*schemaEventMiddleware) InterceptAfter(_ *DbResource, _ *api2go.Request,
	results []map[string]interface{}, _ *sqlx.Tx) ([]map[string]interface{}, error) {
	return results, nil
}

func schemaEventOperation(method string) string {
	switch strings.ToUpper(method) {
	case "POST":
		return "create"
	case "PATCH", "PUT":
		return "update"
	case "DELETE":
		return "delete"
	default:
		return ""
	}
}

func schemaEventContext(dr *DbResource, req *api2go.Request, record map[string]interface{}) map[string]interface{} {
	userID := ""
	if user, ok := req.PlainRequest.Context().Value("user").(*auth.SessionUser); ok && user != nil {
		userID = user.UserReferenceId.String()
	}
	return schemaEventRenderContext(record, dr.envMap, userID)
}

func schemaEventRenderContext(record map[string]interface{}, env map[string]string, userID string) map[string]interface{} {
	context := make(map[string]interface{}, len(record)+4)
	publicRecord := schemaEventPublicRecord(record)
	for key, value := range publicRecord {
		context[key] = value
	}
	context["record"] = publicRecord
	context["env"] = env
	context["now"] = time.Now().UTC().Format(time.RFC3339Nano)
	if timestamp := StringOrEmpty(record["__event_timestamp"]); timestamp != "" {
		context["now"] = timestamp
	}
	context["user"] = map[string]interface{}{"id": userID}
	return context
}

func schemaEventPublicRecord(record map[string]interface{}) map[string]interface{} {
	public := make(map[string]interface{}, len(record))
	for key, value := range record {
		if key != "id" && !strings.HasPrefix(key, "__") {
			public[key] = value
		}
	}
	return public
}

func schemaEventLookup(path string, context map[string]interface{}) (interface{}, error) {
	path = strings.TrimSpace(strings.TrimPrefix(path, "."))
	if path == "" {
		return context["record"], nil
	}
	var value interface{} = context
	for _, part := range strings.Split(path, ".") {
		mapping, ok := value.(map[string]interface{})
		if !ok {
			if env, isEnv := value.(map[string]string); isEnv {
				var exists bool
				value, exists = env[part]
				if !exists {
					return nil, fmt.Errorf("event template path %q is unavailable", path)
				}
				continue
			}
			return nil, fmt.Errorf("event template path %q is not an object", path)
		}
		var exists bool
		value, exists = mapping[part]
		if !exists {
			return nil, fmt.Errorf("event template path %q is unavailable", path)
		}
	}
	return value, nil
}

func schemaEventValue(raw interface{}, context map[string]interface{}) (interface{}, error) {
	switch value := raw.(type) {
	case string:
		var envErr error
		value = schemaEventEnvTemplate.ReplaceAllStringFunc(value, func(match string) string {
			name := strings.TrimSuffix(strings.TrimPrefix(match, "${env."), "}")
			resolved, err := schemaEventLookup("env."+name, context)
			if err != nil {
				envErr = err
				return ""
			}
			return fmt.Sprint(resolved)
		})
		if envErr != nil {
			return nil, envErr
		}
		matches := schemaEventTemplate.FindAllStringSubmatchIndex(value, -1)
		if len(matches) == 0 {
			return value, nil
		}
		if len(matches) == 1 && matches[0][0] == 0 && matches[0][1] == len(value) {
			return schemaEventLookup(value[matches[0][2]:matches[0][3]], context)
		}
		var rendered strings.Builder
		cursor := 0
		for _, match := range matches {
			rendered.WriteString(value[cursor:match[0]])
			resolved, err := schemaEventLookup(value[match[2]:match[3]], context)
			if err != nil {
				return nil, err
			}
			rendered.WriteString(fmt.Sprint(resolved))
			cursor = match[1]
		}
		rendered.WriteString(value[cursor:])
		return rendered.String(), nil
	case map[string]interface{}:
		result := make(map[string]interface{}, len(value))
		for key, item := range value {
			resolved, err := schemaEventValue(item, context)
			if err != nil {
				return nil, err
			}
			result[key] = resolved
		}
		return result, nil
	case []interface{}:
		result := make([]interface{}, len(value))
		for index, item := range value {
			resolved, err := schemaEventValue(item, context)
			if err != nil {
				return nil, err
			}
			result[index] = resolved
		}
		return result, nil
	default:
		return value, nil
	}
}

func schemaEventCondition(expression string, context map[string]interface{}) (bool, error) {
	if strings.TrimSpace(expression) == "" {
		return true, nil
	}
	var rendered strings.Builder
	cursor := 0
	for _, match := range schemaEventTemplate.FindAllStringSubmatchIndex(expression, -1) {
		rendered.WriteString(expression[cursor:match[0]])
		value, err := schemaEventLookup(expression[match[2]:match[3]], context)
		if err != nil {
			return false, err
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return false, err
		}
		rendered.Write(encoded)
		cursor = match[1]
	}
	rendered.WriteString(expression[cursor:])
	vm := goja.New()
	interrupt := time.AfterFunc(100*time.Millisecond, func() { vm.Interrupt("event condition timed out") })
	defer interrupt.Stop()
	result, err := vm.RunString(rendered.String())
	if err != nil {
		return false, fmt.Errorf("event condition %q: %w", expression, err)
	}
	return result.ToBoolean(), nil
}

func runSchemaEventScript(script string, input map[string]interface{}, context map[string]interface{}) (map[string]interface{}, error) {
	vm := goja.New()
	vm.Set("input", input)
	vm.Set("old", context["old"])
	vm.Set("user", context["user"])
	vm.Set("env", context["env"])
	vm.Set("now", context["now"])
	interrupt := time.AfterFunc(100*time.Millisecond, func() { vm.Interrupt("event script timed out") })
	defer interrupt.Stop()
	result, err := vm.RunString("(function(input){" + script + "})(input)")
	if err != nil {
		return nil, err
	}
	changed, ok := result.Export().(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("event script must return a record object")
	}
	return changed, nil
}
