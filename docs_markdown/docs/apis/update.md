# Update

!!! note "Curl example"
    ```curl
    curl '/api/<EntityName>/<ReferenceId>'
    -X PATCH
    -H 'Authorization: Bearer <Token>' --data-binary '{
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

