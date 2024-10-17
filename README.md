
<h1 align="left">
  daptin
  <br>
</h1>

<p align="center">
    <a href="https://travis-ci.org/daptin/daptin"><img alt="Travis" src="https://img.shields.io/travis/daptin/daptin.svg?style=flat-square"></a>
    <a href='https://semaphoreci.com/artpar/daptin'> <img src='https://semaphoreci.com/api/v1/artpar/daptin/branches/master/badge.svg' alt='Build Status'></a>
    <a href='https://circleci.com/gh/daptin/daptin'> <img src='https://circleci.com/gh/daptin/daptin.svg?style=svg' alt='Build Status'></a>	
<p align="center">
    <a href="/LICENSE"><img alt="Software License" src="https://img.shields.io/badge/LICENSE-LGPL%20v3-brightgreen.svg?style=flat-square"></a>
    <a href="https://goreportcard.com/report/github.com/daptin/daptin"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/daptin/daptin?style=flat-square"></a>
    <a href="http://godoc.org/github.com/daptin/daptin"><img alt="Go Doc" src="https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square"></a>
</p>
<p align="center">
    <a href="https://codecov.io/gh/daptin/daptin"><img alt="Codecov branch" src="https://img.shields.io/codecov/c/github/daptin/daptin/master.svg?style=flat-square"></a>
    <a href="https://github.com/daptin/daptin/releases/latest"><img alt="Release" src="https://img.shields.io/github/release/daptin/daptin.svg?style=flat-square"></a>
</p>
<p align="center">
     <a href="https://discord.gg/t564q8SQVk"><img src="https://img.shields.io/badge/JOIN-ON%20DISCORD-blue&?style=for-the-badge&logo=discord"></a> 
</p>


<p align="center">
	<a href="https://github.com/daptin/daptin/releases">Download</a> •
	<a href="https://daptin.github.io/daptin/">Documentation</a> •
	<a href="https://join.slack.com/t/daptin/shared_invite/enQtMzM1NTM1NTkyMDgzLTVlYzBlMmM4YjMyOTk0MDc5MWJmMzFlMTliNzQwYjcxMzc5Mjk0YzEyZDIwYTljZmE5NDU3Yjk3YzQ3MzhkMzI">Community</a>
</p>


```bash
./daptin
.
. // logs truncated for brevity
.
INFO[2024-10-16 11:08:58] Listening websocket server at ... /live
INFO[2024-10-16 11:08:58] Our admin is [artpar@gmail.com]
INFO[2024-10-16 11:08:58] [ProcessId=86403] Listening at port: :6336
INFO[2024-10-16 11:08:58] Get certificate for [Parths-MacBook-Pro.local]: true
INFO[2024-10-16 11:08:58] Listening at: [:6336]
INFO[2024-10-16 11:08:58] TLS server listening on port :6443
INFO[2024-10-16 11:09:03] Member says: Message<members: Joining from 192.168.0.125:5336>
```

Server is up, sqlite database is used since we did not specify mysql or postgres.


### signup, signin, user_account and usergroup

## signup 

call the signup "action" api to create a new user_account

```bash
curl 'http://localhost:6333/action/user_account/signup' -X POST \
--data-raw '{"attributes":{"email":"artpar@gmail.com","password":"artpar@gmail.com","name":"artpar@gmail.com","passwordConfirm":"artpar@gmail.com"}}'
```

On a fresh instance all actions are allowed to be executed by guests, so you shouldn't see this

```json
[
    {
        "Attributes": {
            "message": "http error (403) forbidden and 0 more errors, forbidden",
            "title": "failed",
            "type": "error"
        },
        "ResponseType": "client.notify"
    }
]
```

You should see this

```json
[
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "__type": "client.notify",
      "message": "Sign-up successful. Redirecting to sign in",
      "title": "Success",
      "type": "success"
    }
  },
  {
    "ResponseType": "client.redirect",
    "Attributes": {
      "__type": "client.redirect",
      "delay": 2000,
      "location": "/auth/signin",
      "window": "self"
    }
  }
]
```

#### Sign in to get a JWT Bearer token

