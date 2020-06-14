package resource

import (
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/auth"
	log "github.com/sirupsen/logrus"
)

func IsStandardColumn(colName string) bool {

	for _, col := range StandardColumns {
		if col.ColumnName == colName {
			return true
		}
	}

	return false
}

var StandardColumns = []api2go.ColumnInfo{
	{
		Name:            "id",
		ColumnName:      "id",
		DataType:        "INTEGER",
		IsPrimaryKey:    true,
		IsAutoIncrement: true,
		ExcludeFromApi:  true,
		ColumnType:      "id",
	},
	{
		Name:           "version",
		ColumnName:     "version",
		DataType:       "INTEGER",
		ColumnType:     "measurement",
		DefaultValue:   "1",
		ExcludeFromApi: true,
	},
	{
		Name:         "created_at",
		ColumnName:   "created_at",
		DataType:     "timestamp",
		DefaultValue: "current_timestamp",
		ColumnType:   "datetime",
		IsIndexed:    true,
	},
	{
		Name:       "updated_at",
		ColumnName: "updated_at",
		DataType:   "timestamp",
		IsIndexed:  true,
		IsNullable: true,
		ColumnType: "datetime",
	},
	{
		Name:       "reference_id",
		ColumnName: "reference_id",
		DataType:   "varchar(40)",
		IsIndexed:  true,
		IsUnique:   true,
		IsNullable: false,
		ColumnType: "alias",
	},
	{
		Name:       "permission",
		ColumnName: "permission",
		DataType:   "int(11)",
		IsIndexed:  false,
		ColumnType: "value",
	},
}

var StandardRelations = []api2go.TableRelation{
	api2go.NewTableRelation("action", "belongs_to", "world"),
	api2go.NewTableRelation("feed", "belongs_to", "stream"),
	api2go.NewTableRelation("world", "has_many", "smd"),
	api2go.NewTableRelation("oauth_token", "has_one", "oauth_connect"),
	api2go.NewTableRelation("data_exchange", "has_one", "oauth_token"),
	api2go.NewTableRelationWithNames("user_otp_account", "primary_user_otp", "belongs_to", "user_account", "otp_of_account"),
	api2go.NewTableRelation("timeline", "belongs_to", "world"),
	api2go.NewTableRelation("cloud_store", "has_one", "oauth_token"),
	api2go.NewTableRelation("site", "has_one", "cloud_store"),
	api2go.NewTableRelation("mail_account", "belongs_to", "mail_server"),
	api2go.NewTableRelation("mail_box", "belongs_to", "mail_account"),
	api2go.NewTableRelation("mail", "belongs_to", "mail_box"),
	api2go.NewTableRelationWithNames("task", "task_executed", "has_one", USER_ACCOUNT_TABLE_NAME, "as_user_id"),
}

var SystemSmds []LoopbookFsmDescription
var SystemExchanges []ExchangeContract

