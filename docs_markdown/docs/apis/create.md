# Create API


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