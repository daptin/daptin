"Actions" = {
  "InFields" = {
    "ColumnName" = "vendor"

    "ColumnType" = "vendor"

    "Name" = "Vendor"
  }

  "InFields" = {
    "ColumnName" = "quantity"

    "ColumnType" = "measurement"

    "Name" = "quantity"
  }

  "Label" = "Create a new order for the style"

  "Name" = "new_order"

  "OnType" = "style"

  "OutFields" = {
    "Attributes" = {
      "name" = "~vendor"
    }

    "Method" = "POST"

    "Reference" = "vendor1"

    "Type" = "vendor"
  }

  "OutFields" = {
    "Attributes" = {
      "quantity" = "~quantity"

      "vendor_id" = "$vendor1.reference_id"
    }

    "Method" = "POST"

    "Reference" = "order1"

    "Type" = "orders"
  }

  "OutFields" = {
    "Attributes" = {
      "orders_id" = "$order1.reference_id"

      "style_id" = "$.reference_id"
    }

    "Method" = "POST"

    "Type" = "style_style_id_has_orders_orders_id"
  }
}

"Exchanges" = {
  "Attributes" = {
    "SourceColumn" = "$style.title"

    "TargetColumn" = "Style title"
  }

  "Name" = "Style to excel sync"

  "Options" = {
    "hasHeader" = true
  }

  "SourceAttributes" = {
    "Name" = "style"
  }

  "SourceType" = "self"

  "TargetAttributes" = {
    "sheetUrl" = "https://content-sheets.googleapis.com/v4/spreadsheets/1Ru-bDk3AjQotQj72k8SyxoOs84eXA1Y6sSPumBb3WSA/values/A1:append"
  }

  "TargetType" = "gsheet-append"
}

"Exchanges" = {
  "Attributes" = []

  "Name" = "Order to excel sync"

  "Options" = {
    "hasHeader" = true
  }

  "SourceAttributes" = {
    "Name" = "orders"
  }

  "SourceType" = "self"

  "TargetAttributes" = {
    "sheetUrl" = "https://content-sheets.googleapis.com/v4/spreadsheets/1Ru-bDk3AjQotQj72k8SyxoOs84eXA1Y6sSPumBb3WSA/values/A1:append"
  }

  "TargetType" = "gsheet-append"
}

"Exchanges" = {
  "Attributes" = []

  "Name" = "Vendor to excel sync"

  "Options" = {
    "hasHeader" = true
  }

  "SourceAttributes" = {
    "Name" = "vendor"
  }

  "SourceType" = "self"

  "TargetAttributes" = {
    "sheetUrl" = "https://content-sheets.googleapis.com/v4/spreadsheets/1Ru-bDk3AjQotQj72k8SyxoOs84eXA1Y6sSPumBb3WSA/values/A1:append"
  }

  "TargetType" = "gsheet-append"
}

"Relations" = {
  "Object" = "style"

  "Relation" = "belongs_to"

  "Subject" = "cost"
}

"Relations" = {
  "Object" = "style_file"

  "Relation" = "has_one"

  "Subject" = "style"
}

"Relations" = {
  "Object" = "orders"

  "Relation" = "has_many"

  "Subject" = "style"
}

"Relations" = {
  "Object" = "vendor"

  "Relation" = "has_one"

  "Subject" = "orders"
}

"StateMachineDescriptions" = {
  "Events" = {
    "Dst" = "yellow"

    "Label" = "warn"

    "Name" = "warn"

    "Src" = ["green"]
  }

  "Events" = {
    "Dst" = "red"

    "Label" = "panic"

    "Name" = "panic"

    "Src" = ["yellow", "green"]
  }

  "Events" = {
    "Dst" = "yellow"

    "Label" = "calm"

    "Name" = "calm"

    "Src" = ["red"]
  }

  "Events" = {
    "Dst" = "green"

    "Label" = "calm"

    "Name" = "calm"

    "Src" = ["yellow"]
  }

  "InitialState" = "green"

  "Label" = "Light status"

  "Name" = "light_states"
}

"Tables" = {
  "Columns" = {
    "ColumnType" = "label"

    "DataType" = "varchar(500)"

    "Name" = "name"
  }

  "Columns" = {
    "ColumnType" = "label"

    "DataType" = "varchar(100)"

    "IsUnique" = true

    "Name" = "code"
  }

  "TableName" = "style"
}

"Tables" = {
  "Columns" = {
    "ColumnType" = "measurement"

    "DataType" = "int(11)"

    "Name" = "quantity"
  }

  "TableName" = "orders"
}

"Tables" = {
  "Columns" = {
    "ColumnType" = "label"

    "DataType" = "varchar(100)"

    "Name" = "name"
  }

  "TableName" = "vendor"
}

"Tables" = {
  "Columns" = {
    "ColumnType" = "measurement"

    "DataType" = "int(11)"

    "Name" = "pre_road_show_cost"
  }

  "Columns" = {
    "ColumnType" = "measurement"

    "DataType" = "int(11)"

    "Name" = "Target_cost"
  }

  "Columns" = {
    "ColumnType" = "measurement"

    "DataType" = "int(11)"

    "Name" = "post_road_show_cost"
  }

  "TableName" = "cost"
}

"Tables" = {
  "Columns" = {
    "ColumnType" = "color"

    "DataType" = "varchar(100)"

    "Name" = "color"
  }

  "TableName" = "style_file"
}