```bash
curl 'http://localhost:6336/action/user_account/signin' \
--data-raw '{"attributes":{"email":"artpar@gmail.com","password":"artpar@gmail.com"}}'

[
    {
        "Attributes": {
            "key": "token",
            "value": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImFydHBhckBnbWFpbC5jb20iLCJleHAiOjE3MjkzMjExMjIsImlhdCI6MTcyOTA2MTkyMiwiaXNzIjoiZGFwdGluLTAxOTIyOCIsImp0aSI6IjAxOTI5NDFmLTI2MGUtN2I0Ni1hMWFlLWYxMGZhZTcwMDE3OSIsIm5hbWUiOiJhcnRwYXJAZ21haWwuY29tIiwibmJmIjoxNzI5MDYxOTIyLCJzdWIiOiIwMTkyMmUxYS1kNWVhLTcxYzktYmQzZS02MTZkMjM3ODBmOTMifQ.H-GLmXCT-o7RxXrjo5Of0K8Nw5mpOOw6jgoXnd5KUxo"
        },
        "ResponseType": "client.store.set"
    },
    {
        "Attributes": {
            "key": "token",
            "value": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImFydHBhckBnbWFpbC5jb20iLCJleHAiOjE3MjkzMjExMjIsImlhdCI6MTcyOTA2MTkyMiwiaXNzIjoiZGFwdGluLTAxOTIyOCIsImp0aSI6IjAxOTI5NDFmLTI2MGUtN2I0Ni1hMWFlLWYxMGZhZTcwMDE3OSIsIm5hbWUiOiJhcnRwYXJAZ21haWwuY29tIiwibmJmIjoxNzI5MDYxOTIyLCJzdWIiOiIwMTkyMmUxYS1kNWVhLTcxYzktYmQzZS02MTZkMjM3ODBmOTMifQ.H-GLmXCT-o7RxXrjo5Of0K8Nw5mpOOw6jgoXnd5KUxo; SameSite=Strict"
        },
        "ResponseType": "client.cookie.set"
    },
    {
        "Attributes": {
            "message": "Logged in",
            "title": "Success",
            "type": "success"
        },
        "ResponseType": "client.notify"
    },
    {
        "Attributes": {
            "delay": 2000,
            "location": "/",
            "window": "self"
        },
        "ResponseType": "client.redirect"
    }
]

```

We will use

```bash
export TOKEN=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImFydHBhckBnbWFpbC5jb20iLCJleHAiOjE3MjkzMjExMjIsImlhdCI6MTcyOTA2MTkyMiwiaXNzIjoiZGFwdGluLTAxOTIyOCIsImp0aSI6IjAxOTI5NDFmLTI2MGUtN2I0Ni1hMWFlLWYxMGZhZTcwMDE3OSIsIm5hbWUiOiJhcnRwYXJAZ21haWwuY29tIiwibmJmIjoxNzI5MDYxOTIyLCJzdWIiOiIwMTkyMmUxYS1kNWVhLTcxYzktYmQzZS02MTZkMjM3ODBmOTMifQ.H-GLmXCT-o7RxXrjo5Of0K8Nw5mpOOw6jgoXnd5KUxo 
```

for the rest of the api calls. This is a JWT token with following data

```json
{
  "email": "artpar@gmail.com",                    // user email
  "exp": 1729321122,                              // token expiry
  "iat": 1729061922,                              // token issued at time
  "iss": "daptin-019228",                         // token issuer (your daptin instance)
  "jti": "0192941f-260e-7b46-a1ae-f10fae700179",  // unique identifier for this token
  "name": "artpar@gmail.com",                     // user name
  "nbf": 1729061922,                              // token valid not before timestamp
  "sub": "01922e1a-d5ea-71c9-bd3e-616d23780f93"   // user reference id
}
```


---

So you have an account and a token to authenticate as that account. But do you need it? No. 
Call to fetch all user accounts works without any authorization

```bash
curl http://localhost:6333/api/user_account
```

