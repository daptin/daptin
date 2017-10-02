"Relations" = {
  "Object" = "merchant"

  "Relation" = "belongs_to"

  "Subject" = "integration"
}

"Tables" = {
  "Columns" = {
    "ColumnDescription" = ""

    "ColumnName" = "name"

    "ColumnType" = "label"

    "DataType" = "varchar(500)"

    "IsAutoIncrement" = false

    "IsIndexed" = true

    "IsNullable" = false

    "IsPrimaryKey" = false

    "IsUnique" = false

    "Name" = "name"
  }

  "Columns" = {
    "ColumnDescription" = ""

    "ColumnName" = "image_url"

    "ColumnType" = "label"

    "DataType" = "varchar(500)"

    "IsAutoIncrement" = false

    "IsIndexed" = false

    "IsPrimaryKey" = false

    "IsUnique" = false

    "Name" = "image_url"
  }

  "TableName" = "merchant"
}

"Tables" = {
  "Columns" = {
    "ColumnDescription" = ""

    "ColumnName" = "description"

    "ColumnType" = "content"

    "DataType" = "text"

    "IsNullable" = true

    "Name" = "description"
  }

  "Columns" = {
    "ColumnName" = "language"

    "ColumnType" = "label"

    "DataType" = "varchar(100)"

    "IsIndexed" = true

    "Name" = "language"
  }

  "Columns" = {
    "ColumnName" = "api_sub_type"

    "ColumnType" = "label"

    "DataType" = "varchar(100)"

    "IsAutoIncrement" = false

    "IsIndexed" = true

    "IsNullable" = true

    "IsPrimaryKey" = false

    "IsUnique" = false

    "Name" = "api_sub_type"
  }

  "Columns" = {
    "ColumnDescription" = ""

    "ColumnName" = "name"

    "ColumnType" = "label"

    "DataType" = "varchar(100)"

    "IsAutoIncrement" = false

    "IsIndexed" = true

    "IsNullable" = false

    "IsPrimaryKey" = false

    "IsUnique" = false

    "Name" = "name"
  }

  "Columns" = {
    "ColumnDescription" = ""

    "ColumnName" = "stack"

    "ColumnType" = "label"

    "DataType" = "varchar(100)"

    "IsAutoIncrement" = false

    "IsIndexed" = true

    "IsNullable" = false

    "IsPrimaryKey" = false

    "IsUnique" = false

    "Name" = "stack"
  }

  "Columns" = {
    "ColumnDescription" = ""

    "ColumnName" = "api_type"

    "ColumnType" = "label"

    "DataType" = "varchar(100)"

    "IsAutoIncrement" = false

    "IsIndexed" = true

    "IsNullable" = true

    "IsPrimaryKey" = false

    "IsUnique" = false

    "Name" = "api_type"
  }

  "Columns" = {
    "ColumnDescription" = ""

    "ColumnName" = "change_set"

    "ColumnType" = "json"

    "DataType" = "text"

    "IsAutoIncrement" = false

    "IsIndexed" = true

    "IsNullable" = false

    "IsPrimaryKey" = false

    "IsUnique" = false

    "Name" = "change_set"
  }

  "Columns" = {
    "ColumnDescription" = ""

    "ColumnName" = "icon"

    "ColumnType" = "label"

    "DataType" = "varchar(100)"

    "DefaultValue" = "'devicon-devicon-plain'"

    "IsAutoIncrement" = false

    "IsIndexed" = false

    "IsNullable" = false

    "IsPrimaryKey" = false

    "IsUnique" = false

    "Name" = "icon"
  }

  "Columns" = {
    "ColumnDescription" = ""

    "ColumnName" = "color"

    "ColumnType" = "color"

    "DataType" = "varchar(10)"

    "DefaultValue" = "'#708ad8'"

    "IsAutoIncrement" = false

    "IsIndexed" = false

    "IsNullable" = false

    "IsPrimaryKey" = false

    "IsUnique" = false

    "Name" = "color"
  }

  "TableName" = "integration"
}
