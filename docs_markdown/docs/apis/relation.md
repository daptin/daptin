# Relations

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
    $response = Requests::get('/api/<EntityName>/<ReferenceId>/<RelationName>?sort=&page[number]=1&page[size]=10', $headers);
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