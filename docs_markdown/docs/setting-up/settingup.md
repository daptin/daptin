# Getting started

## Accessing web dashboard

Open up the dashboard on http://localhost:8080/

You will be presented with the Sign-in screen. If you are on a freshly created instance, then you need to create a user first.

### First user

Use the dashboard to sign-up as the first user or call the sign-up API manually to create the first user.

<img src="/images/signup.png" width="300px">

API CALL

As you will see later in [actions](/actions/actions.md) sign up and sign in api's are nothing special but just actions defined on certain tables.

Request 

```bash
curl 'http://localhost/action/user_account/signup' 
--data '{"attributes":{"name":"name","email":"email@domain.com","password":"password123","passwordConfirm":"password123"}}'
```

Response

```json
[{
	"ResponseType": "client.notify",
	"Attributes": {
		"message": "Created user_account",
		"title": "Success",
		"type": "success"
	}
}, {
	"ResponseType": "client.notify",
	"Attributes": {
		"message": "Created usergroup",
		"title": "Success",
		"type": "success"
	}
}, {
	"ResponseType": "client.notify",
	"Attributes": {
		"message": "Created user_account_user_account_id_has_usergroup_usergroup_id",
		"title": "Success",
		"type": "success"
	}
}, {
	"ResponseType": "client.notify",
	"Attributes": {
		"__type": "client.notify",
		"message": "Sign-up successful. Redirecting to sign in",
		"title": "Success",
		"type": "success"
	}
}, {
	"ResponseType": "client.redirect",
	"Attributes": {
		"__type": "client.redirect",
		"delay": 2000,
		"location": "/auth/signin",
		"window": "self"
	}
}]
```

Nothing important in the response of signup to keep track of. 

Successful response means now we can login as a user and become the administrator.

A failure response would look like this:

```json
[{
	"ResponseType": "client.notify",
	"Attributes": {
		"message": "Failed to create user_account. Error 1062: Duplicate entry 'email@domain.com' for key 'i79f4e12e72442d30f2b99a84fce3c392'",
		"title": "Failed",
		"type": "error"
	}
}]
```

Or 

```json
[{
	"ResponseType": "client.notify",
	"Attributes": {
		"message": "http error (400) email and 0 more errors, invalid value for email",
		"title": "failed",
		"type": "error"
	}
}]
```


### Logging in dashboard

<img src="/images/signin.png" width="300px">

API CAll

Request

```bash
curl 'http://localhost/action/user_account/signin' 
--data '{"attributes":{"email":"email@domain.com","password":"password123"}}'
```

Response

```json
[{
	"ResponseType": "client.store.set",
	"Attributes": {
		"key": "token",
		"value": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImFydHBhckBnbWFpbC5jb20iLCJleHAiOjE1ODE2MTcxNTEsImlhdCI6IjIwMjAtMDItMTBUMjM6MzU6NTEuMTc2MjA5ODAxKzA1OjMwIiwiaXNzIjoiZGFwdGluLTNhZTI5ZCIsImp0aSI6IjQ4MTRkYjhhLTg1ZWEtNDc0ZS1iMWQ0LWQ5OGM4MTU5ZDU5MCIsIm5hbWUiOiJwYXJ0aCIsIm5iZiI6MTU4MTM1Nzk1MSwicGljdHVyZSI6Imh0dHBzOi8vd3d3LmdyYXZhdGFyLmNvbS9hdmF0YXIvM2M5MjI3NmI4NmMzNGJkNjZmZjQwMzFlNjNmM2JkZTdcdTAwMjZkPW1vbnN0ZXJpZCJ9.deocIlHXWH_2fsrYBx5lSGQVJxad044tj4j4amy2Zyk"
	}
}, {
	"ResponseType": "client.cookie.set",
	"Attributes": {
		"key": "token",
		"value": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImFydHBhckBnbWFpbC5jb20iLCJleHAiOjE1ODE2MTcxNTEsImlhdCI6IjIwMjAtMDItMTBUMjM6MzU6NTEuMTc2MjA5ODAxKzA1OjMwIiwiaXNzIjoiZGFwdGluLTNhZTI5ZCIsImp0aSI6IjQ4MTRkYjhhLTg1ZWEtNDc0ZS1iMWQ0LWQ5OGM4MTU5ZDU5MCIsIm5hbWUiOiJwYXJ0aCIsIm5iZiI6MTU4MTM1Nzk1MSwicGljdHVyZSI6Imh0dHBzOi8vd3d3LmdyYXZhdGFyLmNvbS9hdmF0YXIvM2M5MjI3NmI4NmMzNGJkNjZmZjQwMzFlNjNmM2JkZTdcdTAwMjZkPW1vbnN0ZXJpZCJ9.deocIlHXWH_2fsrYBx5lSGQVJxad044tj4j4amy2Zyk"
	}
}, {
	"ResponseType": "client.notify",
	"Attributes": {
		"message": "Logged in",
		"title": "Success",
		"type": "success"
	}
}, {
	"ResponseType": "client.redirect",
	"Attributes": {
		"delay": 2000,
		"location": "/",
		"window": "self"
	}
}]
```

The token is to be used in the Authorization header of for all HTTP calls to identify the user.

## Become Administrator

On the main screen of the dashboard under "Users" heading, locate the "Become admin" button.

<img src="/images/users_and_groups.png" width="600px">

Clicking this will make the following changes:

- Disallow the sign-up API for guests
- Makes you the owner of all the data


