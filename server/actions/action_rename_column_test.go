package actions

import (
	"fmt"
	"testing"

	"github.com/daptin/daptin/server/actionresponse"
)

func TestRenameColumnRejectsInvalidInputsBeforePersistence(t *testing.T) {
	for _, field := range []string{"world_name", "column_name", "new_column_name"} {
		for _, value := range []interface{}{
			nil, 42, "", "9column", "column\nname", "column\tname",
			"column`name", `column"name`, "column.name", "column--comment",
			"renamed;\nDROP\nTABLE\nrename_column_probe;\n--",
		} {
			t.Run(field+"/"+fmt.Sprint(value), func(t *testing.T) {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Fatalf("invalid input reached persistence or panicked: %v", recovered)
					}
				}()
				input := map[string]interface{}{
					"world_name": "rename_column_probe", "column_name": "value", "new_column_name": "renamed",
				}
				input[field] = value
				performer := &renameWorldColumnPerformer{}
				_, _, errs := performer.DoAction(actionresponse.Outcome{}, input, nil)
				if len(errs) == 0 || errs[0] == nil {
					t.Fatal("expected invalid identifier error")
				}
			})
		}
	}
	for _, name := range []string{"select", "SELECT", "SeLeCt"} {
		performer := &renameWorldColumnPerformer{}
		_, _, errs := performer.DoAction(actionresponse.Outcome{}, map[string]interface{}{
			"world_name": "rename_column_probe", "column_name": "value", "new_column_name": name,
		}, nil)
		if len(errs) == 0 {
			t.Fatalf("reserved name %q accepted", name)
		}
	}
}
