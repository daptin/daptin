
<h1 align="center">
  <br>
  <a href="https://docs.dapt.in"><img width="300" height="282" src="https://github.com/daptin/daptin/raw/master/images/daptin-22-transparent-background-colored.png" alt="Daptin" title="Daptin" /></a>
  <br>
  Daptin
  <br>
</h1>


<h4 align="center">A modern backend for application developers, designers and testers</h4>


<p align="center">
    <a href="https://join.slack.com/t/daptin/shared_invite/enQtMzM1NTM1NTkyMDgzLTVlYzBlMmM4YjMyOTk0MDc5MWJmMzFlMTliNzQwYjcxMzc5Mjk0YzEyZDIwYTljZmE5NDU3Yjk3YzQ3MzhkMzI"><img src="https://img.shields.io/badge/join-on%20slack-orange.svg?longCache=true&style=for-the-badge" /> <a/>
</p>
<p align="center">
    <a href="https://github.com/daptin/daptin/releases/latest"><img alt="Release" src="https://img.shields.io/github/release/daptin/daptin.svg?style=flat-square"></a>
    <a href="/LICENSE.md"><img alt="Software License" src="https://img.shields.io/badge/license-MIT-brightgreen.svg?style=flat-square"></a>
    <a href="https://travis-ci.org/daptin/daptin"><img alt="Travis" src="https://img.shields.io/travis/daptin/daptin.svg?style=flat-square"></a>
    <a href="https://codecov.io/gh/daptin/daptin"><img alt="Codecov branch" src="https://img.shields.io/codecov/c/github/daptin/daptin/master.svg?style=flat-square"></a>
    <a href="https://goreportcard.com/report/github.com/daptin/daptin"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/daptin/daptin?style=flat-square"></a>
    <a href="http://godoc.org/github.com/daptin/daptin"><img alt="Go Doc" src="https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square"></a>
    <a href='https://semaphoreci.com/artpar/daptin'> <img src='https://semaphoreci.com/api/v1/artpar/daptin/branches/master/badge.svg' alt='Build Status'></a>
</p>


<p align="center">
  <a href="#key-features">Key Features</a> •
  <a href="#how-to-use">How To Use</a>
</p>


<p align="center">
	<a href="https://github.com/daptin/daptin/releases">Download</a> •
	<a href="https://docs.dapt.in">Documentation</a> •
	<a href="https://join.slack.com/t/daptin/shared_invite/enQtMzM1NTM1NTkyMDgzLTVlYzBlMmM4YjMyOTk0MDc5MWJmMzFlMTliNzQwYjcxMzc5Mjk0YzEyZDIwYTljZmE5NDU3Yjk3YzQ3MzhkMzI">Community</a>
</p>

---

