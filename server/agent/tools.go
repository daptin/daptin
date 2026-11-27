package agent

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/api2go/v2/jsonapi"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/llmgateway/contract"
	"github.com/jmoiron/sqlx"
)

func object(properties map[string]interface{}, required ...string) json.RawMessage {
	if required == nil {
		required = []string{}
	}
	encoded, err := encode(map[string]interface{}{"type": "object", "properties": properties, "required": required, "additionalProperties": false})
	if err != nil {
		panic(err)
	} // These schemas contain only backend JSON literals.
	return json.RawMessage(encoded)
}
func stringField() interface{}     { return map[string]interface{}{"type": "string", "minLength": 1} }
func attributesField() interface{} { return map[string]interface{}{"type": "object"} }
func referenceField() interface{} {
	return map[string]interface{}{"type": "string", "pattern": "^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$"}
}
func tools(config Config) []contract.Tool {
	var result []contract.Tool
	add := func(name, description string, schema json.RawMessage) {
		result = append(result, contract.Tool{Type: "function", Function: contract.FunctionDefinition{Name: name, Description: description, Parameters: schema}})
	}
	if len(config.Toolset.Resources) > 0 || len(config.Toolset.Actions) > 0 || len(config.Toolset.Tools) > 0 {
		add("daptin_discover", "Inspect accessible resource schema, relationships, and actions. Omit entity to list resources; follow next_page until absent.", object(map[string]interface{}{"entity": stringField(), "page": map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 1 << 30}}))
	}
	if len(config.Toolset.Resources) > 0 {
		for _, op := range config.Toolset.Operations {
			properties := map[string]interface{}{"entity": stringField()}
			required := []string{"entity"}
			switch op {
			case "discover":
				continue
			case "list":
				properties["query"] = map[string]interface{}{"type": "object", "additionalProperties": map[string]interface{}{"type": "string"}}
			case "read", "delete":
				properties["reference"] = referenceField()
				required = append(required, "reference")
			case "create":
				properties["attributes"] = attributesField()
				required = append(required, "attributes")
			case "update":
				properties["reference"] = referenceField()
				properties["attributes"] = map[string]interface{}{"type": "object", "description": "Include version and the discovered column names to change, with their new values.", "properties": map[string]interface{}{"version": map[string]interface{}{"type": "integer", "minimum": 1}}, "required": []string{"version"}, "additionalProperties": true}
				required = append(required, "reference", "attributes")
			case "relations":
				properties["reference"] = referenceField()
				properties["relation"] = stringField()
				properties["page"] = map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 1 << 30}
				required = append(required, "reference", "relation")
			}
			add("daptin_"+op, "Use Daptin's permissioned "+op+" resource operation. Updates require attributes.version. Relationship writes use update attributes or relation-resource CRUD.", object(properties, required...))
		}
	}
	if len(config.Toolset.Actions) > 0 {
		add("daptin_action", "Execute an exposed named Daptin action. Discover its input schema first.", object(map[string]interface{}{"entity": stringField(), "action": stringField(), "attributes": attributesField()}, "entity", "action", "attributes"))
	}
	for _, b := range config.Toolset.Tools {
		add(b.Name, b.Description, b.Parameters)
	}
	add("agent_decision", "Wait for an authenticated decision, then receive it as this tool's result.", object(map[string]interface{}{"question": stringField()}, "question"))
	add("agent_wait", "Wait until an RFC3339 UTC timestamp, then continue.", object(map[string]interface{}{"until": stringField()}, "until"))
	return result
}
func validateCall(config Config, call contract.ToolCall) (map[string]interface{}, error) {
	if call.ID == "" || call.Type != "function" {
		return nil, errors.New("invalid tool call identity or type")
	}
	for _, tool := range tools(config) {
		if tool.Function.Name == call.Function.Name {
			args, err := validateArguments(call.Function.Arguments, tool.Function.Parameters)
			if err != nil {
				return nil, err
			}
			entity, _ := args["entity"].(string)
			if strings.HasPrefix(call.Function.Name, "daptin_") && call.Function.Name != "daptin_action" && call.Function.Name != "daptin_discover" && !contains(config.Toolset.Resources, entity) {
				return nil, errors.New("resource is not exposed")
			}
			if call.Function.Name == "daptin_action" {
				name, _ := args["action"].(string)
				if !contains(config.Toolset.Actions, actionKey(entity, name)) {
					return nil, errors.New("action is not exposed")
				}
			}
			return args, nil
		}
	}
	return nil, errors.New("unknown or unexposed tool")
}
func actionKey(entity, name string) string { return entity + "." + name }
func exposesEntity(config Config, entity string) bool {
	if contains(config.Toolset.Resources, entity) || contains(config.Toolset.Actions, "*") {
		return true
	}
	for _, action := range config.Toolset.Actions {
		if strings.HasPrefix(action, entity+".") {
			return true
		}
	}
	for _, binding := range config.Toolset.Tools {
		if binding.Entity == entity {
			return true
		}
	}
	return false
}
func exposesAction(config Config, entity, name string) bool {
	if contains(config.Toolset.Actions, actionKey(entity, name)) {
		return true
	}
	for _, binding := range config.Toolset.Tools {
		if binding.Entity == entity && binding.Action == name {
			return true
		}
	}
	return false
}
func (s store) discover(ctx context.Context, state *runState, entity string, page int, tx *sqlx.Tx) (interface{}, error) {
	user, _ := ctx.Value("user").(*auth.SessionUser)
	if user == nil {
		return nil, errors.New("missing execution account")
	}
	names := make([]string, 0, len(s.cruds))
	for name := range s.cruds {
		if exposesEntity(state.Config, name) && (entity == "" || entity == name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	nextPage := 0
	if entity == "" {
		start := (page - 1) * 100
		if start >= len(names) {
			names = nil
		} else {
			end := start + 100
			if end > len(names) {
				end = len(names)
			} else if end < len(names) {
				nextPage = page + 1
			}
			names = names[start:end]
		}
	}
	var result []interface{}
	for _, name := range names {
		crud := s.cruds[name]
		perm := crud.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", name, tx)
		if !perm.CanPeek(user.UserReferenceId, user.Groups, crud.AdministratorGroupId) {
			continue
		}
		if entity == "" {
			result = append(result, name)
			continue
		}
		var actions []interface{}
		if (len(state.Config.Toolset.Actions) > 0 || len(state.Config.Toolset.Tools) > 0) && perm.CanExecute(user.UserReferenceId, user.Groups, crud.AdministratorGroupId) {
			available, err := crud.GetActionsByType(name, tx)
			if err != nil {
				return nil, err
			}
			for _, action := range available {
				if !exposesAction(state.Config, name, action.Name) {
					continue
				}
				permission := crud.GetObjectPermissionByReferenceId("action", action.ReferenceId, tx)
				if permission.CanExecute(user.UserReferenceId, user.Groups, crud.AdministratorGroupId) {
					actions = append(actions, map[string]interface{}{"name": action.Name, "inputs": action.InFields, "validations": action.Validations, "instance_optional": action.InstanceOptional, "subject_input": name + "_id"})
				}
			}
		}
		columns := []interface{}{}
		for _, column := range crud.TableInfo().Columns {
			if !column.ExcludeFromApi {
				columns = append(columns, map[string]interface{}{"name": column.ColumnName, "type": column.ColumnType, "nullable": column.IsNullable})
			}
		}
		relations := []interface{}{}
		for _, declared := range crud.TableInfo().Relations {
			relation, target := declared.GetObjectName(), declared.GetObject()
			if declared.GetSubject() != name {
				relation, target = declared.GetSubjectName(), declared.GetSubject()
			}
			relations = append(relations, map[string]interface{}{"name": relation, "entity": target, "type": declared.GetRelation()})
		}
		result = append(result, map[string]interface{}{"entity": name, "columns": columns, "relations": relations, "actions": actions})
	}
	if entity == "" {
		response := map[string]interface{}{"resources": result}
		if nextPage > 0 {
			response["next_page"] = nextPage
		}
		return response, nil
	}
	return result, nil
}

type toolRejection struct{ error }

func (s store) dispatch(ctx context.Context, state *runState, operation string, call contract.ToolCall) (result interface{}, err error) {
	started := false
	defer func() {
		if err != nil && !started {
			err = &toolRejection{err}
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	args, err := validateCall(state.Config, call)
	if err != nil {
		return nil, err
	}
	name := call.Function.Name
	entity, _ := args["entity"].(string)
	actionName, _ := args["action"].(string)
	attrs, _ := args["attributes"].(map[string]interface{})
	for _, binding := range state.Config.Toolset.Tools {
		if binding.Name == name {
			entity = binding.Entity
			actionName = binding.Action
			attrs = args
			if binding.IdempotencyInput != "" {
				if _, supplied := attrs[binding.IdempotencyInput]; supplied {
					return nil, errors.New("idempotency input is backend-controlled")
				}
				attrs[binding.IdempotencyInput] = operation
			}
			name = "daptin_action"
			break
		}
	}
	tx, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if name == "daptin_discover" {
		result, err := s.discover(ctx, state, entity, toolPage(args), tx)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return result, nil
	}
	crud := s.cruds[entity]
	if crud == nil {
		return nil, errors.New("unknown entity")
	}
	if name == "daptin_action" {
		started = true
		result, err := crud.HandleActionRequest(actionresponse.ActionRequest{Type: entity, Action: actionName, Attributes: attrs}, request(ctx, "EXECUTE", "/action/"+entity+"/"+actionName, nil), tx)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return result, nil
	}
	if !contains(state.Config.Toolset.Resources, entity) {
		return nil, errors.New("resource is not exposed")
	}
	ref, _ := args["reference"].(string)
	if name != "daptin_list" && name != "daptin_create" && daptinid.InterfaceToDIR(ref) == daptinid.NullReferenceId {
		return nil, errors.New("invalid record reference")
	}
	var response api2go.Responder
	switch name {
	case "daptin_list":
		query := url.Values{}
		if values, ok := args["query"].(map[string]interface{}); ok {
			for key, value := range values {
				query.Set(key, value.(string))
			}
		}
		size, err := boundedPageSize(query.Get("page[size]"))
		if err != nil {
			return nil, err
		}
		// The resource API also accepts offset/limit pagination; its limit takes
		// precedence over page[size], so validate both before entering the core path.
		if _, err := boundedPageSize(query.Get("page[limit]")); err != nil {
			return nil, err
		}
		query.Set("page[size]", strconv.Itoa(size))
		rows, err := s.list(ctx, entity, query, tx)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return rows, nil
	case "daptin_read":
		row, err := s.read(ctx, entity, ref, tx)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return row, nil
	case "daptin_create":
		// A persisted logical-operation reference makes readback possible after a crash.
		if _, ok := attrs["reference_id"]; ok {
			return nil, errors.New("create reference_id is assigned by the run")
		}
		attrs["reference_id"] = operation
		table := crud.TableInfo()
		started = true
		response, err = crud.CreateWithTransaction(api2go.NewApi2GoModelWithData(entity, table.Columns, int64(table.DefaultPermission), table.Relations, attrs), request(ctx, "POST", "/"+entity, nil), tx)
	case "daptin_update":
		if _, supplied := attrs["reference_id"]; supplied {
			return nil, errors.New("update cannot replace reference_id")
		}
		if _, supplied := attrs["id"]; supplied {
			return nil, errors.New("internal record IDs are not tool inputs")
		}
		if _, ok := attrs["version"]; !ok {
			return nil, errors.New("update requires attributes.version")
		}
		loaded, readErr := crud.FindOneWithTransaction(daptinid.InterfaceToDIR(ref), request(ctx, "PATCH", "/"+entity+"/"+ref, nil), tx)
		if readErr != nil {
			return nil, readErr
		}
		original := loaded.Result().(api2go.Api2GoModel).GetAllAsAttributes()
		table := crud.TableInfo()
		model := api2go.NewApi2GoModelWithData(entity, table.Columns, int64(table.DefaultPermission), table.Relations, original)
		model.SetAttributes(attrs)
		// Match the public PATCH precondition before entering the lifecycle.
		wanted, parseErr := resource.ResourceRowInt64(attrs["version"])
		if number, ok := attrs["version"].(float64); ok && number >= 1 && number < math.Exp2(63) && math.Trunc(number) == number {
			wanted = int64(number)
			parseErr = nil
		}
		current, _ := resource.ResourceRowInt64(original["version"])
		if parseErr != nil || wanted < 1 || wanted != current {
			return nil, api2go.NewHTTPError(nil, "resource version conflict", 409)
		}
		attrs["version"] = wanted
		started = true
		response, err = crud.UpdateWithTransaction(model, request(ctx, "PATCH", "/"+entity+"/"+ref, nil), tx)
	case "daptin_delete":
		started = true
		response, err = crud.DeleteWithTransaction(daptinid.InterfaceToDIR(ref), request(ctx, "DELETE", "/"+entity+"/"+ref, nil), tx)
	case "daptin_relations":
		// FindRelationship owns its transaction and performs both parent and row checks.
		_ = tx.Rollback()
		relationName := args["relation"].(string)
		relation := jsonapi.Reference{Name: relationName}
		for _, declared := range crud.TableInfo().Relations {
			name, target := declared.GetObjectName(), declared.GetObject()
			if declared.GetSubject() != entity {
				name, target = declared.GetSubjectName(), declared.GetSubject()
			}
			if name == relationName {
				relation.Type = target
				break
			}
		}
		_, response, err = crud.FindRelationship(ref, relation, request(ctx, "GET", "/"+entity+"/"+ref+"/relationships/"+relationName, url.Values{"page[size]": {"100"}, "page[number]": {strconv.Itoa(toolPage(args))}}))
		if err != nil {
			return nil, err
		}
		return publicResult(response), nil
	default:
		return nil, errors.New("unsupported tool")
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return publicResult(response), nil
}
func publicResult(response api2go.Responder) interface{} {
	if response == nil {
		return nil
	}
	switch value := response.Result().(type) {
	case api2go.Api2GoModel:
		return value.GetAllAsAttributes()
	case []api2go.Api2GoModel:
		rows := make([]map[string]interface{}, len(value))
		for i := range value {
			rows[i] = value[i].GetAllAsAttributes()
		}
		return rows
	default:
		return value
	}
}
func repeatable(config Config, name string) bool {
	switch name {
	case "daptin_discover", "daptin_list", "daptin_read", "daptin_relations":
		return true
	}
	for _, binding := range config.Toolset.Tools {
		if binding.Name == name {
			return binding.Repeatable || binding.IdempotencyInput != ""
		}
	}
	return false
}

func boundedPageSize(raw string) (int, error) {
	if raw == "" {
		return 100, nil
	}
	size, err := strconv.Atoi(raw)
	if err != nil || size < 1 || size > 100 {
		return 0, errors.New("page size must be between 1 and 100")
	}
	return size, nil
}

func toolPage(args map[string]interface{}) int {
	if value, ok := args["page"].(float64); ok && value >= 1 && value <= 1<<30 {
		return int(value)
	}
	return 1
}
