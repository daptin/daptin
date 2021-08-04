# Actions list

Use actions to build work flows to carry out tasks like syncing data or emailing your users. You can also give access to these workflows to your users and restrict their access by altering their **permission**.

The following actions are available by default on a fresh instance. These actions cannot be deleted and will be recreated if deleted directly from the database.

Actions can use certain inbuilt methods to perform wide variety of operations.


## Default actions


### Restart daptin

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


### Generate random data

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


### Export data

!!! example ""
    Export data as JSON dump. This will export for a single table if ```table_name``` param is specific, else it will export all data.

### Import data

!!! example ""
    Import data from dump exported by Daptin. Takes in the following parameters:

    - dump_file - json|yaml|toml|hcl
    - truncate_before_insert: default ```false```, specify ```true``` to tuncate tables before importing


### Upload file to a cloud store

!!! example ""
    Upload file to external store [**cloud_store**](/cloudstore/cloudstore), may require [oauth token and connection](/extend/oauth_connection.nd).

    - file: any

### Upload XLS


!!! example ""
    Upload xls to entity, takes in the following parameters:

    - data_xls_file: xls, xlsx
    - entity_name: existing table name or new to create a new entity
    - create_if_not_exists: set ```true``` if creating a new entity (to avoid typo errors in above)
    - add_missing_columns: set ```true``` if the file has extra columns which you want to be created


### Upload CSV

!!! example ""
    Upload CSV to entity

    - data_xls_file: xls, xlsx
    - entity_name: existing table name or new to create a new entity
    - create_if_not_exists: set ```true``` if creating a new entity (to avoid typo errors in above)
    - add_missing_columns: set ```true``` if the file has extra columns which you want to be created


!!! example "Curl"
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


!!! example "NodeJS Example"
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

### Upload schema

!!! example ""
    Upload entity types or actions or any other config to daptin

    - schema_file: json|yaml|toml|hcl

    restart, system_json_schema_update


### Download Schema

!!! example ""
    Download a JSON config of the current daptin instance. This can be imported at a later stage to recreate a similar instance. Note, this contains only the structure and not the actual data. You can take a **data dump** separately. Or of a particular entity type


### Become administrator

!!! example ""
    Become an admin user of the instance. Only the first user can do this, as long as there is no second user.

### Sign up

!!! example ""
    Sign up a new user, takes in the following parameters

    - name
    - email
    - password
    - passwordConfirm

    Creates these rows :

    - a new user
    - a new usergroup for the user
    - user belongs to the usergroup


### Sign in

!!! example ""
    Sign in essentially generates a [JWT token] issued by Daptin which can be used in requests to authenticate as a user.

    - email
    - password

### Oauth login

!!! example ""
    Authenticate via OAuth, this will redirect you to the oauth sign in page of the oauth connection. The response will be handeled by **oauth login response**


### Oauth login response


!!! example ""
    This action is supposed to handle the oauth login response flow and not supposed to be invoked manually.

    Takes in the following parameters (standard oauth2 params)
    - code
    - state
    - authenticator

    Creates :

    - oauth profile exchange: generate a token from oauth provider
    - stores the oauth token + refresh token for later user

### Add data exchange

!!! example ""
    Add new data sync with google-sheets

    - name
    - sheet_id
    - app_key

    Creates a data exchange


# List of inbuilt methods 

These methods can be used in actions

