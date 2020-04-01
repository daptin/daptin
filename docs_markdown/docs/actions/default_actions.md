# Actions list

Use actions to build work flows to carry out tasks like syncing data or emailing your users. You can also give access to these workflows to your users and restrict their access by altering their **permission**.

The following actions are available by default on a fresh instance. These actions cannot be deleted and will be recreated if deleted directly from the database.

## Restart daptin

Restarts daptin immediately and reads file system for new config and data files and apply updates to the APIs as necessary.

Takes about 15 seconds (async) to reconfigure everything.

```
var request = require('request');

var headers = {
    'Authorization': 'Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImFydHBhcjFAZ21haWwuY29tIiwiZXhwIjoxNTIzMTgzMTA0LCJpYXQiOiIyMDE4LTA0LTA1VDE1OjU1OjA0LjYyMzU4NTYxKzA1OjMwIiwiaXNzIjoiZGFwdGluIiwianRpIjoiNmJhMmFhZjgtODBlNS00OGIwLTgwZmItMzEzYzk3Nzg0Y2E4IiwibmFtZSI6InBhcnRoIiwibmJmIjoxNTIyOTIzOTA0LCJwaWN0dXJlIjoiaHR0cHM6Ly93d3cuZ3JhdmF0YXIuY29tL2F2YXRhci9mNGJmNmI2Nzg5NGU5MzAzYjZlMTczMTMyZWE0ZTkwYVx1MDAyNmQ9bW9uc3RlcmlkIn0.eb5Vp00cHLeshZBtwJIyarJ6RQOLeVPj15n8ubVnGYo'
};

var dataString = '{"attributes":{}}';

var options = {
    url: 'http://localhost:6336/action/world/restart_daptin',
    method: 'POST',
    headers: headers,
    body: dataString
};

function callback(error, response, body) {
    if (!error && response.statusCode == 200) {
        console.log(body);
    }
}

request(options, callback);

```


## Generate random data

Generate random data of any entity type to play around. Takes in a ```count``` parameter and generates that many rows. Daptin uses a fake data generator to generate quality random data for a wide variety of fields.

```
var request = require('request');

var headers = {
    'Authorization': 'Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImFydHBhcjFAZ21haWwuY29tIiwiZXhwIjoxNTIzMTgzMTA0LCJpYXQiOiIyMDE4LTA0LTA1VDE1OjU1OjA0LjYyMzU4NTYxKzA1OjMwIiwiaXNzIjoiZGFwdGluIiwianRpIjoiNmJhMmFhZjgtODBlNS00OGIwLTgwZmItMzEzYzk3Nzg0Y2E4IiwibmFtZSI6InBhcnRoIiwibmJmIjoxNTIyOTIzOTA0LCJwaWN0dXJlIjoiaHR0cHM6Ly93d3cuZ3JhdmF0YXIuY29tL2F2YXRhci9mNGJmNmI2Nzg5NGU5MzAzYjZlMTczMTMyZWE0ZTkwYVx1MDAyNmQ9bW9uc3RlcmlkIn0.eb5Vp00cHLeshZBtwJIyarJ6RQOLeVPj15n8ubVnGYo',
};

var dataString = '{"attributes":{"count":100,"world_id":"a82bcd84-db3a-4542-b0ef-80e81fc62f8e"}}';

var options = {
    url: 'http://localhost:6336/action/world/generate_random_data',
    method: 'POST',
    headers: headers,
    body: dataString
};

function callback(error, response, body) {
    if (!error && response.statusCode == 200) {
        console.log(body);
    }
}

request(options, callback);

```


## Export data

!!! note ""
    Export data as JSON dump. This will export for a single table if ```table_name``` param is specific, else it will export all data.

## Import data

!!! note ""
    Import data from dump exported by Daptin. Takes in the following parameters:

    - dump_file - json|yaml|toml|hcl
    - truncate_before_insert: default ```false```, specify ```true``` to tuncate tables before importing


## Upload file to a cloud store

!!! note ""
    Upload file to external store [**cloud_store**](/cloudstore/cloudstore), may require [oauth token and connection](/extend/oauth_connection.nd).

    - file: any

## Upload XLS


!!! note ""
    Upload xls to entity, takes in the following parameters:

    - data_xls_file: xls, xlsx
    - entity_name: existing table name or new to create a new entity
    - create_if_not_exists: set ```true``` if creating a new entity (to avoid typo errors in above)
    - add_missing_columns: set ```true``` if the file has extra columns which you want to be created


## Upload CSV

!!! note ""
    Upload CSV to entity

    - data_xls_file: xls, xlsx
    - entity_name: existing table name or new to create a new entity
    - create_if_not_exists: set ```true``` if creating a new entity (to avoid typo errors in above)
    - add_missing_columns: set ```true``` if the file has extra columns which you want to be created


!!! note "Curl"
```
curl 'http://localhost:6336/action/world/upload_csv_to_system_schema' \
-H 'Authorization: Bearer <Token>' \
--data-binary '{
               	"attributes": {
               		"create_if_not_exists": true,
               		"add_missing_columns": true,
               		"data_csv_file": [{
               			"name": "<file name>.csv",
               			"file": "data:text/csv;base64,<File contents base64 here>",
               			"type": "text/csv"
               		}],
               		"entity_name": "<entity name>"
               	  }
               }'
```


!!! note "NodeJS Example"
    ```
    import requests

    headers = {
        'Authorization': 'Bearer <Token>',
    }

    data =  '{
                "attributes": {
                    "create_if_not_exists": true,
                    "add_missing_columns": true,
                    "data_csv_file": [{
                        "name": "<file name>.csv",
                        "file": "data:text/csv;base64,<File contents base64 here>",
                        "type": "text/csv"
                    }],
                    "entity_name": "<entity name>"
                  }
              }'

    response = requests.post('http://localhost:6336/action/world/upload_csv_to_system_schema', headers=headers, data=data)

    ```

## Upload schema

!!! note ""
    Upload entity types or actions or any other config to daptin

    - schema_file: json|yaml|toml|hcl

    restart, system_json_schema_update


## Download Schema

!!! note ""
    Download a JSON config of the current daptin instance. This can be imported at a later stage to recreate a similar instance. Note, this contains only the structure and not the actual data. You can take a **data dump** separately. Or of a particular entity type


## Become administrator

!!! note ""
    Become an admin user of the instance. Only the first user can do this, as long as there is no second user.

## Sign up

!!! note ""
    Sign up a new user, takes in the following parameters

    - name
    - email
    - password
    - passwordConfirm

    Creates these rows :

    - a new user
    - a new usergroup for the user
    - user belongs to the usergroup


## Sign in

!!! note ""
    Sign in essentially generates a [JWT token] issued by Daptin which can be used in requests to authenticate as a user.

    - email
    - password

## Oauth login

!!! note ""
    Authenticate via OAuth, this will redirect you to the oauth sign in page of the oauth connection. The response will be handeled by **oauth login response**


## Oauth login response


!!! note ""
    This action is supposed to handle the oauth login response flow and not supposed to be invoked manually.

    Takes in the following parameters (standard oauth2 params)
    - code
    - state
    - authenticator

    Creates :

    - oauth profile exchange: generate a token from oauth provider
    - stores the oauth token + refresh token for later user

## Add data exchange

!!! note ""
    Add new data sync with google-sheets

    - name
    - sheet_id
    - app_key

    Creates a data exchange

