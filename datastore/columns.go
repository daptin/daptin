package datastore

import "github.com/artpar/api2go"

var StandardColumns = []api2go.ColumnInfo{
  api2go.ColumnInfo{
    Name: "id",
    ColumnName: "id",
    DataType: "int(11)",
    IsPrimaryKey: true,
    IsAutoIncrement: true,
    IncludeInApi: false,
    ColumnType: "id",
  },
  api2go.ColumnInfo{
    Name: "created_at",
    ColumnName: "created_at",
    DataType: "timestamp",
    DefaultValue: "current_timestamp",
    ColumnType: "datetime",
  },
  api2go.ColumnInfo{
    Name: "updated_at",
    ColumnName: "updated_at",
    DataType: "timestamp",
    DefaultValue: "null",
    IsNullable: true,
    ColumnType: "datetime",
  },
  api2go.ColumnInfo{
    Name: "deleted_at",
    ColumnName: "deleted_at",
    DataType: "timestamp",
    IncludeInApi: false,
    IsIndexed: true,
    IsNullable: true,
    ColumnType: "datetime",
  },
  api2go.ColumnInfo{
    Name: "reference_id",
    ColumnName: "reference_id",
    DataType: "varchar(40)",
    IsIndexed: true,
    ColumnType: "alias",
  },
  api2go.ColumnInfo{
    Name: "permission",
    ColumnName: "permission",
    IncludeInApi: false,
    DataType: "int(11)",
    IsIndexed: false,
    ColumnType: "value",

  },
  api2go.ColumnInfo{
    Name: "status",
    ColumnName: "status",
    DataType: "varchar(20)",
    DefaultValue: "'pending'",
    IsIndexed: true,
    ColumnType: "label",
  },
  api2go.ColumnInfo{
    Name: "user_id",
    ColumnName: "user_id",
    DataType: "int(11)",
    IsIndexed: false,
    IsForeignKey: true,
    IsNullable: true,
    ForeignKeyData: api2go.ForeignKeyData{
      TableName: "user",
      ColumnName: "id",
    },
    ColumnType: "alias",
  },
  api2go.ColumnInfo{
    Name: "usergroup_id",
    ColumnName: "usergroup_id",
    DataType: "int(11)",
    IsIndexed: false,
    IsForeignKey: true,
    IsNullable: true,
    ForeignKeyData: api2go.ForeignKeyData{
      TableName: "usergroup",
      ColumnName: "id",
    },
    ColumnType: "alias",
  },

}

var StandardRelations = []TableRelation{
  TableRelation{
    Subject: "world_column",
    Relation: "belongs_to",
    Object: "world",
  },
}

var StandardTables = []TableInfo{
  TableInfo{
    TableName: "world",
    Columns: []api2go.ColumnInfo{
      api2go.ColumnInfo{
        Name: "table_name",
        ColumnName: "table_name",
        IsNullable:false,
        IsUnique: true,
        DataType: "varchar(30)",
        ColumnType: "name",
      },
      api2go.ColumnInfo{
        Name: "schema_json",
        ColumnName: "schema_json",
        DataType: "text",
        IsNullable: false,
        ColumnType: "content",
      },
      api2go.ColumnInfo{
        Name: "default_permission",
        ColumnName: "default_permission",
        DataType: "int(4)",
        IsNullable: false,
        DefaultValue: "'755'",
        ColumnType: "value",
      },

    },
  },
  TableInfo{
    TableName: "world_column",
    Columns: []api2go.ColumnInfo{
      api2go.ColumnInfo{
        Name: "name",
        ColumnName: "name",
        DataType: "varchar(100)",
        IsNullable: false,
        ColumnType: "name",
      },
      api2go.ColumnInfo{
        Name: "column_name",
        ColumnName: "column_name",
        DataType: "varchar(100)",
        IsNullable: false,
        ColumnType: "name",
      },
      api2go.ColumnInfo{
        Name: "column_type",
        ColumnName: "column_type",
        DataType: "varchar(100)",
        IsNullable: false,
        ColumnType: "label",
      },
      api2go.ColumnInfo{
        Name: "is_primary_key",
        ColumnName: "is_primary_key",
        DataType: "bool",
        IsNullable: false,
        DefaultValue: "false",
        ColumnType: "truefalse",
      },
      api2go.ColumnInfo{
        Name: "is_auto_increment",
        ColumnName: "is_auto_increment",
        DataType: "bool",
        IsNullable: false,
        DefaultValue: "false",
        ColumnType: "truefalse",
      },
      api2go.ColumnInfo{
        Name: "is_indexed",
        ColumnName: "is_indexed",
        DataType: "bool",
        IsNullable: false,
        DefaultValue: "false",
        ColumnType: "truefalse",
      },
      api2go.ColumnInfo{
        Name: "is_unique",
        ColumnName: "is_unique",
        DataType: "bool",
        IsNullable: false,
        DefaultValue: "false",
        ColumnType: "truefalse",
      },
      api2go.ColumnInfo{
        Name: "is_nullable",
        ColumnName: "is_nullable",
        DataType: "bool",
        IsNullable: false,
        DefaultValue: "false",
        ColumnType: "truefalse",
      },
      api2go.ColumnInfo{
        Name: "is_foreign_key",
        ColumnName: "is_foreign_key",
        DataType: "bool",
        IsNullable: false,
        DefaultValue: "false",
        ColumnType: "truefalse",
      },
      api2go.ColumnInfo{
        Name: "include_in_api",
        ColumnName: "include_in_api",
        DataType: "bool",
        IsNullable: false,
        DefaultValue: "true",
        ColumnType: "truefalse",
      },
      api2go.ColumnInfo{
        Name: "foreign_key_data",
        ColumnName: "foreign_key_data",
        DataType: "varchar(100)",
        IsNullable: true,
        ColumnType: "content",
      },
      api2go.ColumnInfo{
        Name: "default_value",
        ColumnName: "default_value",
        DataType: "varchar(100)",
        IsNullable: true,
        ColumnType: "content",
      },
      api2go.ColumnInfo{
        Name: "data_type",
        ColumnName: "data_type",
        DataType: "varchar(50)",
        IsNullable: true,
        ColumnType: "label",
      },
    },
  },
  TableInfo{
    TableName: "user",
    Columns: []api2go.ColumnInfo{
      api2go.ColumnInfo{
        Name: "name",
        ColumnName: "name",
        DataType: "varchar(80)",
        ColumnType: "name",
      },
      api2go.ColumnInfo{
        Name: "email",
        ColumnName: "email",
        DataType: "varchar(80)",
        IsUnique: true,
        IsIndexed: true,
        ColumnType: "email",
      },
    },
  },
  TableInfo{
    TableName: "usergroup",
    Columns: []api2go.ColumnInfo{
      api2go.ColumnInfo{
        Name: "name",
        ColumnName: "name",
        DataType: "varchar(80)",
        ColumnType: "name",
      },
    },
  },
}

type TableInfo struct {
  TableName         string
  TableId           int
  DefaultPermission int
  Columns           []api2go.ColumnInfo
}

type TableRelation struct {
  Subject  string
  Object   string
  Relation string
}
