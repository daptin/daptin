
<a href="#">
    <img src="https://github.com/artpar/goms/raw/master/gomsweb/static/img/logo_blk.png" alt="GoMS logo" title="GoMS" align="right" height="50" />
</a>

# Goms


 [![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy) [![Build Status](https://travis-ci.org/artpar/goms.svg?branch=master)](https://travis-ci.org/artpar/goms) [![Build Status](https://semaphoreci.com/api/v1/artpar/goms/branches/master/badge.svg)](https://semaphoreci.com/artpar/goms)


## A BAAS


Goms is a self-hosted BAAS that generates Web APIs (more specifically, JSON APIs) for your data. Goms combines the best concepts of [the most popular BAAS platforms](http://baas.apievangelist.com/) into a simple run-time tool.


Goms takes features such as

- Entity names
- Relations between entities
- Actions related to different entities
- Users, User groups and their relation to entities
- Integration with external api's

and then generates standards-based JSON based Web APIs with these features baked in.


Since Goms generates JSON API compliant web apis, they can work with many popular languages right out of the box using standard JSON clients, and can be used without a framework because they are just JSON rest APIs. Goms also enables a number of key capabilities on top of Web APIs, in particular Sub site hosting (SSH) without the need to run separate server, an events-actions-outcomes framework, and data-as-objects (instead of just strings).

Compared to building JSON APIs directly, Goms provides extra APIs that makes writing fast frontend apps simpler.

APIs like user signup and registration, create, read, write, update for all entities, and actions with multiple outcome chains, make fast, powerful frontends easy to create, while still maintaining 100% compatibility with common standards.

The user experience is also tuned, and comes with fully featured dashboard and various site designs baked in to bootstrap.


## Why Goms?


Goms was to help build faster, more capable APIs over your data that worked across for all types of frontend.

While Goms primarily targeted Web apps, the emergence of Android and iOS Apps as a rapidly growing target for developers demanded a different approach for building the backend. With developers classic use of traditional frameworks and bundling techniques, we struggle to invest enough time in the business and frontend demands for all sorts of Apps that provide consistent and predictable APIs which perform equally well on fast and slow load, across a diversity of platforms and devices.

Additionally, framework fragmentation had created a APIs development interoperability nightmare, where backend built for one purpose needs a lot of boilerplate and integration with the rest of the system, in a consistent way.

A component system around JSON APIs offered a solution to both problems, allowing more time available to be invested into frontend and business building, and targeting a standards-based JSON/Entity models that all frontends can use.

However, JSON APIs for data manipulation by themselves weren't enough. Building apps required a lot of custom actions, workflows, data integrity, event subscription, integration with external services that were previously locked up inside of traditional web frameworks. Goms was built to pull these features out of traditional frameworks and bring them to the fast emerging JSON API standard in an automated way.



## Quick run

```
docker run -d -p 8080:8080 goms/goms
```

Then open [Goms Dashboard](http://localhost:8080) to sign up and sign-in

## Getting started


- Deploy instance of Goms on a server
- Upload JSON/YAML/TOML/HCL file which describe your entities
- or upload XLS file to create entities and upload data
- Become Admin of the instance (until then its a open for access, that's why you were able to create an account)


## Use cases

Examples

Define your own entities | Define relations | Subscribe to events | Sync changes with other systems | Deploy back end server
--- | --- | --- | --- | ---
Todo | Belongs to project | Send SMS if deadline is today | Update a Google Sheet when todo updated | Build an android app
Todo | Has current status | Update manager when employee updates a todo | SMS when todo is marked complete | Build an Electron app
Cooking Recipe | Has many Ingredients | Get Slack notification when anyone adds new Recipe | Get recipe from Google sheets | Build a quick angular app 
Wedding | has many people called "attendees" | Send everyone SMS on updates to wedding party schedule | Calender changes with every attendees calender | Build a UI using React


## How can you use GoMS

- Goms uses a SQL database and works like a very high level framework/management system
- Goms asks you to define your domain entities along with their relations in the way you want to organise them.
- Goms takes the responsibility of giving you following:

  - A responsive dashboard to interact with the system, tested on desktop browsers and mobile browsers
  - A in-built event framework which you can hook to
  - User notifications - Email/Sms/Messengers/Dashboard
  - Actions - Which can be hooked to events, and have multiple outcomes
  - A status tracking system (Visually design a state machine and make it available for any kind of object)


GoMS is a platform which can be customised using Schema files, which describe your requirements and processes.

## Tech Goals

- Zero config start (sqlite db for fresh install, data can be moved to mysql/postgres using goms)
- A closely knit set of components which work together
- Completely configurable at runtime, can be run without any dev help
- Stateless
- Try to piggyback on used/known standards
- Runnable on all types on devices
- Cross platform app using [qt](https://github.com/therecipe/qt) (very long term goal. A responsive website for now.)


## Competitor products

It will be untrue to say Goms has no competition. These are the possible competing products:

- [Directus](https://getdirectus.com/) - Directus is an API-driven content management framework for custom databases. It decouples content for use in apps, websites, or any other data-driven projects.
- [Cockpit](https://getcockpit.com/) - An API-driven CMS
- [Contentful](https://www.contentful.com/) - Contentful is the essential content management infrastructure for projects of any size, with its flexible APIs and global CDN.
- [Scaphold](https://scaphold.io/) - GraphQL Backend As A Service


*Todo*: complete research and fill this table

|                                           | Goms | Cockpit | Contentful | Scaphold | Airtable | graph.cool | fieldbook |
|-------------------------------------------|------|---------|------------|----------|----------|------------|-----------|
| JSON API                                  | Yes  | Yes     | Yes        | Yes      | Yes      | Yes        | Yes       |
| User defined entities                     | Yes  | Yes     | Yes        | Yes      | Yes      | Yes        | Yes       |
| Dashboard                                 | Yes  | Yes     | Yes        | Yes      | Yes      | Yes        | Yes       |
| In built analytics on your data           |      |         |            |          |          |            |           |
| Relations in entities                     |      |         |            |          |          |            |           |
| Users                                     |      |         |            |          |          |            |           |
| User groups                               |      |         |            |          |          |            |           |
| Authentication (In built User Management) |      |         |            |          |          |            |           |
| Authorization (Roles and Permission)      |      |         |            |          |          |            |           |
| Asset management                          |      |         |            |          |          |            |           |
| Revision History/Auditing                 |      |         |            |          |          |            |           |
| Field data types                          |      |         |            |          |          |            |           |
| Integrate external API                    |      |         |            |          |          |            |           |
| Workflows (If this than that)             |      |         |            |          |          |            |           |
| Cloud store for assets                    |      |         |            |          |          |            |           |
| Sub sites                                 |      |         |            |          |          |            |           |
| Marketplace for plugins                   |      |         |            |          |          |            |           |
| Transformed Streams/Views of your data    |      |         |            |          |          |            |           |
|                                           |      |         |            |          |          |            |           |


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
* [ ] Data conversion/exchange/transformations
* [x] Live editor for subsites - grapesjs
* [x] Store connectors for storing big files/subsites - rclone
* [ ] Market place to allow plugins/extensions to be installed

### Documentation

- Checkout the [wiki](https://github.com/artpar/goms/wiki)


## Tech stack


Backend | FrontEnd | Standards | Frameworks
---|---|---|---
[Golang](golang.org) | [BootStrap](http://getbootstrap.com/) | [RAML](raml.org) | [CoPilot Theme](https://copilot.mistergf.io)
[Api2go](https://github.com/manyminds/api2go) | [BootStrap](http://getbootstrap.com/) | [JsonAPI](jsonapi.org) | [VueJS](https://vuejs.org/v2/guide/)
[rclone](https://github.com/ncw/rclone) |  [grapesJs](grapesjs.com) | | [Element UI](element.eleme.io)