```json
{
  "links": {
    "current_page": 1,
    "from": 0,
    "last_page": 1,
    "per_page": 10,
    "to": 10,
    "total": 1
  },
  "data": [
    {
      "type": "user_account",
      "id": "01929429-3d8f-7e53-8f15-a663e05fb01b",
      "attributes": {
        "__type": "user_account",
        "confirmed": 0,
        "created_at": "2024-10-16T07:09:43.86360642Z",
        "email": "artpar1@gmail.com",
        "name": "artpar1@gmail.com",
        "password": "",
        "permission": 2097151,
        "reference_id": "01929429-3d8f-7e53-8f15-a663e05fb01b",
        "updated_at": "2024-10-16T07:09:43.863622045Z",
        "user_account_id": "01929429-3d8f-7e53-8f15-a663e05fb01b"
      },
      "relationships": { /// ...}
    }
  ]
}
```

And so does all the data in all other tables (eg site, cloud_store, document, usergroup). 
And you can call update and delete APIs as well 
(not demonstrated here, but you can try, delete the sqlite database file after you are done playing to reset it all)


As the first user, it is an option for you to leave it open or enable the multi-tier permission and becoming the Administrator

```bash
curl 'http://localhost:6336/action/world/become_an_administrator' --compressed -X POST \
-H "Authorization:  Bearer $TOKEN" --data-raw '{}'
```

At this point, all other apis are locked-down and only accessible by administrator, that is you. 
You want to open up few or many of actions to guests or users.


... Will be updated soon

## Overview


