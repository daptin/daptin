package resource

import (
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/columns"
	"github.com/daptin/daptin/server/fsm"
	"github.com/daptin/daptin/server/table_info"
	"github.com/daptin/daptin/server/task"
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
		ColumnDescription: "The primary internal identifier for each database record. This auto-incrementing " +
			"integer serves as the unique primary key but is excluded from API responses to maintain data abstraction.",
		ColumnType: "id",
	},
	{
		Name:       "version",
		ColumnName: "version",
		DataType:   "INTEGER",
		ColumnType: "measurement",
		ColumnDescription: "A counter that tracks the number of modifications made to a record. Starting at 1 for " +
			"new records, this integer increments with each update to support optimistic " +
			"concurrency control and change tracking. Exposed through the API.",
		DefaultValue:   "1",
		ExcludeFromApi: true,
	},
	{
		Name:       "created_at",
		ColumnName: "created_at",
		DataType:   "timestamp",
		ColumnDescription: "Timestamp recording when the record was initially created in the database. " +
			"Automatically set to the current time upon record creation and indexed for efficient temporal queries.",
		DefaultValue: "current_timestamp",
		ColumnType:   "datetime",
		IsIndexed:    true,
	},
	{
		Name:       "updated_at",
		ColumnName: "updated_at",
		DataType:   "timestamp",
		ColumnDescription: "Timestamp indicating when the record was last modified. This " +
			"non-nullable field is indexed to enable efficient filtering and sorting of records by modification time.",
		IsIndexed:  true,
		IsNullable: true,
		ColumnType: "datetime",
	},
	{
		Name:       "reference_id",
		ColumnName: "reference_id",
		DataType:   "blob",
		IsIndexed:  true,
		ColumnDescription: "A unique external identifier stored as a blob that allows referencing the record from " +
			"outside systems. This non-nullable field serves as a public-facing alias for the " +
			"internal ID and is indexed for quick lookups.",
		IsUnique:   true,
		IsNullable: false,
		ColumnType: "alias",
	},
	{
		Name:       "permission",
		ColumnName: "permission",
		DataType:   "int(11)",
		ColumnDescription: "An integer BITMASK value representing access control settings for the record. This field " +
			"determines what operations can be performed on the record based on user roles and privileges.",
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
	api2go.NewTableRelationWithNames("data_exchange", "user_data_exchange", "has_one", "user_account", "as_user_id"),
	api2go.NewTableRelation("timeline", "belongs_to", "world"),
	api2go.NewTableRelation("cloud_store", "has_one", "credential"),
	api2go.NewTableRelation("site", "has_one", "cloud_store"),
	api2go.NewTableRelation("mail_account", "belongs_to", "mail_server"),
	api2go.NewTableRelation("mail_box", "belongs_to", "mail_account"),
	api2go.NewTableRelation("mail", "belongs_to", "mail_box"),
	api2go.NewTableRelationWithNames("task", "task_executed", "has_one", USER_ACCOUNT_TABLE_NAME, "as_user_id"),
	api2go.NewTableRelation("calendar", "has_one", "collection"),
	api2go.NewTableRelationWithNames("user_otp_account", "primary_user_otp", "belongs_to", "user_account", "otp_of_account"),
}

var SystemSmds []fsm.LoopbookFsmDescription
var SystemExchanges []ExchangeContract