var SystemActions = []Action{
	{
		Name:             "import_files_from_store",
		Label:            "Import files data to a table",
		OnType:           "world",
		InstanceOptional: false,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "table_name",
				ColumnType: "label",
				ColumnName: "table_name",
			},
		},
		OutFields: []Outcome{
			{
				Type:   "cloud_store.files.import",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"table_name": "$.table_name",
				},
			},
		},
	},
	{
		Name:             "install_integration",
		Label:            "Install integration",
		OnType:           "integration",
		InstanceOptional: false,
		OutFields: []Outcome{
			{
				Type:   "integration.install",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"reference_id": "$.reference_id",
				},
			},
		},
	},
	{
		Name:             "download_certificate",
		Label:            "Download certificate",
		OnType:           "certificate",
		InstanceOptional: false,
		OutFields: []Outcome{
			{
				Type:   "client.file.download",
				Method: "ACTIONRESPONSE",
				Attributes: map[string]interface{}{
					"content":     "!btoa(subject.certificate_pem)",
					"name":        "!subject.hostname + '.pem.crt'",
					"contentType": "application/x-x509-ca-cert",
					"message":     "!'Certificate for ' + subject.hostname",
				},
			},
		},
	},
	{
		Name:             "download_public_key",
		Label:            "Download public key",
		OnType:           "certificate",
		InstanceOptional: false,
		OutFields: []Outcome{
			{
				Type:   "client.file.download",
				Method: "ACTIONRESPONSE",
				Attributes: map[string]interface{}{
					"content":     "!btoa(subject.public_key_pem)",
					"name":        "!subject.hostname + '.pem.key.pub'",
					"contentType": "application/x-x509-ca-cert",
					"message":     "!'Public Key for ' + subject.hostname",
				},
			},
		},
	},
	{
		Name:             "generate_acme_certificate",
		Label:            "Generate ACME certificate",
		OnType:           "certificate",
		InstanceOptional: false,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "email",
				ColumnType: "label",
				ColumnName: "email",
			},
		},
		OutFields: []Outcome{
			{
				Type:   "acme.tls.generate",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"email":       "~email",
					"certificate": "~subject",
				},
			},
		},
	},
	{
		Name:             "generate_self_certificate",
		Label:            "Generate Self certificate",
		OnType:           "certificate",
		InstanceOptional: false,
		InFields:         []api2go.ColumnInfo{},
		OutFields: []Outcome{
			{
				Type:   "self.tls.generate",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"certificate": "~subject",
				},
			},
		},
	},
	{
		Name:             "register_otp",
		Label:            "Register Mobile Number",
		OnType:           USER_ACCOUNT_TABLE_NAME,
		InstanceOptional: false,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "mobile_number",
				ColumnName: "mobile_number",
				ColumnType: "label",
			},
		},
		OutFields: []Outcome{
			{
				Type:      "otp.generate",
				Method:    "EXECUTE",
				Reference: "otp",
				Attributes: map[string]interface{}{
					"email":  "$.email",
					"mobile": "~mobile_number",
				},
			},
			{
				Type:      "2factor.in",
				Method:    "GET_api_key-SMS-phone_number-otp",
				Condition: "!mobile_number != null && mobile_number != undefined && mobile_number != ''",
				Attributes: map[string]interface{}{
					"phone_number": "~mobile_number",
					"otp":          "$otp.otp",
				},
			},
		},
	},
	{
		Name:             "verify_mobile_number",
		Label:            "Verify Mobile Number",
		OnType:           "user_otp_account",
		InstanceOptional: true,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "mobile_number",
				ColumnName: "mobile_number",
				ColumnType: "label",
			},
			{
				Name:       "email",
				ColumnName: "email",
				ColumnType: "label",
			},
			{
				Name:       "otp",
				ColumnName: "otp",
				ColumnType: "label",
			},
		},
		OutFields: []Outcome{
			{
				Type:   "otp.login.verify",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"otp":    "~otp",
					"mobile": "~mobile_number",
				},
			},
		},
	},
	{
		Name:             "send_otp",
		Label:            "Send OTP to mobile",
		OnType:           "user_otp_account",
		InstanceOptional: true,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "mobile_number",
				ColumnName: "mobile_number",
				ColumnType: "label",
			},
			{
				Name:       "email",
				ColumnName: "email",
				ColumnType: "label",
			},
		},
		OutFields: []Outcome{
			{
				Type:      "otp.generate",
				Method:    "EXECUTE",
				Reference: "otp",
				Attributes: map[string]interface{}{
					"email":  "~email",
					"mobile": "~mobile_number",
				},
			},
			{
				Type:      "2factor.in",
				Method:    "GET_api_key-SMS-phone_number-otp",
				Condition: "!mobile_number != null && mobile_number != undefined && mobile_number != ''",
				Attributes: map[string]interface{}{
					"phone_number": "~mobile_number",
					"otp":          "$otp.otp",
				},
			},
		},
	},
	{
		Name:             "verify_otp",
		Label:            "Login with OTP",
		OnType:           "user_account",
		InstanceOptional: true,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "otp",
				ColumnName: "otp",
				ColumnType: "label",
			},
			{
				Name:       "mobile_number",
				ColumnName: "mobile_number",
				ColumnType: "label",
			},
		},
		OutFields: []Outcome{
			{
				Type:   "otp.login.verify",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"otp":    "~otp",
					"mobile": "~mobile_number",
				},
			},
		},
	},
	{
		Name:             "remove_column",
		Label:            "Delete column",
		OnType:           "world",
		InstanceOptional: false,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "column_name",
				ColumnName: "column_name",
				ColumnType: "label",
			},
		},
		OutFields: []Outcome{
			{
				Type:   "world.column.delete",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"world_id":    "$.reference_id",
					"column_name": "~column_name",
				},
			},
		},
	},
	{
		Name:             "rename_column",
		Label:            "Rename column",
		OnType:           "world",
		InstanceOptional: false,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "column_name",
				ColumnName: "column_name",
				ColumnType: "label",
			},
			{
				Name:       "new_column_name",
				ColumnName: "new_column_name",
				ColumnType: "label",
			},
		},
		OutFields: []Outcome{
			{
				Type:   "world.column.rename",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"world_id":        "$.reference_id",
					"column_name":     "~column_name",
					"new_column_name": "~new_column_name",
				},
			},
		},
	},
	{
		Name:             "sync_site_storage",
		Label:            "Sync site storage",
		OnType:           "site",
		InstanceOptional: false,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "Path",
				ColumnName: "path",
				ColumnType: "label",
			},
		},
		OutFields: []Outcome{
			{
				Type:   "site.storage.sync",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"cloud_store_id": "$.cloud_store_id",
					"path":           "~path",
				},
			},
		},
	},
	{
		Name:             "sync_column_storage",
		Label:            "Sync column storage",
		OnType:           "world",
		InstanceOptional: true,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "Table name",
				ColumnName: "table_name",
				ColumnType: "label",
			},
			{
				Name:       "Column name",
				ColumnName: "column_name",
				ColumnType: "label",
			},
		},
		OutFields: []Outcome{
			{
				Type:   "column.storage.sync",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"column_name": "~column_name",
					"table_name":  "~table_name",
				},
			},
		},
	},
	{
		Name:             "sync_mail_servers",
		Label:            "Sync Mail Servers",
		OnType:           "mail_server",
		InstanceOptional: true,
		InFields:         []api2go.ColumnInfo{},
		OutFields: []Outcome{
			{
				Type:       "mail.servers.sync",
				Method:     "EXECUTE",
				Attributes: map[string]interface{}{},
			},
		},
	},
	{
		Name:             "restart_daptin",
		Label:            "Restart system",
		OnType:           "world",
		InstanceOptional: true,
		InFields:         []api2go.ColumnInfo{},
		OutFields: []Outcome{
			{
				Type:   "system_json_schema_update",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"json_schema": "!JSON.parse('[{\"name\":\"empty.json\",\"file\":\"data:application/json;base64,e30K\",\"type\":\"application/json\"}]')",
				},
			},
		},
	},
	//{
	//	Name:             "publish_package_to_market",
	//	Label:            "Update package list",
	//	OnType:           "marketplace",
	//	InstanceOptional: false,
	//	InFields:         []api2go.ColumnInfo{},
	//	OutFields: []Outcome{
	//		{
	//			Type:   "marketplace.package.publish",
	//			Method: "EXECUTE",
	//			Attributes: map[string]interface{}{
	//				"marketplace_id": "$.reference_id",
	//			},
	//		},
	//	},
	//},
	//{
	//	Name:             "visit_marketplace_github",
	//	Label:            "Go to marketplace",
	//	OnType:           "marketplace",
	//	InstanceOptional: false,
	//	InFields:         []api2go.ColumnInfo{},
	//	OutFields: []Outcome{
	//		{
	//			Type:   "client.redirect",
	//			Method: "ACTIONRESPONSE",
	//			Attributes: map[string]interface{}{
	//				"location": "$subject.endpoint",
	//				"window":   "_blank",
	//			}},
	//	},
	//},
	//{
	//	Name:             "refresh_marketplace_packages",
	//	Label:            "Refresh marketplace",
	//	OnType:           "marketplace",
	//	InstanceOptional: false,
	//	InFields:         []api2go.ColumnInfo{},
	//	OutFields: []Outcome{
	//		{
	//			Type:   "marketplace.package.refresh",
	//			Method: "EXECUTE",
	//			Attributes: map[string]interface{}{
	//				"marketplace_id": "$.reference_id",
	//			},
	//		},
	//	},
	//},
	{
		Name:             "generate_random_data",
		Label:            "Generate random data",
		OnType:           "world",
		InstanceOptional: false,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "Number of records",
				ColumnName: "count",
				ColumnType: "measurement",
			},
		},
		OutFields: []Outcome{
			{
				Type:   "generate.random.data",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"count":             "~count",
					"table_name":        "$.table_name",
					"user_reference_id": "$user.reference_id",
					"user_account_id":   "$user.id",
				},
			},
		},
		Validations: []ColumnTag{
			{
				ColumnName: "count",
				Tags:       "gt=0",
			},
		},
	},
	//{
	//
	//	Name: "update_config",
	//	Label: "Update configuration",
	//	OnType: "world",
	//	InstanceOptional: true,name
	//	InFields: []api2go.ColumnInfo{
	//		{
	//			Name: "default_storage",
	//		},
	//	},
	//},
	//{
	//	Name:             "install_marketplace_package",
	//	Label:            "Install package from market",
	//	OnType:           "marketplace",
	//	InstanceOptional: false,
	//	InFields: []api2go.ColumnInfo{
	//		{
	//			Name:       "package_name",
	//			ColumnName: "package_name",
	//			ColumnType: "label",
	//			IsNullable: false,
	//		},
	//	},
	//	OutFields: []Outcome{
	//		{
	//			Type:   "marketplace.package.install",
	//			Method: "EXECUTE",
	//			Attributes: map[string]interface{}{
	//				"package_name":   "~package_name",
	//				"marketplace_id": "$.reference_id",
	//			},
	//		},
	//	},
	//},
	{
		Name:             "export_data",
		Label:            "Export data for backup",
		OnType:           "world",
		InstanceOptional: true,
		InFields:         []api2go.ColumnInfo{},
		OutFields: []Outcome{
			{
				Type:   "__data_export",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"world_reference_id": "$.reference_id",
					"table_name":         "$.table_name",
				},
			},
		},
	},
	{
		Name:             "export_csv_data",
		Label:            "Export CSV data",
		OnType:           "world",
		InstanceOptional: true,
		InFields:         []api2go.ColumnInfo{},
		OutFields: []Outcome{
			{
				Type:   "__csv_data_export",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"world_reference_id": "$.reference_id",
					"table_name":         "$.table_name",
				},
			},
		},
	},
	{
		Name:             "import_data",
		Label:            "Import data from dump",
		OnType:           "world",
		InstanceOptional: false,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "JSON Dump file",
				ColumnName: "dump_file",
				ColumnType: "file.json|yaml|toml|hcl",
				IsNullable: false,
			},
			{
				Name:       "truncate_before_insert",
				ColumnName: "truncate_before_insert",
				ColumnType: "truefalse",
			},
		},
		OutFields: []Outcome{
			{
				Type:   "__data_import",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"world_reference_id":     "$.reference_id",
					"truncate_before_insert": "~truncate_before_insert",
					"dump_file":              "~dump_file",
					"table_name":             "$.table_name",
					"user":                   "~user",
				},
			},
		},
	},
	{
		Name:             "upload_file",
		Label:            "Upload file to external store",
		OnType:           "cloud_store",
		InstanceOptional: false,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "File",
				ColumnName: "file",
				ColumnType: "file.*",
				IsNullable: false,
			},
		},
		OutFields: []Outcome{
			{
				Type:   "__external_file_upload",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"file":           "~file",
					"oauth_token_id": "$.oauth_token_id",
					"store_provider": "$.store_provider",
					"root_path":      "$.root_path",
				},
			},
		},
	},
	{
		Name:             "upload_system_schema",
		Label:            "Upload features",
		OnType:           "world",
		InstanceOptional: true,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "Schema file",
				ColumnName: "schema_file",
				ColumnType: "file.json|yaml|toml|hcl",
				IsNullable: false,
			},
		},
		OutFields: []Outcome{
			{
				Type:   "system_json_schema_update",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"json_schema": "~schema_file",
				},
			},
		},
	},
	{
		Name:             "upload_xls_to_system_schema",
		Label:            "Upload xls to entity",
		OnType:           "world",
		InstanceOptional: true,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "XLSX file",
				ColumnName: "data_xls_file",
				ColumnType: "file.xls|xlsx",
				IsNullable: false,
			},
			{
				Name:       "Entity name",
				ColumnName: "entity_name",
				ColumnType: "label",
				IsNullable: false,
			},
			{
				Name:       "Create entity if not exists",
				ColumnName: "create_if_not_exists",
				ColumnType: "truefalse",
				IsNullable: false,
			},
			{
				Name:       "Add missing columns",
				ColumnName: "add_missing_columns",
				ColumnType: "truefalse",
				IsNullable: false,
			},
		},
		Validations: []ColumnTag{
			{
				ColumnName: "entity_name",
				Tags:       "required",
			},
		},
		OutFields: []Outcome{
			{
				Type:   "__upload_xlsx_file_to_entity",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"data_xls_file":        "~data_xls_file",
					"entity_name":          "~entity_name",
					"add_missing_columns":  "~add_missing_columns",
					"create_if_not_exists": "~create_if_not_exists",
				},
			},
		},
	},
	{
		Name:             "upload_csv_to_system_schema",
		Label:            "Upload CSV to entity",
		OnType:           "world",
		InstanceOptional: true,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "CSV file",
				ColumnName: "data_csv_file",
				ColumnType: "file.csv",
				IsNullable: false,
			},
			{
				Name:       "Entity name",
				ColumnName: "entity_name",
				ColumnType: "label",
				IsNullable: false,
			},
			{
				Name:       "Create entity if not exists",
				ColumnName: "create_if_not_exists",
				ColumnType: "truefalse",
				IsNullable: false,
			},
			{
				Name:       "Add missing columns",
				ColumnName: "add_missing_columns",
				ColumnType: "truefalse",
				IsNullable: false,
			},
		},
		Validations: []ColumnTag{
			{
				ColumnName: "entity_name",
				Tags:       "required",
			},
		},
		OutFields: []Outcome{
			{
				Type:   "__upload_csv_file_to_entity",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"data_csv_file":        "~data_csv_file",
					"entity_name":          "~entity_name",
					"add_missing_columns":  "~add_missing_columns",
					"create_if_not_exists": "~create_if_not_exists",
				},
			},
		},
	},
	{
		Name:             "download_system_schema",
		Label:            "Download system schema",
		OnType:           "world",
		InstanceOptional: true,
		InFields:         []api2go.ColumnInfo{},
		OutFields: []Outcome{
			{
				Type:       "__download_cms_config",
				Method:     "EXECUTE",
				Attributes: map[string]interface{}{},
			},
		},
	},
	{
		Name:             "become_an_administrator",
		Label:            "Become Daptin Administrator",
		InstanceOptional: true,
		OnType:           "world",
		InFields:         []api2go.ColumnInfo{},
		OutFields: []Outcome{
			{
				Type:   "__become_admin",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"user_account_id": "$user.id",
					"user":            "~user",
				},
			},
		},
	},
	{
		Name:             "signup",
		Label:            "Sign up",
		InstanceOptional: true,
		OnType:           USER_ACCOUNT_TABLE_NAME,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "name",
				ColumnName: "name",
				ColumnType: "label",
				IsNullable: false,
			},
			{
				Name:       "email",
				ColumnName: "email",
				ColumnType: "email",
				IsNullable: false,
			},
			{
				Name:       "mobile",
				ColumnName: "mobile",
				ColumnType: "label",
				IsNullable: true,
			},
			{
				Name:       "password",
				ColumnName: "password",
				ColumnType: "password",
				IsNullable: false,
			},
			{
				Name:       "Password Confirm",
				ColumnName: "passwordConfirm",
				ColumnType: "password",
				IsNullable: false,
			},
		},
		Validations: []ColumnTag{
			{
				ColumnName: "email",
				Tags:       "email",
			},
			{
				ColumnName: "name",
				Tags:       "required",
			},
			{
				ColumnName: "password",
				Tags:       "eqfield=InnerStructField[passwordConfirm],min=8",
			},
		},
		Conformations: []ColumnTag{
			{
				ColumnName: "email",
				Tags:       "email",
			},
			{
				ColumnName: "name",
				Tags:       "trim",
			},
			{
				ColumnName: "mobile",
				Tags:       "trim",
			},
		},
		OutFields: []Outcome{
			{
				Type:      USER_ACCOUNT_TABLE_NAME,
				Method:    "POST",
				Reference: "user",
				Attributes: map[string]interface{}{
					"name":      "~name",
					"email":     "~email",
					"password":  "~password",
					"confirmed": "0",
				},
			},
			{
				Type:      "usergroup",
				Method:    "POST",
				Reference: "usergroup",
				Attributes: map[string]interface{}{
					"name": "!'Home group for ' + user.name",
				},
			},
			{
				Type:      "user_account_user_account_id_has_usergroup_usergroup_id",
				Method:    "POST",
				Reference: "user_usergroup",
				Attributes: map[string]interface{}{
					"user_account_id": "$user.reference_id",
					"usergroup_id":    "$usergroup.reference_id",
				},
			},
			{
				Type:      "otp.generate",
				Method:    "EXECUTE",
				Reference: "otp",
				Condition: "!mobile != null && mobile != undefined && mobile != ''",
				Attributes: map[string]interface{}{
					"mobile": "~mobile",
					"email":  "~email",
				},
			},
			{
				Type:   "client.notify",
				Method: "ACTIONRESPONSE",
				Attributes: map[string]interface{}{
					"type":    "success",
					"title":   "Success",
					"message": "Sign-up successful. Redirecting to sign in",
				},
			},
			{
				Type:   "client.redirect",
				Method: "ACTIONRESPONSE",
				Attributes: map[string]interface{}{
					"location": "/auth/signin",
					"window":   "self",
					"delay":    2000,
				},
			},
		},
	},
	{
		Name:             "signin",
		Label:            "Sign in",
		InstanceOptional: true,
		OnType:           USER_ACCOUNT_TABLE_NAME,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "email",
				ColumnName: "email",
				ColumnType: "email",
				IsNullable: false,
			},
			{
				Name:       "password",
				ColumnName: "password",
				ColumnType: "password",
				IsNullable: false,
			},
		},
		OutFields: []Outcome{
			{
				Type:   "jwt.token",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"email":    "~email",
					"password": "~password",
				},
			},
		},
	},
	{
		Name:     "oauth.login.begin",
		Label:    "Authenticate via OAuth",
		OnType:   "oauth_connect",
		InFields: []api2go.ColumnInfo{},
		OutFields: []Outcome{
			{
				Type:   "oauth.client.redirect",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"authenticator": "$.name",
					"scope":         "$.scope",
				},
			},
		},
	},
	{
		Name:             "oauth.login.response",
		Label:            "",
		InstanceOptional: true,
		OnType:           "oauth_token",
		InFields: []api2go.ColumnInfo{
			{
				Name:       "code",
				ColumnName: "code",
				ColumnType: "hidden",
				IsNullable: false,
			},
			{
				Name:       "state",
				ColumnName: "state",
				ColumnType: "hidden",
				IsNullable: false,
			},
			{
				Name:       "authenticator",
				ColumnName: "authenticator",
				ColumnType: "hidden",
				IsNullable: false,
			},
		},
		OutFields: []Outcome{
			{
				Type:           "oauth_connect",
				Method:         "GET",
				SkipInResponse: true,
				Reference:      "connection",
				Attributes: map[string]interface{}{
					"filter":       "~authenticator",
					"page[number]": "1",
					"page[size]":   "1",
				},
			},
			{
				Type:           "oauth.login.response",
				Method:         "EXECUTE",
				SkipInResponse: true,
				Reference:      "auth",
				Attributes: map[string]interface{}{
					"authenticator":     "~authenticator",
					"user_account_id":   "~user.id",
					"user_reference_id": "~user.reference_id",
					"state":             "~state",
					"code":              "~code",
				},
			},
			{
				Type:           "oauth.profile.exchange",
				Method:         "EXECUTE",
				Reference:      "profile",
				SkipInResponse: true,
				Condition:      "$connection[0].allow_login",
				Attributes: map[string]interface{}{
					"authenticator": "~authenticator",
					"token":         "$auth.access_token",
					"tokenInfoUrl":  "$connection[0].token_url",
					"profileUrl":    "$connection[0].profile_url",
				},
			},
			{
				Type:           USER_ACCOUNT_TABLE_NAME,
				Method:         "GET",
				Reference:      "user",
				SkipInResponse: true,
				Condition:      "$connection[0].allow_login",
				Attributes: map[string]interface{}{
					"filter": "!profile.email || profile.emailAddress",
				},
			},
			{
				Type:           USER_ACCOUNT_TABLE_NAME,
				Method:         "POST",
				Reference:      "user",
				SkipInResponse: true,
				Condition:      "!!user || (!user.length && !user.reference_id)",
				Attributes: map[string]interface{}{
					"email":    "!profile.email || profile.emailAddress",
					"name":     "$profile.displayName",
					"password": "$profile.id",
				},
			},
			{
				Type:           "usergroup",
				Method:         "POST",
				Reference:      "usergroup",
				SkipInResponse: true,
				Condition:      "!!user || (!user.length && !user.reference_id)",
				Attributes: map[string]interface{}{
					"name": "!'Home group for ' + profile.emails[0].value",
				},
			},
			{
				Type:           "user_account_user_account_id_has_usergroup_usergroup_id",
				Method:         "POST",
				SkipInResponse: true,
				Condition:      "!!user || (!user.length && !user.reference_id)",
				Attributes: map[string]interface{}{
					"user_account_id": "$user.reference_id",
					"usergroup_id":    "$usergroup.reference_id",
				},
			},
			{
				Type:   "jwt.token",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"email":             "!profile.email || profile.emailAddress",
					"skipPasswordCheck": true,
				},
			},
			{
				Type:   "client.redirect",
				Method: "ACTIONRESPONSE",
				Attributes: map[string]interface{}{
					"location": "/",
					"window":   "self",
					"delay":    2000,
				},
			},
		},
	},
	{
		Name:             "add_exchange",
		Label:            "Add new data exchange",
		OnType:           "world",
		InstanceOptional: false,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "name",
				ColumnName: "name",
				ColumnType: "label",
				IsNullable: false,
			},
			{
				Name:       "sheet_id",
				ColumnName: "sheet_id",
				ColumnType: "alias",
				IsNullable: false,
			},
			{
				Name:       "app_key Key",
				ColumnName: "app_key",
				ColumnType: "alias",
				IsNullable: false,
			},
		},
		OutFields: []Outcome{
			{
				Type:   "data_exchange",
				Method: "POST",
				Attributes: map[string]interface{}{
					"name":              "!'Export ' + subject.table_name + ' to excel sheet'",
					"source_attributes": "!JSON.stringify({name: subject.table_name})",
					"source_type":       "self",
					"target_type":       "gsheet-append",
					"options":           "!JSON.stringify({hasHeader: true})",
					"attributes":        "!JSON.stringify([{SourceColumn: '$self.description', TargetColumn: 'Task description'}])",
					"target_attributes": "!JSON.stringify({sheetUrl: 'https://content-sheets.googleapis.com/v4/spreadsheets/' + sheet_id + '/values/A1:append', appKey: app_key})",
				},
			},
		},
	},
}

