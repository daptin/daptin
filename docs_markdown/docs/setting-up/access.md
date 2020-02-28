# User management


<img src="/images/users_and_groups.png">

Daptin maintains its own ```User accounts``` and ```User groups``` entries in the database. Users are identified by ```email``` which is a unique key in the ```user_account``` entity. Passwords are stored using bcrypt with a cost of 11. Password field has a column_type ```password``` which makes daptin to bcrypt it before storing, and password fields are never returned in any JSONAPI call.

## Authentication

Authentication involves identifying the current user of the request. Daptin expectes a JWT token issued at signin as ```Authorization: Bearer <Token>``` header, otherwise the request is considered coming from a [guest](#Guests).

### Sign Up

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



#### Signup API

Sign up action can be allowed to guests to allow open registration by anyone. Users with enough permission over the `user_account` table can create users manually.

Users registered using signup action are their own owners. Hence they can update and delete themselves. These permission can be changed based on the use case.

!!! note "POST call for user registration"
    ```bash
    curl 'http://localhost:6336/action/user_account/signup' \
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


#### Signup CURL example

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


### Sign In

[Sign In is also an action](/actions/actions) on user entity. Sign in takes two inputs:

- Email
- Password

When the user initiates Sign in action, the following things happen:

- Check if guests can peek users table (Peek permission)
- Check if guests can peek the particular user (Peek Permission)
- Match if the provided password bcrypted matches the stored bcrypted password
- If true, issue a JWT token, which is used for future calls

The main outcome of the Sign In action is the jwt token, which is to be used in the ```Authorization``` header of following calls.


#### Sign in CURL example

!!! note "POST call for sign in"
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


#### Directly into user_account table


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

response = requests.post('http://localhost:6336/api/user', headers=headers, data=data)

```


You can manually add users from the users page, or allow sign-up action to be performed by guests which will take care of creating a user and an associated usergroup for that user. All new signed up users will also be added to the "users" usergroup.




### Guests

Requests **without** a valid `Authorization Bearer` `token` will be referred to as "guests requests". Requests with a valid token will have an identified user in the context.

## Authorization

Daptin has a built-in authorization framework based on users groups and permissions. Users are identified by their authorization token or other means of identification. Each request is identified as coming from a registered user or a guest.


### Permission model

Every read/write to the system passes through two level of permission check.

- Type level: apply permission on all types of entities at the same time
- Data level: object level permission


The `world` table contains two columns:

- `Permission`: defines the entity level permission
- `Default permission`: defines the default permission for a new object of this entity type

The default permission for an object is picked from the default permission setting, and can be changed after the object creation (if the permission allows).

#### Peek

**Peek** gives access to the user to read data in the system but not allow it in response as data. So while the query to read the data will execute and certain **actions** can be allowed over them, directly trying to read the data in response will fail.

#### [C] Create

**Create** allows a new row to be created by using the POST api. Note: this doesn't apply over indirect creations using *actions**.

#### [R] Read

**Read** allows the data to be served in the http response body. The response will usually follow the JSONAPI.org structure.

#### [U] Update

**Update** allows the data fields to be updated using the PUT/PATCH http methods.

#### [D] Delete

**Delete** gives permission to be delete a row or certain type of data using DELETE http method. Unless you have enabled **auditing**, you will permanently loose this data.

#### [R] Refer

**Refer** gives permission to add data/users to usergroups. Note that you will also need certain permission on the **usergroup** as well.

#### [X] Execute

**Execute** gives permission to invoke action over data (like export). Note that giving access to a **type of data** doesn't give access to all rows of that **entity type**.

### Authorization

Authorization is the part where daptin decides if the caller has enough permission to execute the call. Access check happens at two levels:

- Entity level check
- Object level check

Both the checks have a "before" and "after" part.


#### Object level permission check

Once the call clears the entity level check, an object level permission check is applied. This happens in cases where the action is going to affect/read an existing row. The permission is stored in the same way. Each table has a permission column which stores the permission in ```OOOGGGXXX``` format.

#### Order of permission check

The permission is checked in order of:

- Check if the user is owner, if yes, check if permission allows the current action, if yes do action
- Check if the user belongs to a group to which this object also belongs, if yes, check if permisison allows the current action, if yes do action
- User is guest, check if guest permission allows this actions, if yes do action, if no, unauthorized

Things to note here:

- There is no negative permission (this may be introduced in the future)
  - eg, you cannot say owner is 'not allowed' to read but read by guest is allowed.
- Permission check is done in a hierarchy type order

#### Access flow

Every "interaction" in daptin goes through two levels of access. Each level has a ```before``` and ```after``` check.

- Entity level access: does the user invoking the interaction has the appropriate permission to invoke this (So for sign up, the user table need to be writable by guests, for sign in the user table needs to be peakable by guests)
- Instance level access: this is the second level, even if a User Account has access to "user" entity, not all "user" rows would be accessible by them


So the actual checks happen in following order:

- "Before check" for entity
- "Before check" for instance
- "After check" for instance
- "After check" for entity

Each of these checks can filter out objects where the user does not have enough permission.

#### Entity level permission

Entity level permission are set in the world table and can be updated from dashboard. This can be done by updating the "permission" column for the entity.

For these changes to take effect a restart is necessary.

#### Instance level permission

Like we saw in the [entity documentation](/setting-up/entities), every table has a ```permission``` column. No restart is necessary for changes in these permission.


You can choose to disable new user registration by changing the `signup` action permissions.

### User data API Examples


Users are just like any other data you maintain. User information is stored in the `user_account` table and exposed over ```/api/user_account``` endpoint.


You can choose to allow ```read/write``` permission directly to that ```table``` to allow other users/processes to use this api to ```read/create/update/delete``` users.

## User groups

User groups is a group concept that helps you manage "who" can interact with daptin, and in what ways.

All objects (including users and groups) belong to one or more user group.

Users can interact with objects which also belong to their group based on the defined group permission setting

## Social login

Oauth connection can be used to allow guests to identify themselves based on the email provided by the oauth id provider.

### Social login


Allow users to login using their existing social accounts like twitter/google/github.

Daptin can work with any oauth flow aware identity provider to allow new users to be registered (if you have disabled normal signup).

Create a [OAuth Connection](/extend/oauth_connection) and mark "Allow login" to enable APIs for OAuth flow.

Examples

!!! note "Google login configuration"
    ![Google oauth](/images/oauth/google.png)

!!! note "Dropbox login configuration"
    ![Google oauth](/images/oauth/dropbox.png)

!!! note "Github login configuration"
    ![Google oauth](/images/oauth/github.png)

!!! note "Linkedin login configuration"
    ![Google oauth](/images/oauth/linkedin.png)


!!! note "Encrypted values"
    The secrets are stored after encryption so the value you see in above screenshots are encrypted values.



## Configuring default user group

You can configure which User groups should newly registered users be added to after their signup.

This can be configured in the table properties from the dashboard or by updating the entity configuration from the API

!!!note "Restart required"
    Restart is required for default group settings to take effect


### Authentication token

The authentication token is a JWT token issued by daptin on sign in action. Users can create new actions to allow other means of generating JWT token. It is as simple as adding another outcome to an action.

#### Server side

Daptin uses OAuth 2 based authentication strategy. HTTP calls are checked for ```Authorization``` header, and if present, validated as a JWT token. The JWT token should have been issued by daptin earlier and should not have expired. To see how to generate JWT token, checkout the [sing-in action](/actions/signin).

The JWT token contains the issuer information (daptin) plus basic user profile (email). The JWT token has a one hour (configurable) expiry from the time of issue.

If the token is absent or invalid, the user is considered as a guest. Guests also have certain permissions. Checkout the [Authorization docs](/auth/authorization) for details.

#### Client side

On the client side, for dashboard, the token is stored in local storage. The local storage is cleared on logout or if the server responds with a 401 Unauthorized status.


