Daptin
===

<a class="github-button" href="https://github.com/daptin/daptin" data-size="large" data-show-count="true" aria-label="Star daptin/daptin on GitHub">Star</a>


<img src="/images/logo.png" width="400" style="float: right"/>

Daptin is an **open-source backend** to develop and deploy **production-ready APIs** based applications. With Daptin you can design your data model and have a production ready JSON API online in minutes.

By following shared conventions, you can increase productivity, take advantage of generalized tooling, and focus on what matters: your application.


Easily consume the following features on any device

- Relational database backed persistent data
- CRUD **JSON API**
- User registration and login system
- Social login with oauth2: tested with google, github, linkedin
- Extensive state tracking APIs
- Enable *Data Auditing* from a single switch
- [Market place](https://github.com/daptin/market) enabling a variety of features
- **Cloud storage sync** like gdrive, dropbox, b2, s3 and more
- Manage multiple websites under separate sub-domain/sub-paths
- Connect with external APIs by using internal extension points

- **Database** to easily evolves your data schema & migrates your database [Postgres/MySQL/SQLite]
- **Flexible auth** using the JWT-based authentication & permission system
- **Works with all frontend frameworks** like React, Vue.js, Angular, Android, iOS
- **Very low memory requirement** and horizontally scalable
- **Can be deployed on a wide range of hardware** arm5,arm6,arm7,arm64,mips,mips64,mips64le,mipsle (or build for your target using go)


# Documentation

## Installation

- [Native](setting-up/native.md)
- [Heroku](setting-up/heroku.md)
- [Docker](setting-up/docker.md)
- [Docker Compose](setting-up/docker-compose.md)
- [Kubernetes](setting-up/kubernetes.md)
- [Choose your data storage](setting-up/database_configuration.md)

## Setup and data

- [Designing data model](setting-up/entities.md)
- [Linking data with one another](setting-up/entity_relations.md)
- [Database configuration](setting-up/database_configuration.md)
- [Import data](setting-up/data_import.md)

## APIs

- CRUD APIs
    - [Read, search, filter](apis/read.md)
    - [Create](apis/create.md)
    - [Update](apis/update.md)
    - [Delete](apis/delete.md)
    - [Relations](apis/relation.md)
    - [Execute](apis/execute.md)
- Action APIs
    - [Using actions](actions/actions.md)
    - [Actions list](actions/default_actions.md)
- User APIs
    - [User registration/signup](actions/signup.md)
    - [User login/signin](actions/signin.md)
- State tracking APIs
    - [State machines](state/machines.md)

## Users

- [Guests](auth/guests.md)
- [Adding users](auth/users.md)
- [Usergroups](auth/usergroups.md)
- [Data access permission](auth/permissions.md)
- [Social login](auth/social_login.md)

## Auth & Auth

- [User Authentication](auth/authentication.md)
- [Authorization](auth/authorization.md)

## Asset and file storage

- [Cloud storage](cloudstore/cloudstore.md)

## Sub-sites

- [Creating a subsite](subsite/subsite.md)


# Where to begin

The first thing you want to do after deploying a new instance is register yourself as a user on the dashboard. This part can be automated for redistributable applications.

## Become admin

Only the first user can become an **administrator** and only until no one else signs up. If the first user doesn't invoke "Become admin" before another user signs up, then it becomes a public instance which is something you would rarely want.

When you "Become admin", daptin will restart itself and schedule an update for itself where it makes you [the owner](/auth/authorization.md) of everything and update permission of all exposed apis. At this point guest users will not be allowed to invoke sign up process.

## Usage Road map

Quick road map to various things you can do from here:

* [Enable sign up for guests](/actions/signin.md)
* [Expose APIs](/setting-up/entities.md)
* [Set access permission](/auth/permissions.md)
* [Get a client library](http://jsonapi.org/implementations) for your frontend
* [Enable Auditing](/data-modeling/auditing.md) to maintain change logs
* [Connect to a cloud storage](/data-modeling/data_storage.md)
* Host a [static-site](/subsite/subsite.md)


## Import data

* [XLS](/actions/default_actions/#upload-xls)
* [CSV](/actions/default_actions/#upload-csv)
* [JSON](/actions/default_actions/#upload-json)
* [Schema](/actions/default_actions/#upload-schema)

# Guides

- [Create a site using a google drive folder](https://medium.com/@012parth/daptin-walk-through-oauth2-google-drive-subsites-and-grapejs-a6de27d9658a)
- [Creating a todo list backend](https://hackernoon.com/creating-a-todolist-backend-with-persistence-a1e8d7d39f62)

