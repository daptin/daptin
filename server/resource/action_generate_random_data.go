package resource

import (
	"context"
	"github.com/artpar/api2go"
	"github.com/artpar/go.uuid"
	"github.com/daptin/daptin/server/auth"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
)

type RandomDataGeneratePerformer struct {
	cmsConfig *CmsConfig
	cruds     map[string]*DbResource
	tableMap  map[string][]api2go.ColumnInfo
}

func (d *RandomDataGeneratePerformer) Name() string {
	return "generate.random.data"
}

func (d *RandomDataGeneratePerformer) DoAction(request Outcome, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	//subjectInstance := inFields["subject"].(map[string]interface{})
	userReferenceId := ""
	//userIdInt := uint64(1)
	var err error
	log.Infof("%v", inFields)

	if inFields["user_reference_id"] != nil {
		userReferenceId = inFields["user_reference_id"].(string)
	}

	userIdInt, _ := strconv.ParseInt(inFields[USER_ACCOUNT_ID_COLUMN].(string), 10, 32)

	//userIdInt, err = d.Cruds["user"].GetReferenceIdToId("user", userReferenceId)
	if err != nil {
		log.Errorf("Failed to get user id from user reference id: %v", err)
	}
	tableName := inFields["table_name"].(string)

	count := int(inFields["count"].(float64))

	rows := make([]map[string]interface{}, 0)
	for i := 0; i < count; i++ {
		row := GetFakeRow(d.tableMap[tableName])
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

		_, err := d.cruds[tableName].Create(api2go.NewApi2GoModelWithData(tableName, nil, 0, nil, row), req)
		if err != nil {
			log.Errorf("Was about to insert this fake object: %v", row)
			log.Errorf("Failed to fake insert into table [%v] : %v", tableName, err)
		}
	}
	return nil, responses, nil
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

	handler := RandomDataGeneratePerformer{
		cmsConfig: initConfig,
		cruds:     cruds,
		tableMap:  tableMap,
	}

	return &handler, nil

}
