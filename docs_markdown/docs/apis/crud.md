# API reference

Daptin exposes various endpoints for each entity defined in the schema:

 - Create
 - Find one
 - Update
 - Delete
 - Find all
 - Find relations
 - Execute action
 - Aggregate
 - State management


All endpoints allow authentication using the Authorization Header.

## API Overview

### CRUD API

Read/Create/Update/Delete

| GET    | /api/{entityName}                                         | Query Params                          | Request Body                                                                                  | Description                                                                                           |
| ------ | --------------------------------------------------------- | ------------------------------------- | --------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------- |
| GET    | /api/{entityName}                                         | page[size]= page[number] query filter |                                                                                                    | Description                                                                                           |
| POST   | /api/{entityName}                                         |                                       |                                                                                               | Find all rows, paginated with query and filters                                                       |
| PATCH  | /api/{entityName}/{id}                                    |                                       | {"attributes": { ...{fields} } "type": "{entityType} }                                                   | Update row by reference id                                                                            |
| PUT    | /api/{entityName}/{id}                                    |                                       | {"attributes": { } "type": "{entityType} }                                                    | Update row by reference id                                                                            |
| DELETE | /api/{entityName}/{id}                                    |                                       |                                                                                               | Delete a row                                                                                          |


### Action API


| GET    | /action/{entityName}/{actionName}                         | Query Params                          | Request Body                                                                                  | Description                                                                                           |
| ------ | --------------------------------------------------------- | ------------------------------------- | --------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------- |
| POST   | /api/{entityName}/                                         | action parameters |     Action Parameters| Execute action |

### Relation APIs


| Method | Path | Query params  | Request body | Description |
| ------ | ---- | ------------- | ------------ | ----------- |
| GET    | /api/{entityName}/{id}/{relationName}                     | page[size]= page[number] query filter |                                                                                               | Find all related rows by relation name, eg, "posts" of a user                                         |
| DELETE | /api/{entityName}/{id}/{relationName}                     |                                       | {"id": , "type":  }                                                                           | Delete a related row, eg: delete post of a user. this only removes a relation and not the actual row. |
| GET    | /action/{entityName}/{actionName}                         | Parameters for action                 |                                                                                               | Invoke an action on an entity                                                                         |
| POST   | /action/{entityName}/{actionName}                         |                                       | { "attribute": { Parameters for action }, "id": "< object id >" type: "< entity type >" }     | Invoke an action on an entity                                                                         |


### Aggregate API

| Method | Path | Query params  | Request body | Description |
| ------ | ---- | ------------- | ------------ | ----------- |
| GET   | /stats/{typeName}         |  group/filter/join/column/timestamp/timefrom/timeto/order     |         | Run aggregate function over entity table  |


### State machine APIs

Enabled for the entities for which you have enabled state machines

| Method | Path | Query params  | Request body | Description |
| ------ | ---- | ------------- | ------------ | ----------- |
| POST   | /track/start/{stateMachineId}                             |                                       | { "id": " < reference id >", type: " < entity type > " }                                      | Start tracking according to the state machine for an object                                           |
| POST   | /track/event/{typename}/{objectStateId}/{eventName}       |                                       |                                                                                               | Invoke an event on a particular track of the state machine for a object                               |


### Websocket API (wip)

Listed to incoming updates to data over websocket live

| Method | Path | Query params  | Request body | Description |
| ------ | ---- | ------------- | ------------ | ----------- |
| GET    | /live                                                     |                                       |                                                                                               | Initiate a web socket connection                                                                      |


### Metadata API

Use metadata to build and design your appliction in a more intuitive way

| Method | Path | Query params  | Request body | Description |
| ------ | ---- | ------------- | ------------ | ----------- |
| GET    | /apispec.raml                                             |                                       |                                                                                               | RAML Spec for all API's                                                                               |
| GET    | /ping                                                     |                                       |                                                                                               | Replies with PONG, Good for liveness probe                                                            |
| GET    | /statistics                                                     |                                       |                                                                                               | Replies with PONG, Good for liveness probe                                                            |



## Read

### Parameters

| Name               |  parameter type          |  default value |  example value                                            |
|--------------------|--------------------------|----------------|-----------------------------------------------------------|
| page[number]       |  integer                 |  1             |  5                                                        |
| page[size]         |  integer                 |  10            |  100                                                      |
| query              |  json base64             |  []            | [{"column": "name", "operator": "is", "value": "england"}] |
| group              |  string                  |  -             |  [{"column": "name", "order": "desc"}]                     |
| included_relations |  comma separated string  |  -             |  user post author                                         |
| sort               |  comma seaparated string |  -             |  created_at amount guest_count                            |
| filter             |  string                  |  -             |  england                                                  |


### Response

!!! note "Response example"
    ```
    {
        "links": {
            "current_page": 1,
            "from": 0,
            "last_page": 1,
            "per_page": 10,
            "to": 10,
            "total": 1
        },
        "data": [{
            "type": "book",
            "id": "29d11cb3-3fad-4972-bf3b-9cfc6da9e6a6",
            "attributes": {
                "__type": "book",
                "confirmed": 0,
                "created_at": "2018-04-05 15:47:29",
                "title": "book title",
                "name": "book name",
                "permission": 127127127,
                "reference_id": "29d11cb3-3fad-4972-bf3b-9cfc6da9e6a6",
                "updated_at": null,
                "user_id": "696c98d3-3b8b-41da-a510-08e6948cf661"
            },
            "relationships": {
                "author_id": {
                    "links": {
                        "related": "/api/book/<book-id/author_id",
                        "self": "/api/book/<book-id>/relationships/author_id"
                    },
                    "data": []
                }
            }
        }]
    }
    ```

### Examples

#### Curl example

    curl '/api/<entityName>?sort=&page[number]=1&page[size]=10' \
      -H 'Authorization: Bearer <AccessToken>'


#### jQuery ajax example

    $.ajax({
        method: "GET",
        url: '/api/<entityName>?sort=&page[number]=1&page[size]=10',
        success: function(response){
            console.log(response.data);
        }
      })



#### Node js example

    var request = require('request');

    var headers = {
        'Authorization': 'Bearer <AccessToken>'
    };

    var options = {
        url: '/api/<entityName>?sort=&page[number]=1&page[size]=10',
        headers: headers
    };

    function callback(error, response, body) {
        if (!error && response.statusCode == 200) {
            console.log(body);
        }
    }

    request(options, callback);



#### Python example

    import requests

    headers = {
        'Authorization': 'Bearer <AccessToken>',
    }

    params = (
        ('sort', '-created_at'),
        ('page[number]', '1'),
        ('page[size]', '10'),
    )

    response = requests.get('http://localhost:6336/api/laptop', headers=headers, params=params)



#### PHP example


    <?php
    include('vendor/rmccue/requests/library/Requests.php');
    Requests::register_autoloader();
    $headers = array(
        'Authorization' => 'Bearer <AccessToken>'
    );
    $response = Requests::get('http://localhost:6336/api/laptop?sort=&page[number]=1&page[size]=10', $headers);

### Filtering

Used to search items in a table that matche the filter's conditions. Filters follow the syntax `query=[{"column": "<column_name>", "operator": "<compare-operator>", "value":"<value>"}]`

| Daptin operator|  SQL compare operator  |
|----------------|------------------------|
| contains       |  like  '%\<value>'     |
| not contains   |  not like  '%\<value>' |
| is             |  =                     |
| is not         |  !=                    |
| before         |  <                     |
| less then      |  <                     |
| after          |  >                     |
|  more then     |  >                     |
|  any of        |  in                    |
|  none of       |  not in                |
|  is empty      |  is null               |
|  is not empty  |  is not null           |

#### Example

    curl '/api/world?query=[{"column": "is_hidden", "operator": "any of", "value":"1,0"}] \
      -H 'Authorization: Bearer <AccessToken>'


## Create

!!! note "Curl Example"
     ```curl
     curl '/api/<EntityName>'
         -H 'Authorization: Bearer <Token>'
         --data-binary '{
                     "data": {
                         "type": "<EntityName>",
                         "attributes": {
                             "name": "name"
                         }
                     }
                }'
     ```


!!! note "Nodejs example"
     ```nodejs
     var request = require('request');

     var headers = {
         'Authorization': 'Bearer <Token>',
     };

     var dataString = '{
                         "data": {
                             "type": "<EntityName>",
                             "attributes": {
                                 "name": "name"
                             }
                         }
                       }';

     var options = {
         url: '/api/<EntityName>',
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


!!! note "Python example"
     ```python
     import requests

     headers = {
         'Authorization': 'Bearer <Token>',
     }

     data = '{
                 "data": {
                     "type": "<EntityName>",
                     "attributes": {
                         "name": "name"
                     }
                 }
             }'

     response = requests.post('/api/<EntityName>', headers=headers, data=data)
     ```


!!! note "PHP Example"
     ```php
     <?php
     include('vendor/rmccue/requests/library/Requests.php');
     Requests::register_autoloader();
     $headers = array(
         'Authorization' => 'Bearer <Token>',
     );
     $data = '{
                 "data": {
                     "type": "<EntityName>",
                     "attributes": {
                         "name": "name"
                     }
                 }
              }';
     $response = Requests::post('/api/<EntityName>', $headers, $data);
     ```


## Update

!!! note "Curl example"
    ```curl
    curl '/api/<EntityName>/<ReferenceId>' \
    -X PATCH \
    -H 'Authorization: Bearer <Token>' \
    --data-binary '{
                    "data": {
                        "type": "<EntityName>",
                        "attributes": {
                            "confirmed": false,
                            "email": "update@gmail.com",
                            "name": "new name",
                            "password": ""
                        },
                        "id": "<ReferenceId>"
                    }
                  }'
    ```

!!! note "Nodejs example"
    ```nodejs
    var request = require('request');

    var headers = {
        'Authorization': 'Bearer <Token>'
    };

    var dataString = '{
                      "data": {
                          "type": "<EntityName>",
                          "attributes": {
                              "confirmed": false,
                              "email": "update@gmail.com",
                              "name": "new name",
                              "password": "",
                              "permission": 127127127,
                          },
                          "relationships": {
                              "relation_name": [ ... ]
                          },
                          "id": "<ReferenceId>"
                      }
                    }';

    var options = {
        url: '/api/<EntityName>/<ReferenceId>',
        method: 'PATCH',
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



!!! note "Python example"
    ```python
    import requests

    headers = {
        'Authorization': 'Bearer <Token>',
    }

    data = '{
              "data": {
                  "type": "<EntityName>",
                  "attributes": {
                      "confirmed": false,
                      "email": "update@gmail.com",
                      "name": "new name",
                      "password": "",
                      "permission": 127127127,
                  },
                  "relationships": {
                      "relation_name": [ ... ]
                  },
                  "id": "<ReferenceId>"
              }
            }'

    response = requests.patch('/api/<EntityName>/<ReferenceId>', headers=headers, data=data)
    ```


!!! note "PHP example"
    ```php
    <?php
    include('vendor/rmccue/requests/library/Requests.php');
    Requests::register_autoloader();
    $headers = array(
        'Authorization' => 'Bearer <Token>'
    );
    $data = '{
               "data": {
                   "type": "<EntityName>",
                   "attributes": {
                       "confirmed": false,
                       "email": "update@gmail.com",
                       "name": "new name",
                       "password": "",
                       "permission": 127127127,
                   },
                   "relationships": {
                       "relation_name": [ ... ]
                   },
                   "id": "<ReferenceId>"
               }
             }';
    $response = Requests::patch('/api/<EntityName>/<ReferenceId>', $headers, $data);
    ```

## Delete

Delete a row from a table

!!! note "Curl example"
    ```bash
    curl '/api/user_account/a5b9add2-ea56-4717-a785-7dee71a2ae46' -X DELETE  -H 'Authorization: Bearer <Token>'
    ```

!!! note "Nodejs example"
    ```nodejs
    var request = require('request');

    var headers = {
        'Authorization': 'Bearer <Token>'
    };

    var options = {
        url: '/api/user_account/a5b9add2-ea56-4717-a785-7dee71a2ae46',
        method: 'DELETE',
        headers: headers
    };

    function callback(error, response, body) {
        if (!error && response.statusCode == 200) {
            console.log(body);
        }
    }

    request(options, callback);
    ```


!!! note "Python example"
    ```python
    import requests

    headers = {
        'Authorization': 'Bearer <Token>',
    }

    response = requests.delete('/api/user_account/a5b9add2-ea56-4717-a785-7dee71a2ae46', headers=headers)
    ```

!!! note "PHP example"
    ```php
    <?php
    include('vendor/rmccue/requests/library/Requests.php');
    Requests::register_autoloader();
    $headers = array(
        'Authorization' => 'Bearer <Token>'
    );
    $response = Requests::delete('/api/user_account/a5b9add2-ea56-4717-a785-7dee71a2ae46', $headers);
    ```

## Execute

Execute an action on an entity type or instance

!!! note "Curl example"
    ```curl
    curl '/action/<EntityName>/<ActionName>'  -H 'Authorization: Bearer <Token>'  --data-binary '{"attributes":{}}'
    ```


!!! note "PHP Example"
    ```php
    <?php
    include('vendor/rmccue/requests/library/Requests.php');
    Requests::register_autoloader();
    $headers = array(
        'Authorization' => 'Bearer <Token>'
    );
    $data = '{"attributes":{}}';
    $response = Requests::post('/action/<EntityName>/<ActionName>', $headers, $data);
    ```

!!! note "Nodejs example"
    ```nodejs
    var request = require('request');

    var headers = {
        'Authorization': 'Bearer <Token>'
    };

    var dataString = '{"attributes":{}}';

    var options = {
        url: '/action/<EntityName>/<ActionName>',
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

!!! note "Python example"
    ```python
    import requests

    headers = {
        'Authorization': 'Bearer <Token>',
    }

    data = '{"attributes":{}}'

    response = requests.post('/action/<EntityName>/<ActionName>', headers=headers, data=data)
    ```

## Relations

!!! note "curl example"
    ```bash
    curl '/api/<EntityName>/<ReferenceId>/<RelationName>?sort=&page[number]=1&page[size]=10'  -H 'Authorization: Bearer <Token>'
    ```

!!! note "php example"
    ```php
    <?php
    include('vendor/rmccue/requests/library/Requests.php');
    Requests::register_autoloader();
    $headers = array(
        'Authorization' => 'Bearer <Token>'
    );
    ```



!!! note "python example"
    ```python
    import requests

    headers = {
        'Authorization': 'Bearer <Token>',
    }

    params = (
        ('sort', ''),
        ('page/[number/]', '1'),
        ('page/[size/]', '10'),
    )

    response = requests.get('http://localhost:6336/api/user_account/696c98d3-3b8b-41da-a510-08e6948cf661/marketplace_id', headers=headers, params=params)

    #NB. Original query string below. It seems impossible to parse and
    #reproduce query strings 100% accurately so the one below is given
    #in case the reproduced version is not "correct".
    # response = requests.get('http://localhost:6336/api/user_account/696c98d3-3b8b-41da-a510-08e6948cf661/marketplace_id?sort=&page\[number\]=1&page\[size\]=10', headers=headers)

    ```

!!! note "nodejs example"
    ```nodejs
    var request = require('request');

    var headers = {
        'Authorization': 'Bearer <Token>'
    };

    var options = {
        url: '/api/<EntityName>/<ReferenceId>/<RelationName>?sort=&page[number]=1&page[size]=10',
        headers: headers
    };

    function callback(error, response, body) {
        if (!error && response.statusCode == 200) {
            console.log(body);
        }
    }

    request(options, callback);
    ```

!!! note "python example"
    ```python
    import requests

    headers = {
        'Authorization': 'Bearer <Token>',
    }

    params = (
        ('sort', ''),
        ('page[number]', '1'),
        ('page[size]', '10'),
    )

    response = requests.get('/api/<EntityName>/<ReferenceId>/<RelationName>', headers=headers, params=params)

    ```