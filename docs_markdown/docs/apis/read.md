# GET /api/&lt;entityName&gt;


## Parameters

| Name               |  parameter type          |  default value |  example value                                            |
|--------------------|--------------------------|----------------|-----------------------------------------------------------|
| page[number]       |  integer                 |  1             |  5                                                        |
| page[size]         |  integer                 |  10            |  100                                                      |
| query              |  json base64             |  []            |  [{'column': 'name' 'operator': 'eq' 'value': 'england'}] |
| group              |  string                  |  -             |  [{'column': 'name' 'order': 'desc'}]                     |
| included_relations |  comma separated string  |  -             |  user post author                                         |
| sort               |  comma seaparated string |  -             |  created_at amount guest_count                            |
| filter             |  string                  |  -             |  england                                                  |


# Response

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

# Examples

## Curl example

    curl '/api/<entityName>?sort=&page[number]=1&page[size]=10' \
      -H 'Authorization: Bearer <AccessToken>'


## jQuery ajax example

    $.ajax({
        method: "GET",
        url: '/api/<entityName>?sort=&page[number]=1&page[size]=10',
        success: function(response){
            console.log(response.data);
        }
      })



## Node js example

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



## Python example

    import requests

    headers = {
        'Authorization': 'Bearer <AccessToken>',
    }

    params = (
        ('sort', '-created_at'),
        ('page[number]', '1'),
        ('page[size]', '10'),
    )

    response = requests.get('http://api.daptin.com:6336/api/laptop', headers=headers, params=params)



## PHP example


    <?php
    include('vendor/rmccue/requests/library/Requests.php');
    Requests::register_autoloader();
    $headers = array(
        'Authorization' => 'Bearer <AccessToken>'
    );
    $response = Requests::get('http://api.daptin.com:6336/api/laptop?sort=&page[number]=1&page[size]=10', $headers);


