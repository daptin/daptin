# Delete

Delete a row from a table

!!! note "Curl example"
    ```bash
    curl '/api/user/a5b9add2-ea56-4717-a785-7dee71a2ae46' -X DELETE  -H 'Authorization: Bearer <Token>'
    ```

!!! note "Nodejs example"
    ```nodejs
    var request = require('request');

    var headers = {
        'Authorization': 'Bearer <Token>'
    };
    
    var options = {
        url: '/api/user/a5b9add2-ea56-4717-a785-7dee71a2ae46',
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

    response = requests.delete('/api/user/a5b9add2-ea56-4717-a785-7dee71a2ae46', headers=headers)
    ```

!!! note "PHP example"
    ```php
    <?php
    include('vendor/rmccue/requests/library/Requests.php');
    Requests::register_autoloader();
    $headers = array(
        'Authorization' => 'Bearer <Token>'
    );
    $response = Requests::delete('/api/user/a5b9add2-ea56-4717-a785-7dee71a2ae46', $headers);
    ```