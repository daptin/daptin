
### Sign In

Sign In is an [action](actions/actions) on user entity. Sign in takes two inputs:

- Email
- Password

When the user initiates Sign in action, the following things happen:

- Check if guests can peek users table (Peek permission)
- Check if guests can peek the particular user (Peek Permission)
- Match if the provided password bcrypted matches the stored bcrypted password
- If true, issue a JWT token, which is used for future calls

The main outcome of the Sign In action is the jwt token, which is to be used in the ```Authorization``` header of following calls.


#### Sign in CURL example

!!! example"POST call for sign in"
```bash
curl 'http://localhost:6336/action/user_account/signin' \
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
