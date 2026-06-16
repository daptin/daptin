## Sign Up Action

Sign up is an [action](/actions/actions) on user entity. Sign up takes four inputs:

- Name
- Email
- Password
- PasswordConfirm

When the user initiates a Sign up action, the following things happen

- Check if guests can initiate sign in action
- Check if guests can create a new user (create permission)
- Create a new user row
- Check if guests can create a new usergroup (create permission)
- Create a new usergroup row
- Associate the user to the usergroup (refer permission)

This means that every user has his own dedicated user group by default.

## Curl Example


```bash
curl 'http://localhost:6336/action/user_account/signup' -H 'Content-Type: application/json;charset=utf-8'
--data-raw '{"attributes":{"email":"email@host.com","password":"password","name":"name","passwordConfirm":"password"}}'
```


## Sign Up Action Permissions

First you need to fetch the available actions

```curl -H "Authorization: Bearer <token>" 'http://localhost:6336/api/action' | python -m json.tool```

More specifically you are looking for the signup action

```curl -H "Authorization: Bearer <token>" 'http://localhost:6336/api/action?filter=signup' | python -m json.tool```

After `become_an_administrator`, Daptin locks `signup` down and re-opens only `signin`.

If you want to re-enable public signup after admin setup, patch the `signup` action permission by adding `GuestExecute` (`32`) to the current post-admin value.

Note the reference id of the signup action in the response, we need it to update its permission.

```bash
curl 'http://localhost:6336/api/action/<reference_id>' \
-X PATCH \
-H 'Content-Type: application/vnd.api+json' \
-H 'Authorization: Bearer <token>' \
--data-raw '{"data":{"type":"action","attributes":{"permission":"2085152"},"id":"<reference_id>"},"meta":{}}'
```

Note that the `reference_id` is in two places there.

#### Permission

- Post-admin locked `signup`: `2085120`
- Re-enable public signup: `2085152` (`2085120 + GuestExecute`)
- Generic public-action profile: `561441`

`561441` works for public actions in general, but `signup` does not need to be reset to that broader profile after admin setup.

## New user from CRUD API

Users can be created by directly create an entry in the `user_account` table.

Creating a user manually

```
curl '/api/user_account' \
  -H 'Authorization: Bearer <Auth Token>' \
  --data-binary '{
                    "data": {
                        "type": "user_account",
                        "attributes": {
                            "email": "test@user.com",
                            "name": "test",
                            "password": "password",
                        }
                    }
                 }'
```
