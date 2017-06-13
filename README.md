
# Goms

[![Build Status](https://travis-ci.org/artpar/goms.svg?branch=master)](https://travis-ci.org/artpar/goms) [![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy)

Not related to [GOMS](https://en.wikipedia.org/wiki/GOMS)

Goms is a adaptable management system.

## Use it before reading on

```
docker run -p 8080:6336 goms/goms
```

Then open [console](http://localhost:8080)

Goms is targeted for small to medium complexity use cases. You can build blog, a survey management system, a vendor management system, a forum, e-commerce website.

## Goals

- Zero config start
- Focus on user requirements more then ease of development
- Completely configurable at runtime
- Stateless
- Try to piggyback on used/known standards
- Runnable on all types on devices

- Cross platform app using [qt](https://github.com/therecipe/qt)

### Documentation state

Incomplete, might be confusing.

Please suggest changes using issues or [email me](mailto:artpar@gmail.com)


## Subsystems 

### Currently present

- Json Api, with CRUD and Relationships
- OAuth Authentication using auth0, email is required
- Authorization based on a slightly modified linux FS permission model
- Objects and action chains

### Road Map

| Goal                | Objectives                                                                                                                                                  |
|---------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------|
| State based objects | Objects to have multiple state machines concurrently maintained.                                                                                            |
| Object events       | created/modified/deleted                                                                                                                                    |
| Views               | Composing views on run time                                                                                                                                 |
| Data connectors     | - Event/action triggered Input/Output from the environment/services/apis - Consume data from other services and send data to them - Handle format exchanges |
| Plugin system       |                                                                                                                                                             |
| Pages/Sub-sites     |                                                                                                                                                             |
| Tags                | Native tagging, user tags, group tags on objects                                                                                                            |


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

All entities are stored in a relational database. Currently the following database support is targeted

- mysql
- sqlite
- postgres


## Environment definition

Goms keeps the configuration in database in two tables

- world

Each table being used by Goms will have an entry in ```world``` table. It contains the schema in json as well a default permission column, for new objects in that table.

- world_column

Each column known to Goms will have an entry in world_column table. It also contains the metadata about the column. 


## Tech stack

- Golang
- Semantic UI
- JSONAPI
- VueJS