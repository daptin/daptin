
<a href="#">
    <img src="https://github.com/artpar/goms/raw/master/gomsweb/static/img/logo_blk.png" alt="GoMS logo" title="GoMS" align="right" height="50" />
</a>

# Goms



 [![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy) [![Build Status](https://travis-ci.org/artpar/goms.svg?branch=master)](https://travis-ci.org/artpar/goms)


### State: pre-alpha

GoMS is a ready-to-deploy schema driven adaptable platform for quick apps

## Run it and take the tours before reading on

```
docker run -d -p 8080:8080 goms/goms
```

Then open [console](http://localhost:8080)

## How can you use GoMS

Goms uses a SQL database and works are a very high level framework/management system. 

Goms asks you to define your domain entities plus along with their relations in the way you want to organise them, and provide you a complete dashboard with following

- a responsive dashboard, tested on desktop browsers and mobile browsers
- A status tracking system 



GoMS is a platform which can be customised using Schema files, which describe your requirements and processes.

## Tech Goals

- Zero config start (sqlite db for fresh install, data can be later automatically moved to mysql/postgres using goms)
- A closely knit set of components to work together
- Completely configurable at runtime, can be run without any dev help
- Stateless
- Try to piggyback on used/known standards
- Runnable on all types on devices
- Cross platform app using [qt](https://github.com/therecipe/qt) (very long term goal. A responsive website for now.)

### Documentation state

Incomplete, might be confusing.

Please suggest changes using issues or [email me](mailto:artpar@gmail.com)


## Subsystems 

### Currently present

- Normalised Db Design from JSON schema upload
- Json Api, with CRUD and Relationships
- OAuth Authentication, inbuilt jwt token generator (setups up secret itself)
- Authorization based on a slightly modified linux FS permission model
- Objects and action chains

### Road Map

| Goal                | Objectives                                                                                                                                                  |
|---------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------|
| State based objects | Objects to have multiple state machines concurrently maintained.                                                                                            |
| Object events       | created/modified/deleted                                                                                                                                    |
| Views               | Composing views on run time                                                                                                                                 |
| Data connectors     | Event/action triggered Input/Output from the environment/services/apis |
|       | Consume data from other services and send data to them |
|       | Handle format exchanges |
| Plugin system       |    Compose your desired modifications using resuable JSON files                                                                                                                                                         |
| Pages/Sub-sites     |  Frequently there is a need for showing "list of items" to public   |
| Tags                | Native support object tagging   |
| Rich fields collection |   The system will understand the data better to provide you a lot more features       |

### Target

## User system

Goms makes two tables for user management

- user

Every user who is interacting with the system will be associated with a user in Goms.

By default a user is ```guest```

- usergroup

User and every other entity in Goms is associated to multiple user groups

Each user has his own user group

## Ownership

Every object in the system is owned by someone and belongs to multiple Usergroups.

The person who creates the object is the owner by default

Ownership can be changed (by someone who has permission to "write" on that object)

## Authentication

Every user is either a guest or a known user (logged in via one of the login providers).

## Authorization

Each object in Goms can belong to multiple user groups, where the admin specifies the permission that group users will have for the associated objects.

Permissions are linux filesystem style permission, are 3 digit numbers

- First digit for owners of the object
- Second digit for users in the groups which that object belongs to (multiple groups)
- Third digit for everyone else

```
1 = Execute Only
2 = Write Only
3 = Write + Execute
4 = Read Only
5 = Read + Execute
6 = Read + Write
7 = Read + Write + Execute
```

Each table also has these permissions, which are picked up from the ```world``` table.

## Entities

Goms work with relational entities. You can create entities to represent your work and the relations with other entities.

All entities are stored in a relational database. Currently the following database system support is targeted

- mysql
- sqlite
- postgres


## Environment definition

Goms keeps the configuration in database in two tables

- world

Each table being used by Goms will have an entry in ```world``` table. It contains the schema in json as well a default permission column, for new objects in that table.

- world_column

Each column known to Goms will have an entry in world_column table. It also contains the metadata about the column. 

- actions

Actions are defined on entities, have a set of Input Fields, and a set of Outcomes.

## Tech stack

- Golang
- Semantic UI
- JSONAPI
- VueJS
- CoPilot theme
- a lot of libraries...