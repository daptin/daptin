# Daptin


<img src="images/dashboard/index-page.png">


The most powerful ready to use data and services API server.

```
- User management
  - Sign up and sign in (1fa/2fa with password/TOTP)
  - Extensive authorization control table level/action level and row level
  - Rate limiting/connection throttling at IP/API level
- Data management
  - Declarative schema definition, relations and column properties
  - CRUD APIs with Authorization/Pagination/Search/Relations
  - File asset columns to store images/video/audio/blobs
- Storage management
  - localhost/gDrive/S3/B2/DropBox/FTP and many more supported
- Site management
  - Create HTTP sites based by storage anywhere
  - Enable HTTPS using LetsEncrypt
  - Create and Build HUGO static sites
  - Expose directories as FTP sites
- Integration and action management
  - Create workflows and expose as APIs
  - Call any 3rd party API by importing OpenAPI Spec
- Mail management
  - Enable SMTPS and IMAPS services and use daptin as your regular email provider
  - Multi hostname mail server
  - Multiple email accounts
- With a clean white-branded dashboard
```

<br />

## Features

Consume the following features easily on any device

- [Database backed](setting-up/installation/#database-configuration) persistence, 3NF normalized tables
- [JSON API](apis/overview/)/[GraphQL](features/enable-graphql/) for CRUD apis
- [User](setting-up/access/) and [group management](setting-up/access/) and access control
- Social login with [OAuth](extend/oauth_connection/): tested with google, github, linkedin
- [Actions](actions/overview/) for abstracting out business flows
- Extensive [state tracking APIs](state/machines/)
- Enable [Data Auditing](features/enable-data-auditing.md) from a single toggle
- [Synchronous Data Exchange](extend/data_exchange/) with 3rd party APIs
- [Multilingual tables](features/enable-multilingual-table.md) support, supports Accept-Language header 
- [Cloud storage sync](cloudstore/cloudstore/) like gdrive, dropbox, b2, s3 and more
- [Asset column](cloudstore/assetcolumns/) to hold file and blob data, backed by storage
- [Multiple websites](subsite/subsite/) under separate sub-domain/sub-paths
- [Connect with external APIs](integrations/overview/) by using extension points
- [Data View Streams](streams/streams/)
- Flexible [data import](setting-up/data_import/) (auto create new tables and automated schema generation)
    - XLSX 
    - JSON
    - CSV

- **Database** to have consistent single source of truth [Postgres/MySQL/SQLite]
- **Flexible auth** using the JWT-based authentication & permission system
- **Works with all frontend frameworks** like React, Vue.js, Angular, Android, iOS
- **Very low memory requirement** and horizontally scalable
- **Can be deployed on a wide range of hardware** arm5,arm6,arm7,arm64,mips,mips64,mips64le,mipsle (or build for your target using go)


## Guides

- [Create a site using a google drive folder](https://medium.com/@012parth/daptin-walk-through-oauth2-google-drive-subsites-and-grapejs-a6de27d9658a)
- [Creating a todo list backend](https://hackernoon.com/creating-a-todolist-backend-with-persistence-a1e8d7d39f62)

