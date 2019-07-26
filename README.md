
<h1 align="center">
  <br>
  <a href="https://docs.dapt.in"><img width="100" height="100" src="https://github.com/daptin/daptin/raw/master/images/daptin-22-transparent-background-colored.png" alt="Daptin" title="Daptin" /></a>
  <br>
  Daptin
  <br>
</h1>


<h4 align="center">Headless CMS</h4>


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
    <a href='https://circleci.com/gh/daptin/daptin'> <img src='https://circleci.com/gh/daptin/daptin.svg?style=svg' alt='Build Status'></a>
    <a href='https://coveralls.io/github/daptin/daptin'><img src='https://coveralls.io/repos/github/daptin/daptin/badge.svg' alt='Coverage Status' /></a>
</p>



<p align="center">
	<a href="https://github.com/daptin/daptin/releases">Download</a> •
	<a href="https://docs.dapt.in">Documentation</a> •
	<a href="https://join.slack.com/t/daptin/shared_invite/enQtMzM1NTM1NTkyMDgzLTVlYzBlMmM4YjMyOTk0MDc5MWJmMzFlMTliNzQwYjcxMzc5Mjk0YzEyZDIwYTljZmE5NDU3Yjk3YzQ3MzhkMzI">Community</a>
</p>


<p align="center">
  <a href="#key-features">Features</a> •
  <a href="#getting-started">Getting Started</a>
</p>

**Daptin** is a headless CMS server for building reusable APIs for accessing the database. Daptin takes in your desired structure of table via a YAML/JSON configuration file, creates those tables in the database of your choice (mysql/postgres/sqlite) and provides you frequently used features built in:

* Versioning of the data
* GET all, GET by id, Search by filter API with pagination
* Create, update and delete API
* Authentication and authorization
* JSON API endpoint
* Graphql endpoint


Get Started
---

* [Native binary](https://docs.dapt.in/setting-up/installation/#native-binary)
* [Heroku](https://docs.dapt.in/setting-up/installation/#heroku-deployment)
* [Docker image](https://docs.dapt.in/setting-up/installation/#docker-image)
* [Kubernetes YAML](https://docs.dapt.in/setting-up/installation/#kubernetes-deployment)

## Features

* Declarative Data Modeling system
  - Unique/Primary keys
  - Single/Multiple Relation
  - Normalizations and conformations
  - Scripting using JS
* CRUD JSON APIs' for all tables
  - Create, Read, Update, Delete
  - Sort, filter, search, group by single/multiple columns
  - Authentication and Group based authorization
  - Pluggable middleware, conformations and normalizations
  - Trigger actions/pipelines
* GraphQL APIs
  - Read and Mutations APIs for all tables
  - One level of relationship fetching
* Client SDK libraries for all platforms
* Rich data types
  - Column types ranging from number to json to file/image assets
* Sub sites hosting
  - Expose multiple websites from a single instance
  - Connect multiple domains/sub-domains
* Pluggable Social Auth, Basic Auth or Username/Password Auth
* Cloud storage
  - Connect to external cloud storage services seamlessly
  - Pull data/Push data
* Action APIs
  - Define work-flows
  - Expose custom endpoints for other services
* Ready to use web dashboard
  - Responsive to desktop, mobile and table
* Cross platform
  - Windows, Mac, Linux and more


### APIs

- CRUD APIs
    - [Read, search, filter](https://docs.dapt.in/apis/read)
    - [Create](https://docs.dapt.in/apis/create)
    - [Update](https://docs.dapt.in/apis/update)
    - [Delete](https://docs.dapt.in/apis/delete)
    - [Relations](https://docs.dapt.in/apis/relation)
    - [Execute](https://docs.dapt.in/apis/execute)
- Action APIs
    - [Using actions](https://docs.dapt.in/actions/actions)
    - [Actions list](https://docs.dapt.in/actions/default_actions)
- User APIs
    - [User registration/signup](https://docs.dapt.in/actions/signup)
    - [User login/signin](https://docs.dapt.in/actions/signin)
- State tracking APIs
    - [State machines](https://docs.dapt.in/state/machines)

### Users

- [Guests](https://docs.dapt.in/auth/guests)
- [Adding users](https://docs.dapt.in/auth/users)
- [Usergroups](https://docs.dapt.in/auth/usergroups)
- [Data access permission](https://docs.dapt.in/auth/permissions)
- [Social login](https://docs.dapt.in/auth/social_login)


### Asset and file storage

- [Cloud storage](https://docs.dapt.in/cloudstore/cloudstore)

### Sub-sites

- [Creating a subsite](https://docs.dapt.in/subsite/subsite)



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

- Zero config start (sqlite db for no-config install, mysql/postgres is recommended for serious use)
- A closely knit set of functionality which work together
- Completely configurable at runtime
- Stateless (Horizontally scalable)
- Piggyback on used/known standards
- Runnable on all types on devices
- Cross platform app using [qt](https://github.com/therecipe/qt) (very long term goal. A responsive website for now.)


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
* [x] Pages/Sub-sites -> Create a sub-site for a target audiance
* [x] Define events all around the system
* [x] Data conversion/exchange/transformations
* [x] Live editor for subsites - grapesjs
* [x] Store connectors for storing big files/subsites - rclone
* [x] Market place to allow plugins/extensions to be installed
* [x] Online entity designer
* [x] Excel to entity identification
* [x] CSV to entity identification

### Documentation

- Checkout the [documentation for daptin](https://daptin.github.io/docs/)
