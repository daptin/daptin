
<h1 align="center">
  <br>
  <a href="https://daptin.github.io/daptin"><img width="100" height="100" src="https://github.com/daptin/daptin/raw/master/images/daptin-22-transparent-background-colored.png" alt="Daptin" title="Daptin" /></a>
  <br>
  Daptin
  <br>
</h1>




<p align="center">
    <a href="https://travis-ci.org/daptin/daptin"><img alt="Travis" src="https://img.shields.io/travis/daptin/daptin.svg?style=flat-square"></a>
    <a href='https://semaphoreci.com/artpar/daptin'> <img src='https://semaphoreci.com/api/v1/artpar/daptin/branches/master/badge.svg' alt='Build Status'></a>
    <a href='https://circleci.com/gh/daptin/daptin'> <img src='https://circleci.com/gh/daptin/daptin.svg?style=svg' alt='Build Status'></a>	
    <a href="https://app.wercker.com/project/byKey/4fe8e76660ff5cb02e502c4d9a221997"><img alt="Wercker status" src="https://app.wercker.com/status/4fe8e76660ff5cb02e502c4d9a221997/s/master"></a>
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
    <a href="https://join.slack.com/t/daptin/shared_invite/enQtMzM1NTM1NTkyMDgzLWJmZmRlN2M4YzRkOTI4MDhlNWQ1YzBiMDNhMzE0NTVmNzA3NjA5ZDdkMDExZmI0M2UyMmM2NzhlNDc3M2VhZTQ"><img src="https://img.shields.io/badge/join-on%20slack-orange.svg?longCache=true&style=for-the-badge" /> <a/> 
     <a href="https://discord.gg/t564q8SQVk"><img src="https://img.shields.io/badge/JOIN-ON%20DISCORD-blue&?style=for-the-badge&logo=discord" /> <a/> 
</p>


Daptin is a server exposing HTTP APIs for web and application developers providing to talk to database and persistent storage.

- Consistent API with authentication and authorization for database table and metadata
- User management and user group management API with row and table level ownership
- Stateless and easily scalable



- Dadadash : https://github.com/daptin/dadadash/
  - `docker run -p 8080:8080 daptin/dadadash`



|      |    |
|------------------------------------------------|------------------------------------------------------|
| ![ new workspace](https://github.com/daptin/daptin/raw/master/images/workspace-create.png)     | ![ worksapce view](https://github.com/daptin/dadadash/raw/master/assets/workspaceView.png)         |
| ![ new base](https://github.com/daptin/daptin/raw/master/images/admin-dashboard-home.png)               | ![ new app item menu](https://github.com/daptin/dadadash/raw/master/assets/newAppItemMenu.png)     |
| ![ document editor](https://github.com/daptin/dadadash/raw/master/assets/documentEditor.png) | ![ spreadsheet editor](https://github.com/daptin/dadadash/raw/master/assets/spreadsheetEditor.png) |
| ![ data tables](https://github.com/daptin/dadadash/raw/master/assets/dataTable.png)          | ![ file browser](https://github.com/daptin/dadadash/raw/master/assets/fileBrowser.png)             |
| ![ calendar](https://github.com/daptin/dadadash/raw/master/assets/newCalendarEvent.png)      | ![ File browser](https://github.com/daptin/dadadash/raw/master/assets/7.png)                       |



<p align="center">
	<a href="https://github.com/daptin/daptin/releases">Download</a> •
	<a href="https://daptin.github.io/daptin/">Documentation</a> •
	<a href="https://join.slack.com/t/daptin/shared_invite/enQtMzM1NTM1NTkyMDgzLTVlYzBlMmM4YjMyOTk0MDc5MWJmMzFlMTliNzQwYjcxMzc5Mjk0YzEyZDIwYTljZmE5NDU3Yjk3YzQ3MzhkMzI">Community</a>
</p>

The most powerful ready to use data and services API server.


- **Define data tables and relations from config files or API calls**
  - Middleware for handling data normalizations and conformations 
  - Create indexes, constraints on columns
  - Column can be have images/video/audio/blobs attachments, stored appropriately in #cloudstore
- **Authentication and Authorization on APIs, define auth using APIs**
  - Add users and user groups
  - RWX based permission system for rows and tables
  - JWT token with configurable expiry time
  - User sign in/sign up/password reset flows
- **JSON API and GraphQL API**
  - [JSONAPI.org](https://jsonapi.org) complaint endpoints
  - GraphQL endpoint with Data/Query and Mutations available
  - Pagination and filtering using page number or cursor based
  - Fetch relationships in a single call
- **Cloud storage, create storage using API**
  - Connect to [over 30 storage providers](https://rclone.org/overview/) (localhost/HTTP/FTP/GDrive/Dropbox/S3 and many more)
  - Manage files using daptin actions
  - Automated 1 hour sync scheduled
- **Static and HUGO sites**
  - Host site on multiple domains
  - Inbuilt HTTPS certificate generation with lets-encrypt
  - Web file browser and FTP access (disabled by default)
- **Action workflows & 3rd party API integration, create new integration using API calls**
  - Supports v2/v3 openapi in yaml or json format
  - Call any 3rd party API by importing OpenAPI Spec
- **Email server**
  - Enable SMTPS and IMAPS services and use daptin as your daily email provider
  - Multi hostname mail server
  - Multiple email accounts, database backed email storage     


<br />



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

