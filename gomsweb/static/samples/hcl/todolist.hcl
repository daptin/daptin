"Actions" = {
  "InFields" = {
    "ColumnName" = "description"

    "ColumnType" = "label"

    "Name" = "Description"
  }

  "InFields" = {
    "ColumnName" = "schedule"

    "ColumnType" = "date"

    "Name" = "Scheduled at"
  }

  "InstanceOptional" = true

  "Label" = "New to do"

  "Name" = "new_task"

  "OnType" = "project"

  "OutFields" = {
    "Attributes" = {
      "project_id" = "$.reference_id"

      "schedule" = "~schedule"

      "title" = "~description"
    }

    "Method" = "POST"

    "Type" = "todo"
  }

  "OutFields" = {
    "Attributes" = {
      "message" = "Created new todo, taking you to it."

      "title" = "Wait for it"

      "type" = "success"
    }

    "Method" = "ACTIONRESPONSE"

    "Type" = "client.notify"
  }

  "OutFields" = {
    "Attributes" = {
      "delay" = "1000"

      "location" = "/in/item/todo/$.reference_id"

      "window" = "self"
    }

    "Method" = "ACTIONRESPONSE"

    "Type" = "client.redirect"
  }

  "OutFields" = {
    "Attributes" = {
      "key" = "$.reference_id"

      "value" = "last_created_todo"
    }

    "Method" = "ACTIONRESPONSE"

    "Type" = "client.store.set"
  }

  "OutFields" = {
    "Attributes" = {
      "message" = "show this in error notification body"
    }

    "Method" = "ACTIONRESPONSE"

    "Type" = "error"
  }
}

"Actions" = {
  "Label" = "Completed"

  "Name" = "completed"

  "OnType" = "todo"

  "OutFields" = {
    "Attributes" = {
      "completed" = "1"

      "reference_id" = "$.reference_id"
    }

    "Method" = "UPDATE"

    "Type" = "todo"
  }
}

"Actions" = {
  "InFields" = {
    "ColumnName" = "name"

    "ColumnType" = "label"

    "Name" = "name"
  }

  "InstanceOptional" = true

  "Label" = "New project category"

  "Name" = "new_project"

  "OnType" = "project"

  "OutFields" = {
    "Attributes" = {
      "name" = "~name"
    }

    "Method" = "POST"

    "Type" = "project"
  }

  "OutFields" = {
    "Attributes" = {
      "delay" = "1000"

      "location" = "/in/item/project/$.reference_id"

      "window" = "self"
    }

    "Method" = "ACTIONRESPONSE"

    "Type" = "client.redirect"
  }
}

"Actions" = {
  "InFields" = {
    "ColumnName" = "sheetUrl"

    "ColumnType" = "url"

    "Name" = "Sheet url"
  }

  "InFields" = {
    "ColumnName" = "include_all_columns"

    "ColumnType" = "boolean"

    "DefaultValue" = "false"

    "Name" = "Include all Columns ?"
  }

  "Label" = "Sync tasks to Google sheet"

  "Name" = "new_data_exchange"

  "OnType" = "data_exchange"

  "OutFields" = {
    "Attributes" = {
      "description" = "~description"

      "project_id" = "$.reference_id"

      "schedule" = "~schedule"
    }

    "Method" = "POST"

    "Type" = "data_exchange"
  }
}

"Exchanges" = {
  "Attributes" = {
    "SourceColumn" = "$self.description"

    "TargetColumn" = "Task description"
  }

  "Attributes" = {
    "SourceColumn" = "self.schedule"

    "TargetColumn" = "Scheduled at"
  }

  "Name" = "Task to excel sheet"

  "Options" = {
    "hasHeader" = true
  }

  "SourceAttributes" = {
    "Name" = "todo"
  }

  "SourceType" = "self"

  "TargetAttributes" = {
    "appKey" = "AIzaSyAC2xame4NShrzH9ZJeEpWT5GkySooa0XM"

    "sheetUrl" = "https://content-sheets.googleapis.com/v4/spreadsheets/1Ru-bDk3AjQotQj72k8SyxoOs84eXA1Y6sSPumBb3WSA/values/A1:append"
  }

  "TargetType" = "gsheet-append"
}

"Relations" = {
  "Object" = "project"

  "Relation" = "has_one"

  "Subject" = "todo"
}

"Relations" = {
  "Object" = "tag"

  "Relation" = "has_many"

  "Subject" = "todo"
}

"StateMachineDescriptions" = {
  "Events" = {
    "Dst" = "started"

    "Label" = "Start"

    "Name" = "start"

    "Src" = ["to_be_done", "delayed"]
  }

  "Events" = {
    "Dst" = "delayed"

    "Label" = "Unable to pick up"

    "Name" = "delayed"

    "Src" = ["to_be_done"]
  }

  "Events" = {
    "Dst" = "ongoing"

    "Label" = "Record progress"

    "Name" = "ongoing"

    "Src" = ["started", "ongoing"]
  }

  "Events" = {
    "Dst" = "interrupted"

    "Label" = "Interrupted"

    "Name" = "interrupted"

    "Src" = ["started", "ongoing"]
  }

  "Events" = {
    "Dst" = "ongoing"

    "Label" = "Resume from interruption"

    "Name" = "resume"

    "Src" = ["interrupted"]
  }

  "Events" = {
    "Dst" = "completed"

    "Label" = "Mark as completed"

    "Name" = "completed"

    "Src" = ["ongoing", "started"]
  }

  "InitialState" = "to_be_done"

  "Label" = "Task Status"

  "Name" = "task_status"
}

"Tables" = {
  "Columns" = {
    "ColumnType" = "label"

    "DataType" = "varchar(500)"

    "IsIndexed" = true

    "Name" = "title"
  }

  "Columns" = {
    "ColumnType" = "url"

    "DataType" = "varchar(200)"

    "IsNullable" = true

    "Name" = "url"
  }

  "Columns" = {
    "ColumnType" = "truefalse"

    "DataType" = "int(1)"

    "DefaultValue" = "false"

    "Name" = "completed"
  }

  "Columns" = {
    "ColumnType" = "date"

    "DataType" = "date"

    "IsNullable" = true

    "Name" = "schedule"
  }

  "Columns" = {
    "ColumnType" = "measurement"

    "DataType" = "int(4)"

    "DefaultValue" = "10"

    "Name" = "order"

    "columnName" = "item_order"
  }

  "Columns" = {
    "ColumnType" = "content"

    "DataType" = "text"

    "IsNullable" = true

    "Name" = "text"
  }

  "Conformations" = {
    "ColumnName" = "order"

    "Tags" = "numeric"
  }

  "TableName" = "todo"

  "validations" = {
    "ColumnName" = "title"

    "Tags" = "required"
  }
}

"Tables" = {
  "Columns" = {
    "ColumnType" = "label"

    "DataType" = "varchar(100)"

    "IsIndexed" = true

    "Name" = "label"
  }

  "TableName" = "tag"
}

"Tables" = {
  "Columns" = {
    "ColumnType" = "name"

    "DataType" = "varchar(200)"

    "IsIndexed" = true

    "Name" = "name"
  }

  "TableName" = "project"
}
