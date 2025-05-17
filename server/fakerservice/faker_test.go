package fakerservice

import (
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/table_info"
	log "github.com/sirupsen/logrus"
	"testing"
)

func TestNewFakeInstance(t *testing.T) {

	resource.InitialiseColumnManager()
	table := &table_info.TableInfo{
		TableName: "test",
		Columns:   []api2go.ColumnInfo{},
	}

	for _, ty := range resource.ColumnTypes {
		table.Columns = append(table.Columns, api2go.ColumnInfo{
			ColumnName: ty.Name,
			ColumnType: ty.Name,
		})
	}

	fi := NewFakeInstance(table.Columns)
	for _, ty := range resource.ColumnTypes {
		if ty.Name == "id" {
			continue
		}
		if fi[ty.Name] == nil {
			t.Errorf("No fake value generated for %v", ty.Name)
		}
		log.Printf(" [%v] value : %v", ty.Name, fi[ty.Name])
	}

}
