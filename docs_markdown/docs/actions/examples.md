## Examples

Actions are entity dependent APIs which you want to expose which may have an outcome of events. Most basic example is the login action which generates an oauth2 token as an outcome.

Use action to expose endpoints for your forms and processes. Here is an example of creating a "/action/project/new_task" API:

!!! note "New task action YAML"
    ```yaml
    Actions:
    - Name: new_task
      Label: New to do
      OnType: project
      InstanceOptional: true
      InFields:
      - ColumnName: description
        Name: Description
        ColumnType: label
      - ColumnName: schedule
        Name: Scheduled at
        ColumnType: date
      OutFields:
      - Type: todo
        Method: POST
        Attributes:
          schedule: "~schedule"
          title: "~description"
          project_id: "$.reference_id"
      - Type: client.notify
        Method: ACTIONRESPONSE
        Attributes:
          type: success
          message: Created new todo, taking you to it.
          title: Wait for it
    ```


!!! note "New task action JSON"
    ```json
    {
      "Actions": [
        {
          "Name": "new_task",
          "Label": "New to do",
          "OnType": "project",
          "InstanceOptional": true,
          "InFields": [
            {
              "ColumnName": "description",
              "Name": "Description",
              "ColumnType": "label"
            },
            {
              "ColumnName": "schedule",
              "Name": "Scheduled at",
              "ColumnType": "date"
            }
          ],
          "OutFields": [
            {
              "Type": "todo",
              "Method": "POST",
              "Attributes": {
                "schedule": "~schedule",
                "title": "~description",
                "project_id": "$.reference_id"
              }
            },
            {
              "Type": "client.notify",
              "Method": "ACTIONRESPONSE",
              "Attributes": {
                "type": "success",
                "message": "Created new todo, taking you to it.",
                "title": "Wait for it"
              }
            }
          ]
        }
      ]
    }
    ```
