package resource

import (
	"context"
	"errors"
	"github.com/artpar/api2go"
	"github.com/artpar/go.uuid"
	"github.com/daptin/daptin/server/auth"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
)

type randomDataGeneratePerformer struct {
	cmsConfig *CmsConfig
	cruds     map[string]*DbResource
	tableMap  map[string][]api2go.ColumnInfo
}

func (d *randomDataGeneratePerformer) Name() string {
	return "generate.random.data"
}

func (d *randomDataGeneratePerformer) DoAction(request Outcome, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	log.Printf("Generate random data for table %s", inFields["table_name"])
	//subjectInstance := inFields["subject"].(map[string]interface{})
	userReferenceId := ""
	//userIdInt := uint64(1)
	var err error
	log.Printf("%v", inFields)

	if inFields["user_reference_id"] != nil {
		userReferenceId = inFields["user_reference_id"].(string)
	}

	userIdInt, err := strconv.ParseInt(inFields[USER_ACCOUNT_ID_COLUMN].(string), 10, 32)

	//userIdInt, err = d.Cruds["user"].GetReferenceIdToId("user", userReferenceId)
	if err != nil {
		log.Errorf("Failed to get user id from user reference id: %v", err)
	}
	tableName := inFields["table_name"].(string)

	tableResource := d.cruds[tableName]
	if tableResource == nil {
		log.Errorf("Table [%v] is not created yet", tableName)
		return nil, nil, []error{errors.New("table not found")}
	}

	count := int(inFields["count"].(float64))

	rows := make([]map[string]interface{}, 0)
	for i := 0; i < count; i++ {
		columns := d.tableMap[tableName]
		row := GetFakeRow(columns)
		for _, column := range columns {
			if column.IsForeignKey {
				if column.ForeignKeyData.DataSource == "self" {
					foreignRow, err := d.cruds[column.ForeignKeyData.Namespace].GetRandomRow(column.ForeignKeyData.Namespace, 1)
					if len(foreignRow) < 1 || err != nil {
						log.Printf("no rows to select from for type %v", column.ForeignKeyData.Namespace)
						continue
					}
					row[column.ColumnName] = foreignRow[0]["reference_id"].(string)
				}
			}
		}

		u, _ := uuid.NewV4()
		row["reference_id"] = u.String()
		row["permission"] = auth.DEFAULT_PERMISSION
		rows = append(rows, row)
	}

	httpRequest := &http.Request{
		Method: "POST",
	}

	sessionUser := &auth.SessionUser{
		UserId:          userIdInt,
		UserReferenceId: userReferenceId,
		Groups:          []auth.GroupPermission{},
	}
	httpRequest = httpRequest.WithContext(context.WithValue(context.Background(), "user", sessionUser))

	req := api2go.Request{
		PlainRequest: httpRequest,
	}

	for _, row := range rows {

		_, err := tableResource.Create(api2go.NewApi2GoModelWithData(tableName, nil, 0, nil, row), req)
		if err != nil {
			log.Errorf("Was about to insert this fake object: %v", row)
			log.Errorf("Failed to fake insert into table [%v] : %v", tableName, err)
		}
	}
	responder := api2go.Response{
		Res: &api2go.Api2GoModel{
			Data: map[string]interface{}{
				"message": "Random data generated",
			},
		},
		Code: 201,
	}
	return responder, responses, nil
}

func GetFakeRow(columns []api2go.ColumnInfo) map[string]interface{} {

	row := make(map[string]interface{})

	for _, col := range columns {

		if col.IsForeignKey {
			continue
		}

		isStandardColumn := false
		for _, c := range StandardColumns {
			if col.ColumnName == c.ColumnName {
				isStandardColumn = true
			}
		}

		if isStandardColumn {
			continue
		}

		fakeValue := ColumnManager.GetFakeData(col.ColumnType)

		row[col.ColumnName] = fakeValue

	}

	return row

}

func NewRandomDataGeneratePerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	tableMap := make(map[string][]api2go.ColumnInfo)
	for _, table := range initConfig.Tables {
		tableMap[table.TableName] = table.Columns
	}

	handler := randomDataGeneratePerformer{
		cmsConfig: initConfig,
		cruds:     cruds,
		tableMap:  tableMap,
	}

	return &handler, nil

}
