package table_info

import (
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/columns"
	"github.com/daptin/daptin/server/fsm"
)

type TableRelation struct {
	api2go.TableRelation
	OnDelete string
}

type TableInfo struct {
	TableName              string `db:"table_name"`
	TableId                int
	DefaultPermission      auth.AuthPermission `db:"default_permission"`
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
	DefaultGroups          []string            `db:"default_groups"`
	DefaultRelations       map[string][]string `db:"default_relations"`
	Validations            []columns.ColumnTag
	Conformations          []columns.ColumnTag
	DefaultOrder           string
	Icon                   string
	CompositeKeys          [][]string
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
