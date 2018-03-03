# Daptin

<p align="left">
    <a href="https://github.com/daptin/daptin/releases/latest"><img alt="Release" src="https://img.shields.io/github/release/daptin/daptin.svg?style=flat-square"></a>
    <a href="/LICENSE.md"><img alt="Software License" src="https://img.shields.io/badge/license-MIT-brightgreen.svg?style=flat-square"></a>
    <a href="https://travis-ci.org/daptin/daptin"><img alt="Travis" src="https://img.shields.io/travis/daptin/daptin.svg?style=flat-square"></a>
    <a href="https://codecov.io/gh/daptin/daptin"><img alt="Codecov branch" src="https://img.shields.io/codecov/c/github/daptin/daptin/master.svg?style=flat-square"></a>
    <a href="https://goreportcard.com/report/github.com/daptin/daptin"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/daptin/daptin?style=flat-square"></a>
    <a href="http://godoc.org/github.com/daptin/daptin"><img alt="Go Doc" src="https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square"></a>
    <a href='https://semaphoreci.com/artpar/daptin-2'> <img src='https://semaphoreci.com/api/v1/artpar/daptin-2/branches/master/badge.svg' alt='Build Status'></a>
</p>
  <p align="left">

  <img src="https://github.com/daptin/daptin/raw/master/daptinweb/static/img/logo_blk.png" alt="Daptin logo" style="float: right;" title="Daptin" height="140" />
    </p>


**Daptin is an open-source backend development framework** to develop and deploy production-ready JSONAPI microservices. With Daptin you can design your data model and have a production ready JSON API online in minutes.

By following shared conventions, you can increase productivity, take advantage of generalized tooling, and focus on what matters: your application.

Daptin works as an interface between your users and your data. 