- [Database backed](https://daptin.github.io/daptin/setting-up/installation/#database-configuration) persistence, 3NF normalized tables
- [JSON API](https://daptin.github.io/daptin/apis/overview/)/[GraphQL](https://daptin.github.io/daptin/features/enable-graphql/) for CRUD apis
- [User](https://daptin.github.io/daptin/setting-up/access/) and [group management](https://daptin.github.io/daptin/setting-up/access/) and access control
- Social login with [OAuth](https://daptin.github.io/daptin/extend/oauth_connection/): tested with google, github, linkedin
- [Actions](https://daptin.github.io/daptin/actions/actions/) for abstracting out business flows
- Extensive [state tracking APIs](https://daptin.github.io/daptin/state/machines/)
- Enable [Data Auditing](https://daptin.github.io/daptin/features/enable-data-auditing/) from a single toggle
- [Synchronous Data Exchange](https://daptin.github.io/daptin/extend/data_exchange/) with 3rd party APIs
- [Multilingual tables](https://daptin.github.io/daptin/features/enable-multilingual-table/) support, supports Accept-Language header 
- [Cloud storage sync](https://daptin.github.io/daptin/cloudstore/cloudstore/) like gdrive, dropbox, b2, s3 and more
- [Asset column](https://daptin.github.io/daptin/cloudstore/assetcolumns/) to hold file and blob data, backed by storage
- [Multiple websites](https://daptin.github.io/daptin/subsite/subsite/) under separate sub-domain/sub-paths
- [Connect with external APIs](https://daptin.github.io/daptin/integrations/overview/) by using extension points
- [Data View Streams](https://daptin.github.io/daptin/streams/streams/)
- Flexible [data import](https://daptin.github.io/daptin/setting-up/data_import/) (auto create new tables and automated schema generation)
    - XLSX 
    - JSON
    - CSV

Javascript/Typescript Client
===

https://github.com/daptin/daptin-js-client

Starter kit: https://github.com/daptin/vue_typescript_starter_kit


Define Schema

<img src="https://github.com/daptin/daptin/raw/master/images/api.jpg">

Find
<img src="https://github.com/daptin/daptin/raw/master/images/apigetall.png">

Get By Id
<img src="https://github.com/daptin/daptin/raw/master/images/apigetbyid.png">

Create
<img src="https://github.com/daptin/daptin/raw/master/images/apicreate.png">

Delete
<img src="https://github.com/daptin/daptin/raw/master/images/apidelete.png">

Delete relations
<img src="https://github.com/daptin/daptin/raw/master/images/apideleterelated.png">

List relations
<img src="https://github.com/daptin/daptin/raw/master/images/apifetchrelated.png">



* Versioning of the data
* Authentication and authorization
* JSON API endpoint
* GraphQL endpoint
* Actions and integrations with external services


Get Started
---

* [Native binary](https://daptin.github.io/daptin/setting-up/installation/#native-binary)
* [Heroku](https://daptin.github.io/daptin/setting-up/installation/#heroku-deployment)
* [Docker image](https://daptin.github.io/daptin/setting-up/installation/#docker-image)
* [Kubernetes YAML](https://daptin.github.io/daptin/setting-up/installation/#kubernetes-deployment)


### APIs

- CRUD APIs
    - [Read, search, filter](https://daptin.github.io/daptin/apis/read)
    - [Create](https://daptin.github.io/daptin/apis/create)
    - [Update](https://daptin.github.io/daptin/apis/update)
    - [Delete](https://daptin.github.io/daptin/apis/delete)
    - [Relations](https://daptin.github.io/daptin/apis/relation)
    - [Execute](https://daptin.github.io/daptin/apis/execute)
- Action APIs
    - [Using actions](https://daptin.github.io/daptin/actions/actions)
    - [Actions list](https://daptin.github.io/daptin/actions/default_actions)
- User APIs
    - [User registration/signup](https://daptin.github.io/daptin/actions/signup)
    - [User login/signin](https://daptin.github.io/daptin/actions/signin)
- State tracking APIs
    - [State machines](https://daptin.github.io/daptin/state/machines)

### Users

- [Guests](https://daptin.github.io/daptin/setting-up/access/#guests)
- [Adding users](https://daptin.github.io/daptin/setting-up/access/#signup-api)
- [User groups](https://daptin.github.io/daptin/setting-up/access/#user-groups)
- [Data access permission](https://daptin.github.io/daptin/setting-up/access/#authorization)
- [Social login](https://daptin.github.io/daptin/setting-up/access/#social-login)


### Asset and file storage

- [Cloud storage](https://daptin.github.io/daptin/cloudstore/cloudstore)

### Sub-sites

- [Create a subsite](https://daptin.github.io/daptin/subsite/subsite)


## Client library

|                                                                                |                                                                        |                                                                                |
| ------------------------------------------------------------------------------ | ---------------------------------------------------------------------- | ------------------------------------------------------------------------------ |
| [Ruby](http://jsonapi.org/implementations/#client-libraries-ruby)              | [Python](http://jsonapi.org/implementations/#client-libraries-python)  | [Javascript](http://jsonapi.org/implementations/#client-libraries-javascript)  |
| [Typescript](http://jsonapi.org/implementations/#client-libraries-typescript)  | [PHP](http://jsonapi.org/implementations/#client-libraries-php)        | [Dart](http://jsonapi.org/implementations/#client-libraries-dart)              |
| [.NET](http://jsonapi.org/implementations/#client-libraries-net)               | [Java](http://jsonapi.org/implementations/#client-libraries-java)      | [iOS](http://jsonapi.org/implementations/#client-libraries-ios)                |
| [Elixir](http://jsonapi.org/implementations/#client-libraries-elixir)          | [R](http://jsonapi.org/implementations/#client-libraries-r)             | [Perl](http://jsonapi.org/implementations/#client-libraries-perl)               |

## API spec RAML

OpenAPI V3 spec is auto generated for each endpoint exposed. This can be use to generate further documentation and clients.

![YAML API documentation](docs_markdown/docs/images/api-documentation.png)

```curl http://localhost/apispec.yaml```


## Road Map


* [x] Normalised Db Design from JSON schema upload
* [x] Json Api, with CRUD and Relationships
* [x] OAuth Authentication, inbuilt jwt token generator (setups up secret itself)
* [x] Authorization based on a slightly modified linux FS permission model
* [x] Objects and action chains
* [x] State tracking using state machine
* [x] Data connectors -> Incoming/Outgoing data
* [x] Plugin system -> Grow the system according to your needs
* [x] Native support for different data types (geo location/time/colors/measurements)
* [x] Configurable intelligent Validation for data in the APIs
* [x] Pages/Sub-sites -> Create a sub-site for a target audience
* [x] Define events all around the system
* [x] Data conversion/exchange/transformations
* [x] Live editor for subsites - grapesjs
* [x] Store connectors for storing big files/subsites - rclone
* [x] Market place to allow plugins/extensions to be installed
* [x] Online entity designer
* [x] Excel to entity identification
* [x] CSV to entity identification

![Alt](https://repobeats.axiom.co/api/embed/f833f4480ea5c9966619d330b90e49f882831f03.svg "Repobeats analytics image")
