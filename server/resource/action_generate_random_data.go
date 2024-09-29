package resource

import (
	"context"
	"errors"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	uuid "github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"strconv"
)

type randomDataGeneratePerformer struct {
	cmsConfig *CmsConfig
	cruds     map[string]*DbResource
	tableMap  map[string][]api2go.ColumnInfo
}

func (actionPerformer *randomDataGeneratePerformer) Name() string {
	return "generate.random.data"
}

func (actionPerformer *randomDataGeneratePerformer) DoAction(request Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	log.Printf("Generate random data for table %s", inFields["table_name"])
	//subjectInstance := inFields["subject"].(map[string]interface{})
	var userReferenceId daptinid.DaptinReferenceId //userIdInt := uint64(1)
	var err error
	log.Printf("%v", inFields)

	if inFields["user_reference_id"] != nil {
		userReferenceId = daptinid.InterfaceToDIR(inFields["user_reference_id"])
	}

	userIdInt, err := strconv.ParseInt(inFields[USER_ACCOUNT_ID_COLUMN].(string), 10, 32)

	//userIdInt, err = actionPerformer.Cruds["user"].GetReferenceIdToId("user", userReferenceId)
	if err != nil {
		log.Errorf("Failed to get user id from user reference id: %v", err)
	}
	tableName := inFields["table_name"].(string)

	tableResource := actionPerformer.cruds[tableName]
	if tableResource == nil {
		log.Errorf("Table [%v] is not created yet", tableName)
		return nil, nil, []error{errors.New("table not found")}
	}

	count := int(inFields["count"].(float64))

	rows := make([]map[string]interface{}, 0)
	for i := 0; i < count; i++ {
		columns := actionPerformer.tableMap[tableName]
		row := GetFakeRow(columns)
		for _, column := range columns {
			if column.IsForeignKey {
				if column.ForeignKeyData.DataSource == "self" {
					foreignRow, err := actionPerformer.cruds[column.ForeignKeyData.Namespace].GetRandomRow(column.ForeignKeyData.Namespace, 1, transaction)
					if len(foreignRow) < 1 || err != nil {
						log.Printf("no rows to select from for type %v", column.ForeignKeyData.Namespace)
						continue
					}
					row[column.ColumnName] = daptinid.InterfaceToDIR(foreignRow[0]["reference_id"]).String()
				}
			}
		}

		u, _ := uuid.NewV7()
		row["reference_id"] = u.String()
		row["permission"] = auth.DEFAULT_PERMISSION
		rows = append(rows, row)
	}
	ur, _ := url.Parse("/" + tableName)

	httpRequest := &http.Request{
		Method: "POST",
		URL:    ur,
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

		_, err := tableResource.CreateWithTransaction(api2go.NewApi2GoModelWithData(tableName, nil, 0, nil, row), req, transaction)
		if err != nil {
			log.Errorf("Was about to insert this fake object: %v", row)
			log.Errorf("Failed to fake insert into table [%v] : %v", tableName, err)
		}
	}
	responder := api2go.Response{
		Res: api2go.NewApi2GoModelWithData(
			"", nil, 0, nil, map[string]interface{}{
				"message": "Random data generated",
			}),
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
		//log.Printf("Fake value for [%s][%s] => [%s]", col.ColumnType, col.ColumnName, fakeValue)

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
