# In built actions

!!! note ""
    In-built actions are not required to be imported and will be recreated even on deletion. You can restrict their access by altering their **permission**.

## Restart daptin

!!! note ""
    Restarts daptin immediately and reads file system for new config and data files and apply updates to the APIs as necessary.

    Takes about 15 seconds to restart.

## Publish package to marketplace

!!! note ""
    Export the JSON schema of your APIs to be re-used by other users from a [**marketplace**](/extend/marketplace.md).

## Update package list

!!! note ""
    Exports the schema of your APIs as a package to a [**marketplace**](/extend/marketplace.md). You can later install this package.


## Visit marketplace

!!! note ""
    Redirects you to the [**marketplace**](/extend/marketplace.md) repository

## Refresh marketplace packages

!!! note ""
    Checks for updates in the git repository, and pulls changes. Changes to packages already installed are not immediately applied and these packages should be installed again for updates.

## Generate random data

!!! note ""
    Generate random data of any entity type to play around. Takes in a ```count``` parameter and generates that many rows. Daptin uses a fake data generator to generate quality random data for a wide variety of fields.

## Install marketplace package

!!! note ""
    Install package from a [**marketplace**](/extend/marketplace.md) by specifying a ```package_name```. This will restart daptin and apply the necessary changes. Note: any updates to this package in the marketplace will not be applied automatically.


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
    Upload file to external store [**cloud_store**](/cloudstore/cloudstore.md), may require [oauth token and connection](/extend/oauth_connection.nd).

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