| Method Identifier            | Method Inputs                                                | Description                                                                                            |   |   |
|------------------------------|--------------------------------------------------------------|--------------------------------------------------------------------------------------------------------|---|---|
| __become_admin               | User auth token is required                                  | tries to make the caller the administrator of the instance                                             |   |   |
| cloudstore.file.delete       | cloudstore id and path                                       | delete a file at a path in cloud store                                                                 |   |   |
| cloudstore.file.upload       | cloudstore id, path and file blob                            | upload a file at a path in cloud store                                                                 |   |   |
| cloudstore.folder.create     | cloudstore id and path                                       | create a folder at a path in cloud store                                                               |   |   |
| cloudstore.path.move         | cloudstore id, old path and new path                         | move a file/folder or rename path in a cloud store                                                     |   |   |
| cloudstore.site.create       | cloudstore id, site hostname and path on cloudstoure to host | create a new hugo/static site at a path in a cloud store                                               |   |   |
| column.storage.sync          | table id and column name                                     | sync all changes to the asset column store with cloud store provider                                   |   |   |
| __upload_csv_file_to_entity  | csv file and target entity name                              | upload data from CSV file to a table                                                                   |   |   |
| world.column.delete          | table id and column name                                     | delete a column in a table                                                                             |   |   |
| world.delete                 | table id                                                     | delete a table                                                                                         |   |   |
| __download_cms_config        | no inputs                                                    | exports the internal config as JSON, should never be accessible to public                              |   |   |
| __enable_graphql             | no inputs                                                    | enable the graphql endpoint by setting config to true , should never be accessible to public           |   |   |
| __csv_data_export            | table id                                                     | export data from a table as csv, should never be accessible to public                                  |   |   |
| __data_export                | table id                                                     | export data from a table as json, should never be accessible to public                                 |   |   |
| acme.tls.generate            | site id                                                      | generate a certificate for a site from LetsEncrypt                                                     |   |   |
| jwt.token                    | email and password of the user account                       | generates a JWT token valid for 4 days (configurable)                                                  |   |   |
| oauth.token                  | oauth token id                                               | returns the access token for the stored oauth token                                                    |   |   |
| password.reset.begin         | email id                                                     | start password reset process by sending a password reset email to user from the configured mail server |   |   |
| password.reset.verify        | email id, token, new password                                | verify password reset and let the user set a new password if token is valid                            |   |   |
| generate.random.data         | table id                                                     | generate N rows fit for table, random data generated for each field                                    |   |   |
| self.tls.generate            | site id                                                      | create a self-generated SSL certificate for HTTPS enabled sites                                        |   |   |
| cloud_store.files.import     | table id, cloudstore id, path                                | import files from a cloud store to a table                                                             |   |   |
| __data_import                | file dump                                                    | import data from JSON/YAML dump direct to database                                                     |   |   |
| integration.install          | integration id                                               | Import all operations defined in the integration spec as actions                                       |   |   |
| mail.servers.sync            | no input                                                     | synchronise mail server interface                                                                      |   |   |
| response.create              | response_type                                                | create a custom response to be returned                                                                |   |   |
| $network.request             | Headers,Query,Body                                           | call an external API                                                                                   |   |   |
| oauth.client.redirect        | authenticator                                                | send the user to the 3rd party oauth login page                                                        |   |   |
| oauth.login.response         | authenticator, state, user id                                | handle the response code from 3rd party login                                                          |   |   |
| oauth.profile.exchange       | authenticator, profileUrl, token                             | exchange the token from 3rd party oauth service for the user profile                                   |   |   |
| otp.generate                 | email/mobile                                                 | generate a TOTP for the account (can be sent via SMS/EMAIL)                                            |   |   |
| otp.login.verify             | email/mobile and otp code                                    | verify a TOTP code presented by the user, generate a JWT token if valid                                |   |   |
| world.column.rename          | table id, column name, new column name                       | try to rename a table column                                                                           |   |   |
| __restart                    | no input                                                     | reload all configurations and settings (after changes in config/site/cloudstore etc)                   |   |   |
| site.file.get                | site id, file path                                           | get file contents at the certain path                                                                  |   |   |
| site.file.list               | site id, path                                                | get list of contents of a folder                                                                       |   |   |
| site.storage.sync            | site id                                                      | sync down all changes from the storage provider                                                        |   |   |
| __upload_xlsx_file_to_entity | xlsx file, table id                                          | import XLS and insert rows into a table                                                                |   |   |
