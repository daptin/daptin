# User signup/registration API

Sign up action can be allowed to guests to allow open registration by anyone. Users with enough permission over the `user` table can create users manually.

Users registered using signup action are their own owners. Hence they can update and delete themselves. These permission can be changed based on the use case.

!!! note "POST call for user registration"
    ```bash
    curl 'http://api.daptin.com:6336/action/user/signup' \
    -H 'Authorization: Bearer null' \
    -H 'Content-Type: application/json;charset=UTF-8' \
    -H 'Accept: application/json, text/plain, */*' \
    --data-binary '{"attributes":{"name":"username","email":"<UserEmail>","password":"<Password>","passwordConfirm":"<Password>"}}'
    ```

You can either allow guests to be able to invoke `sign up` action or allow only a particular user to be able to create new users or a usergroup.

```json
[
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "message": "Created user",
      "title": "Success",
      "type": "success"
    }
  }
]
```

This user can sign in now (generate an auth token). But what he can access is again based on the permission of the system.