var SystemActions = []actionresponse.Action{
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
		OutFields: []actionresponse.Outcome{
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
		OutFields: []actionresponse.Outcome{
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
		OutFields: []actionresponse.Outcome{
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
		Name:             "get_action_schema",
		Label:            "Get Action Schema",
		OnType:           "action",
		InstanceOptional: false,
		OutFields: []actionresponse.Outcome{
			{
				Type:   "client.file.download",
				Method: "ACTIONRESPONSE",
				Attributes: map[string]interface{}{
					"content":     "!btoa(subject.action_schema)",
					"name":        "!subject.action_name + '.action.json'",
					"contentType": "application/json",
					"message":     "!'Action Schema for ' + subject.action_name",
				},
			},
		},
	},
	{
		Name:             "download_public_key",
		Label:            "Download public key",
		OnType:           "certificate",
		InstanceOptional: false,
		OutFields: []actionresponse.Outcome{
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
		OutFields: []actionresponse.Outcome{
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
		OutFields: []actionresponse.Outcome{
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
		OutFields: []actionresponse.Outcome{
			{
				Type:      "otp.generate",
				Method:    "EXECUTE",
				Reference: "otp",
				Attributes: map[string]interface{}{
					"email":  "$.email",
					"mobile": "~mobile_number",
				},
			},
			//{
			//	Type:      "2factor.in",
			//	Method:    "GET_api_key-SMS-phone_number-otp",
			//	Condition: "!mobile_number != null && mobile_number != undefined && mobile_number != ''",
			//	Attributes: map[string]interface{}{
			//		"phone_number": "~mobile_number",
			//		"otp":          "$otp.otp",
			//	},
			//},
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
		OutFields: []actionresponse.Outcome{
			{
				Type:   "otp.login.verify",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"otp":    "~otp",
					"mobile": "~mobile_number",
					"email":  "~email",
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
		OutFields: []actionresponse.Outcome{
			{
				Type:      "otp.generate",
				Method:    "EXECUTE",
				Reference: "otp",
				Attributes: map[string]interface{}{
					"email":  "~email",
					"mobile": "~mobile_number",
				},
			},
			//{
			//	Type:      "2factor.in",
			//	Method:    "GET_api_key-SMS-phone_number-otp",
			//	Condition: "!mobile_number != null && mobile_number != undefined && mobile_number != ''",
			//	Attributes: map[string]interface{}{
			//		"phone_number": "~mobile_number",
			//		"otp":          "$otp.otp",
			//	},
			//},
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
			{
				Name:       "email",
				ColumnName: "email",
				ColumnType: "label",
			},
		},
		OutFields: []actionresponse.Outcome{
			{
				Type:   "otp.login.verify",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"otp":    "~otp",
					"mobile": "~mobile_number",
					"email":  "~email",
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
		OutFields: []actionresponse.Outcome{
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
		Name:             "remove_table",
		Label:            "Delete table",
		OnType:           "world",
		InstanceOptional: false,
		InFields:         []api2go.ColumnInfo{},
		OutFields: []actionresponse.Outcome{
			{
				Type:   "world.delete",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"world_id": "$.reference_id",
				},
			},
		},
	},
	{
		Name:             "rename_column",
		Label:            "Rename column",
		OnType:           "world",
		InstanceOptional: true,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "table_name",
				ColumnName: "table_name",
				ColumnType: "label",
			},
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
		OutFields: []actionresponse.Outcome{
			{
				Type:   "world.column.rename",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"world_name":      "~table_name",
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
		OutFields: []actionresponse.Outcome{
			{
				Type:   "site.storage.sync",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"cloud_store_id": "$.cloud_store_id",
					"site_id":        "$.reference_id",
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
			{
				Name:       "Credential name",
				ColumnName: "credential_name",
				ColumnType: "label",
			},
		},
		OutFields: []actionresponse.Outcome{
			{
				Type:   "column.storage.sync",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"column_name":     "~column_name",
					"credential_name": "~credential_name",
					"table_name":      "~table_name",
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
		OutFields: []actionresponse.Outcome{
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
		OutFields: []actionresponse.Outcome{
			{
				Type:   "system_json_schema_update",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"json_schema": "!JSON.parse('[{\"name\":\"empty.json\",\"file\":\"data:application/json;base64,e30K\",\"type\":\"application/json\"}]')",
				},
			},
		},
	},
	{
		Name:             "generate_random_data",
		Label:            "Generate random data",
		OnType:           "world",
		InstanceOptional: true,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "Number of records",
				ColumnName: "count",
				ColumnType: "measurement",
			},
			{
				Name:       "Table name",
				ColumnName: "table_name",
				ColumnType: "label",
			},
		},
		OutFields: []actionresponse.Outcome{
			{
				Type:   "generate.random.data",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"count":             "~count",
					"table_name":        "~table_name",
					"user_reference_id": "$user.reference_id",
					"user_account_id":   "$user.id",
				},
			},
		},
		Validations: []columns.ColumnTag{
			{
				ColumnName: "count",
				Tags:       "gt=0",
			},
		},
	},
	{
		Name:             "export_data",
		Label:            "Export data for backup",
		OnType:           "world",
		InstanceOptional: true,
		InFields: []api2go.ColumnInfo{
			{
				ColumnName: "table_name",
				Name:       "table_name",
				ColumnType: "label",
			},
			{
				ColumnName:   "format",
				Name:         "format",
				ColumnType:   "label",
				DefaultValue: "json",
			},
			{
				ColumnName: "columns",
				Name:       "columns",
				ColumnType: "label",
			},
			{
				ColumnName: "include_headers",
				Name:       "include_headers",
				ColumnType: "truefalse",
			},
		},
		OutFields: []actionresponse.Outcome{
			{
				Type:   "__data_export",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"table_name":      "~table_name",
					"format":          "~format",
					"include_headers": "~include_headers",
					"columns":         "~columns",
				},
			},
		},
	},
	{
		Name:             "export_csv_data",
		Label:            "Export CSV data",
		OnType:           "world",
		InstanceOptional: true,
		InFields: []api2go.ColumnInfo{
			{
				ColumnName: "table_name",
				Name:       "table_name",
				ColumnType: "label",
			},
		},
		OutFields: []actionresponse.Outcome{
			{
				Type:   "__csv_data_export",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"table_name": "~table_name",
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
				Name:       "Import file",
				ColumnName: "dump_file",
				ColumnType: "file.json|yaml|toml|hcl|csv|docx|xlsx|pdf|html",
				IsNullable: false,
			},
			{
				Name:       "truncate_before_insert",
				ColumnName: "truncate_before_insert",
				ColumnType: "truefalse",
			},
			{
				Name:       "batch_size",
				ColumnName: "batch_size",
				ColumnType: "measurement",
			},
		},
		OutFields: []actionresponse.Outcome{
			{
				Type:   "__data_import",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"world_reference_id":     "$.reference_id",
					"truncate_before_insert": "~truncate_before_insert",
					"dump_file":              "~dump_file",
					"table_name":             "$.table_name",
					"batch_size":             "~batch_size",
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
			{
				Name:         "Path",
				ColumnName:   "path",
				ColumnType:   "label",
				IsNullable:   true,
				DefaultValue: "",
			},
		},
		OutFields: []actionresponse.Outcome{
			{
				Type:   "cloudstore.file.upload",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"file":            "~file",
					"credential_name": "$.credential_name",
					"store_provider":  "$.store_provider",
					"path":            "~path",
					"root_path":       "$.root_path",
				},
			},
		},
	},
	{
		Name:             "create_site",
		Label:            "Create new site on this store",
		OnType:           "cloud_store",
		InstanceOptional: false,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "Site type",
				ColumnName: "site_type",
				ColumnType: "label",
				IsNullable: false,
			},
			{
				Name:         "Path",
				ColumnName:   "path",
				ColumnType:   "label",
				IsNullable:   false,
				DefaultValue: "",
			},
			{
				Name:       "Hostname",
				ColumnName: "hostname",
				ColumnType: "label",
				IsNullable: false,
			},
		},
		OutFields: []actionresponse.Outcome{
			{
				Type:   "cloudstore.site.create",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"credential_name": "$.credential_name",
					"store_provider":  "$.store_provider",
					"cloud_store_id":  "$.reference_id",
					"path":            "~path",
					"user_account_id": "$user.reference_id",
					"hostname":        "~hostname",
					"site_type":       "~site_type",
					"root_path":       "$.root_path",
				},
			},
		},
	},

	{
		Name:             "delete_path",
		Label:            "Delete path on a site",
		OnType:           "cloud_store",
		InstanceOptional: false,
		InFields: []api2go.ColumnInfo{
			{
				Name:         "Path",
				ColumnName:   "path",
				ColumnType:   "label",
				IsNullable:   true,
				DefaultValue: "",
			},
		},
		OutFields: []actionresponse.Outcome{
			{
				Type:   "site.file.delete",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"credential_name": "$.credential_name",
					"store_provider":  "$.store_provider",
					"path":            "~path",
					"root_path":       "$.root_path",
				},
			},
		},
	},

	{
		Name:             "create_folder",
		Label:            "Create folder on a cloud store",
		OnType:           "cloud_store",
		InstanceOptional: false,
		InFields: []api2go.ColumnInfo{
			{
				Name:         "Path",
				ColumnName:   "path",
				ColumnType:   "label",
				IsNullable:   true,
				DefaultValue: "",
			},
			{
				Name:       "Name",
				ColumnName: "name",
				ColumnType: "label",
				IsNullable: false,
			},
		},
		OutFields: []actionresponse.Outcome{
			{
				Type:   "cloudstore.folder.create",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"credential_name": "$.credential_name",
					"store_provider":  "$.store_provider",
					"path":            "~path",
					"name":            "~name",
					"root_path":       "$.root_path",
				},
			},
		},
	},

	{
		Name:             "move_path",
		Label:            "Create folder on a cloud store",
		OnType:           "cloud_store",
		InstanceOptional: false,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "Source path",
				ColumnName: "source",
				ColumnType: "label",
			},
			{
				Name:       "Destination path",
				ColumnName: "destination",
				ColumnType: "label",
				IsNullable: false,
			},
		},
		OutFields: []actionresponse.Outcome{
			{
				Type:   "cloudstore.path.move",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"credential_name": "$.credential_name",
					"store_provider":  "$.store_provider",
					"source":          "~source",
					"destination":     "~destination",
					"root_path":       "$.root_path",
				},
			},
		},
	},
	{
		Name:             "list_files",
		Label:            "List files in the site path",
		OnType:           "site",
		InstanceOptional: false,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "path",
				ColumnName: "path",
				ColumnType: "label",
				IsNullable: false,
			},
		},
		OutFields: []actionresponse.Outcome{
			{
				Type:   "site.file.list",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"site_id": "$.reference_id",
					"path":    "~path",
				},
			},
		},
	},

	{
		Name:             "get_file",
		Label:            "Get file at the path in site",
		OnType:           "site",
		InstanceOptional: false,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "path",
				ColumnName: "path",
				ColumnType: "label",
				IsNullable: false,
			},
		},
		OutFields: []actionresponse.Outcome{
			{
				Type:   "site.file.get",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"site_id": "$.reference_id",
					"path":    "~path",
				},
			},
		},
	},
	{
		Name:             "delete_file",
		Label:            "Delete file in the site",
		OnType:           "site",
		InstanceOptional: false,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "path",
				ColumnName: "path",
				ColumnType: "label",
				IsNullable: false,
			},
		},
		OutFields: []actionresponse.Outcome{
			{
				Type:   "site.file.delete",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"site_id": "$.reference_id",
					"path":    "~path",
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
		OutFields: []actionresponse.Outcome{
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
		Validations: []columns.ColumnTag{
			{
				ColumnName: "entity_name",
				Tags:       "required",
			},
		},
		OutFields: []actionresponse.Outcome{
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
				Name:         "Create entity if not exists",
				ColumnName:   "create_if_not_exists",
				ColumnType:   "truefalse",
				DefaultValue: "false",
				IsNullable:   true,
			},
			{
				Name:         "Add missing columns",
				ColumnName:   "add_missing_columns",
				ColumnType:   "truefalse",
				DefaultValue: "false",
				IsNullable:   true,
			},
		},
		Validations: []columns.ColumnTag{
			{
				ColumnName: "entity_name",
				Tags:       "required",
			},
		},
		OutFields: []actionresponse.Outcome{
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
		OutFields: []actionresponse.Outcome{
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
		OutFields: []actionresponse.Outcome{
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
		Validations: []columns.ColumnTag{
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
		Conformations: []columns.ColumnTag{
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
		OutFields: []actionresponse.Outcome{
			{
				Type:           USER_ACCOUNT_TABLE_NAME,
				Method:         "POST",
				Reference:      "user",
				SkipInResponse: true,
				Attributes: map[string]interface{}{
					"name":      "~name",
					"email":     "~email",
					"password":  "~password",
					"confirmed": "0",
				},
			},
			{
				Type:           "otp.generate",
				Method:         "EXECUTE",
				Reference:      "otp",
				SkipInResponse: true,
				Condition:      "!mobile != null && mobile != undefined && mobile != ''",
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
		Name:             "reset-password",
		Label:            "Reset password",
		InstanceOptional: true,
		OnType:           USER_ACCOUNT_TABLE_NAME,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "email",
				ColumnName: "email",
				ColumnType: "email",
				IsNullable: false,
			},
		},
		Validations: []columns.ColumnTag{
			{
				ColumnName: "email",
				Tags:       "email",
			},
		},
		Conformations: []columns.ColumnTag{
			{
				ColumnName: "email",
				Tags:       "email",
			},
		},
		OutFields: []actionresponse.Outcome{
			{
				Type:           USER_ACCOUNT_TABLE_NAME,
				Method:         "GET",
				Reference:      "user",
				SkipInResponse: true,
				Attributes: map[string]interface{}{
					"query": "[{\"column\": \"email\", \"operator\": \"is\", \"value\": \"$email\"}]",
				},
			},
			{
				Type:           "otp.generate",
				Method:         "EXECUTE",
				Reference:      "otp",
				SkipInResponse: true,
				Attributes: map[string]interface{}{
					"email": "$email",
				},
			},
			{
				Type:           "mail.send",
				Method:         "EXECUTE",
				SkipInResponse: true,
				Attributes: map[string]interface{}{
					"to":      "~email",
					"subject": "Request for password reset",
					"body":    "Your verification code is: $otp.otp",
					"from":    "no-reply@localhost",
				},
			},
		},
	},
	{
		Name:             "reset-password-verify",
		Label:            "Reset password verify code",
		InstanceOptional: true,
		OnType:           USER_ACCOUNT_TABLE_NAME,
		InFields: []api2go.ColumnInfo{
			{
				Name:       "email",
				ColumnName: "email",
				ColumnType: "email",
				IsNullable: false,
			}, {
				Name:       "otp",
				ColumnName: "otp",
				ColumnType: "value",
				IsNullable: false,
			},
		},
		Validations: []columns.ColumnTag{
			{
				ColumnName: "email",
				Tags:       "email",
			},
		},
		Conformations: []columns.ColumnTag{
			{
				ColumnName: "email",
				Tags:       "email",
			},
		},
		OutFields: []actionresponse.Outcome{
			{
				Type:           USER_ACCOUNT_TABLE_NAME,
				Method:         "GET",
				Reference:      "user",
				SkipInResponse: true,
				Attributes: map[string]interface{}{
					"query": "[{\"column\": \"email\", \"operator\": \"is\", \"value\": \"$email\"}]",
				},
			},
			{
				Type:   "otp.login.verify",
				Method: "EXECUTE",
				Attributes: map[string]interface{}{
					"otp":   "~otp",
					"email": "~email",
				},
			},
			{
				Type:           "random.generate",
				Method:         "EXECUTE",
				Reference:      "newPassword",
				SkipInResponse: true,
				Attributes: map[string]interface{}{
					"type": "password",
				},
			},
			{
				Type:           USER_ACCOUNT_TABLE_NAME,
				SkipInResponse: true,
				Method:         "PATCH",
				Attributes: map[string]interface{}{
					"reference_id": "$user[0].reference_id",
					"password":     "!newPassword.value",
				},
			},
			{
				Type:           "mail.send",
				Method:         "EXECUTE",
				SkipInResponse: true,
				Attributes: map[string]interface{}{
					"to":      "~email",
					"subject": "Request for password reset",
					"body":    "Your new password is: $newPassword.value",
					"from":    "no-reply@localhost",
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
		OutFields: []actionresponse.Outcome{
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
		Name:     "oauth_login_begin",
		Label:    "Authenticate via OAuth",
		OnType:   "oauth_connect",
		InFields: []api2go.ColumnInfo{},
		OutFields: []actionresponse.Outcome{
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
		Label:            "Handle OAuth login response code and state",
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
		OutFields: []actionresponse.Outcome{
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
		OutFields: []actionresponse.Outcome{
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

var StandardTasks []task.Task

var StandardTables = []table_info.TableInfo{
	{
		TableName:     "document",
		IsHidden:      false,
		DefaultGroups: adminsGroup,
		Icon:          "fa-file",
		Columns: []api2go.ColumnInfo{
			{
				ColumnName:        "document_name",
				Name:              "document_name",
				ColumnType:        "label",
				DataType:          "varchar(99999)",
				IsNullable:        false,
				IsIndexed:         true,
				ColumnDescription: "The name of the document file, used for identification and display purposes. This field is indexed for quick searching and retrieval.",
			},
			{
				ColumnName:        "document_path",
				Name:              "document_path",
				ColumnType:        "label",
				DataType:          "varchar(99999)",
				IsNullable:        false,
				IsIndexed:         true,
				ColumnDescription: "The file system path or URL where the document is stored. Supports paths up to 2000 characters and is indexed for efficient lookup.",
			},
			{
				ColumnName:        "document_extension",
				Name:              "document_extension",
				ColumnType:        "label",
				DataType:          "varchar(1000)",
				IsNullable:        false,
				IsIndexed:         true,
				ColumnDescription: "The file extension of the document (e.g., pdf, docx, txt), used for file type identification and filtering. Indexed for quick file type searches.",
			},
			{
				ColumnName:        "mime_type",
				Name:              "mime_type",
				ColumnType:        "label",
				DataType:          "varchar(1000)",
				IsNullable:        false,
				IsIndexed:         true,
				ColumnDescription: "The MIME type of the document (e.g., application/pdf, text/plain), which identifies the file format. Indexed to support content type filtering.",
			},
			{
				ColumnName:        "document_content",
				Name:              "document_content",
				IsForeignKey:      true,
				IsNullable:        false,
				ColumnType:        "file.*",
				DataType:          "longblob",
				ColumnDescription: "The actual binary content of the document, stored as a reference to cloud storage. Supports files of any type through the generic file.* handler.",
			},
		},
	},
	{
		TableName:     "calendar",
		IsHidden:      false,
		DefaultGroups: adminsGroup,
		Icon:          "fa-calendar-alt",
		Columns: []api2go.ColumnInfo{
			{
				ColumnName:        "rpath",
				Name:              "rpath",
				ColumnType:        "label",
				IsUnique:          true,
				DataType:          "varchar(500)",
				IsNullable:        false,
				ColumnDescription: "The resource path for the calendar item, serving as a unique identifier within the calendaring system. Must be unique across all calendar resources.",
			},
			{
				ColumnName:        "content",
				Name:              "content",
				ColumnType:        "file.ical",
				DataType:          "longblob",
				IsNullable:        false,
				IsForeignKey:      true,
				ColumnDescription: "The iCalendar (RFC 5545) format content of the calendar entry, stored as a binary blob. This contains all event details including dates, recurrence rules, and attendees.",
			},
		},
	},
	{
		TableName:     "collection",
		IsHidden:      false,
		DefaultGroups: adminsGroup,
		Icon:          "fa-folder-open",
		Columns: []api2go.ColumnInfo{
			{
				ColumnName:        "name",
				Name:              "name",
				ColumnType:        "label",
				DataType:          "varchar(500)",
				IsNullable:        false,
				IsIndexed:         true,
				ColumnDescription: "The name of the collection, which serves as the primary identifier for users. This field is indexed to enable quick lookups and filtering by collection name.",
			},
			{
				ColumnName:        "description",
				Name:              "description",
				ColumnType:        "label",
				DataType:          "text",
				IsNullable:        false,
				IsIndexed:         true,
				ColumnDescription: "A detailed description of the collection's purpose, contents, and other relevant information. Indexed to support searching collections by their descriptions.",
			},
		},
	},
	{
		TableName:     "credential",
		IsHidden:      false,
		DefaultGroups: adminsGroup,
		Icon:          "fa-key",
		Columns: []api2go.ColumnInfo{
			{
				ColumnName:        "name",
				Name:              "name",
				ColumnType:        "label",
				DataType:          "varchar(500)",
				IsNullable:        false,
				IsIndexed:         true,
				ColumnDescription: "A human-readable identifier for the credential entry, allowing users to recognize the purpose or system associated with these credentials. Indexed for quick search and reference.",
			},
			{
				ColumnName:        "content",
				Name:              "content",
				ColumnType:        "encrypted",
				DataType:          "text",
				IsNullable:        false,
				IsIndexed:         false,
				ColumnDescription: "The sensitive credential information stored in encrypted format. May contain passwords, API keys, tokens, or other authentication data that requires security protection.",
			},
		},
	},
	{
		TableName:     "certificate",
		IsHidden:      false,
		DefaultGroups: adminsGroup,
		Icon:          "fa-certificate",
		Columns: []api2go.ColumnInfo{
			{
				Name:              "hostname",
				ColumnName:        "hostname",
				IsUnique:          true,
				IsIndexed:         true,
				ColumnType:        "label",
				DataType:          "varchar(100)",
				IsNullable:        false,
				ColumnDescription: "The fully qualified domain name (FQDN) for which this certificate is valid. This field is unique and indexed as it serves as the primary identifier for certificate lookup.",
			},
			{
				Name:              "issuer",
				ColumnName:        "issuer",
				ColumnType:        "label",
				DataType:          "varchar(100)",
				IsNullable:        false,
				DefaultValue:      "'self'",
				ColumnDescription: "The authority that issued the certificate, such as a Certificate Authority (CA) name or 'self' for self-signed certificates. Defaults to 'self' when not specified.",
			},
			{
				Name:              "generated_at",
				ColumnName:        "generated_at",
				ColumnType:        "datetime",
				DataType:          "timestamp",
				IsNullable:        true,
				ColumnDescription: "The timestamp when the certificate was generated. This helps track certificate age and can be used to determine when renewal is necessary.",
			},
			{
				Name:              "certificate_pem",
				ColumnName:        "certificate_pem",
				ColumnType:        "content",
				DataType:          "text",
				IsNullable:        true,
				ColumnDescription: "The X.509 certificate in PEM format, containing the public key and certificate information. This is the primary certificate data used by services.",
			},
			{
				Name:              "root_certificate",
				ColumnName:        "root_certificate",
				ColumnType:        "content",
				DataType:          "text",
				IsNullable:        true,
				ColumnDescription: "The root certificate in PEM format, used to establish the chain of trust for this certificate. May be null for self-signed certificates or when not available.",
			},
			{
				Name:              "private_key_pem",
				ColumnName:        "private_key_pem",
				ColumnType:        "encrypted",
				DataType:          "text",
				IsNullable:        true,
				ColumnDescription: "The private key corresponding to the certificate in PEM format, stored with encryption for security. This is used for certificate authentication and signing operations.",
			},
			{
				Name:              "public_key_pem",
				ColumnName:        "public_key_pem",
				ColumnType:        "content",
				DataType:          "text",
				IsNullable:        true,
				ColumnDescription: "The public key extracted from the certificate in PEM format. This is used for encryption and verification operations.",
			},
		},
	},
	{
		TableName:     "feed",
		IsHidden:      false,
		DefaultGroups: adminsGroup,
		Icon:          "fa-rss",
		Columns: []api2go.ColumnInfo{
			{
				Name:              "feed_name",
				ColumnName:        "feed_name",
				IsUnique:          true,
				IsIndexed:         true,
				ColumnType:        "label",
				DataType:          "varchar(100)",
				IsNullable:        false,
				ColumnDescription: "A unique identifier for the feed that serves as the primary reference key. This name is used in URLs and API calls to identify the specific feed resource.",
			},
			{
				Name:              "title",
				ColumnName:        "title",
				ColumnType:        "label",
				DataType:          "varchar(500)",
				IsNullable:        false,
				DefaultValue:      "''",
				ColumnDescription: "The display title of the feed shown to users and subscribers. This is the primary human-readable identifier in feed readers and syndication services.",
			},
			{
				Name:              "description",
				ColumnName:        "description",
				ColumnType:        "label",
				DataType:          "text",
				IsNullable:        false,
				ColumnDescription: "A detailed description of the feed's content and purpose. This text appears in feed readers to help users understand what the feed contains.",
			},
			{
				Name:              "link",
				ColumnName:        "link",
				ColumnType:        "label",
				DataType:          "varchar(1000)",
				IsNullable:        false,
				DefaultValue:      "''",
				ColumnDescription: "The URL to the website or resource associated with this feed. This allows users to visit the original source of the content.",
			},
			{
				Name:              "author_name",
				ColumnName:        "author_name",
				ColumnType:        "label",
				DataType:          "varchar(500)",
				IsNullable:        false,
				DefaultValue:      "''",
				ColumnDescription: "The name of the feed author or publisher, displayed in feed readers to identify the content creator or provider.",
			},
			{
				Name:              "author_email",
				ColumnName:        "author_email",
				ColumnType:        "label",
				DataType:          "varchar(500)",
				IsNullable:        false,
				DefaultValue:      "''",
				ColumnDescription: "The contact email address of the feed author or administrator, used for communication regarding the feed content.",
			},
			{
				Name:              "enable",
				ColumnName:        "enable",
				ColumnType:        "truefalse",
				DataType:          "bool",
				IsNullable:        false,
				DefaultValue:      "false",
				ColumnDescription: "Controls whether the feed is active and publicly accessible. When set to false, the feed is disabled and cannot be accessed by subscribers.",
			},
			{
				Name:              "enable_atom",
				ColumnName:        "enable_atom",
				ColumnType:        "truefalse",
				DataType:          "bool",
				IsNullable:        false,
				DefaultValue:      "true",
				ColumnDescription: "Determines whether the feed is available in Atom format. When enabled, the system generates and serves an Atom version of this feed.",
			},
			{
				Name:              "enable_json",
				ColumnName:        "enable_json",
				ColumnType:        "truefalse",
				DataType:          "bool",
				IsNullable:        false,
				DefaultValue:      "true",
				ColumnDescription: "Controls whether the feed is available in JSON Feed format. When enabled, the system generates and serves a JSON version of this feed.",
			},
			{
				Name:              "enable_rss",
				ColumnName:        "enable_rss",
				ColumnType:        "truefalse",
				DataType:          "bool",
				IsNullable:        false,
				DefaultValue:      "true",
				ColumnDescription: "Specifies whether the feed is available in RSS format. When enabled, the system generates and serves an RSS version of this feed.",
			},
			{
				Name:              "page_size",
				ColumnName:        "page_size",
				ColumnType:        "measurement",
				DataType:          "int(11)",
				IsNullable:        false,
				DefaultValue:      "1000",
				ColumnDescription: "The maximum number of items to include in a single feed response. This controls pagination and feed size to optimize loading times and bandwidth usage.",
			},
		},
	},
	{
		TableName:     "integration",
		IsHidden:      false,
		DefaultGroups: adminsGroup,
		Icon:          "fa-exchange-alt",
		Columns: []api2go.ColumnInfo{
			{
				Name:              "name",
				ColumnName:        "name",
				IsUnique:          true,
				IsIndexed:         true,
				ColumnType:        "label",
				DataType:          "varchar(100)",
				IsNullable:        false,
				ColumnDescription: "A unique identifier for the integration that serves as the primary reference key. Used in logs, API calls, and internal references to identify this specific integration.",
			},
			{
				Name:              "specification_language",
				ColumnName:        "specification_language",
				ColumnType:        "label",
				DataType:          "varchar(20)",
				IsNullable:        false,
				ColumnDescription: "The language or format used to define the integration specification, such as 'OpenAPI', 'GraphQL', 'WSDL', or custom formats. This determines how the specification is interpreted.",
			},
			{
				Name:              "specification_format",
				ColumnName:        "specification_format",
				ColumnType:        "label",
				DataType:          "varchar(10)",
				DefaultValue:      "'json'",
				ColumnDescription: "The file format of the specification document, typically 'json', 'yaml', or 'xml'. Defaults to 'json' when not explicitly specified.",
			},
			{
				Name:              "specification",
				ColumnName:        "specification",
				ColumnType:        "content",
				DataType:          "mediumtext",
				IsNullable:        false,
				ColumnDescription: "The actual integration specification document that defines the API endpoints, operations, parameters, and other technical details of the integration.",
			},
			{
				Name:              "authentication_type",
				ColumnName:        "authentication_type",
				ColumnType:        "label",
				ColumnDescription: "The authentication method used for this integration, such as 'API Key', 'OAuth2', 'Basic Auth', or 'JWT'. This determines how authentication credentials are processed.",
			},
			{
				Name:              "authentication_specification",
				ColumnName:        "authentication_specification",
				ColumnType:        "encrypted",
				DataType:          "text",
				IsNullable:        false,
				ColumnDescription: "Encrypted authentication details required to access the integrated service, such as API keys, tokens, or credentials. Stored securely with encryption.",
			},
			{
				Name:              "enable",
				ColumnName:        "enable",
				ColumnType:        "truefalse",
				DataType:          "bool",
				DefaultValue:      "true",
				IsNullable:        false,
				ColumnDescription: "Controls whether the integration is active and available for use. When set to false, the integration is disabled and will not be executed regardless of triggers.",
			},
		},
	},
	{
		TableName:     "task",
		IsHidden:      false,
		DefaultGroups: adminsGroup,
		Icon:          "fa-tasks",
		Columns: []api2go.ColumnInfo{
			{
				Name:              "name",
				ColumnName:        "name",
				DataType:          "varchar(100)",
				ColumnType:        "label",
				IsIndexed:         true,
				ColumnDescription: "A descriptive name for the task that serves as the primary user-facing identifier. This field is indexed to support efficient searching and sorting of tasks.",
			},
			{
				Name:              "action_name",
				ColumnName:        "action_name",
				DataType:          "varchar(100)",
				ColumnType:        "label",
				ColumnDescription: "The name of the action to be executed when this task runs. References an action defined in the system that contains the implementation logic.",
			},
			{
				Name:              "entity_name",
				ColumnName:        "entity_name",
				DataType:          "varchar(100)",
				ColumnType:        "label",
				ColumnDescription: "The name of the data entity or resource type that this task operates on. This determines the context and scope of the task's execution.",
			},
			{
				Name:              "schedule",
				ColumnName:        "schedule",
				DataType:          "varchar(100)",
				ColumnType:        "label",
				ColumnDescription: "The execution schedule for the task, typically defined as a cron expression or time interval pattern. Controls when and how frequently the task runs.",
			},
			{
				Name:              "active",
				ColumnName:        "active",
				DataType:          "bool",
				ColumnType:        "truefalse",
				ColumnDescription: "Indicates whether the task is currently active and should be executed according to its schedule. Inactive tasks are not run regardless of their schedule.",
			},
			{
				Name:              "attributes",
				ColumnName:        "attributes",
				DataType:          "text",
				ColumnType:        "json",
				ColumnDescription: "A JSON object containing additional parameters, settings, and configuration options specific to this task. These attributes are passed to the action when executed.",
			},
			{
				Name:              "job_type",
				ColumnName:        "job_type",
				DataType:          "varchar(100)",
				ColumnType:        "label",
				ColumnDescription: "Categorizes the task by type of job (e.g., 'backup', 'sync', 'report', 'maintenance'), allowing for filtering and grouping related tasks.",
			},
		},
	},
	{
		TableName:     "template",
		IsHidden:      false,
		DefaultGroups: adminsGroup,
		Icon:          "fa-file-alt",
		Columns: []api2go.ColumnInfo{
			{
				ColumnName:        "name",
				Name:              "name",
				ColumnType:        "label",
				DataType:          "varchar(500)",
				IsNullable:        false,
				IsUnique:          true,
				IsIndexed:         true,
				ColumnDescription: "A unique identifier for the template that serves as the primary reference key. Used in code, URLs, and API calls to identify specific templates.",
			},
			{
				ColumnName:        "content",
				Name:              "content",
				ColumnType:        "content",
				DataType:          "text",
				IsNullable:        false,
				IsIndexed:         false,
				ColumnDescription: "The actual template content, which may include HTML, text with placeholders, or template language syntax that will be processed when the template is rendered.",
			},
			{
				ColumnName:        "action_config",
				Name:              "action_config",
				ColumnType:        "json",
				DataType:          "text",
				IsNullable:        true,
				DefaultValue:      "'{}'",
				IsIndexed:         false,
				ColumnDescription: "JSON configuration for actions that can be performed on or with this template, such as pre-processing, post-processing, or conditional rendering logic.",
			},
			{
				ColumnName:        "cache_config",
				Name:              "cache_config",
				ColumnType:        "json",
				DataType:          "text",
				IsNullable:        true,
				DefaultValue:      "'{}'",
				IsIndexed:         false,
				ColumnDescription: "JSON configuration for caching behavior, including cache duration, invalidation rules, and cache storage options to optimize performance.",
			},
			{
				ColumnName:        "mime_type",
				Name:              "mime_type",
				ColumnType:        "label",
				DataType:          "varchar(500)",
				IsNullable:        false,
				IsIndexed:         false,
				ColumnDescription: "The MIME type of the content this template generates, such as 'text/html', 'application/json', or 'text/plain'. Used for HTTP response headers.",
			},
			{
				ColumnName:        "headers",
				Name:              "headers",
				ColumnType:        "json",
				DataType:          "text",
				IsNullable:        true,
				IsIndexed:         false,
				ColumnDescription: "JSON object containing additional HTTP headers to be included when serving content generated from this template. Used for custom headers beyond content type.",
			},
			{
				ColumnName:        "url_pattern",
				Name:              "url_pattern",
				ColumnType:        "json",
				DataType:          "text",
				IsNullable:        false,
				IsIndexed:         false,
				ColumnDescription: "JSON definition of URL patterns that map to this template, enabling dynamic routing and parameter extraction from request URLs.",
			},
		},
	},
	//{
	//	TableName:     "marketplace",
	//	IsHidden: false,
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
		IsHidden:      false,
		Columns: []api2go.ColumnInfo{
			{
				Name:              "schema_name",
				ColumnName:        "schema_name",
				ColumnType:        "label",
				DataType:          "varchar(100)",
				IsNullable:        false,
				ColumnDescription: "A unique identifier for the JSON schema stored in the system. This name is used to reference and retrieve specific schemas for validation and documentation purposes.",
			},
			{
				Name:              "json_schema",
				ColumnType:        "json",
				DataType:          "text",
				ColumnName:        "json_schema",
				ColumnDescription: "The actual JSON Schema document stored as a JSON text field. Contains schema definitions including data types, validation rules, property constraints, and other schema specifications.",
			},
		},
	},
	{
		TableName:     "timeline",
		Icon:          "fa-history",
		DefaultGroups: adminsGroup,
		IsHidden:      false,
		Columns: []api2go.ColumnInfo{
			{
				Name:              "event_type",
				ColumnName:        "event_type",
				ColumnType:        "label",
				DataType:          "varchar(50)",
				IsNullable:        false,
				ColumnDescription: "Categorizes the timeline event by type (e.g., 'create', 'update', 'delete', 'login'), allowing for filtering and grouping related events in chronological sequences.",
			},
			{
				Name:              "title",
				ColumnName:        "title",
				ColumnType:        "label",
				IsIndexed:         true,
				DataType:          "varchar(50)",
				IsNullable:        false,
				ColumnDescription: "A concise summary of the event displayed to users when viewing the timeline. This field is indexed to enable efficient timeline searches and filtering by event title.",
			},
			{
				Name:              "payload",
				ColumnName:        "payload",
				ColumnType:        "content",
				DataType:          "text",
				IsNullable:        true,
				ColumnDescription: "Additional data or context related to the event, stored as text content. May contain JSON, XML, or plaintext details that provide comprehensive information about what occurred.",
			},
		},
	},
	{
		TableName:     "world",
		IsHidden:      false,
		DefaultGroups: adminsGroup,
		DefaultOrder:  "+table_name",
		Icon:          "fa-globe",
		Columns: []api2go.ColumnInfo{
			{
				Name:              "table_name",
				ColumnName:        "table_name",
				IsNullable:        false,
				IsUnique:          true,
				IsIndexed:         true,
				DataType:          "varchar(200)",
				ColumnType:        "name",
				ColumnDescription: "The name of the database table this world entity represents. This is a unique, indexed identifier used throughout the system to reference specific data models.",
			},
			{
				Name:              "world_schema_json",
				ColumnName:        "world_schema_json",
				DataType:          "text",
				IsNullable:        false,
				ColumnType:        "json",
				ColumnDescription: "A JSON representation of the complete schema for this world entity, including all columns, relationships, validations, and other metadata needed to define the data model.",
			},
			{
				Name:              "default_permission",
				ColumnName:        "default_permission",
				DataType:          "int(4)",
				IsNullable:        false,
				DefaultValue:      "644",
				ColumnType:        "value",
				ColumnDescription: "An integer representing the default Unix-style permission setting (e.g., 644) that controls the base access rights for records in this table for various user roles.",
			},
			{
				Name:              "is_top_level",
				ColumnName:        "is_top_level",
				DataType:          "bool",
				IsNullable:        false,
				DefaultValue:      "true",
				ColumnType:        "truefalse",
				ColumnDescription: "Indicates whether this entity appears at the top level of the data hierarchy in the system. Top-level entities are directly accessible via API endpoints and user interfaces.",
			},
			{
				Name:              "is_hidden",
				ColumnName:        "is_hidden",
				DataType:          "bool",
				IsNullable:        false,
				DefaultValue:      "false",
				ColumnType:        "truefalse",
				ColumnDescription: "Controls the visibility of this entity in user interfaces and API documentation. When set to true, the entity exists but is not displayed in standard listings.",
			},
			{
				Name:              "is_join_table",
				ColumnName:        "is_join_table",
				DataType:          "bool",
				IsNullable:        false,
				DefaultValue:      "false",
				ColumnType:        "truefalse",
				ColumnDescription: "Specifies whether this table serves as a many-to-many join table between other entities. Join tables typically contain primarily foreign key references to related entities.",
			},
			{
				Name:              "is_state_tracking_enabled",
				ColumnName:        "is_state_tracking_enabled",
				DataType:          "bool",
				IsNullable:        false,
				DefaultValue:      "false",
				ColumnType:        "truefalse",
				ColumnDescription: "Determines whether state transitions for records in this table are tracked and logged. When enabled, the system maintains a history of status changes over time.",
			},
			{
				Name:              "default_order",
				ColumnName:        "default_order",
				DataType:          "varchar(100)",
				IsNullable:        true,
				DefaultValue:      "'+id'",
				ColumnType:        "value",
				ColumnDescription: "A string defining the default sorting order for records in this table. Uses a format like '+column' for ascending or '-column' for descending sort, defaulting to '+id'.",
			},
			{
				Name:              "icon",
				ColumnName:        "icon",
				DataType:          "varchar(20)",
				IsNullable:        true,
				DefaultValue:      "'fa-star'",
				ColumnType:        "label",
				ColumnDescription: "The Font Awesome icon identifier used to represent this entity in the user interface. Provides visual identification of entity types in lists and navigation elements.",
			},
		},
	},
	{
		TableName:     "stream",
		Icon:          "fa-stream",
		DefaultGroups: adminsGroup,
		IsHidden:      false,
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
				DataType:     "bool",
				IsNullable:   false,
				ColumnType:   "truefalse",
				DefaultValue: "true",
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
		IsHidden:      false,
		DefaultGroups: []string{},
		Columns: []api2go.ColumnInfo{
			{
				ColumnName:        "mobile_number",
				IsIndexed:         true,
				IsNullable:        true,
				DataType:          "varchar(20)",
				ColumnType:        "label",
				ColumnDescription: "The user's mobile phone number used for receiving OTP (One-Time Password) messages. This field is indexed to enable quick lookups for verification and authentication processes.",
			},
			{
				ColumnName:        "otp_secret",
				IsIndexed:         true,
				ExcludeFromApi:    true,
				DataType:          "varchar(100)",
				ColumnType:        "encrypted",
				ColumnDescription: "The encrypted secret key used to generate OTP (One-Time Password) codes for this account. This sensitive field is indexed for authentication lookups but excluded from API responses.",
			},
			{
				ColumnName:        "verified",
				DataType:          "bool",
				DefaultValue:      "false",
				ColumnType:        "truefalse",
				Name:              "verified",
				ColumnDescription: "Indicates whether the user's OTP account has been successfully verified through a validation process. Defaults to false until verification is completed.",
			},
		},
	},
	{
		TableName:     USER_ACCOUNT_TABLE_NAME,
		Icon:          "fa-user",
		DefaultGroups: []string{"users"},
		Columns: []api2go.ColumnInfo{
			{
				Name:              "name",
				ColumnName:        "name",
				IsIndexed:         true,
				DataType:          "varchar(80)",
				ColumnType:        "label",
				ColumnDescription: "The display name of the user, used for identification in the UI and communications. This field is indexed to support quick user lookups and searching by name.",
			},
			{
				Name:              "email",
				ColumnName:        "email",
				DataType:          "varchar(80)",
				IsIndexed:         true,
				IsUnique:          true,
				ColumnType:        "email",
				ColumnDescription: "The primary email address of the user, serving as a unique identifier for authentication and communication. This field is indexed and must be unique across all users.",
			},
			{
				Name:              "password",
				ColumnName:        "password",
				DataType:          "varchar(100)",
				ColumnType:        "password",
				IsNullable:        true,
				ColumnDescription: "The user's password stored in a secure hashed format. This field is nullable to support alternative authentication methods like OAuth or OTP.",
			},
			{
				Name:              "confirmed",
				ColumnName:        "confirmed",
				DataType:          "bool",
				ColumnType:        "truefalse",
				IsNullable:        false,
				DefaultValue:      "false",
				ColumnDescription: "Indicates whether the user has confirmed their account, typically through email verification. Defaults to false until the confirmation process is completed.",
			},
		},
		Validations: []columns.ColumnTag{
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
		Conformations: []columns.ColumnTag{
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
				Name:              "name",
				ColumnName:        "name",
				IsIndexed:         true,
				IsUnique:          true,
				ColumnDescription: "A unique identifier for the user group that serves as the primary reference key. This indexed field ensures groups have distinct names for clear identification in permission assignments and user management.", DataType: "varchar(80)",
				ColumnType: "label",
			},
		},
	},
	{
		TableName:     "action",
		DefaultGroups: adminsGroup,
		IsHidden:      false,
		Icon:          "fa-bolt",
		CompositeKeys: [][]string{
			{"action_name", "world_id"},
		},
		Columns: []api2go.ColumnInfo{
			{
				Name:              "action_name",
				IsIndexed:         true,
				ColumnName:        "action_name",
				DataType:          "varchar(100)",
				ColumnType:        "label",
				ColumnDescription: "The internal identifier for the action that forms part of the composite primary key along with world_id. This indexed field enables quick lookups of actions by name.",
			},
			{
				Name:              "label",
				ColumnName:        "label",
				IsIndexed:         true,
				DataType:          "varchar(100)",
				ColumnType:        "label",
				ColumnDescription: "A human-readable display name for the action shown in user interfaces. This field is indexed to facilitate filtering and searching actions by their display names.",
			},
			{
				Name:              "instance_optional",
				ColumnName:        "instance_optional",
				IsIndexed:         false,
				DataType:          "bool",
				ColumnType:        "truefalse",
				DefaultValue:      "true",
				ColumnDescription: "Determines whether this action requires a specific instance to operate on. When true, the action can be executed at the entity level without selecting a particular record.",
			},
			{
				Name:              "action_schema",
				ColumnName:        "action_schema",
				DataType:          "text",
				ExcludeFromApi:    true,
				ColumnType:        "json",
				ColumnDescription: "The JSON schema defining the structure, input fields, validations, and outcome configurations for this action. Excluded from API responses due to its internal technical nature.",
			},
		},
	},
	{
		TableName:     "smd",
		IsHidden:      false,
		Icon:          "fa-project-diagram",
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
		Icon:          "fa-plug",
		IsHidden:      false,
		DefaultGroups: adminsGroup,
		Columns: []api2go.ColumnInfo{
			{
				Name:              "name",
				ColumnName:        "name",
				IsUnique:          true,
				IsIndexed:         true,
				DataType:          "varchar(80)",
				ColumnType:        "label",
				ColumnDescription: "A unique identifier for the OAuth connection that serves as the primary reference key. This indexed field allows for quick lookup of specific OAuth provider configurations.",
			},
			{
				Name:              "client_id",
				ColumnName:        "client_id",
				DataType:          "varchar(150)",
				ColumnType:        "label",
				ColumnDescription: "The OAuth client identifier issued by the authentication provider when registering the application. This is used to identify the application to the OAuth service.",
			},
			{
				Name:              "client_secret",
				ColumnName:        "client_secret",
				DataType:          "varchar(500)",
				ColumnType:        "encrypted",
				ColumnDescription: "The confidential OAuth client secret issued by the authentication provider, stored with encryption for security. Used with the client_id to authenticate the application.",
			},
			{
				Name:              "scope",
				ColumnName:        "scope",
				DataType:          "varchar(1000)",
				ColumnType:        "content",
				DefaultValue:      "'https://www.googleapis.com/auth/spreadsheets'",
				ColumnDescription: "The OAuth permission scopes requested for this connection, defining what resources and operations the application can access. Defaults to Google Sheets access scope.",
			},
			{
				Name:              "response_type",
				ColumnName:        "response_type",
				DataType:          "varchar(80)",
				ColumnType:        "label",
				DefaultValue:      "'code'",
				ColumnDescription: "The OAuth response type expected from the authorization server, typically 'code' for authorization code flow or 'token' for implicit flow. Defaults to 'code'.",
			},
			{
				Name:              "redirect_uri",
				ColumnName:        "redirect_uri",
				DataType:          "varchar(80)",
				ColumnType:        "url",
				DefaultValue:      "'/oauth/response'",
				ColumnDescription: "The URL to which the OAuth provider will redirect users after authentication, sending authorization codes or tokens. Defaults to '/oauth/response'.",
			},
			{
				Name:              "auth_url",
				ColumnName:        "auth_url",
				DataType:          "varchar(200)",
				DefaultValue:      "'https://accounts.google.com/o/oauth2/auth'",
				ColumnType:        "url",
				ColumnDescription: "The OAuth provider's authorization endpoint URL where users are redirected to authenticate. Defaults to Google's OAuth authorization endpoint.",
			},
			{
				Name:              "token_url",
				ColumnName:        "token_url",
				DataType:          "varchar(200)",
				DefaultValue:      "'https://accounts.google.com/o/oauth2/token'",
				ColumnType:        "url",
				ColumnDescription: "The OAuth provider's token endpoint URL used to exchange authorization codes for access tokens. Defaults to Google's OAuth token endpoint.",
			},
			{
				Name:              "profile_url",
				ColumnName:        "profile_url",
				DataType:          "varchar(200)",
				DefaultValue:      "'https://www.googleapis.com/oauth2/v1/userinfo?alt=json'",
				ColumnType:        "url",
				ColumnDescription: "The URL to fetch user profile information after authentication, used for user creation or profile updates. Defaults to Google's user info endpoint.",
			},
			{
				Name:              "profile_email_path",
				ColumnName:        "profile_email_path",
				DataType:          "varchar(200)",
				DefaultValue:      "'email'",
				ColumnType:        "label",
				ColumnDescription: "The JSON path to extract the user's email from the profile response. Specifies where to find the email address in the provider's user profile data structure.",
			},
			{
				Name:              "allow_login",
				ColumnName:        "allow_login",
				DataType:          "bool",
				DefaultValue:      "false",
				ColumnType:        "truefalse",
				ColumnDescription: "Controls whether this OAuth connection can be used for user authentication and login. When enabled, users can sign in using this OAuth provider.",
			},
			{
				Name:              "access_type_offline",
				ColumnName:        "access_type_offline",
				DataType:          "bool",
				DefaultValue:      "false",
				ColumnType:        "truefalse",
				ColumnDescription: "Determines whether to request refresh tokens for offline access to resources. When enabled, the application can access resources even when the user is not present.",
			},
		},
	},
	{
		TableName:     "data_exchange",
		IsHidden:      false,
		Icon:          "fa-sync",
		DefaultGroups: adminsGroup,
		Columns: []api2go.ColumnInfo{
			{
				Name:              "name",
				ColumnName:        "name",
				ColumnType:        "label",
				IsUnique:          true,
				DataType:          "varchar(200)",
				IsIndexed:         true,
				ColumnDescription: "A unique identifier for the data exchange configuration that serves as the primary reference key. This indexed field enables quick lookups of specific data exchange processes.",
			},
			{
				Name:              "source_attributes",
				ColumnName:        "source_attributes",
				ColumnType:        "json",
				DataType:          "text",
				ColumnDescription: "JSON configuration specifying details about the data source, including connection parameters, authentication credentials, and source-specific settings required for data extraction.",
			},
			{
				Name:              "source_type",
				ColumnName:        "source_type",
				ColumnType:        "label",
				DataType:          "varchar(100)",
				ColumnDescription: "The type of data source system (e.g., 'database', 'api', 'file', 'self') that identifies where data will be extracted from and determines how source_attributes are interpreted.",
			},
			{
				Name:              "target_attributes",
				ColumnName:        "target_attributes",
				ColumnType:        "json",
				DataType:          "text",
				ColumnDescription: "JSON configuration defining the destination for the exchanged data, including connection parameters, authentication details, and target-specific settings for data loading.",
			},
			{
				Name:              "attributes",
				ColumnName:        "attributes",
				ColumnType:        "json",
				DataType:          "text",
				ColumnDescription: "JSON mapping of source columns to target columns, defining how data fields are transformed, renamed, or modified during the exchange process between systems.",
			},
			{
				Name:              "target_type",
				ColumnName:        "target_type",
				ColumnType:        "label",
				DataType:          "varchar(100)",
				ColumnDescription: "The type of destination system (e.g., 'database', 'api', 'gsheet-append') that identifies where data will be loaded to and determines how target_attributes are interpreted.",
			},
			{
				Name:              "options",
				ColumnName:        "options",
				ColumnType:        "json",
				DataType:          "text",
				ColumnDescription: "JSON configuration containing additional exchange options like error handling, validation rules, scheduling parameters, and behavior controls for the data transfer process.",
			},
		},
	},
	{
		TableName:     "oauth_token",
		IsHidden:      false,
		Icon:          "fa-shield-alt",
		DefaultGroups: adminsGroup,
		Columns: []api2go.ColumnInfo{
			{
				Name:              "access_token",
				ColumnName:        "access_token",
				ColumnType:        "encrypted",
				DataType:          "varchar(1000)",
				ColumnDescription: "The encrypted OAuth access token used to authenticate API requests to the provider. This token is stored securely and used for authorized access to protected resources.",
			},
			{
				Name:              "expires_in",
				ColumnName:        "expires_in",
				ColumnType:        "measurement",
				DataType:          "int(11)",
				ColumnDescription: "The lifetime of the access token in seconds, indicating when the token will expire and require renewal. Used to determine when refresh operations should be triggered.",
			},
			{
				Name:              "refresh_token",
				ColumnName:        "refresh_token",
				ColumnType:        "encrypted",
				DataType:          "varchar(1000)",
				ColumnDescription: "The encrypted OAuth refresh token used to obtain new access tokens when they expire. This long-lived token enables continuous access without user re-authentication.",
			},
			{
				Name:              "token_type",
				ColumnName:        "token_type",
				ColumnType:        "label",
				DataType:          "varchar(20)",
				ColumnDescription: "The type of access token issued by the OAuth provider, typically 'Bearer'. This determines how the token should be included in subsequent API authorization headers.",
			},
		},
	},
	{
		TableName:     "cloud_store",
		Icon:          "fa-cloud",
		DefaultGroups: adminsGroup,
		IsHidden:      false,
		Columns: []api2go.ColumnInfo{
			{
				Name:              "Name",
				ColumnName:        "name",
				ColumnType:        "label",
				DataType:          "varchar(100)",
				IsUnique:          true,
				ColumnDescription: "A unique identifier for the cloud storage connection that serves as the primary reference key. Used in file operations, site configurations, and throughout the system.",
			},
			{
				Name:              "store_type",
				ColumnName:        "store_type",
				ColumnType:        "label",
				DataType:          "varchar(100)",
				ColumnDescription: "Categorizes the storage service by type (e.g., 'local', 'cloud'), defining the general classification of storage architecture used by this connection.",
			},
			{
				Name:              "store_provider",
				ColumnName:        "store_provider",
				ColumnType:        "label",
				DataType:          "varchar(100)",
				ColumnDescription: "Identifies the specific cloud provider or service (e.g., 'aws', 'gcs', 'Azure Blob Storage', 'local') used for this storage connection.",
			},
			{
				Name:              "root_path",
				ColumnName:        "root_path",
				ColumnType:        "label",
				DataType:          "varchar(1000)",
				ColumnDescription: "The base directory path within the storage service where operations will be performed. Acts as the starting point for all relative paths used with this connection.",
			},
			{
				Name:              "credential_name",
				ColumnName:        "credential_name",
				ColumnType:        "label",
				IsNullable:        true,
				DataType:          "varchar(1000)",
				ColumnDescription: "References the name of a credential record containing authentication details for this storage service. Can be null for services that don't require authentication.",
			},
			{
				Name:              "store_parameters",
				ColumnName:        "store_parameters",
				ColumnType:        "json",
				DataType:          "text",
				ColumnDescription: "JSON configuration containing additional connection parameters specific to the storage provider, such as region, endpoint URLs, timeout settings, and retry policies.",
			},
		},
	},
	{
		TableName:     "site",
		DefaultGroups: adminsGroup,
		Icon:          "fa-sitemap",
		IsHidden:      false,
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
				Name:              "path",
				ColumnName:        "path",
				ColumnType:        "label",
				DataType:          "varchar(100)",
				ColumnDescription: "path on the cloud store to host as base directory",
			},
			{
				Name:         "enable",
				ColumnName:   "enable",
				ColumnType:   "truefalse",
				DataType:     "bool",
				DefaultValue: "true",
			},
			{
				Name:         "ftp_enabled",
				ColumnName:   "ftp_enabled",
				ColumnType:   "truefalse",
				DataType:     "bool",
				DefaultValue: "false",
			},
			{
				Name:         "site_type",
				ColumnName:   "site_type",
				ColumnType:   "label",
				DataType:     "varchar(20)",
				DefaultValue: "'static'",
			},
		},
	},
	{
		TableName:     "mail_server",
		IsHidden:      false,
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
				DataType:     "bool",
				ColumnType:   "truefalse",
				DefaultValue: "false",
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
				DataType:     "bool",
				ColumnType:   "truefalse",
				DefaultValue: "false",
			},
			{
				Name:         "always_on_tls",
				ColumnName:   "always_on_tls",
				DataType:     "bool",
				ColumnType:   "truefalse",
				DefaultValue: "true",
			},
			{
				Name:         "authentication_required",
				ColumnName:   "authentication_required",
				DataType:     "bool",
				ColumnType:   "truefalse",
				DefaultValue: "true",
			},
		},
	},
	{
		TableName:     "mail_account",
		IsHidden:      false,
		DefaultGroups: adminsGroup,
		Icon:          "fa-at",
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
		IsHidden:      false,
		Icon:          "fa-inbox",
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
		IsHidden:      false,
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
		IsHidden:      false,
		Icon:          "fa-paper-plane",
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
				DataType:     "bool",
				DefaultValue: "false",
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
