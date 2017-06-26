package resource

import (
  "github.com/artpar/api2go"
)

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
    Name:           "deleted_at",
    ColumnName:     "deleted_at",
    DataType:       "timestamp",
    ExcludeFromApi: true,
    IsIndexed:      true,
    IsNullable:     true,
    ColumnType:     "datetime",
  },
  {
    Name:       "reference_id",
    ColumnName: "reference_id",
    DataType:   "varchar(40)",
    IsIndexed:  true,
    ColumnType: "alias",
  },
  {
    Name:       "permission",
    ColumnName: "permission",
    DataType:   "int(11)",
    IsIndexed:  false,
    ColumnType: "value",
  },
  {
    Name:         "status",
    ColumnName:   "status",
    DataType:     "varchar(20)",
    DefaultValue: "'pending'",
    IsIndexed:    true,
    ColumnType:   "state",
  },
}

var StandardRelations = []api2go.TableRelation{
  api2go.NewTableRelation("world_column", "belongs_to", "world"),
  api2go.NewTableRelation("action", "belongs_to", "world"),
  api2go.NewTableRelation("world", "has_many", "smd"),
  api2go.NewTableRelation("oauth_token", "has_one", "oauth_connect"),
  api2go.NewTableRelation("data_exchange", "has_one", "oauth_token"),
}

var SystemSmds = []LoopbookFsmDescription{}
var SystemActions = []Action{
  {
    Name:   "upload_system_schema",
    Label:  "Upload features",
    OnType: "world",
    InFields: []api2go.ColumnInfo{
      {
        Name:       "Schema JSON file",
        ColumnName: "schema_json_file",
        ColumnType: "file.json",
        IsNullable: false,
      },
    },
    OutFields: []Outcome{
      {
        Type:   "system_json_schema_update",
        Method: "EXECUTE",
        Attributes: map[string]string{
          "json_schema": "$schema_json_file",
        },
      },
    },
  },
  {
    Name:     "download_system_schema",
    Label:    "Download system schema",
    OnType:   "world",
    InFields: []api2go.ColumnInfo{},
    OutFields: []Outcome{
      {
        Type:       "system_json_schema_download",
        Method:     "EXECUTE",
        Attributes: map[string]string{},
      },
    },
  },
  {
    Name:     "invoke_become_admin",
    Label:    "Become GoMS admin",
    OnType:   "world",
    InFields: []api2go.ColumnInfo{},
    OutFields: []Outcome{
      {
        Type:   "become_admin",
        Method: "EXECUTE",
        Attributes: map[string]string{
          "user_id": "$user.id",
        },
      },
    },
  },
  {
    Name:   "signup",
    Label:  "Sign up on Goms",
    OnType: "user",
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
    OutFields: []Outcome{
      {
        Type:      "user",
        Method:    "POST",
        Reference: "user",
        Attributes: map[string]string{
          "name":     "$name",
          "email":    "$email",
          "password": "$password",
        },
      },
      {
        Type:      "usergroup",
        Method:    "POST",
        Reference: "usergroup",
        Attributes: map[string]string{
          "name": "!'Home group for ' + user.name",
        },
      },
      {
        Type:      "user_user_id_has_usergroup_usergroup_id",
        Method:    "POST",
        Reference: "user_usergroup",
        Attributes: map[string]string{
          "user_id":      "$user.reference_id",
          "usergroup_id": "$usergroup.reference_id",
        },
      },
    },
  },
  {
    Name:   "signin",
    Label:  "Sign in to Goms",
    OnType: "user",
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
        Attributes: map[string]string{
          "email":    "$email",
          "password": "$password",
        },
      },
    },
  },
  {
    Name:   "oauth.login.begin",
    Label:  "Authenticate via OAuth",
    OnType: "oauth_connect",
    InFields: []api2go.ColumnInfo{
      {
        Name:         "scope",
        ColumnName:   "scope",
        ColumnType:   "name",
        IsNullable:   false,
        DefaultValue: "https://www.googleapis.com/auth/spreadsheets",
      },
    },
    OutFields: []Outcome{
      {
        Type:   "oauth.client.redirect",
        Method: "EXECUTE",
        Attributes: map[string]string{
          "authenticator": "$.name",
        },
      },
    },
  },
  {
    Name:   "oauth.login.response",
    Label:  "",
    OnType: "oauth_token",
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
        Type:   "oauth.login.response",
        Method: "EXECUTE",
        Attributes: map[string]string{
          "authenticator": "$authenticator",
        },
      },
    },
  },
  {
    Name:   "add.exchange",
    Label:  "Add new data exchange",
    OnType: "oauth_token",
    InFields: []api2go.ColumnInfo{
      {
        Name:       "name",
        ColumnName: "name",
        ColumnType: "name",
        IsNullable: false,
      },
    },
  },
}

