# Authentication

Daptin maintains its own user accounts and usergroups as well. Users are identified by ```email``` which is a unique key in the ```user``` entity. Passwords are stored using bcrypt with a cost of 11. Password field has a column_type ```password``` which makes daptin to bcrypt it before storing, and password fields are never returned in any JSONAPI call.


## Authentication token

The authentication token is a JWT token issued by daptin on sign in action. Users can create new actions to allow other means of generating JWT token. It is as simple as adding another outcome to an action.

## Server side

Daptin uses oAuth2 based authentication strategy. HTTP calls are checked for ```Authorization``` header, and if present, validated as a JWT token. The JWT token should have been issued by daptin earlier and should not have expired. To see how to generate JWT token, checkout the [sing-in action](/actions/signin.md).

The JWT token contains the issuer information (daptin) plus basic user profile (email). The JWT token has a one hour (configurable) expiry from the time of issue.

If the token is absent or invalid, the user is considered as a guest. Guests also have certain permissions. Checkout the [Authorization docs](/auth/authorization.md) for details.

## Client side

On the client side, for dashboard, the token is stored in local storage. The local storage is cleared on logout or if the server responds with a 401 Unauthorized status.

## Authentication using other systems

There is planned road map to allow user logins via external oauth2 servers as well (login via google/facebook/twitter... and so on). This feature is not complete yet. Documentation will be updated to reflect changes.

## Sign Up

Sign up is an action on user entity. Sign up takes four inputs:

- Name
- Email
- Password
- PasswordConfirm

When the user initates a Sign up action, the following things happen

- Check if guests can initiate sign in action
- Check if guests can create a new user (create permission)
- Create a new user row
- Check if guests can create a new usergroup (create permission)
- Create a new usergroup row
- Associate the user to the usergroup (refer permission)

This means that every user has his own dedicated usergrou by default. 

## Sign In

Sign In is also an action on user entity. Sign in takes two inputs:

- Email
- Password

When the user initiates Sign in action, the following things happen:

- Check if guests can peek users table (Peek permission)
- Check if guests can peek the particular user (Peek Permission)
- Match if the provided password bcrypted matches the stored bcrypted password
- If true, issue a JWT token, which is used for future calls

The main outcome of the Sign In action is the jwt token, which is to be used in the ```Authorization``` header of following calls.