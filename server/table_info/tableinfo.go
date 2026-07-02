package table_info

import (
	"encoding/json"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/columns"
	"github.com/daptin/daptin/server/fsm"
)

type TableRelation struct {
	api2go.TableRelation
	OnDelete string
}

type DefaultGroupBinding struct {
	Name       string
	Permission *auth.AuthPermission
}

type DefaultGroupList []DefaultGroupBinding

func DefaultGroups(names ...string) DefaultGroupList {
	groups := make(DefaultGroupList, 0, len(names))
	for _, name := range names {
		groups = append(groups, DefaultGroupBinding{Name: name})
	}
	return groups
}

func (groups DefaultGroupList) Names() []string {
	names := make([]string, 0, len(groups))
	for _, group := range groups {
		if group.Name != "" {
			names = append(names, group.Name)
		}
	}
	return names
}

func (groups *DefaultGroupList) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*groups = nil
		return nil
	}

	var rawGroups []json.RawMessage
	if err := json.Unmarshal(data, &rawGroups); err != nil {
		return err
	}

	result := make(DefaultGroupList, 0, len(rawGroups))
	for _, rawGroup := range rawGroups {
		var name string
		if err := json.Unmarshal(rawGroup, &name); err == nil {
			result = append(result, DefaultGroupBinding{Name: name})
			continue
		}

		var binding DefaultGroupBinding
		if err := json.Unmarshal(rawGroup, &binding); err != nil {
			return err
		}
		result = append(result, binding)
	}

	*groups = result
	return nil
}

func (groups DefaultGroupList) MarshalJSON() ([]byte, error) {
	if groups == nil {
		return []byte("null"), nil
	}

	allStringForm := true
	names := make([]string, 0, len(groups))
	for _, group := range groups {
		if group.Permission != nil {
			allStringForm = false
			break
		}
		names = append(names, group.Name)
	}
	if allStringForm {
		return json.Marshal(names)
	}

	type defaultGroupBinding DefaultGroupBinding
	bindings := make([]defaultGroupBinding, 0, len(groups))
	for _, group := range groups {
		bindings = append(bindings, defaultGroupBinding(group))
	}
	return json.Marshal(bindings)
}

type MeteringConfig struct {
	Enabled            bool                      `json:"enabled,omitempty"`
	CostExpr           string                    `json:"cost_expr,omitempty"`
	MeterType          string                    `json:"meter_type,omitempty"`
	PostMeteringAction string                    `json:"post_metering_action,omitempty"`
	EnforceMode        string                    `json:"enforce_mode,omitempty"`
	OnActions          map[string]MeteringConfig `json:"on_actions,omitempty"`
}

type TableInfo struct {
	TableName              string `db:"table_name"`
	TableId                int
	TableDescription       string
	DefaultPermission      auth.AuthPermission
	Columns                []api2go.ColumnInfo
	StateMachines          []fsm.LoopbookFsmDescription
	Relations              []api2go.TableRelation
	IsTopLevel             bool `db:"is_top_level"`
	Permission             auth.AuthPermission
	UserId                 uint64              `db:"user_account_id"`
	IsHidden               bool                `db:"is_hidden"`
	IsJoinTable            bool                `db:"is_join_table"`
	IsStateTrackingEnabled bool                `db:"is_state_tracking_enabled"`
	IsAuditEnabled         bool                `db:"is_audit_enabled"`
	TranslationsEnabled    bool                `db:"translation_enabled"`
	DefaultGroups          DefaultGroupList    `db:"default_groups"`
	AccessGroups           DefaultGroupList    `db:"access_groups"`
	DefaultRelations       map[string][]string `db:"default_relations"`
	Validations            []columns.ColumnTag
	Conformations          []columns.ColumnTag
	DefaultOrder           string
	Icon                   string
	CompositeKeys          [][]string
	Metering               *MeteringConfig `json:"metering,omitempty"`
	ExplicitFields         map[string]bool `json:"-" db:"-"`
}

func (ti *TableInfo) GetColumnByName(name string) (*api2go.ColumnInfo, bool) {

	for _, col := range ti.Columns {
		if col.Name == name || col.ColumnName == name {
			return &col, true
		}
	}

	return nil, false

}
func (ti *TableInfo) GetRelationByName(name string) (*api2go.TableRelation, bool) {

	for _, relation := range ti.Relations {
		if relation.SubjectName == name || relation.ObjectName == name {
			return &relation, true
		}
	}

	return nil, false

}

func (ti *TableInfo) AddRelation(relations ...api2go.TableRelation) {

	if ti.Relations == nil {
		ti.Relations = make([]api2go.TableRelation, 0)
	}

	for _, relation := range relations {
		exists := false
		hash := relation.Hash()

		for _, existingRelation := range ti.Relations {
			if existingRelation.Hash() == hash {
				exists = true
				//log.Debugf("Relation already exists: %v", relation)
				break
			}
		}

		if !exists {
			ti.Relations = append(ti.Relations, relation)
		}
	}

}
