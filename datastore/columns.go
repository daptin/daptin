package datastore

import "github.com/artpar/api2go"

var StandardColumns = []api2go.ColumnInfo{
  api2go.ColumnInfo{
    Name: "id",
    ColumnName: "id",
    DataType: "int(11)",
    IsPrimaryKey: true,
    IsAutoIncrement: true,
  },
  api2go.ColumnInfo{
    Name: "created_at",
    ColumnName: "created_at",
    DataType: "timestamp",
    DefaultValue: "current_timestamp",
  },
  api2go.ColumnInfo{
    Name: "updated_at",
    ColumnName: "updated_at",
    DataType: "timestamp",
    DefaultValue: "null",
    IsNullable: true,
  },
  api2go.ColumnInfo{
    Name: "deleted_at",
    ColumnName: "deleted_at",
    DataType: "timestamp",
    IsIndexed: true,
    IsNullable: true,
  },
  api2go.ColumnInfo{
    Name: "reference_id",
    ColumnName: "reference_id",
    DataType: "varchar(40)",
    IsIndexed: true,
  },
  api2go.ColumnInfo{
    Name: "permission",
    ColumnName: "permission",
    DataType: "int(11)",
    IsIndexed: false,
  },
  api2go.ColumnInfo{
    Name: "status",
    ColumnName: "status",
    DataType: "varchar(20)",
    DefaultValue: "'pending'",
    IsIndexed: true,
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
      },
      api2go.ColumnInfo{
        Name: "schema_json",
        ColumnName: "schema_json",
        DataType: "text",
        IsNullable: false,
      },
      api2go.ColumnInfo{
        Name: "default_permission",
        ColumnName: "default_permission",
        DataType: "int(4)",
        IsNullable: false,
        DefaultValue: "'755'",
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
      },
      api2go.ColumnInfo{
        Name: "email",
        ColumnName: "email",
        DataType: "varchar(80)",
        IsUnique: true,
        IsIndexed: true,
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
      },
    },
  },
}


type TableInfo struct {
  TableName         string
  DefaultPermission int
  Columns           []api2go.ColumnInfo
}

type TableRelation struct {
  Subject  string
  Object   string
  Relation string
}
