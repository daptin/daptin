## Accessing web dashboard

Open up the dashboard on http://localhost:8080/

You will be presented with the Sign-in screen. If you are on a freshly created instance, then you need to create a user first.

## First user

Use the dashboard to sign-up as the first user or call the sign-up API manually to create the first user.

<img src="/images/signup.png" width="300px">

## Logging in dashboard

<img src="/images/signin.png" width="300px">

## Become Administrator

On the main screen of the dashboard under "Users" heading, locate the "Become admin" button.

<img src="/images/users_and_groups.png" width="600px">

Clicking this will make the following changes:

- Disallow the sign-up API for guests
- Disallow the sign-in API for guests
- Makes you the owner of all the data

## Enable sign-up

Enable sign-up action by navigating to:

You need to change two settings to allow guests to signup (after becoming admin)

Since the "Sign in" action is defined on user_account entity, you need to allow guests to execute

Dashboard -> All tables -> Search "User" and locate the "User account" entity -> Edit -> Permissions -> Guests -> Check "Execute strict"

<img src="/images/execute.png" width="600px">


Also allow guests to execute the sign-up action itself

Dashboard -> Actions -> Search "Signup" -> Edit -> Permissions -> Guest -> Check "Execute strict"

