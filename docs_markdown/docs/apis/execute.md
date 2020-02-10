# Execute

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