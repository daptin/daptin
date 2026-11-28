package agent

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
)

type store struct {
	cruds map[string]*resource.DbResource
}

func request(ctx context.Context, method, path string, query url.Values) api2go.Request {
	u, _ := url.Parse(path)
	return api2go.Request{PlainRequest: (&http.Request{Method: method, URL: u}).WithContext(ctx), QueryParams: query}
}
func userContext(ctx context.Context, user *auth.SessionUser) context.Context {
	return context.WithValue(ctx, "user", user)
}
func (s store) begin(ctx context.Context) (*sqlx.Tx, error) {
	return s.cruds["world"].BeginTransaction(ctx)
}
func (s store) load(table, ref string, tx *sqlx.Tx) (map[string]interface{}, error) {
	id := daptinid.InterfaceToDIR(ref)
	if id == daptinid.NullReferenceId {
		return nil, errors.New("invalid reference_id")
	}
	if table == "agentrun" || table == "agentstp" || table == "agenttry" {
		return s.cruds[table].AgentExecutionRow(id, executionFields(table), tx)
	}
	row, _, err := s.cruds[table].GetSingleRowByReferenceIdWithTransaction(table, id, nil, tx)
	return row, err
}
func (s store) read(ctx context.Context, table, ref string, tx *sqlx.Tx) (map[string]interface{}, error) {
	response, err := s.cruds[table].FindOneWithTransaction(daptinid.InterfaceToDIR(ref), request(ctx, "GET", "/"+table+"/"+ref, nil), tx)
	if err != nil {
		return nil, err
	}
	model, ok := response.Result().(api2go.Api2GoModel)
	if !ok {
		return nil, fmt.Errorf("invalid %s response", table)
	}
	return model.GetAllAsAttributes(), nil
}

// Controls use the normal resource permission chain with a sparse projection;
// initial input and attempt bodies are not part of a control request.
func (s store) controlRun(ctx context.Context, ref string, tx *sqlx.Tx) (map[string]interface{}, error) {
	filter, err := encode([]map[string]interface{}{{"column": "reference_id", "operator": "is", "value": ref}})
	if err != nil {
		return nil, err
	}
	rows, err := s.list(ctx, "agentrun", url.Values{"query": {filter}, "fields": {strings.Join(append(executionFields("agentrun"), "config"), ",")}, "page[size]": {"1"}}, tx)
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, errors.New("agent run is not readable")
	}
	return rows[0], nil
}
func (s store) create(ctx context.Context, table string, attrs map[string]interface{}, tx *sqlx.Tx) (map[string]interface{}, error) {
	t := s.cruds[table].TableInfo()
	response, err := s.cruds[table].CreateAgentExecution(api2go.NewApi2GoModelWithData(table, t.Columns, int64(t.DefaultPermission), t.Relations, attrs),
		request(resource.WithMeteringInternal(ctx), "POST", "/"+table, nil), tx)
	if err != nil {
		return nil, err
	}
	return response.Result().(api2go.Api2GoModel).GetAllAsAttributes(), nil
}
func (s store) update(ctx context.Context, table string, row, attrs map[string]interface{}, tx *sqlx.Tx) (map[string]interface{}, error) {
	if user, _ := ctx.Value("user").(*auth.SessionUser); user == nil {
		owner, err := s.user(daptinid.InterfaceToDIR(row["user_account_id"]).String(), tx)
		if err != nil {
			return nil, err
		}
		ctx = userContext(ctx, owner)
	}
	t := s.cruds[table].TableInfo()
	model := api2go.NewApi2GoModelWithData(table, t.Columns, int64(t.DefaultPermission), t.Relations, row)
	// The resource model's structural comparison cannot distinguish time.Time
	// values (their fields are private). Supply the canonical datetime wire value
	// so lease renewals are tracked and persisted by the normal resource path.
	for name, value := range attrs {
		if instant, ok := value.(time.Time); ok {
			attrs[name] = instant.UTC().Format(time.RFC3339Nano)
		}
	}
	model.SetAttributes(attrs)
	response, err := s.cruds[table].UpdateAgentExecution(model, request(resource.WithMeteringInternal(ctx), "PATCH", "/"+table+"/"+reference(row), nil), tx)
	if err != nil {
		return nil, err
	}
	return response.Result().(api2go.Api2GoModel).GetAllAsAttributes(), nil
}
func reference(row map[string]interface{}) string {
	return daptinid.InterfaceToDIR(row["reference_id"]).String()
}
func (s store) list(ctx context.Context, table string, query url.Values, tx *sqlx.Tx) ([]map[string]interface{}, error) {
	if query == nil {
		query = url.Values{}
	}
	if query.Get("page[size]") == "" {
		query.Set("page[size]", "100")
	}
	_, response, err := s.cruds[table].PaginatedFindAllWithTransaction(request(ctx, "GET", "/"+table, query), tx)
	if err != nil {
		return nil, err
	}
	models, ok := response.Result().([]api2go.Api2GoModel)
	if !ok {
		return nil, fmt.Errorf("invalid %s list response", table)
	}
	rows := make([]map[string]interface{}, len(models))
	for i := range models {
		rows[i] = models[i].GetAllAsAttributes()
	}
	return rows, nil
}
func (s store) user(ref string, tx *sqlx.Tx) (*auth.SessionUser, error) {
	row, err := s.load("user_account", ref, tx)
	if err != nil {
		return nil, err
	}
	id, err := resource.ResourceRowInt64(row["id"])
	if err != nil || id <= 0 {
		return nil, errors.New("invalid execution account")
	}
	version := auth.AuthVersionOrDefault(row[auth.AuthVersionColumn])
	groups := s.cruds["user_account"].GetObjectUserGroupsByWhereWithTransaction("user_account", tx, "id", id)
	return &auth.SessionUser{UserId: id, UserReferenceId: daptinid.InterfaceToDIR(row["reference_id"]), Groups: groups, AuthVersion: version}, nil
}
func (s store) clock(tx *sqlx.Tx) (time.Time, error) {
	v, err := resource.AgentDatabaseTime(tx)
	if err != nil {
		return time.Time{}, err
	}
	return v, nil
}
func (s store) authorizeRun(ctx context.Context, row map[string]interface{}, tx *sqlx.Tx) (*auth.SessionUser, error) {
	user, err := s.user(daptinid.InterfaceToDIR(row["user_account_id"]).String(), tx)
	if err != nil {
		return nil, err
	}
	version, err := resource.ResourceRowInt64(row["auth_version"])
	if err != nil || version != user.AuthVersion {
		return nil, errors.New("background delegation revoked")
	}
	crud := s.cruds["agentrun"]
	perm := crud.GetObjectPermissionByReferenceId("agentrun", daptinid.InterfaceToDIR(row["reference_id"]), tx)
	if !perm.CanExecute(user.UserReferenceId, user.Groups, crud.AdministratorGroupId) {
		return nil, errors.New("run execute permission revoked")
	}
	return user, nil
}
