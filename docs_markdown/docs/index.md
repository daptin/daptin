# Deploy

[Create an instance](setting-up/settingup.md)

# Import data

<img src="images/create_data.png">

You can either directly import a csv or xls file to import unknown data and let daptin identify the columns and types.

Read more [here](http://localhost:8000/setting-up/entities/) 


### Upload from dashboard

Upload one of these files:

| File        | Usage                              |
| ----------- | ---------------------------------- |
| Schema JSON | Create schema and apis             |
| CSV         | Auto create entity and upload data |
| XLSX        | Auto create entity and upload data |
| Data JSON   | Upload data from dumps             |

# APIs



| GET    | /api/{entityName}                                         | Query Params                          | Request Body                                                                                  | Description                                                                                           |
| ------ | --------------------------------------------------------- | ------------------------------------- | --------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------- |
| POST   | /api/{entityName}                                         | page[size]= page[number] query filter |                                                                                               | Find all rows, paginated with query and filters                                                       |
| PATCH  | /api/{entityName}/{id}                                    |                                       | {"attributes": {  } "type": "{entityType} }                                                   | Update row by reference id                                                                            |
| PUT    | /api/{entityName}/{id}                                    |                                       | {"attributes": { } "type": "{entityType} }                                                    | Update row by reference id                                                                            |
| DELETE | /api/{entityName}/{id}                                    |                                       |                                                                                               | Delete a row                                                                                          |
| GET    | /api/{entityName}/{id}/{relationName}                     | page[size]= page[number] query filter |                                                                                               | Find all related rows by relation name, eg, "posts" of a user                                         |
| DELETE | /api/{entityName}/{id}/{relationName}                     |                                       | {"id": , "type":  }                                                                           | Delete a related row, eg: delete post of a user. this only removes a relation and not the actual row. |
| GET    | /action/{entityName}/{actionName}                         | Parameters for action                 |                                                                                               | Invoke an action on an entity                                                                         |
| POST   | /action/{entityName}/{actionName}                         |                                       | { "attribute": { Parameters for action }, "id": "< object id >" type: "< entity type >" }     | Invoke an action on an entity                                                                         |
| POST   | /track/start/{stateMachineId}                             |                                       | { "id": " < reference id >", type: " < entity type > " }                                      | Start tracking according to the state machine for an object                                           |
| POST   | /track/event/{typename}/{objectStateId}/{eventName}       |                                       |                                                                                               | Invoke an event on a particular track of the state machine for a object                               |
| GET    | /live                                                     |                                       |                                                                                               | Initiate a web socket connection                                                                      |
|        |                                                           |                                       |                                                                                               |                                                                                                       |
| GET    | /apispec.raml                                             |                                       |                                                                                               | RAML Spec for all API's                                                                               |
| GET    | /ping                                                     |                                       |                                                                                               | Replies with PONG, Good for liveness probe                                                            |


# Users and Usergroups

Daptin has a built-in user system. Users are identified by their authorization token or other means of identification. Users are idenfied as registered users or guests.

You can choose to disable new user registration by disallowing the signup action in permissions.

## Adding users

You can manually add users from the users page, or allow sign-up action to be performed by guests which will take care of creating a user and an associated usergroup for that user. All new signed up users will also be added to the "users" usergroup.

Users and usergroups are just like other data in the system and is available over `/api/users` endpoint. You can choose to allow read/write permission directly to that table to allow other users/processes to use this api to read/create/update/delete users.

## Social login

Daptin can work with any oauth flow aware identity provider to allow new users to be registered (if you have disabled normal signup).

Create a [OAuth Connection](/extend/oauth_connection.md) and mark "Allow login" to enable APIs for OAuth flow.

Examples

!!! note "Google login configuration"
    ![Google oauth](images/oauth/google.png)
    
!!! note "Dropbox login configuration"
    ![Google oauth](images/oauth/dropbox.png)
    
!!! note "Github login configuration"
    ![Google oauth](images/oauth/github.png)
    
!!! note "Linkedin login configuration"
    ![Google oauth](images/oauth/linkedin.png)
    

!!! note "Encrypted values"
    The secrets are stored after encryption so the value you see in above screenshots are encrypted values.


## Creating usergroups

### Tables

Tables are the basic data structure. Tables have columns. Each column has a particular data type. These tables are exposed over APIs under the `/api/` path. Checkout [entity documentation](/setting-up/entities.md) for details.

!!! note "YAML example"
    ```yaml
    Tables:
    - TableName: todo
      Columns:
      - Name: title
        DataType: varchar(500)
        ColumnType: label
        IsIndexed: true
    ```


!!! note "JSON example"
    ```json
    {
      "Tables": [
        {
          "TableName": "todo",
          "Columns": [
            {
              "Name": "title",
              "DataType": "varchar(500)",
              "ColumnType": "label",
              "IsIndexed": true
            }
          ]
        }
      ]
    }
    ```

### Relations

Relations are constraints among tables and help you keep clean and consistent data. Relational data is easily accessible over APIs using a path structure like `/api/<entityName>/<id>/<relationName>` and the response is consistent with [JSONAPI.org](https://JSONAPI.org).

!!! note "YAML example"
    ```yaml
    Relations:
    - Subject: todo
      Relation: has_one
      Object: project
    ```

!!! note "JSON example"
    ```json
    {
      "Relations": [
        {
          "Subject": "todo",
          "Relation": "has_one",
          "Object": "project"
        }
      ]
    }
    ```

## Actions

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


## Data Exchanges <img style="float: right" src="images/exchanges.png" class="cloud-provider">

Exchanges are internal hooks to external apis, to either push data and update an external service, or pull data and update itself from some external service.

Example, use exchange to sync data creation call to Google Sheets. So on every row created using the POST API also creates a corresponding row in your google sheet.

!!! note "Google drive exchange YAML"
    ```yaml
    Exchanges:
    - Name: Task to excel sheet
      SourceAttributes:
        Name: todo
      SourceType: self
      TargetAttributes:
        sheetUrl: https://content-sheets.googleapis.com/v4/spreadsheets/1Ru-bDk3AjQotQj72k8SyxoOs84eXA1Y6sSPumBb3WSA/values/A1:append
        appKey: AIzaSyAC2xame4NShrzH9ZJeEpWT5GkySooa0XM
      TargetType: gsheet-append
      Attributes:
      - SourceColumn: "$self.description"
        TargetColumn: Task description
      - SourceColumn: self.schedule
        TargetColumn: Scheduled at
      Options:
        hasHeader: true
    ```

## State Machine APIs

Using state machine descriptions with daptin expose couple of super useful apis to manage state based data.

Enabling `task_status` state machine on `todo` entity will expose the following APIs


```bash
POST /track/start/:stateMachineId {"typeName": "todo", "referenceId": "objectId"} # Start tracking a particular object by id
```

This returns a state machine id.

```bash
POST /track/event/:typename/:objectStateMachineId/:eventName {} # Trigger event on current state
```

This either moves the object state to next state, or fails on invalid event name.

Example, tracking the status of a task in a todo list, the description below describes the states it can take and the events.

!!! note "State machine description YAML"
    ```yaml
    StateMachineDescriptions:
    - Name: task_status
      Label: Task Status
      InitialState: to_be_done
      Events:
      - Name: start
        Label: Start
        Src:
        - to_be_done
        - delayed
        Dst: started
      - Name: delayed
        Label: Unable to pick up
        Src:
        - to_be_done
        Dst: delayed
      - Name: ongoing
        Label: Record progress
        Src:
        - started
        - ongoing
        Dst: ongoing
      - Name: interrupted
        Label: Interrupted
        Src:
        - started
        - ongoing
        Dst: interrupted
      - Name: resume
        Label: Resume from interruption
        Src:
        - interrupted
        Dst: ongoing
      - Name: completed
        Label: Mark as completed
        Src:
        - ongoing
        - started
        Dst: completed
    ```

## Users and access

<img src="images/users_and_groups.png">

Requests **without** a valid `Authorization Bearer` `token` will be referred to as "guests requests". Requests with a valid token will have an identified user.

## User registration API

Sign up action can be allowed to guests to allow open registration by anyone. Users with enough permission over the `user` table can create users manually.

Users registered using signup action are their own owners. Hence they can update and delete themselves. These permission can be changed based on the use case.

!!! note "POST call for user registration"
    ```bash
    curl 'http://api.daptin.com:6336/action/user/signup' \
    -H 'Authorization: Bearer null' \
    -H 'Content-Type: application/json;charset=UTF-8' \
    -H 'Accept: application/json, text/plain, */*' \
    --data-binary '{"attributes":{"name":"username","email":"<UserEmail>","password":"<Password>","passwordConfirm":"<Password>"}}' 
    ```

You can either allow guests to be able to invoke `sign up` action or allow only a particular user to be able to create new users or a usergroup.

```json
[
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "message": "Created user",
      "title": "Success",
      "type": "success"
    }
  }
]
```

This user can sign in now (generate an auth token). But what he can access is again based on the permission of the system.

## User sign in API

!!! note "POST call for sign in"
    ```bash
    curl 'http://api.daptin.com:6336/action/user/signin' \
    -H 'Content-Type: application/json;charset=UTF-8' \
    -H 'Accept: application/json, text/plain, */*' \
    --data-binary '{"attributes":{"email":"<Email>","password":"<Password>"}}'
    ```

```json
[
  {
    "ResponseType": "client.store.set",
    "Attributes": {
      "key": "token",
      "value": "<AccessToken>"
    }
  },
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "message": "Logged in",
      "title": "Success",
      "type": "success"
    }
  },
  {
    "ResponseType": "client.redirect",
    "Attributes": {
      "delay": 2000,
      "location": "/",
      "window": "self"
    }
  }
]
```

## Host static sites

<img src="images/subsite.png">

Expose folders on cloud storage services as websites using your daptin instance.

!!! note "New subsite"
    - Select a cloud storage
    - Choose a domain/sub-domain
    - Choose a sub-path

Restart is required to reflect changes.


## Data auditing

To enable recoding of all historical data for a particular entity, enable data audit for it in the worlds configuration.

Audits are ready only and cannot be manipulated over api. You can configure the permission for your use case.

## OAuth Connection

<img src="images/oauth.png">

Connecting to your cloud storage and external services requires authentication, OAuth covers a large number of those services.


# Concepts

## Data

- [Data storage and table structures](data-modeling/data_storage.md)
- [Auditing](data-modeling/auditing.md)
- [Data validations](data-modeling/data_validation.md)
- [Data conformations](data-modeling/data_conformation.md)
- [State tracking for entities](data-modeling/state_tracking.md)

## Actions and streams

- [Actions](actions-streams/actions.md)
- [Data streams](actions-streams/streams.md)
- [Oauth connections](extend/oauth_connection.md)
- [Oauth tokens](extend/oauth_token.md)

## Authentication and Authorization
- [Users and user groups](auth/users_and_usergroups.md)
- [Authentication](auth/authentication.md)
- [Authorization](auth/authorization.md)
- [Permissions](auth/permissions.md)

## External APIs and integrations

- Interacting with third party APIs
- Storage connectors

## Sub-sites, Marketplace

- [Sub sites](subsite/subsite.md)
- Json schemas