![Create entity and add item](https://raw.githubusercontent.com/daptin/daptin/master/docs_markdown/docs/gifs/create_entity_and_add.gif)



## Key Features

* JSON APIs
  - create, read, update, delete
* GraphQL APIs
  - work in progress
* Client libraries for all platforms
* Rich data types
* Websocket support
* Sub sites hosting
* Oauth token and connections
* Cloud storage sync
* Action APIs and Relational data APIs
* Validation and conformation support
* Daptil will expose all APIs for easy use
* Fully featured dashboard
  - Responsive to desktop, mobile and table
* Cross platform
  - Windows, Mac and Linux ready.

**Daptin** is an open-source backend development framework to develop and deploy production-ready JSON API based applications. With Daptin you can design your data model and have a production ready JSON API online in minutes.


### Installation

- [Read me first](setting-up/settingup.md)
- [Native](setting-up/native.md)
- [Heroku](setting-up/heroku.md)
- [Docker](setting-up/docker.md)
- [Docker Compose](setting-up/docker-compose.md)
- [Kubernetes](setting-up/kubernetes.md)
- [Choose your storage](setting-up/database_configuration.md)

### Setup and data

- [Designing data model](setting-up/entities.md)
- [Linking data with one another](setting-up/entity_relations.md)
- [Database configuration](setting-up/database_configuration.md)
- [Import data](setting-up/data_import.md)

### APIs

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

### Users

- [Guests](auth/guests.md)
- [Adding users](auth/users.md)
- [Usergroups](auth/usergroups.md)
- [Data access permission](auth/permissions.md)
- [Social login](auth/social_login.md)

### Auth & Auth

- [User Authentication](auth/authentication.md)
- [Authorization](auth/authorization.md)

### Asset and file storage

- [Cloud storage](cloudstore/cloudstore.md)

### Sub-sites

- [Creating a subsite](subsite/subsite.md)



## Client library

|                                                                                |                                                                        |                                                                                |
| ------------------------------------------------------------------------------ | ---------------------------------------------------------------------- | ------------------------------------------------------------------------------ |
| [Ruby](http://jsonapi.org/implementations/#client-libraries-ruby)              | [Python](http://jsonapi.org/implementations/#client-libraries-python)  | [Javascript](http://jsonapi.org/implementations/#client-libraries-javascript)  |
| [Typescript](http://jsonapi.org/implementations/#client-libraries-typescript)  | [PHP](http://jsonapi.org/implementations/#client-libraries-php)        | [Dart](http://jsonapi.org/implementations/#client-libraries-dart)              |
| [.NET](http://jsonapi.org/implementations/#client-libraries-net)               | [Java](http://jsonapi.org/implementations/#client-libraries-java)      | [iOS](http://jsonapi.org/implementations/#client-libraries-ios)                |
| [Elixir](http://jsonapi.org/implementations/#client-libraries-elixir)          | [R](http://jsonapi.org/implementations/#client-libraries-r)             | [Perl](http://jsonapi.org/implementations/#client-libraries-perl)               |

## API spec RAML

RAML spec is auto generated for each endpoint exposed. This can be use to generate further documentation and clients.

![RAML API documentatnon](docs_markdown/docs/images/api-documentation.png)

```curl http://localhost/apispec.raml```

```
{
    "data": [
        {
            "type": "tableName",
            "attributes": {
                "col1": "",
                "col2": "",
            },
            "id": "",
        }
    ],
    "included": [
        {
            "type": "tableName",
            "attributes": {},
            "id": "",
        },
        .
        .
    ],
    "links": {
        "current_page": 1,
        "from": 0,
        "last_page": 100,
        "per_page": 50,
        "to": 50,
        "total": 5000
    }
}
```

## Web Dashboard

![Sign up and Sign in](https://raw.githubusercontent.com/daptin/daptin/master/docs_markdown/docs/gifs/signup_and_signin.gif)
![Create entity and add item](https://raw.githubusercontent.com/daptin/daptin/master/docs_markdown/docs/gifs/create_entity_and_add.gif)
![Generate random data to show tables](https://raw.githubusercontent.com/daptin/daptin/master/docs_markdown/docs/gifs/generate_random_show_tables.gif)

Use the following APIs

- [JSON](http://jsonapi.org) based CRUD+eXecute APIs for all your entities
- Authentication and authorization with user management
- Access control for data
- Extensible system with useful integrations (eg sync data updates to 3rd party api)
- [Client libraries](http://jsonapi.org/implementations/) to consume JSON API seamlessly


- Sub site hosting (SSH) without the need to run separate server
- An events-actions-outcomes framework to extend system
- Data-as-objects (instead of just strings)

Compared to building JSON APIs directly, Daptin provides APIs that makes writing fast frontend apps simpler.


## Why Daptin?


Daptin was to help build faster, more capable APIs over your data that worked across for all types of frontend.

While Daptin primarily targeted Web apps, the emergence of Android and iOS Apps as a rapidly growing target for developers demanded a different approach for building the backend. With developers classic use of traditional frameworks and bundling techniques, we struggle to invest enough time in the business and frontend demands for all sorts of Apps that provide consistent and predictable APIs which perform equally well on fast and slow load, across a diversity of platforms and devices.

Additionally, framework fragmentation had created a APIs development interoperability nightmare, where backend built for one purpose needs a lot of boilerplate and integration with the rest of the system, in a consistent way.

A component system around JSON APIs offered a solution to both problems, allowing more time available to be invested into frontend and business building, and targeting a standards-based JSON/Entity models that all frontends can use.

However, JSON APIs for data manipulation by themselves weren't enough. Building apps required a lot of custom actions, workflows, data integrity, event subscription, integration with external services that were previously locked up inside of traditional web frameworks. Daptin was built to pull these features out of traditional frameworks and bring them to the fast emerging JSON API standard in an automated way.


## Getting started


- Deploy instance of Daptin on a server
- Upload JSON/YAML/TOML/HCL file which describe your entities (or use marketplace to get started)
- or upload XLS file to create entities and upload data
- Become Admin of the instance (until then its a open for access, that's why you were able to create an account)


## Tech Goals

- Zero config start (sqlite db for fresh install, mysql/postgres is recommanded for serious use)
- A closely knit set of functionality which work together
- Completely configurable at runtime, can be run without any dev help
- Stateless(Horizontally scalable)
- Try to piggyback on used/known standards
- Runnable on all types on devices
- Cross platform app using [qt](https://github.com/therecipe/qt) (very long term goal. A responsive website for now.)


## Road Map


* [x] Normalised Db Design from JSON schema upload
* [x] Json Api, with CRUD and Relationships
* [x] OAuth Authentication, inbuilt jwt token generator (setups up secret itself)
* [x] Authorization based on a slightly modified linux FS permission model
* [x] Objects and action chains
* [x] State tracking using state machine
* [ ] Native tag support for user defined entities
* [x] Data connectors -> Incoming/Outgoing data
* [x] Plugin system -> Grow the system according to your needs
* [x] Native support for different data types (geo location/time/colors/measurements)
* [x] Configurable intelligent Validation for data in the APIs
* [x] Pages/Sub-sites -> Create a sub-site for a target audiance
* [ ] Define events all around the system
* [ ] Ability to define hooks on events from UI
* [x] Data conversion/exchange/transformations
* [x] Live editor for subsites - grapesjs
* [x] Store connectors for storing big files/subsites - rclone
* [x] Market place to allow plugins/extensions to be installed
* [x] Online entity designer
* [x] Excel to entity identification
* [x] CSV to entity identification

### Documentation

- Checkout the [documentation for daptin](http://docs.dapt.in)


## Tech stack


Backend | Frontend | Standards | Frameworks
---|---|---|---
[Golang](golang.org) | [BootStrap](http://getbootstrap.com/) | [RAML](raml.org) | [CoPilot Theme](https://copilot.mistergf.io)
[Api2go](https://github.com/manyminds/api2go) |  | [JsonAPI](jsonapi.org) | [VueJS](https://vuejs.org/v2/guide/)
[rclone](https://github.com/ncw/rclone) |  [grapesJs](grapesjs.com) | | [Element UI](https://element.eleme.io)