var adminsGroup = []string{"administrators"}

var StandardTasks []Task

var StandardTables = []TableInfo{
	{
		TableName:     "certificate",
		IsHidden:      true,
		DefaultGroups: adminsGroup,
		Icon:          "fa-certificate",
		Columns: []api2go.ColumnInfo{
			{
				Name:       "hostname",
				ColumnName: "hostname",
				IsUnique:   true,
				IsIndexed:  true,
				ColumnType: "label",
				DataType:   "varchar(100)",
				IsNullable: false,
			},
			{
				Name:         "issuer",
				ColumnName:   "issuer",
				ColumnType:   "label",
				DataType:     "varchar(100)",
				IsNullable:   false,
				DefaultValue: "'self'",
			},
			{
				Name:       "generated_at",
				ColumnName: "generated_at",
				ColumnType: "datetime",
				DataType:   "timestamp",
				IsNullable: true,
			},
			{
				Name:       "certificate_pem",
				ColumnName: "certificate_pem",
				ColumnType: "content",
				DataType:   "text",
				IsNullable: true,
			},
			{
				Name:       "root_certificate",
				ColumnName: "root_certificate",
				ColumnType: "content",
				DataType:   "text",
				IsNullable: true,
			},
			{
				Name:       "private_key_pem",
				ColumnName: "private_key_pem",
				ColumnType: "encrypted",
				DataType:   "text",
				IsNullable: true,
			},
			{
				Name:       "public_key_pem",
				ColumnName: "public_key_pem",
				ColumnType: "content",
				DataType:   "text",
				IsNullable: true,
			},
		},
	},
	{
		TableName:     "feed",
		IsHidden:      true,
		DefaultGroups: adminsGroup,
		Icon:          "fa-rss",
		Columns: []api2go.ColumnInfo{
			{
				Name:       "feed_name",
				ColumnName: "feed_name",
				IsUnique:   true,
				IsIndexed:  true,
				ColumnType: "label",
				DataType:   "varchar(100)",
				IsNullable: false,
			},
			{
				Name:         "title",
				ColumnName:   "title",
				ColumnType:   "label",
				DataType:     "varchar(500)",
				IsNullable:   false,
				DefaultValue: "''",
			},
			{
				Name:         "title",
				ColumnName:   "title",
				ColumnType:   "label",
				DataType:     "varchar(500)",
				IsNullable:   false,
				DefaultValue: "''",
			},
			{
				Name:       "description",
				ColumnName: "description",
				ColumnType: "label",
				DataType:   "text",
				IsNullable: false,
			},
			{
				Name:         "link",
				ColumnName:   "link",
				ColumnType:   "label",
				DataType:     "varchar(1000)",
				IsNullable:   false,
				DefaultValue: "''",
			},
			{
				Name:         "author_name",
				ColumnName:   "author_name",
				ColumnType:   "label",
				DataType:     "varchar(500)",
				IsNullable:   false,
				DefaultValue: "''",
			},
			{
				Name:         "author_email",
				ColumnName:   "author_email",
				ColumnType:   "label",
				DataType:     "varchar(500)",
				IsNullable:   false,
				DefaultValue: "''",
			},
			{
				Name:         "enable",
				ColumnName:   "enable",
				ColumnType:   "truefalse",
				DataType:     "int(1)",
				IsNullable:   false,
				DefaultValue: "0",
			},
			{
				Name:         "enable_atom",
				ColumnName:   "enable_atom",
				ColumnType:   "truefalse",
				DataType:     "int(1)",
				IsNullable:   false,
				DefaultValue: "1",
			},
			{
				Name:         "enable_json",
				ColumnName:   "enable_json",
				ColumnType:   "truefalse",
				DataType:     "int(1)",
				IsNullable:   false,
				DefaultValue: "1",
			},
			{
				Name:         "enable_rss",
				ColumnName:   "enable_rss",
				ColumnType:   "truefalse",
				DataType:     "int(1)",
				IsNullable:   false,
				DefaultValue: "1",
			},
			{
				Name:         "page_size",
				ColumnName:   "page_size",
				ColumnType:   "measurement",
				DataType:     "int(11)",
				IsNullable:   false,
				DefaultValue: "1000",
			},
		},
	},
	{
		TableName:     "integration",
		IsHidden:      true,
		DefaultGroups: adminsGroup,
		Icon:          "fa-exchange-alt",
		Columns: []api2go.ColumnInfo{
			{
				Name:       "name",
				ColumnName: "name",
				IsUnique:   true,
				IsIndexed:  true,
				ColumnType: "label",
				DataType:   "varchar(100)",
				IsNullable: false,
			},
			{
				Name:       "specification_language",
				ColumnName: "specification_language",
				ColumnType: "label",
				DataType:   "varchar(20)",
				IsNullable: false,
			},
			{
				Name:         "specification_format",
				ColumnName:   "specification_format",
				ColumnType:   "label",
				DataType:     "varchar(10)",
				DefaultValue: "'json'",
			},
			{
				Name:       "specification",
				ColumnName: "specification",
				ColumnType: "content",
				DataType:   "text",
				IsNullable: false,
			},
			{
				Name:       "authentication_type",
				ColumnName: "authentication_type",
				ColumnType: "label",
			},
			{
				Name:       "authentication_specification",
				ColumnName: "authentication_specification",
				ColumnType: "encrypted",
				DataType:   "text",
				IsNullable: false,
			},
			{
				Name:         "enable",
				ColumnName:   "enable",
				ColumnType:   "truefalse",
				DataType:     "int(1)",
				DefaultValue: "1",
				IsNullable:   false,
			},
		},
	},
	{
		TableName:     "task",
		IsHidden:      true,
		DefaultGroups: adminsGroup,
		Icon:          "fa-clock",
		Columns: []api2go.ColumnInfo{
			{
				Name:       "name",
				ColumnName: "name",
				DataType:   "varchar(100)",
				ColumnType: "label",
				IsIndexed:  true,
			},
			{
				Name:       "action_name",
				ColumnName: "action_name",
				DataType:   "varchar(100)",
				ColumnType: "label",
			},
			{
				Name:       "entity_name",
				ColumnName: "entity_name",
				DataType:   "varchar(100)",
				ColumnType: "label",
			},
			{
				Name:       "schedule",
				ColumnName: "schedule",
				DataType:   "varchar(100)",
				ColumnType: "label",
			},
			{
				Name:       "active",
				ColumnName: "active",
				DataType:   "int(1)",
				ColumnType: "truefalse",
			},
			{
				Name:       "attributes",
				ColumnName: "attributes",
				DataType:   "text",
				ColumnType: "json",
			},
			{
				Name:       "job_type",
				ColumnName: "job_type",
				DataType:   "varchar(100)",
				ColumnType: "label",
			},
		},
	},
	//{
	//	TableName:     "marketplace",
	//	IsHidden:      true,
	//	DefaultGroups: adminsGroup,
	//	Icon:          "fa-shopping-cart",
	//	Columns: []api2go.ColumnInfo{
	//		{
	//			Name:       "name",
	//			ColumnName: "name",
	//			DataType:   "varchar(100)",
	//			ColumnType: "label",
	//			IsIndexed:  true,
	//		},
	//		{
	//			Name:       "endpoint",
	//			ColumnName: "endpoint",
	//			DataType:   "varchar(200)",
	//			ColumnType: "url",
	//		},
	//		{
	//			Name:         "root_path",
	//			ColumnName:   "root_path",
	//			DataType:     "varchar(100)",
	//			ColumnType:   "label",
	//			DefaultValue: "''",
	//		},
	//	},
	//},
	{
		TableName:     "json_schema",
		Icon:          "fa-code",
		DefaultGroups: adminsGroup,
		IsHidden:      true,
		Columns: []api2go.ColumnInfo{
			{
				Name:       "schema_name",
				ColumnName: "schema_name",
				ColumnType: "label",
				DataType:   "varchar(100)",
				IsNullable: false,
			},
			{
				Name:       "json_schema",
				ColumnType: "json",
				DataType:   "text",
				ColumnName: "json_schema",
			},
		},
	},
	{
		TableName:     "timeline",
		Icon:          "fa-clock-o",
		DefaultGroups: adminsGroup,
		IsHidden:      true,
		Columns: []api2go.ColumnInfo{
			{
				Name:       "event_type",
				ColumnName: "event_type",
				ColumnType: "label",
				DataType:   "varchar(50)",
				IsNullable: false,
			},
			{
				Name:       "title",
				ColumnName: "title",
				ColumnType: "label",
				IsIndexed:  true,
				DataType:   "varchar(50)",
				IsNullable: false,
			},
			{
				Name:       "payload",
				ColumnName: "payload",
				ColumnType: "content",
				DataType:   "text",
				IsNullable: true,
			},
		},
	},
	{
		TableName:     "world",
		IsHidden:      true,
		DefaultGroups: adminsGroup,
		Icon:          "fa-home",
		Columns: []api2go.ColumnInfo{
			{
				Name:       "table_name",
				ColumnName: "table_name",
				IsNullable: false,
				IsUnique:   true,
				IsIndexed:  true,
				DataType:   "varchar(200)",
				ColumnType: "name",
			},
			{
				Name:       "world_schema_json",
				ColumnName: "world_schema_json",
				DataType:   "text",
				IsNullable: false,
				ColumnType: "json",
			},
			{
				Name:         "default_permission",
				ColumnName:   "default_permission",
				DataType:     "int(4)",
				IsNullable:   false,
				DefaultValue: "644",
				ColumnType:   "value",
			},
			{
				Name:         "is_top_level",
				ColumnName:   "is_top_level",
				DataType:     "bool",
				IsNullable:   false,
				DefaultValue: "true",
				ColumnType:   "truefalse",
			},
			{
				Name:         "is_hidden",
				ColumnName:   "is_hidden",
				DataType:     "bool",
				IsNullable:   false,
				DefaultValue: "false",
				ColumnType:   "truefalse",
			},
			{
				Name:         "is_join_table",
				ColumnName:   "is_join_table",
				DataType:     "bool",
				IsNullable:   false,
				DefaultValue: "false",
				ColumnType:   "truefalse",
			},
			{
				Name:         "is_state_tracking_enabled",
				ColumnName:   "is_state_tracking_enabled",
				DataType:     "bool",
				IsNullable:   false,
				DefaultValue: "false",
				ColumnType:   "truefalse",
			},
			{
				Name:         "default_order",
				ColumnName:   "default_order",
				DataType:     "varchar(100)",
				IsNullable:   true,
				DefaultValue: "'+id'",
				ColumnType:   "value",
			},
			{
				Name:         "icon",
				ColumnName:   "icon",
				DataType:     "varchar(20)",
				IsNullable:   true,
				DefaultValue: "'fa-star'",
				ColumnType:   "label",
			},
		},
	},
	{
		TableName:     "stream",
		Icon:          "fa-strikethrough",
		DefaultGroups: adminsGroup,
		IsHidden:      true,
		Columns: []api2go.ColumnInfo{
			{
				Name:       "stream_name",
				ColumnName: "stream_name",
				DataType:   "varchar(100)",
				IsNullable: false,
				ColumnType: "label",
				IsIndexed:  true,
			},
			{
				Name:         "enable",
				ColumnName:   "enable",
				DataType:     "int(1)",
				IsNullable:   false,
				ColumnType:   "truefalse",
				DefaultValue: "1",
			},
			{
				Name:       "stream_contract",
				ColumnName: "stream_contract",
				DataType:   "text",
				IsNullable: false,
				ColumnType: "json",
			},
		},
	},
	{
		TableName:     "user_otp_account",
		Icon:          "fa-sms",
		IsHidden:      true,
		DefaultGroups: []string{},
		Columns: []api2go.ColumnInfo{
			{
				ColumnName: "mobile_number",
				IsIndexed:  true,
				IsUnique:   true,
				DataType:   "varchar(20)",
				ColumnType: "label",
			},
			{
				ColumnName:     "otp_secret",
				IsIndexed:      true,
				ExcludeFromApi: true,
				DataType:       "varchar(100)",
				ColumnType:     "encrypted",
			},
			{
				ColumnName:   "verified",
				DataType:     "int(1)",
				DefaultValue: "0",
				ColumnType:   "truefalse",
				Name:         "verified",
			},
		},
	},
	{
		TableName:     USER_ACCOUNT_TABLE_NAME,
		Icon:          "fa-user",
		DefaultGroups: []string{"users"},
		Columns: []api2go.ColumnInfo{
			{
				Name:       "name",
				ColumnName: "name",
				IsIndexed:  true,
				DataType:   "varchar(80)",
				ColumnType: "label",
			},
			{
				Name:       "email",
				ColumnName: "email",
				DataType:   "varchar(80)",
				IsIndexed:  true,
				IsUnique:   true,
				ColumnType: "email",
			},

			{
				Name:       "password",
				ColumnName: "password",
				DataType:   "varchar(100)",
				ColumnType: "password",
				IsNullable: true,
			},
			{
				Name:         "confirmed",
				ColumnName:   "confirmed",
				DataType:     "boolean",
				ColumnType:   "truefalse",
				IsNullable:   false,
				DefaultValue: "false",
			},
		},
		Validations: []ColumnTag{
			{
				ColumnName: "email",
				Tags:       "email",
			},
			{
				ColumnName: "password",
				Tags:       "required",
			},
			{
				ColumnName: "name",
				Tags:       "required",
			},
		},
		Conformations: []ColumnTag{
			{
				ColumnName: "email",
				Tags:       "email",
			},
		},
	},
	{
		TableName: "usergroup",
		Icon:      "fa-users",
		IsHidden:  true,
		Columns: []api2go.ColumnInfo{
			{
				Name:       "name",
				ColumnName: "name",
				IsIndexed:  true,
				DataType:   "varchar(80)",
				ColumnType: "label",
			},
		},
	},
	{
		TableName:     "action",
		DefaultGroups: adminsGroup,
		IsHidden:      true,
		Icon:          "fa-bolt",
		Columns: []api2go.ColumnInfo{
			{
				Name:       "action_name",
				IsIndexed:  true,
				ColumnName: "action_name",
				DataType:   "varchar(100)",
				ColumnType: "label",
			},
			{
				Name:       "label",
				ColumnName: "label",
				IsIndexed:  true,
				DataType:   "varchar(100)",
				ColumnType: "label",
			},
			{
				Name:         "instance_optional",
				ColumnName:   "instance_optional",
				IsIndexed:    false,
				DataType:     "bool",
				ColumnType:   "truefalse",
				DefaultValue: "true",
			},
			{
				Name:       "action_schema",
				ColumnName: "action_schema",
				DataType:   "text",
				ColumnType: "json",
			},
		},
	},
	{
		TableName:     "smd",
		IsHidden:      true,
		DefaultGroups: adminsGroup,
		Columns: []api2go.ColumnInfo{
			{
				Name:       "name",
				ColumnName: "name",
				IsIndexed:  true,
				DataType:   "varchar(100)",
				ColumnType: "label",
				IsNullable: false,
			},
			{
				Name:       "label",
				ColumnName: "label",
				DataType:   "varchar(100)",
				ColumnType: "label",
				IsNullable: false,
			},
			{
				Name:       "initial_state",
				ColumnName: "initial_state",
				DataType:   "varchar(100)",
				ColumnType: "label",
				IsNullable: false,
			},
			{
				Name:       "events",
				ColumnName: "events",
				DataType:   "text",
				ColumnType: "json",
				IsNullable: false,
			},
		},
	},
	{
		TableName:     "oauth_connect",
		IsHidden:      true,
		DefaultGroups: adminsGroup,
		Columns: []api2go.ColumnInfo{
			{
				Name:       "name",
				ColumnName: "name",
				IsUnique:   true,
				IsIndexed:  true,
				DataType:   "varchar(80)",
				ColumnType: "label",
			},
			{
				Name:       "client_id",
				ColumnName: "client_id",
				DataType:   "varchar(150)",
				ColumnType: "label",
			},
			{
				Name:       "client_secret",
				ColumnName: "client_secret",
				DataType:   "varchar(500)",
				ColumnType: "encrypted",
			},
			{
				Name:         "scope",
				ColumnName:   "scope",
				DataType:     "varchar(1000)",
				ColumnType:   "content",
				DefaultValue: "'https://www.googleapis.com/auth/spreadsheets'",
			},
			{
				Name:         "response_type",
				ColumnName:   "response_type",
				DataType:     "varchar(80)",
				ColumnType:   "label",
				DefaultValue: "'code'",
			},
			{
				Name:         "redirect_uri",
				ColumnName:   "redirect_uri",
				DataType:     "varchar(80)",
				ColumnType:   "url",
				DefaultValue: "'/oauth/response'",
			},
			{
				Name:         "auth_url",
				ColumnName:   "auth_url",
				DataType:     "varchar(200)",
				DefaultValue: "'https://accounts.google.com/o/oauth2/auth'",
				ColumnType:   "url",
			},
			{
				Name:         "token_url",
				ColumnName:   "token_url",
				DataType:     "varchar(200)",
				DefaultValue: "'https://accounts.google.com/o/oauth2/token'",
				ColumnType:   "url",
			},
			{
				Name:         "profile_url",
				ColumnName:   "profile_url",
				DataType:     "varchar(200)",
				DefaultValue: "'https://www.googleapis.com/oauth2/v1/userinfo?alt=json'",
				ColumnType:   "url",
			},
			{
				Name:         "profile_email_path",
				ColumnName:   "profile_email_path",
				DataType:     "varchar(200)",
				DefaultValue: "'email'",
				ColumnType:   "label",
			},
			{
				Name:         "allow_login",
				ColumnName:   "allow_login",
				DataType:     "boolean",
				DefaultValue: "false",
				ColumnType:   "truefalse",
			},
			{
				Name:         "access_type_offline",
				ColumnName:   "access_type_offline",
				DataType:     "boolean",
				DefaultValue: "false",
				ColumnType:   "truefalse",
			},
		},
	},
	{
		TableName:     "data_exchange",
		IsHidden:      true,
		DefaultGroups: adminsGroup,
		Columns: []api2go.ColumnInfo{
			{
				Name:       "name",
				ColumnName: "name",
				ColumnType: "label",
				DataType:   "varchar(200)",
				IsIndexed:  true,
			},
			{
				Name:       "source_attributes",
				ColumnName: "source_attributes",
				ColumnType: "json",
				DataType:   "text",
			},
			{
				Name:       "source_type",
				ColumnName: "source_type",
				ColumnType: "label",
				DataType:   "varchar(100)",
			},
			{
				Name:       "target_attributes",
				ColumnName: "target_attributes",
				ColumnType: "json",
				DataType:   "text",
			},
			{
				Name:       "target_type",
				ColumnName: "target_type",
				ColumnType: "label",
				DataType:   "varchar(100)",
			},
			{
				Name:       "attributes",
				ColumnName: "attributes",
				ColumnType: "json",
				DataType:   "text",
			},
			{
				Name:       "options",
				ColumnName: "options",
				ColumnType: "json",
				DataType:   "text",
			},
		},
	},
	{
		TableName:     "oauth_token",
		IsHidden:      true,
		DefaultGroups: adminsGroup,
		Columns: []api2go.ColumnInfo{
			{
				Name:       "access_token",
				ColumnName: "access_token",
				ColumnType: "encrypted",
				DataType:   "varchar(1000)",
			},
			{
				Name:       "expires_in",
				ColumnName: "expires_in",
				ColumnType: "measurement",
				DataType:   "int(11)",
			},
			{
				Name:       "refresh_token",
				ColumnName: "refresh_token",
				ColumnType: "encrypted",
				DataType:   "varchar(1000)",
			},
			{
				Name:       "token_type",
				ColumnName: "token_type",
				ColumnType: "label",
				DataType:   "varchar(20)",
			},
		},
	},
	{
		TableName:     "cloud_store",
		DefaultGroups: adminsGroup,
		IsHidden:      true,
		Columns: []api2go.ColumnInfo{
			{
				Name:       "Name",
				ColumnName: "name",
				ColumnType: "label",
				DataType:   "varchar(100)",
				IsUnique:   true,
			},
			{
				Name:       "store_type",
				ColumnName: "store_type",
				ColumnType: "label",
				DataType:   "varchar(100)",
			},
			{
				Name:       "store_provider",
				ColumnName: "store_provider",
				ColumnType: "label",
				DataType:   "varchar(100)",
			},
			{
				Name:       "root_path",
				ColumnName: "root_path",
				ColumnType: "label",
				DataType:   "varchar(1000)",
			},
			{
				Name:       "store_parameters",
				ColumnName: "store_parameters",
				ColumnType: "json",
				DataType:   "text",
			},
		},
	},
	{
		TableName:     "site",
		DefaultGroups: adminsGroup,
		IsHidden:      true,
		Columns: []api2go.ColumnInfo{
			{
				Name:       "name",
				ColumnName: "name",
				ColumnType: "label",
				DataType:   "varchar(100)",
			},
			{
				Name:       "hostname",
				ColumnName: "hostname",
				ColumnType: "label",
				DataType:   "varchar(100)",
			},
			{
				Name:       "path",
				ColumnName: "path",
				ColumnType: "label",
				DataType:   "varchar(100)",
			},
			{
				Name:         "enable",
				ColumnName:   "enable",
				ColumnType:   "truefalse",
				DataType:     "bool",
				DefaultValue: "true",
			},
			{
				Name:         "enable_https",
				ColumnName:   "enable_https",
				ColumnType:   "truefalse",
				DataType:     "bool",
				DefaultValue: "false",
			},

			{
				Name:         "ftp_enabled",
				ColumnName:   "ftp_enabled",
				ColumnType:   "truefalse",
				DataType:     "int(1)",
				DefaultValue: "0",
			},
		},
	},
	{
		TableName:     "mail_server",
		IsHidden:      true,
		Icon:          "fa-envelope",
		DefaultGroups: adminsGroup,
		Columns: []api2go.ColumnInfo{
			{
				Name:       "hostname",
				ColumnName: "hostname",
				DataType:   "varchar(100)",
				ColumnType: "label",
			},
			{
				Name:         "is_enabled",
				ColumnName:   "is_enabled",
				DataType:     "int(1)",
				ColumnType:   "truefalse",
				DefaultValue: "0",
			},
			{
				Name:         "listen_interface",
				ColumnName:   "listen_interface",
				DataType:     "varchar(100)",
				ColumnType:   "label",
				DefaultValue: "'0.0.0.0:465'",
			},
			{
				Name:         "max_size",
				ColumnName:   "max_size",
				DataType:     "int(11)",
				ColumnType:   "measurement",
				DefaultValue: "10000",
			},
			{
				Name:         "max_clients",
				ColumnName:   "max_clients",
				DataType:     "int(11)",
				ColumnType:   "measurement",
				DefaultValue: "20",
			},
			{
				Name:         "xclient_on",
				ColumnName:   "xclient_on",
				DataType:     "int(1)",
				ColumnType:   "truefalse",
				DefaultValue: "0",
			},
			{
				Name:         "always_on_tls",
				ColumnName:   "always_on_tls",
				DataType:     "int(1)",
				ColumnType:   "truefalse",
				DefaultValue: "1",
			},
			{
				Name:         "authentication_required",
				ColumnName:   "authentication_required",
				DataType:     "int(1)",
				ColumnType:   "truefalse",
				DefaultValue: "1",
			},
		},
	},
	{
		TableName:     "mail_account",
		IsHidden:      true,
		DefaultGroups: adminsGroup,
		Icon:          "fa-envelope",
		Columns: []api2go.ColumnInfo{
			{
				Name:       "username",
				ColumnName: "username",
				DataType:   "varchar(100)",
				ColumnType: "label",
				IsUnique:   true,
			},
			{
				Name:       "password",
				ColumnName: "password",
				ColumnType: "password",
			},
			{
				Name:       "password_md5",
				ColumnName: "password_md5",
				ColumnType: "md5-bcrypt",
			},
		},
	},
	{
		TableName:     "mail_box",
		IsHidden:      true,
		DefaultGroups: adminsGroup,
		Columns: []api2go.ColumnInfo{
			{
				Name:       "name",
				ColumnName: "name",
				DataType:   "varchar(100)",
				ColumnType: "label",
			},
			{
				Name:         "subscribed",
				ColumnName:   "subscribed",
				DataType:     "bool",
				ColumnType:   "truefalse",
				DefaultValue: "true",
			},
			{
				Name:         "uidvalidity",
				ColumnName:   "uidvalidity",
				DataType:     "int(11)",
				ColumnType:   "value",
				DefaultValue: "1",
			},
			{
				Name:         "nextuid",
				ColumnName:   "nextuid",
				DataType:     "int(11)",
				ColumnType:   "value",
				DefaultValue: "1",
			},
			{
				Name:       "attributes",
				ColumnName: "attributes",
				DataType:   "varchar(100)",
				ColumnType: "label",
			},
			{
				Name:       "flags",
				ColumnName: "flags",
				DataType:   "varchar(100)",
				ColumnType: "label",
			},
			{
				Name:       "permanent_flags",
				ColumnName: "permanent_flags",
				DataType:   "varchar(100)",
				ColumnType: "label",
			},
		},
	},
	{
		TableName:     "mail",
		IsHidden:      true,
		Icon:          "fa-envelope",
		DefaultGroups: adminsGroup,
		Columns: []api2go.ColumnInfo{
			{
				Name:       "message_id",
				ColumnName: "message_id",
				DataType:   "varchar(100)",
				ColumnType: "label",
			},
			{
				Name:       "mail_id",
				ColumnName: "mail_id",
				DataType:   "varchar(100)",
				ColumnType: "label",
			},
			{
				Name:       "from_address",
				ColumnName: "from_address",
				DataType:   "varchar(200)",
				ColumnType: "label",
			},
			{
				Name:       "internal_date",
				ColumnName: "internal_date",
				DataType:   "timestamp",
				ColumnType: "datetime",
			},
			{
				Name:       "to_address",
				ColumnName: "to_address",
				DataType:   "varchar(200)",
				ColumnType: "label",
			},
			{
				Name:       "reply_to_address",
				ColumnName: "reply_to_address",
				DataType:   "varchar(200)",
				ColumnType: "label",
			},
			{
				Name:       "sender_address",
				ColumnName: "sender_address",
				DataType:   "varchar(200)",
				ColumnType: "label",
			},
			{
				Name:       "subject",
				ColumnName: "subject",
				DataType:   "varchar(200)",
				ColumnType: "label",
			},
			{
				Name:       "body",
				ColumnName: "body",
				DataType:   "text",
				ColumnType: "label",
			},
			{
				Name:       "mail",
				ColumnName: "mail",
				ColumnType: "gzip",
				DataType:   "blob",
			},
			{
				Name:       "spam_score",
				ColumnName: "spam_score",
				ColumnType: "measurement",
				DataType:   "float",
			},
			{
				Name:       "hash",
				ColumnName: "hash",
				DataType:   "varchar(100)",
				ColumnType: "label",
			},
			{
				Name:       "content_type",
				ColumnName: "content_type",
				DataType:   "text",
				ColumnType: "content",
			},
			{
				Name:       "recipient",
				ColumnName: "recipient",
				DataType:   "varchar(200)",
				ColumnType: "label",
			},
			{
				Name:         "has_attachment",
				ColumnName:   "has_attachment",
				DataType:     "bool",
				ColumnType:   "truefalse",
				DefaultValue: "false",
			},
			{
				Name:       "ip_addr",
				ColumnName: "ip_addr",
				DataType:   "varchar(30)",
				ColumnType: "label",
			},
			{
				Name:       "return_path",
				ColumnName: "return_path",
				DataType:   "VARCHAR(255)",
				ColumnType: "content",
			},
			{
				Name:         "is_tls",
				ColumnName:   "is_tls",
				DataType:     "bool",
				ColumnType:   "truefalse",
				DefaultValue: "false",
			},
			{
				Name:         "seen",
				ColumnName:   "seen",
				DataType:     "bool",
				ColumnType:   "truefalse",
				DefaultValue: "false",
			},
			{
				Name:         "recent",
				ColumnName:   "recent",
				DataType:     "bool",
				ColumnType:   "truefalse",
				DefaultValue: "true",
			},
			{
				Name:         "deleted",
				ColumnName:   "deleted",
				DataType:     "bool",
				ColumnType:   "truefalse",
				DefaultValue: "false",
			},
			{
				Name:         "spam",
				ColumnName:   "spam",
				DataType:     "bool",
				ColumnType:   "truefalse",
				DefaultValue: "false",
			},
			{
				Name:       "size",
				ColumnName: "size",
				DataType:   "int(11)",
				ColumnType: "value",
			},
			{
				Name:         "flags",
				ColumnName:   "flags",
				DataType:     "varchar(500)",
				ColumnType:   "label",
				DefaultValue: "",
			},
		},
	},
	{
		TableName:     "outbox",
		IsHidden:      true,
		Icon:          "fa-envelope",
		DefaultGroups: adminsGroup,
		Columns: []api2go.ColumnInfo{
			{
				Name:       "from_address",
				ColumnName: "from_address",
				DataType:   "varchar(200)",
				ColumnType: "label",
			},
			{
				Name:       "to_address",
				ColumnName: "to_address",
				DataType:   "varchar(200)",
				ColumnType: "label",
			},
			{
				Name:       "to_host",
				ColumnName: "to_host",
				DataType:   "varchar(200)",
				ColumnType: "label",
			},
			{
				Name:       "mail",
				ColumnName: "mail",
				ColumnType: "gzip",
				DataType:   "blob",
			},
			{
				Name:         "sent",
				ColumnName:   "sent",
				ColumnType:   "truefalse",
				DataType:     "int(1)",
				DefaultValue: "0",
			},
		},
	},
}

