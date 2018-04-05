# Users

Daptin has a built-in user system. Users are identified by their authorization token or other means of identification. Users are idenfied as registered users or guests.

You can choose to disable new user registration by changing the `signup` action permissions.

## API

Users are just like any other data you maintain. Users information is stored in the `user` table and exposed over ```/api/user``` endpoint.


You can choose to allow ```read/write``` permission directly to that ```table``` to allow other users/processes to use this api to ```read/create/update/delete``` users.

## CURL

```
curl '/api/user' \
  -H 'Authorization: Bearer <Auth Token>' \
  --data-binary '{
                    "data": {
                        "type": "user",
                        "attributes": {
                            "email": "test@user.com",
                            "name": "test",
                            "password": "password",
                        }
                    }
                 }'
```

## Node JS

```
import requests

headers = {
    'Authorization': 'Bearer <Auth Token>',
}

data = '{
        	"data": {
        		"type": "user",
        		"attributes": {
        			"email": "test@user.com",
        			"name": "test",
        			"password": "password",
        		}
        	}
        }'

response = requests.post('http://api.daptin.com:6336/api/user', headers=headers, data=data)

```
You can manually add users from the users page, or allow sign-up action to be performed by guests which will take care of creating a user and an associated usergroup for that user. All new signed up users will also be added to the "users" usergroup.

## Using Dashboard

You can create a new user from the sign-up page on a new instance and later make that page available to guests.
