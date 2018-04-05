# User sign in API

!!! note "POST call for sign in"
    ```bash
    curl 'http://api.daptin.com:6336/action/user/signin' \
    -H 'Content-Type: application/json;charset=UTF-8' \
    -H 'Accept: application/json, text/plain, */*' \
    --data-binary '{"attributes":{"email":"<Email>","password":"<Password>"}}'
    ```

```json
[
  {
    "ResponseType": "client.store.set",
    "Attributes": {
      "key": "token",
      "value": "<AccessToken>"
    }
  },
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "message": "Logged in",
      "title": "Success",
      "type": "success"
    }
  },
  {
    "ResponseType": "client.redirect",
    "Attributes": {
      "delay": 2000,
      "location": "/",
      "window": "self"
    }
  }
]
```
