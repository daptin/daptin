## Sign Up Action

[Sign up is an action](/actions/actions) on user entity. Sign up takes four inputs:

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
