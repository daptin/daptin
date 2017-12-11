# Daptin

Daptin is a ready-to-deploy api-first backend. 

You can setup daptin on any machine/server of your choice.

## Native binary

Daptin is available as a native binary. You can fetch the lastest binary from the releases

[https://github.com/daptin/daptin/releases](https://github.com/daptin/daptin/releases)

To start daptin, execute ```./daptin``` which will create a local sqlite database and start listening on port 6336. To change the database or port, read below.

## Docker

A docker image is also available which can be deployed on any docker compatible hosting provider (aws, gce, linode, digitalocean, azure)

[https://hub.docker.com/r/daptin/daptin/](https://hub.docker.com/r/daptin/daptin/)

To start daptin using docker

```docker run daptin/daptin```

## Database and data persistence

Daptin can use one of the following database for data persistence

- Mysql
- Postgres
- SQLite [Default]

If nothing specified, a sqlite database is created on the local file system and is used for all purposes. (uploads/blobs are not stored in database)

You can customise the database connection properties when starting daptin

### mysql

To use mysql, start daptin as follows

```./daptin -db_type=mysql -db_connection_string='<username>:<password>@tcp(<hostname>:<port>)/<db_name>'```

### postgres

```./daptin -db_type=postgres -db_connection_string='host=<hostname> port=<port> user=<username> password=<password> dbname=<db_name> sslmode=enable/disable'```

### sqlite

By default a "daptin.db" file is created to store data

```./daptin -db_type=sqlite -db_connection_string=db_file_name.db```

## Port

Daptin will listen on port 6336 by default. You can change it by using the following argument

```-port=8080```

## Restart

Daptin relies on self restarts to configure new entities and apis and changes to the other parts of the ststem. As soon as you upload a schema file, daptin will write the file to disk, and restart itself. When it starts it will read the schema file, make appropriate changes to the database and expose JSON apis for the entities and actions.

You can issue a daptin restart from the dashboard. Daptin takes about 15 seconds approx to start up and configure everything.


# Data 

## Entites

Entities are the foundation for daptin. Everything in daptin work on one or more entities. To define an entity, we specify the fields and its relations with other entities. A collection of entities and their relations will be called a schema.

Multiple schema json files can be uploaded, and changes are merged accordingly.

Lets imagine we were creating a todo application and wanted to keep a track the following for each todo item

- the todo text - lets call this "title"
- a description - a longer text, which may or may not be empty
- a deadline date - a date field to capture the deadline
- completed - a true/false field, which captures if the todo is done
- order - a field to store the priority of each todo

Along with the fields mentioned above, we might want certain validations and conformations whenever we store a new todo

- title cannot be empty
- order has to be numeric

Once we have come up with the above picture in mind, we can use one of the following ways to tell daptin about this:

## Online entity designer

The entity designer is accessible from dashboard using the "Online designer" button. Here you can set the name, add columns and relations and create it. This is a basic designer and more advanced features to customise every aspect of the entity will be added later.

![Entity designer](images/create_entity.png)


## Market place

Checkout [marketplace documentation]


## JSON / YAML files

JSON/YAML files are the primary way to create new entites in daptin. The above two ways ultimatele create a JSON file or fetch from the market.

The JSON for our hypothetical todo entity will look as follows:

```json
    {
        "Tables": [{
            "TableName": "todo",
            "Columns": [{
                    "Name": "title",
                    "DataType": "varchar(500)",
                    "ColumnType": "label",
                    "IsIndexed": true
                },
                {
                    "Name": "completed",
                    "DataType": "int(1)",
                    "ColumnType": "truefalse",
                    "DefaultValue": "false"
                },
                {
                    "Name": "deadline",
                    "DataType": "date",
                    "ColumnType": "date",
                    "IsNullable": true
                },
                {
                    "Name": "order",
                    "ColumnName": "item_order",
                    "DataType": "int(4)",
                    "ColumnType": "measurement",
                    "DefaultValue": "10"
                },
                {
                    "Name": "text",
                    "DataType": "text",
                    "ColumnType": "content",
                    "IsNullable": true
                }
            ],
            "Conformations": [{
                "ColumnName": "order",
                "Tags": "numeric"
            }],
            "validations": [{
                "ColumnName": "title",
                "Tags": "required"
            }]
]}
```


- Name: Name is a human readable name
- Column Name: Name of the column in the table
- Column Type: The type of the column. Daptin supports a variety of types and these allow daptin to give you useful options in future (eg for viewing a timeline, a date/datetime column is required)
- Default value: Columns can have default values, which is used a new row is created and no value for that column is specified.

While the same description in YAML will look as follows

```yaml
Tables:
- TableName: todo
  Columns:
  - Name: title
    DataType: varchar(500)
    ColumnType: label
    IsIndexed: true
  - Name: url
    DataType: varchar(200)
    ColumnType: url
    IsNullable: true
  - Name: completed
    DataType: int(1)
    ColumnType: truefalse
    DefaultValue: 'false'
  - Name: schedule
    DataType: date
    ColumnType: date
    IsNullable: true
  - Name: order
    ColumnName: item_order
    DataType: int(4)
    ColumnType: measurement
    DefaultValue: '10'
  - Name: text
    DataType: text
    ColumnType: content
    IsNullable: true
  Conformations:
  - ColumnName: order
    Tags: numeric
  Validations:
  - ColumnName: title
Tags: required
```

You can choose to work with either json or yaml. Once the schema is ready, it can be uploaded directly from daptin dashboard.

## Column specifications

Columns of the entity can be cusomised:

Property Name | Property Type | Description
--- | --- | ---
	Name       |       string |        human readable name, can be skipped
	ColumnName  |      string |        column name in the table
	ColumnDescription| string |        human readable description
	ColumnType       | string |        column type is a rich type of the column
	IsIndexed        | boolean|        true to add an index on this column
	IsUnique         | boolean|        true to set a unique constraint on this column
	IsNullable       | boolean|        are null values allowed
	Permission       | uint64 |        permission column (check authorization docs)
	DataType         | string |        the column type inside the database
	DefaultValue     | string |        default value if any (has to be inside single quotes for static values

## Column types

Daptin supports a variety of rich data types, which helps it to automatically make intelligent decisions and validations. Here is a list of all column types and what should they be used for

Type Name | Description | Example
--- | --- | ---
	 id |an identity column, mostly for internal purposes | 1
	 alias|a foreign key column | uuid v4
	 date|full date, no time| 2017-12-30
	 time|time/time interval, no date| 12:34:54
	 day| day of the month|1 to 31
	 month|month of the year| 1 to 12
	 year|Year| 2017
	 minute|minute of the hour|0 to 59
	 hour|hour of the dat| 0 - 23
	 datetime|date + time (not stored as timestamp, served at date time string)| 2017-12-30T12:34:54
	 email|email| test@domain.com
	 name|column to be used as name of the entity| daptin
	 json|JSON data| ```{}```
	 password|password - are bcrypted with cost 11|$2a$11$z/VlxycDgZ...
	 value|value is enumeration type| completed
	 truefalse|boolean| 1
	 timestamp|timestamp (stored as timestamp, served as timestamp)| 123123123
	 location.latitude|only latitude|34.2938
	 location|latitude + longitude in geoJson format|[34.223,64.123]
	 location.longitude|only longitude| 64.123
	 location.altitude|only altitude| 34
	 color|hex color string |#ABCDE1
	 rating.10|rating on a scale of 10|8
	 measurement|numeric column|534
	 label|a label for the entity, similar to name but can be more than one| red
	 content|larger contents - texts/html/json/yaml| very long text
	 file|uploads, connect storage for using this|
	 url| Urls/links| http://docs.dapt.in

## Excel file upload

Excel upload provides an easy way to create entities. This takes away the complexity of writing each column type. Daptin uses a combination of rules to identify columns and their types based on the data in the excel.

You can upload data from XLS. Daptin will take care of going through your XLS file and identifying column types. This is one of the easiest and fastest ways to create entities and uploading data in daptin. You can specify relations among entities later from the online designer.


# Relations

## Entity relations

A data oriented system with no relational knowledge of the data is next to an Excel sheet. Specifying relations in your data is the most important thing after creating your entities.

### Relations in JSON/YAML schema

When uploading schema using a JSON / YAML file, relations can be added in the same file and daptin will create appropriate constraints and foreign keys in your underlying database.

Continuing with our example of todos, lets say we want to group todo's in "projects" and each todo can belong to only a single project.

Lets design a "project" entity:

```yaml
- TableName: project
  Columns:
  - Name: name
    DataType: varchar(200)
    ColumnType: name
    IsIndexed: true
```

A very simple table with just a name column. Now we can tell daptin about the relation between todos and projects

```yaml
Relations:
- Subject: todo
  Relation: has_one
  Object: project
```

This tells daptin that todo "has_one" project.

### Relations types

Any entity can be associated to any other entity (or to itself) as one of the follows

Relation Name | Relation Descriptio | Can be empty 
--- | --- | ---
belongs_to | a single object relation | No
has_one | a single object relation | Yes
has_many | many related objects | Yes

### Default relations

Every entity created on daptin has at least two relations

Relation Type | Related Entity 
--- | ---
belongs | user
has many | usergroup

To understand why these two relations will always exist, checkout [daptin authorization model]


### Multiple relation

There can be a scenario where two entities are related in more then 1 way. Consider the following example

- A blog entity
- A post entity
- Blog has many posts
- Each blog can have a "highlighted post" (blog has one "highlighted post")

To achieve the above scenario, our schema would look like as follows

```yaml
Tables:
- TableName: blog
  Columns:
  - Name: title
    DataType: varchar(500)
    ColumnType: label
  - Name: view_count
    DataType: int(11)
    ColumnType: measurement
- TableName: post
  Columns:
  - Name: title
    DataType: varchar(200)
    ColumnType: label
  - Name: body
    DataType: text
    ColumnType: content
- TableName: comment
  Columns:
  - Name: body
    DataType: text
    ColumnType: content
  - Name: likes_count
    ColumnName: likes_count
    DataType: int(11)
    ColumnType: measurement
Relations:
- Subject: comment
  Relation: belongs_to
  Object: post
- Subject: post
  Relation: belongs_to
  Object: blog                   // this is our post belongs to blog relation
- Subject: blog
  Relation: has_one
  Object: post
  ObjectName: current_post
  SubjectName: current_post_of   // this is our highlighted post relation
```

Notice the "SubjectName" and "ObjectName" keys which helps to name our relations more intuitively.


### SQL constraints

#### belongs to

- A column is added to the subject entity, which refers to the Object entity, set to non nullable

#### has one

- Same as above, but nullable

#### has many

- A join table is created

# Extensions

## Marketplace

Documentation not ready yet

## Data storage

Daptin relies on a relational database for all data persistence requirements. As covered in the [setting up guide] currently the following relational database are supported:

- MySQL
- PostgreSQL
- SQLite

This document goes into the detail of how the database is used and what are the tables created.

### Standard columns


The following 5 columns are present in every table

| ColumnName   | ColumnType  | DataType    | Attributes                                           |
|--------------|-------------|-------------|------------------------------------------------------|
| id           | id          | int(11)       | primary key  Auto increment Never exposed externally |
| version      | integer     | int(11)       | get incremented every time a change is made          |
| created_at   | timestamp   | timestamp   | the timestamp when the row was created               |
| updated_at   | timestamp   | timestamp   | the timestamp when the row was last updated          |
| reference_id | alias       | varchar(40) | The id exposed in APIs                               |
| permission   | integer     | int(4)      | Permissions - check Authorization documentation      |
| user_id      | foreign key | int(11)       | the owner of this object                             |

Other columns are created based on the schema. 

The ```id``` column is completely for internal purposes and is never exposed in an JSON API.
Every row of data inherently belongs to one user. This is the user who created that row. The associated user can be changed later.

### World table

The ```world``` table holds the structure for all the entities and relations (including for itself).

Each row contains the schema for the table in a "world_schema_json" column.

## Data Audits

All changes in daptin are recorded and history is maintained in audit tables. Audit table are entities just like regular entities. All Patch/Put/Delete calls to daptin will create an entry if the audit table if the entity is changed.

### Audit tables

For any entity named ```x```, another tables ```x_audit``` is added by daptin. The audit table will contain all the columns which are present in the original table, plus an extra column ```is_audit_of``` is added, which contains the ID of the original row. The ```is_audit_of``` is a foreign key column to the parent tables ```id``` column.

### Audit row

Each row in the audit table is the copy of the original row just before it is being modified. The audit rows can be accessed just like any other relation.

### Audit table permissions

By default, everyone has the access to create audit row, and noone has the access to update or delete them. These permissions can be changed, but it is not recommanded at present.

Type | Permission
--- | ---
Audit table permission | 007007007
Audit object permission | 003003003

## Data validation

Daptin uses the excellent [go-playground/validator](https://github.com/go-playground/validator) library to provide extensive validations when creating and updating data.

It gives us the following unique features:

- Cross Field and Cross Struct validations by using validation tags or custom validators.
- Slice, Array and Map diving, which allows any or all levels of a multidimensional field to be validated.

## Data conformations

Daptin uses the excellent [leebenson/conform](https://github.com/leebenson/conform) library to apply conformations on data before storing them in the database

- Conform: keep user input in check (go, golang)
- Trim, sanitize, and modify struct string fields in place, based on tags.

Use it for names, e-mail addresses, URL slugs, or any other form field where formatting matters.

Conform doesn't attempt any kind of validation on your fields.


## State tracking for entities

Tracking the status of things is one of the most common operation in most business flows. Daptin has a native support for state tracking and allows a lot of convienent features.

### State machine

A state machine is a description of "states" which the object can be in, and list of all valid transactions from one state to another. Lets begin with an example:

The following JSON defines a state machine which has (a hypothetical state machine for tracking todos):

- Initial state: to_be_done
- List of valid states: to_be_done, delayed, started, ongoing, interrupted, completed
- List of valid transitions, giving name to each event

```json
		{
        "Name": "task_status",
        "Label": "Task Status",
        "InitialState": "to_be_done",
        "Events": [{
                "Name": "start",
                "Label": "Start",
                "Src": [
                    "to_be_done",
                    "delayed"
                ],
                "Dst": "started"
            },
            {
                "Name": "delayed",
                "Label": "Unable to pick up",
                "Src": [
                    "to_be_done"
                ],
                "Dst": "delayed"
            },
            {
                "Name": "ongoing",
                "Label": "Record progress",
                "Src": [
                    "started",
                    "ongoing"
                ],
                "Dst": "ongoing"
            },
            {
                "Name": "interrupted",
                "Label": "Interrupted",
                "Src": [
                    "started",
                    "ongoing"
                ],
                "Dst": "interrupted"
            },
            {
                "Name": "resume",
                "Label": "Resume from interruption",
                "Src": [
                    "interrupted"
                ],
                "Dst": "ongoing"
            },
            {
                "Name": "completed",
                "Label": "Mark as completed",
                "Src": [
                    "ongoing",
                    "started"
                ],
                "Dst": "completed"
            }
        ]
    }

```

State machines can be uploaded to Daptin just like entities and actions. A JSON/YAML file with a ```StateMachineDescriptions``` top level key can contain an array of state machine descriptions.


### Enabling state tracking for entity

First we need to tell goms that an entity is trackable. To do this, go to the world table page and edit the corresponding entity. Check the "Is state tracking enabled" checkbox.

This "is_state_tracking_enabled" options tells daptin to create the associated state table for the entity. Even though we have not yet specified which state machines are available for this entity.

To make a state machine available for an entity, go to the "SMD" tab of the entity and add the state machine by searching it by name.

It would not make a lot of sense if the above state machine was allowed for all type of entities. Also since state of the objects in maintained in a separate table

![Entity designer](gifs/enable_state_machine_for_todo.gif)

## Actions

Actions are the most useful things in Daptin and we will see that everything you have done in Daptin was as action itself.

Actions can be thought of as follows:

- A set of inputs
- A set of outcomes based on the inputs

### What are actions and why do I need this

Create/Read/Update/Delete (CRUD) APIs are only the most basic apis exposed on the database, and you would rarely want to make those API available to your end user. Reasons could be multiple

- The end user doesn't (immediately) owe the data they create
- Creating a "row"/"data entry" entry doesnt signify completion of a process or a flow
- Usually a "set of entities" is to created and not just a single entity (when you create a user, you also want to create a usergroup also and associate the user to usergroup)
- You could allow user to update only some fields of an entity and not all fields (eg user can change their name, but not email)
- Changes based on some entity (when you are going though a project, a new todo should automatically belong to that project)


Actions provide a powerful abstraction over the CRUD and handle all of these use cases.

To quickly understand what actions are, lets see what happened when you "signed up" on Daptin.

Lets take a look at how "Sign up" action is defined in Daptin. We will go through each part of this definition
An action is performed on an entity. Lets also remember that ```world``` is an entity itself.

### Action schema

```golang
	{
		Name:             "signup",
		Label:            "Sign up",
		InstanceOptional: true,
		OnType:           "user",
		InFields: []api2go.ColumnInfo{
			{
				Name:       "name",
				ColumnName: "name",
				ColumnType: "label",
				IsNullable: false,
			},
			{
				Name:       "email",
				ColumnName: "email",
				ColumnType: "email",
				IsNullable: false,
			},
			{
				Name:       "password",
				ColumnName: "password",
				ColumnType: "password",
				IsNullable: false,
			},
			{
				Name:       "Password Confirm",
				ColumnName: "passwordConfirm",
				ColumnType: "password",
				IsNullable: false,
			},
		},
		Validations: []ColumnTag{
			{
				ColumnName: "email",
				Tags:       "email",
			},
			{
				ColumnName: "name",
				Tags:       "required",
			},
			{
				ColumnName: "password",
				Tags:       "eqfield=InnerStructField[passwordConfirm],min=8",
			},
		},
		Conformations: []ColumnTag{
			{
				ColumnName: "email",
				Tags:       "email",
			},
			{
				ColumnName: "name",
				Tags:       "trim",
			},
		},
		OutFields: {
			{
				Type:      "user",
				Method:    "POST",
				Reference: "user",
				Attributes: {
					"name":      "~name",
					"email":     "~email",
					"password":  "~password",
					"confirmed": "0",
				},
			},
			{
				Type:      "usergroup",
				Method:    "POST",
				Reference: "usergroup",
				Attributes: {
					"name": "!'Home group for ' + user.name",
				},
			},
			{
				Type:      "user_user_id_has_usergroup_usergroup_id",
				Method:    "POST",
				Reference: "user_usergroup",
				Attributes: {
					"user_id":      "$user.reference_id",
					"usergroup_id": "$usergroup.reference_id",
				},
			},
			{
				Type:   "client.notify",
				Method: "ACTIONRESPONSE",
				Attributes: {
					"type":    "success",
					"title":   "Success",
					"message": "Signup Successful",
				},
			},
			{
				Type:   "client.redirect",
				Method: "ACTIONRESPONSE",
				Attributes: {
					"location": "/auth/signin",
					"window":   "self",
				},
			},
		},
	}
```



### Action Name

		Name:             "signup",

Name of the action, this should be unique for each actions. Actions are identified by this name

### Action Label

		Label:            "Sign up",

Label is for humans


### OnType

		OnType:           "user",

The primary type of entity on which the action happens. This is used to know where the actions should come up on the UI


### Action instance

		InstanceOptional: true,

If the action requires an "instance" of that type on which the action is defined (more about this below). So "Sign up" is defined on "user" table, but an instance of "user" is not required to initiate the action. This is why the "Sign up" doesnt ask you to select a user (which wouldn't make sense either)


### Input fields

        InFields: []api2go.ColumnInfo

This is a set of inputs which the user need to fill in to initiate that action. As we see here in case of "Sign up", we ask for four inputs

- Name
- Email
- Password
- Confirm password

Note that the ColumnInfo structure is the same one we used to [define tables].


### Validations

        Validations: []ColumnTag

Validations validate the user input and rejects if some validation fails


      {
				ColumnName: "email",
				Tags:       "email",
			},


This tells that the "email" input should actually be an email.


One of the more interesting validations is cross field check

			{
				ColumnName: "password",
				Tags:       "eqfield=InnerStructField[passwordConfirm],min=8",
			},

This tells that the value entered by user in the password field should be equal to the value in passwordConfirm field. And the minimum length should be 8 characters.

### Conformations


        Conformations: []ColumnTag

Conformations help to clean the data before the action is carried out. The frequently one used are ```trim``` and ```email```.

- Trim: trim removes white spaces, which are sometimes accidently introduced when entering data
- Email: email conformation will normalize the email. Things like lowercase + trim


### OutFields


        OutFields: []Outcome

OutFields are the list of outcomes which the action will result in. The outcomes are evaluated in a top to bottom manner, and the result of one outcome is accessible when evaluating the next outcome.


We have defined three outcomes in our "Sign Up" action.

- Create a user


			{
				Type:      "user",
				Method:    "POST",
				Reference: "user",
				Attributes: map[string]interface{}{
					"name":      "~name",
					"email":     "~email",
					"password":  "~password",
					"confirmed": "0",
				},
			},

This tells us that, the first outcome is of type "user". The outcome is a "New User" (POST). It could alternatively have been a Update/Find/Delete operation.

The attributes maps the input fields to the fields of our new user.

- ```~name``` will be the value entered by user in the name field
- ```~email``` will be the entered in the email field, and so on

If we skip the ```~```, like ```"confirmed": "0"``` Then the literal value is used.


```Reference: "user",``` We have this to allow the "outcome" to be referenced when evaluating the next outcome. Let us see the other outcomes


### Scripted fields - "!..."

			{
				Type:      "usergroup",
				Method:    "POST",
				Reference: "usergroup",
				Attributes: map[string]interface{}{
					"name": "!'Home group for ' + user.name",
				},
			},

Daptin includes the [otto js engine](https://github.com/robertkrimen/otto). An exclamation mark tell daptin to evaluate the rest of the string as Javascript.


```'Home group for ' + user.name``` becomes "Home group for parth"


### Referencing previous outcomes

			{
				Type:      "user_user_id_has_usergroup_usergroup_id",
				Method:    "POST",
				Reference: "user_usergroup",
				Attributes: map[string]interface{}{
					"user_id":      "$user.reference_id",
					"usergroup_id": "$usergroup.reference_id",
				},
			},

the ```$``` sign is to refer the previous outcomes. Here this outcome adds the newly created user to the newly created usergroup.

## Streams

Streams are complimentary to [actions]. Think of streams as views in SQL. A stream is basically one entity + set of transformations and filters on the entity. Streams are read-only and exposed with similar semantics of that of entities. Daptin will expose JSONAPI for each stream just like it does for entities.

Here is an example of a stream which exposes list of completed todos only

```
{
		StreamName:     "transformed_user",
		RootEntityName: "todo",
		Columns: []api2go.ColumnInfo{          // List of columns in this stream
			{
				Name:       "transformed_todo_title",  
				ColumnType: "label",
			},
			{
				Name:       "completed_on",
				ColumnType: "datetime",
			},
		},
		QueryParams: QueryParams{
			"Filter": "completed=true",
			"Select": "title,deadline",
		},
		Transformations: []Transformation{
			{
				Operation: "select",
				Attributes: map[string]interface{}{
					"columns": []string{"title", "deadline"},
				},
			},
			{
				Operation: "rename",
				Attributes: map[string]interface{}{
					"oldName": "title",
					"newName": "transformed_todo_title",
				},
			},
			{
				Operation: "rename",
				Attributes: map[string]interface{}{
					"oldName": "deadline",
					"newName": "completed_on",
				},
			},
		},
}	
```

Daptin uses the library [kniren/gota](github.com/kniren/gota/dataframe) to systematically specific list of transformations which are applied to the original data stream.

## User Management

Daptin natively manages users and usergroups so that it has no dependency on external user management services. Though it can be integrated with such services.

### Users

Users are native objects in Daptin. Every item in daptin belongs to one user. A user which is not identified is a guest user. User identification is based on the JWT token in the ```Authorization``` header

By default each user has one usergroup. A user can belong more user groups.

### User groups

User groups is a group concept that helps you manage "who" can interact with daptin, and in what ways.

Users and Objects belong to one or more user group.

## Authentication

Daptin maintains its own user accounts and usergroups as well. Users are identified by ```email``` which is a unique key in the ```user``` entity. Passwords are stored using bcrypt with a cost of 11. Password field has a column_type ```password``` which makes daptin to bcrypt it before storing, and password fields are never returned in any JSONAPI call.


### Authentication token

The authentication token is a JWT token issued by daptin on sign in action. Users can create new actions to allow other means of generating JWT token. It is as simple as adding another outcome to an action.

### Server side

Daptin uses oAuth2 based authentication strategy. HTTP calls are checked for ```Authorization``` header, and if present, validates the token as a JWT token.

The JWT token contains the issuer info (Daptin in this case) plus basic user profile (email). The JWT token has a one hour expiry from the time of issue.

If the token is absent or invalid, the user is considered as a guest. Guests also have certain permissions. Checkout the [Authorization docs] for details. 

### Client side

On the client side, for dashboard, the token is stored in local storage. The local storage is cleared on logout or if the server responds with a 401 Unauthorized status.

### Authentication using other systems

There is planned road map to allow user logins via external oauth2 servers as well (login via google/facebook/twitter... and so on). This feature is not complete yet. Documentation will be updated to reflect changes.

### Sign Up

Sign up is an action on user entity. Sign up takes four inputs:

- Name
- Email
- Password
- PasswordConfirm

When the user initates a Sign up action, the following things happen

- Check if guests can initiate sign in action
- Check if guests can create a new user (create permission)
- Create a new user row
- Check if guests can create a new usergroup (create permission)
- Create a new usergroup row
- Associate the user to the usergroup (refer permission)

This means that every user has his own dedicated usergrou by default. 

### Sign In

Sign In is also an action on user entity. Sign in takes two inputs:

- Email
- Password

When the user initiates Sign in action, the following things happen:

- Check if guests can peek users table (Peek permission)
- Check if guests can peek the particular user (Peek Permission)
- Match if the provided password bcrypted matches the stored bcrypted password
- If true, issue a JWT token, which is used for future calls

The main outcome of the Sign In action is the jwt token, which is to be used in the ```Authorization``` header of following calls.

## Access Authorization

Authorization is the part where daptin decides if the caller has enough permission to execute the call. Currently daptin has the following permissions.

### Entity level permission check

The world table has the list of all entities. Consider the scenario where we created a todo list. The world table would have a row to represent this entity

Entity | Permission
--- | ---
todo | 112000006 |

Here:

- 112 is for owners, which basically means 64 + 32 + 16 = Refer/Execute/Delete
- 000 is for group users, no permission allowed in this case
- 006 is for guest users, which is 2 + 4 = Read/Create


### Object level permission check

Once the call clears the entity level check, an object level permission check is applied. This happens in cases where the action is going to affect/read an existing row. The permission is stored in the same way. Each table has a permission column which stores the permission in ```OOOGGGXXX``` format.

### Order of permission check

The permission is checked in order of:

- Check if the user is owner, if yes, check if permission allows the current action, if yes do action
- Check if the user belongs to a group to which this object also belongs, if yes, check if permisison allows the current action, if yes do action
- User is guest, check if guest permission allows this actions, if yes do action, if no, unauthorized

Things to note here:

- There is no negative permission (this may be introduced in the future)
  - eg, you cannot say owner is 'not allowed' to read but read by guest is allowed. 
- Permission check is done in a hierarchy type order

### Access flow

Every "interaction" in daptin goes through two levels of access. Each level has a ```before``` and ```after``` check.

- Entity level access: does the user invoking the interaction has the appropriate permission to invoke this (So for sign up, the user table need to be writable by guests, for sign in the user table needs to be peakable by guests)
- Instance level access: this is the second level, even if a user has access to "user" entity, not all "user" rows would be accessible by them


So the actual checks happen in following order:

- "Before check" for entity
- "Before check" for instance
- "After check" for instance
- "After check" for entity

Each of these checks can filter out objects where the user does not have enough permission.

### Entity level permission

Entity level permission are set in the world table and can be updated from dashboard. This can be done by updating the "permission" column for the entity.

For these changes to take effect a restart is necessary.

### Instance level permission

Like we saw in the [entity documentation], every table has a ```permission``` column. No restart is necessary for changes in these permission.

### Permission column

The permission column contains a nine digit number, which decides the access for guests (the world), user groups and owner

The nine digits can be represented as follows:

```UUUGGGWWW```

Each entity has a permission field which is added by daptin. The permission field is a 9 digit number, in the following format

The first three digits(UUU) represent the permission for the owner.
The next three digits(GGG) represent the permission for the group.
The last three digits(WWW)  represent the permission for guest users.

U = User
G = Group
W = World

- Peek - 1
- Read - 2
- Create - 4
- Update - 8
- Delete - 16
- Execute - 32
- Refer - 64


Here is another way of looking at it:

Permissions:

002,000,000 read by owner
000,020,000 read by group
000,000,002 read by anybody (other)
004,000,000 write by owner
000,004,000 write by group
000,000,004 write by anybody
032,000,000 execute by owner
000,032,000 execute by group
000,000,032 execute by anybody

To get a combination, just add them up.

For example, to get

- read, write, execute by owner
- read, execute, by group
- execute by anybody

you would add (002 + 004 + 032),(002 + 032),(032) to give 038034032.

## Permission model

The world table contains to relevant columns:

- Permission: defines the entity level permission
- Default permission: defines the default permission for a new object of this entity type

The default permission for an object is picked from the default permission setting, and can be changed after the object creation (if the permission allows so).

## OAuth Connections

Daptin is natively aware of oauth2 flows and can seamlessly handle both oauth tokens and refresh tokens (if provided).

To begin using oauth involved flows (eg GoogleDrive as data storage) first goms need to be told about oauth integration. 

