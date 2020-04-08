
<h1 align="center">
  <br>
  <a href="https://daptin.github.io/daptin"><img width="100" height="100" src="https://github.com/daptin/daptin/raw/master/images/daptin-22-transparent-background-colored.png" alt="Daptin" title="Daptin" /></a>
  <br>
  Daptin
  <br>
</h1>
<h4 align="center">Headless CMS server</h4>



<p align="center">
    <a href="https://travis-ci.org/daptin/daptin"><img alt="Travis" src="https://img.shields.io/travis/daptin/daptin.svg?style=flat-square"></a>
    <a href='https://semaphoreci.com/artpar/daptin'> <img src='https://semaphoreci.com/api/v1/artpar/daptin/branches/master/badge.svg' alt='Build Status'></a>
    <a href='https://circleci.com/gh/daptin/daptin'> <img src='https://circleci.com/gh/daptin/daptin.svg?style=svg' alt='Build Status'></a>	
    <a href="https://app.wercker.com/project/byKey/4fe8e76660ff5cb02e502c4d9a221997"><img alt="Wercker status" src="https://app.wercker.com/status/4fe8e76660ff5cb02e502c4d9a221997/s/master"></a>
<p align="center">
    <a href="/LICENSE.md"><img alt="Software License" src="https://img.shields.io/badge/license-MIT-brightgreen.svg?style=flat-square"></a>
    <a href="https://goreportcard.com/report/github.com/daptin/daptin"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/daptin/daptin?style=flat-square"></a>
    <a href="http://godoc.org/github.com/daptin/daptin"><img alt="Go Doc" src="https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square"></a>
</p>
<p align="center">
    <a href="https://codecov.io/gh/daptin/daptin"><img alt="Codecov branch" src="https://img.shields.io/codecov/c/github/daptin/daptin/master.svg?style=flat-square"></a>
    <a href="https://github.com/daptin/daptin/releases/latest"><img alt="Release" src="https://img.shields.io/github/release/daptin/daptin.svg?style=flat-square"></a>
</p>
<p align="center">
    <a href="https://join.slack.com/t/daptin/shared_invite/enQtMzM1NTM1NTkyMDgzLWJmZmRlN2M4YzRkOTI4MDhlNWQ1YzBiMDNhMzE0NTVmNzA3NjA5ZDdkMDExZmI0M2UyMmM2NzhlNDc3M2VhZTQ"><img src="https://img.shields.io/badge/join-on%20slack-orange.svg?longCache=true&style=for-the-badge" /> <a/>
</p>


<p align="center">
	<a href="https://github.com/daptin/daptin/releases">Download</a> •
	<a href="https://daptin.github.io/daptin/">Documentation</a> •
	<a href="https://join.slack.com/t/daptin/shared_invite/enQtMzM1NTM1NTkyMDgzLTVlYzBlMmM4YjMyOTk0MDc5MWJmMzFlMTliNzQwYjcxMzc5Mjk0YzEyZDIwYTljZmE5NDU3Yjk3YzQ3MzhkMzI">Community</a>
</p>


<p align="center">
  <a href="#why-use-daptin">Features</a> •
  <a href="#getting-started">Getting Started</a>
</p>


## Why use daptin

Easily consume the following features on any device