//var StandardMarketplaces = []Marketplace{
//	{
//		RootPath: "",
//		Endpoint: "https://github.com/daptin/market.git",
//		Name:     "daptin",
//	},
//}

var StandardStreams = []StreamContract{
	{
		StreamName:     "table",
		RootEntityName: "world",
		Columns: []api2go.ColumnInfo{
			{
				Name:       "table_name",
				ColumnType: "label",
			},
			{
				Name:       "reference_id",
				ColumnType: "label",
			},
		},
	},
	{
		StreamName:     "transformed_user",
		RootEntityName: USER_ACCOUNT_TABLE_NAME,
		Columns: []api2go.ColumnInfo{
			{
				Name:       "transformed_user_name",
				ColumnType: "label",
			},
			{
				Name:       "primary_email",
				ColumnType: "label",
			},
		},
		Transformations: []Transformation{
			{
				Operation: "select",
				Attributes: map[string]interface{}{
					"Columns": []string{"name", "email"},
				},
			},
			{
				Operation: "rename",
				Attributes: map[string]interface{}{
					"OldName": "name",
					"NewName": "transformed_user_name",
				},
			},
			{
				Operation: "rename",
				Attributes: map[string]interface{}{
					"OldName": "email",
					"NewName": "primary_email",
				},
			},
			{
				Operation:  "filter",
				Attributes: map[string]interface{}{},
			},
		},
	},
}

type TableRelation struct {
	api2go.TableRelation
	OnDelete string
}

type TableInfo struct {
	TableName              string `db:"table_name"`
	TableId                int
	DefaultPermission      auth.AuthPermission `db:"default_permission"`
	Columns                []api2go.ColumnInfo
	StateMachines          []LoopbookFsmDescription
	Relations              []api2go.TableRelation
	IsTopLevel             bool `db:"is_top_level"`
	Permission             auth.AuthPermission
	UserId                 uint64   `db:"user_account_id"`
	IsHidden               bool     `db:"is_hidden"`
	IsJoinTable            bool     `db:"is_join_table"`
	IsStateTrackingEnabled bool     `db:"is_state_tracking_enabled"`
	IsAuditEnabled         bool     `db:"is_audit_enabled"`
	TranslationsEnabled    bool     `db:"translation_enabled"`
	DefaultGroups          []string `db:"default_groups"`
	Validations            []ColumnTag
	Conformations          []ColumnTag
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
				log.Infof("Relation already exists: %v", relation)
				break
			}
		}

		if !exists {
			ti.Relations = append(ti.Relations, relation)
		}
	}

}

type ColumnTag struct {
	ColumnName string
	Tags       string
}
