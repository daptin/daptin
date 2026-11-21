package resource

import (
	"net/http"
	"testing"

	"github.com/artpar/api2go/v2"
)

func TestRelatedUsergroupRelation(t *testing.T) {
	access := api2go.TableRelation{Subject: "document", Object: "usergroup", Relation: "has_many"}
	custom := api2go.TableRelation{Subject: "custom", SubjectName: "parent_id", Object: "usergroup", Relation: "has_many"}
	unrelated := api2go.TableRelation{Subject: "other", Object: "user_account", Relation: "has_many"}
	toOne := api2go.TableRelation{Subject: "single", Object: "usergroup", Relation: "belongs_to"}
	tests := []struct {
		name, table string
		params      map[string][]string
		want        string
		invalid     bool
	}{
		{name: "ordinary list", table: "usergroup"},
		{name: "unrelated name", table: "usergroup", params: map[string][]string{"displayName": {"hello"}}},
		{name: "reverse two parameters", table: "usergroup", params: map[string][]string{"documentName": {"usergroup_id"}, "document_id": {"reference"}}, want: access.GetJoinTableName()},
		{name: "forward", table: "document", params: map[string][]string{"usergroupName": {"document_id"}, "usergroup_id": {"reference"}}, want: access.GetJoinTableName()},
		{name: "custom subject column", table: "usergroup", params: map[string][]string{"customName": {"usergroup_id"}, "custom_id": {"reference"}}, want: custom.GetJoinTableName()},
		{name: "custom forward", table: "custom", params: map[string][]string{"usergroupName": {"parent_id"}, "usergroup_id": {"reference"}}, want: custom.GetJoinTableName()},
		{name: "advisory payload", table: "usergroup", params: map[string][]string{"id`,(SELECT password FROM user_account LIMIT 1)-- Name": {"usergroup_id"}, "a": {"1"}, "b": {"2"}}, invalid: true},
		{name: "unknown identifier", table: "usergroup", params: map[string][]string{"missingName": {"usergroup_id"}, "missing_id": {"reference"}}, invalid: true},
		{name: "real unrelated relation", table: "usergroup", params: map[string][]string{"otherName": {"usergroup_id"}, "other_id": {"reference"}}, invalid: true},
		{name: "wrong relation kind", table: "usergroup", params: map[string][]string{"singleName": {"usergroup_id"}, "single_id": {"reference"}}, invalid: true},
		{name: "missing parent", table: "usergroup", params: map[string][]string{"documentName": {"usergroup_id"}}, invalid: true},
		{name: "empty parent", table: "usergroup", params: map[string][]string{"documentName": {"usergroup_id"}, "document_id": {""}}, invalid: true},
		{name: "multiple values", table: "usergroup", params: map[string][]string{"documentName": {"usergroup_id", "usergroup_id"}, "document_id": {"reference"}}, invalid: true},
		{name: "ambiguous selectors", table: "usergroup", params: map[string][]string{"documentName": {"usergroup_id"}, "document_id": {"reference"}, "customName": {"usergroup_id"}, "custom_id": {"reference"}}, invalid: true},
		{name: "forward empty selector", table: "document", params: map[string][]string{"usergroupName": {}, "usergroup_id": {"reference"}}, invalid: true},
		{name: "forward unknown column", table: "document", params: map[string][]string{"usergroupName": {"other_id"}, "usergroup_id": {"reference"}}, invalid: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crud := &DbResource{model: api2go.NewApi2GoModel(tt.table, nil, 0, []api2go.TableRelation{access, custom, unrelated, toOne})}
			relation, err := crud.relatedUsergroupRelation(tt.params)
			if tt.invalid {
				// No request or transaction is provided: invalid selectors must fail
				// before identity lookups, permission queries, or SQL preparation.
				_, _, _, _, err = crud.PaginatedFindAllWithoutFilters(api2go.Request{QueryParams: tt.params}, nil)
				httpErr, ok := err.(api2go.HTTPError)
				if !ok || httpErr.Status() != http.StatusBadRequest {
					t.Fatalf("error = %v, want HTTP 400", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.want == "" {
				if relation != nil {
					t.Fatalf("unexpected relation: %v", relation)
				}
			} else if relation == nil || relation.GetJoinTableName() != tt.want {
				t.Fatalf("relation = %v, want %s", relation, tt.want)
			}
		})
	}
}
