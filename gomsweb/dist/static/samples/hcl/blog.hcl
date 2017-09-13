"Actions" = {
  "InFields" = {
    "ColumnName" = "name"

    "ColumnType" = "label"

    "Name" = "Author Name"
  }

  "InFields" = {
    "ColumnName" = "comment"

    "ColumnType" = "content"

    "Name" = "Your comment"
  }

  "InFields" = {
    "ColumnName" = "like"

    "ColumnType" = "rating"

    "Name" = "Rating"
  }

  "Label" = "Leave a comment"

  "Name" = "comment"

  "OnType" = "post"

  "OutFields" = {
    "Attributes" = {
      "body" = "~comment"

      "likes_count" = "~like"

      "post_id" = "$.reference_id"
    }

    "Method" = "POST"

    "Type" = "comment"
  }
}

"Actions" = {
  "InFields" = {
    "ColumnName" = "title"

    "ColumnType" = "label"

    "Name" = "Post title"
  }

  "InFields" = {
    "ColumnName" = "body"

    "ColumnType" = "content"

    "Name" = "Blog"
  }

  "Label" = "Create a new post"

  "Name" = "post"

  "OnType" = "blog"

  "OutFields" = {
    "Attributes" = {
      "blog_id" = "$.reference_id"

      "body" = "~body"

      "title" = "~title"
    }

    "Method" = "POST"

    "Type" = "post"
  }
}

"Exchanges" = {
  "Attributes" = {
    "sourceColumn" = "$blog.title"

    "targetColumn" = "Blog title"
  }

  "Attributes" = {
    "sourceColumn" = "$blog.view_count"

    "targetColumn" = "View count"
  }

  "Name" = "Blog to excel sheet sync"

  "Options" = {
    "hasHeader" = true
  }

  "SourceAttributes" = {
    "Name" = "blog"
  }

  "SourceType" = "self"

  "TargetAttributes" = {
    "sheetUrl" = "https://content-sheets.googleapis.com/v4/spreadsheets/1Ru-bDk3AjQotQj72k8SyxoOs84eXA1Y6sSPumBb3WSA/values/A1:append"
  }

  "TargetType" = "gsheet-append"
}

"Relations" = {
  "Object" = "post"

  "Relation" = "belongs_to"

  "Subject" = "comment"
}

"Relations" = {
  "Object" = "blog"

  "Relation" = "belongs_to"

  "Subject" = "post"
}

"Relations" = {
  "Object" = "post"

  "ObjectName" = "current_post"

  "Relation" = "has_one"

  "Subject" = "blog"

  "SubjectName" = "current_post_of"
}

"StateMachineDescriptions" = {
  "Events" = {
    "Dst" = "ready-to-publish"

    "Label" = "Mark as edited"

    "Name" = "mark_as_edited"

    "Src" = ["draft", "ready-to-publish"]
  }

  "Events" = {
    "Dst" = "published"

    "Label" = "Publish"

    "Name" = "publish"

    "Src" = ["draft", "ready-to-publish"]
  }

  "Events" = {
    "Dst" = "deleted"

    "Label" = "Delete"

    "Name" = "delete"

    "Src" = ["draft", "ready-to-publish", "published"]
  }

  "InitialState" = "draft"

  "Label" = "Publish Status"

  "Name" = "publish_status"
}

"Tables" = {
  "Columns" = {
    "ColumnType" = "label"

    "DataType" = "varchar(500)"

    "Name" = "title"
  }

  "Columns" = {
    "ColumnType" = "measurement"

    "DataType" = "int(11)"

    "Name" = "view_count"
  }

  "TableName" = "blog"
}

"Tables" = {
  "Columns" = {
    "ColumnType" = "label"

    "DataType" = "varchar(200)"

    "Name" = "title"
  }

  "Columns" = {
    "ColumnType" = "content"

    "DataType" = "text"

    "Name" = "body"
  }

  "TableName" = "post"
}

"Tables" = {
  "Columns" = {
    "ColumnType" = "content"

    "DataType" = "text"

    "Name" = "body"
  }

  "Columns" = {
    "ColumnName" = "likes_count"

    "ColumnType" = "measurement"

    "DataType" = "int(11)"

    "Name" = "likes_count"
  }

  "TableName" = "comment"
}