- Easily consume the API on any device using a [JSONAPI.org client](http://JSONAPI.org/implementations.org)
- Interact with users as guests or known users by using a JWT token
- Grant permission of data/actions to users based on ownership, groups and guests
- Build custom actions to expose custom APIs over your data apart from the usual CRUD APIs
- Sync with cloud storage services like gdrive, dropbox, b2, s3 and more
- Sync folders and expose these as static websites under separate sub-domain/sub-paths
- Connect with other services by directly connecting with any external API


- **Database** to easily evolves your data schema & migrates your database [Postgres/MySQL/SQLite]
- **Flexible auth** using the JWT-based authentication & permission system
- **Works with all frontend frameworks** like React, Vue.js, Angular, Android, iOS
- **Very low memory requirement** and horizontally scalable 
- **Can be deployed on a wide range of hardware** arm5,arm6,arm7,arm64,mips,mips64,mips64le,mipsle (or build for your target using go)


## Deploy and get started

| Deployment preference      | Getting started                                                                                                               |
| -------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| Heroku                     | [![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/daptin/daptin) |
| Docker                     | `docker run -p 8080:8080 daptin/daptin`                                                                                         |
| Kubernetes                 | [Service & Deployment YAML](https://docs.dapt.in/setting-up/settingup/#kubernetes)                                                                                      |
| Local                      | `go get github.com/daptin/daptin`                                                                                         |
| Linux (386/amd64/arm5,6,7) | [Download static linux builds](https://github.com/daptin/daptin/releases)                                                     |
| Windows                    | `go get github.com/daptin/daptin`                                                                                               |
| OS X                       | `go get github.com/daptin/daptin`                                                                                               |
| Load testing               | [Docker compose](https://docs.dapt.in/setting-up/settingup/#docker-compose)                                                                                             |
| Raspberry Pi               | [Linux arm 7 static build](https://github.com/daptin/daptin/releases)                                                         |


## Database persistence

Store data on MySQL, PostgreSQL for heavy use cases (thousands of users) or SQLite for light use cases (iot, embedded).

## Client

|                                                                                |                                                                        |                                                                                |
| ------------------------------------------------------------------------------ | ---------------------------------------------------------------------- | ------------------------------------------------------------------------------ |
| [Ruby](http://jsonapi.org/implementations/#client-libraries-ruby)              | [Python](http://jsonapi.org/implementations/#client-libraries-python)  | [Javascript](http://jsonapi.org/implementations/#client-libraries-javascript)  |
| [Typescript](http://jsonapi.org/implementations/#client-libraries-typescript)  | [PHP](http://jsonapi.org/implementations/#client-libraries-php)        | [Dart](http://jsonapi.org/implementations/#client-libraries-dart)              |
| [.NET](http://jsonapi.org/implementations/#client-libraries-net)               | [Java](http://jsonapi.org/implementations/#client-libraries-java)      | [iOS](http://jsonapi.org/implementations/#client-libraries-ios)                |
| [Elixir](http://jsonapi.org/implementations/#client-libraries-elixir)          | [R](http://jsonapi.org/implementations/#client-libraries-r)             | [Perl](http://jsonapi.org/implementations/#client-libraries-perl)               |

### API spec RAML

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

### Querying


| all rows from a table | single row from table by id | related rows using a foreign key relation |
|-----------------------|-----------------------------|-------------------------------------------|
| /api/{tableName}      | /api/{tableName}/{id}       | /api/{tableName}/{id}/{relationName}      |
|                       |                             |                                           |

---
### Pagination


| Number          | Size            |
|-----------------|-----------------|
| ?page[number]=1 | ?page[size]=200 |


### Projection, Sort, Filter

| Column projection       | Sorting          | Filtering          |
|-------------------------|------------------|--------------------|
| ?fields=column1,column2 | ?sort=col1,-col2 | ?filter=query_text |



## Usage

```yaml
Tables:
- TableName: todo
  Columns:
  - Name: title
    DataType: varchar(500)
    ColumnType: label
    IsIndexed: true
  - Name: completed
    DataType: int(1)
    ColumnType: truefalse
    DefaultValue: 'false'
  Validations:
  - ColumnName: title
    Tags: required
- TableName: project
  Columns:
  - Name: name
    DataType: varchar(200)
    ColumnType: name
    IsIndexed: true
Relations:
- Subject: todo
  Relation: has_one
  Object: project
```

## Web Dashboard

![Sign up and Sign in](https://raw.githubusercontent.com/daptin/daptin/master/docs_markdown/docs/gifs/signup_and_signin.gif)
![Create entity and add item](https://raw.githubusercontent.com/daptin/daptin/master/docs_markdown/docs/gifs/create_entity_and_add.gif)
![Generate random data to show tables](https://raw.githubusercontent.com/daptin/daptin/master/docs_markdown/docs/gifs/generate_random_show_tables.gif)

Daptin will provide

- [JSON](http://jsonapi.org) based CRUD+eXecute APIs for all your entities
- Integrated authentication and authorization with user management
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
* [ ] Native support for different data types (geo location/time/colors/measurements)
* [ ] Configurable intelligent Validation for data in the APIs
* [x] Pages/Sub-sites -> Create a sub-site for a target audiance
* [ ] Define events all around the system
* [ ] Ability to define hooks on events from UI
* [x] Data conversion/exchange/transformations
* [x] Live editor for subsites - grapesjs
* [x] Store connectors for storing big files/subsites - rclone
* [x] Market place to allow plugins/extensions to be installed
* [x] Online entity designer
* [x] Excel to entity identification

### Documentation

- Checkout the [documentation for daptin](http://docs.dapt.in)


## Tech stack


Backend | Frontend | Standards | Frameworks
---|---|---|---
[Golang](golang.org) | [BootStrap](http://getbootstrap.com/) | [RAML](raml.org) | [CoPilot Theme](https://copilot.mistergf.io)
[Api2go](https://github.com/manyminds/api2go) |  | [JsonAPI](jsonapi.org) | [VueJS](https://vuejs.org/v2/guide/)
[rclone](https://github.com/ncw/rclone) |  [grapesJs](grapesjs.com) | | [Element UI](https://element.eleme.io)