- [Database backed](https://daptin.github.io/daptin/setting-up/installation/#database-configuration) persistence, 3NF normalized tables
- [JSON API](https://daptin.github.io/daptin/apis/overview/)/[GraphQL](https://daptin.github.io/daptin/features/enable-graphql/) for CRUD apis
- [User](https://daptin.github.io/daptin/setting-up/access/) and [group management](https://daptin.github.io/daptin/setting-up/access/) and access control
- Social login with [OAuth](https://daptin.github.io/daptin/extend/oauth_connection/): tested with google, github, linkedin
- [Actions](https://daptin.github.io/daptin/actions/actions/) for abstracting out business flows
- Extensive [state tracking APIs](https://daptin.github.io/daptin/state/machines/)
- Enable [Data Auditing](https://daptin.github.io/daptin/features/enable-data-auditing/) from a single toggle
- [Synchronous Data Exchange](https://daptin.github.io/daptin/extend/data_exchange/) with 3rd party APIs
- [Multilingual tables](https://daptin.github.io/daptin/features/enable-multilingual-table/) support, supports Accept-Language header 
- [Market place](https://daptin.github.io/daptin/extend/marketplacce/) API to manage and share schemas
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
- [Usergroups](https://daptin.github.io/daptin/setting-up/access/#user-groups)
- [Data access permission](https://daptin.github.io/daptin/setting-up/access/#authorization)
- [Social login](https://daptin.github.io/daptin/setting-up/access/#social-login)


### Asset and file storage

- [Cloud storage](https://daptin.github.io/daptin/cloudstore/cloudstore)

### Sub-sites

- [Creating a subsite](https://daptin.github.io/daptin/subsite/subsite)



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
* [x] Pages/Sub-sites -> Create a sub-site for a target audience
* [x] Define events all around the system
* [x] Data conversion/exchange/transformations
* [x] Live editor for subsites - grapesjs
* [x] Store connectors for storing big files/subsites - rclone
* [x] Market place to allow plugins/extensions to be installed
* [x] Online entity designer
* [x] Excel to entity identification
* [x] CSV to entity identification

### Documentation

- Checkout the [documentation for daptin](https://daptin.github.io/daptin/)


### Golang Dependencies list


|                  DEPENDENCY                   |                        REPOURL                        |   LICENSE    |
|-----------------------------------------------|-------------------------------------------------------|--------------|
| github.com/GeertJohan/go.rice                 | https://github.com/GeertJohan/go.rice                 | bsd-2-clause |
| github.com/artpar/go-guerrilla                | https://github.com/artpar/go-guerrilla                | MIT          |
| github.com/gin-gonic/gin                      | https://github.com/gin-gonic/gin                      | MIT          |
| github.com/sirupsen/logrus                    | https://github.com/sirupsen/logrus                    | MIT          |
| github.com/Masterminds/squirrel               | https://github.com/Masterminds/squirrel               | Other        |
| github.com/PuerkitoBio/goquery                | https://github.com/PuerkitoBio/goquery                | bsd-3-clause |
| github.com/anthonynsimon/bild                 | https://github.com/anthonynsimon/bild                 | MIT          |
| github.com/artpar/api2go                      | https://github.com/artpar/api2go                      | MIT          |
| github.com/artpar/api2go-adapter              | https://github.com/artpar/api2go-adapter              | MIT          |
| github.com/artpar/go-imap                     | https://github.com/artpar/go-imap                     | MIT          |
| github.com/artpar/go.uuid                     | https://github.com/artpar/go.uuid                     | MIT          |
| github.com/artpar/parsemail                   | https://github.com/artpar/parsemail                   | MIT          |
| github.com/artpar/rclone                      | https://github.com/artpar/rclone                      | MIT          |
| github.com/artpar/stats                       | https://github.com/artpar/stats                       | MIT          |
| github.com/bjarneh/latinx                     | https://github.com/bjarneh/latinx                     | bsd-3-clause |
| github.com/emersion/go-sasl                   | https://github.com/emersion/go-sasl                   | MIT          |
| github.com/julienschmidt/httprouter           | https://github.com/julienschmidt/httprouter           | bsd-3-clause |
| golang.org/x/net/context                      | https://go.googlesource.com/net                       |              |
| github.com/advance512/yaml                    | https://github.com/advance512/yaml                    | Other        |
| golang.org/x/crypto/bcrypt                    | https://go.googlesource.com/crypto                    |              |
| github.com/alexeyco/simpletable               | https://github.com/alexeyco/simpletable               | MIT          |
| github.com/araddon/dateparse                  | https://github.com/araddon/dateparse                  | MIT          |
| github.com/artpar/conform                     | https://github.com/artpar/conform                     | Other        |
| github.com/artpar/resty                       | https://github.com/artpar/resty                       | MIT          |
| github.com/emersion/go-message                | https://github.com/emersion/go-message                | MIT          |
| github.com/go-playground/locales              | https://github.com/go-playground/locales              | MIT          |
| github.com/go-playground/universal-translator | https://github.com/go-playground/universal-translator | MIT          |
| golang.org/x/oauth2                           | https://go.googlesource.com/oauth2                    |              |
| gopkg.in/go-playground/validator.v9           | https://github.com/go-playground/validator            | MIT          |
| golang.org/x/net/websocket                    | https://go.googlesource.com/net                       |              |

