# Guests

<img src="images/users_and_groups.png">

Requests **without** a valid `Authorization Bearer` `token` will be referred to as "guests requests". Requests with a valid token will have an identified user in the context.


## Sign-up

Guests can be given permission to execute signup action and so allowing them to register themselves.

## Social login

Oauth connection can be used to allow guests to identify themselves based on the email provided by the oauth id provider.

[Checkout sample configurations here](/auth/social_login.md)

## Auto add new users to groups

You can configure which usergroups should newly registered users be added to after their signup.

This can be configured in the table properties from the dashboard or by updating the entity configuration from the API