var StandardTables = []TableInfo{
  {
    TableName: "world",
    IsHidden:  true,
    Columns: []api2go.ColumnInfo{
      {
        Name:       "table_name",
        ColumnName: "table_name",
        IsNullable: false,
        IsUnique:   true,
        DataType:   "varchar(200)",
        ColumnType: "name",
      },
      {
        Name:       "schema_json",
        ColumnName: "schema_json",
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
    },
  },
  {
    TableName: "world_column",
    IsHidden:  true,
    Columns: []api2go.ColumnInfo{
      {
        Name:       "name",
        ColumnName: "name",
        DataType:   "varchar(100)",
        IsIndexed:  true,
        IsNullable: false,
        ColumnType: "name",
      },
      {
        Name:       "column_name",
        ColumnName: "column_name",
        DataType:   "varchar(100)",
        IsIndexed:  true,
        IsNullable: false,
        ColumnType: "name",
      },
      {
        Name:       "column_type",
        ColumnName: "column_type",
        DataType:   "varchar(100)",
        IsNullable: false,
        ColumnType: "label",
      },
      {
        Name:         "is_primary_key",
        ColumnName:   "is_primary_key",
        DataType:     "bool",
        IsNullable:   false,
        DefaultValue: "false",
        ColumnType:   "truefalse",
      },
      {
        Name:         "is_auto_increment",
        ColumnName:   "is_auto_increment",
        DataType:     "bool",
        IsNullable:   false,
        DefaultValue: "false",
        ColumnType:   "truefalse",
      },
      {
        Name:         "is_indexed",
        ColumnName:   "is_indexed",
        DataType:     "bool",
        IsNullable:   false,
        DefaultValue: "false",
        ColumnType:   "truefalse",
      },
      {
        Name:         "is_unique",
        ColumnName:   "is_unique",
        DataType:     "bool",
        IsNullable:   false,
        DefaultValue: "false",
        ColumnType:   "truefalse",
      },
      {
        Name:         "is_nullable",
        ColumnName:   "is_nullable",
        DataType:     "bool",
        IsNullable:   false,
        DefaultValue: "false",
        ColumnType:   "truefalse",
      },
      {
        Name:         "is_foreign_key",
        ColumnName:   "is_foreign_key",
        DataType:     "bool",
        IsNullable:   false,
        DefaultValue: "false",
        ColumnType:   "truefalse",
      },
      {
        Name:         "include_in_api",
        ColumnName:   "include_in_api",
        DataType:     "bool",
        IsNullable:   false,
        DefaultValue: "true",
        ColumnType:   "truefalse",
      },
      {
        Name:       "foreign_key_data",
        ColumnName: "foreign_key_data",
        DataType:   "varchar(100)",
        IsNullable: true,
        ColumnType: "content",
      },
      {
        Name:       "default_value",
        ColumnName: "default_value",
        DataType:   "varchar(100)",
        IsNullable: true,
        ColumnType: "content",
      },
      {
        Name:       "data_type",
        ColumnName: "data_type",
        DataType:   "varchar(50)",
        IsNullable: true,
        ColumnType: "label",
      },
    },
  },
  {
    TableName: "user",
    Columns: []api2go.ColumnInfo{
      {
        Name:       "name",
        ColumnName: "name",
        IsIndexed:  true,
        DataType:   "varchar(80)",
        ColumnType: "name",
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
  },
  {
    TableName: "usergroup",
    Columns: []api2go.ColumnInfo{
      {
        Name:       "name",
        ColumnName: "name",
        IsIndexed:  true,
        DataType:   "varchar(80)",
        ColumnType: "name",
      },
    },
  },
  {
    TableName: "action",
    Columns: []api2go.ColumnInfo{
      {
        Name:       "action_name",
        IsIndexed:  true,
        ColumnName: "action_name",
        DataType:   "varchar(100)",
        ColumnType: "name",
      },
      {
        Name:       "label",
        ColumnName: "label",
        IsIndexed:  true,
        DataType:   "varchar(100)",
        ColumnType: "label",
      },
      {
        Name:       "in_fields",
        ColumnName: "in_fields",
        DataType:   "text",
        ColumnType: "json",
      },
      {
        Name:       "out_fields",
        ColumnName: "out_fields",
        DataType:   "text",
        ColumnType: "json",
      },
    },
  },
  {
    TableName: "smd",
    IsHidden:  true,
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
    TableName: "oauth_connect",
    IsHidden:  true,
    Columns: []api2go.ColumnInfo{
      {
        Name:       "name",
        ColumnName: "name",
        IsUnique:   true,
        IsIndexed:  true,
        DataType:   "varchar(80)",
        ColumnType: "name",
      },
      {
        Name:       "client_id",
        ColumnName: "client_id",
        DataType:   "varchar(80)",
        ColumnType: "name",
      },
      {
        Name:       "client_secret",
        ColumnName: "client_secret",
        DataType:   "varchar(80)",
        ColumnType: "encrypted",
      },
      {
        Name:         "response_type",
        ColumnName:   "response_type",
        DataType:     "varchar(80)",
        ColumnType:   "name",
        DefaultValue: "'code'",
      },
      {
        Name:       "redirect_uri",
        ColumnName: "redirect_uri",
        DataType:   "varchar(80)",
        ColumnType: "name",
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
    },
  },
  {
    TableName: "data_exchange",
    IsHidden:  true,
    Columns: []api2go.ColumnInfo{
      {
        Name:       "name",
        ColumnName: "name",
        ColumnType: "name",
        DataType:   "varchar(100)",
        IsIndexed:  true,
      },
    },
  },
  {
    TableName: "oauth_token",
    IsHidden:  true,
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
        ColumnType: "minutes",
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
}

type TableInfo struct {
  TableName              string `db:"table_name"`
  TableId                int
  DefaultPermission      int64 `db:"default_permission"`
  Columns                []api2go.ColumnInfo
  StateMachines          []LoopbookFsmDescription
  Relations              []api2go.TableRelation
  IsTopLevel             bool `db:"is_top_level"`
  Permission             int64
  UserId                 uint64 `db:"user_id"`
  IsHidden               bool   `db:"is_hidden"`
  IsJoinTable            bool   `db:"is_join_table"`
  IsStateTrackingEnabled bool   `db:"is_state_tracking_enabled"`
}
