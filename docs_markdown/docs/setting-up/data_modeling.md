
# Data model

Tables are the basic data structure. Tables have columns. Each column has a particular data type. Tables are exposed as JSON APIs under the `/api/<entityName>` path.

## Automatic creation

Import CSV or XLS file and you can let Daptin create the entities for you based on intelligent data pre-processor.

## Manual creation
## YAML/JSON schema

If you are looking for a more reproducible way, design your entities and create JSON or YAML files. These files can be used again to create an exact same replica.

Multiple schema json files can be uploaded, and changes are merged accordingly.

Lets imagine we were creating a todo application and wanted to keep a track of the following for each todo item

!!! example "Todo list example"
    - the todo text field - title


!!! note "YAML example"
    ```yaml
    Tables:
    - TableName: todo
      Columns:
      - Name: title
        DataType: varchar(500)
        ColumnType: label
        IsIndexed: true
    ```


!!! note "JSON example"
    ```json
    {
      "Tables": [
        {
          "TableName": "todo",
          "Columns": [
            {
              "Name": "title",
              "DataType": "varchar(500)",
              "ColumnType": "label",
              "IsIndexed": true
            }
          ]
        }
      ]
    }
    ```


## Data validations

Along with the fields mentioned above, we might want certain validations and conformations whenever we store a new todo

!!! example "Validations"
    - title cannot be empty
    - order has to be numeric

Once we have come up with the above picture in mind, we can use one of the following ways to tell daptin about this.

Daptin uses the excellent [go-playground/validator](https://github.com/go-playground/validator) library to provide extensive validations when creating and updating data.

It gives us the following unique features:

- Cross Field and Cross Struct validations by using validation tags or custom validators.
- Slice, Array and Map diving, which allows any or all levels of a multidimensional field to be validated.


### Validation Example

### JSON example

JSON files are the primary way to create new entities in daptin. The above two ways ultimately create a JSON file or fetch from the market.

The JSON for our `todo` entity will look as follows:

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


### YAML example

YAML example for `todo` entity is as follows

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


## Online entity designer

The entity designer is accessible from dashboard using the "Online designer" button. Here you can set the name, add columns and relations and create it. This is a basic designer and more advanced features to customise every aspect of the entity will be added later.

![Entity designer](/images/create_entity.png)


## Column specifications

Columns of the entity can be customized:

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
Options     | Array[value,label] |        Valid values if column in enum type

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

## Data relations

A data oriented system with no relational knowledge of the data is next to an Excel sheet. Specifying relations in your data is the most important thing after creating your entities.

Relations are constraints among tables and help you keep clean and consistent data. Relational data is easily accessible over APIs using a path structure like `/api/<entityName>/<id>/<relationName>` and the response is consistent with [JSONAPI.org](https://JSONAPI.org).

Checkout the [relation apis](/apis/relation) exposed by daptin.

!!! note "YAML example"
    ```yaml
    Relations:
    - Subject: todo
      Relation: has_one
      Object: project
    ```

!!! note "JSON example"
    ```json
    {
      "Relations": [
        {
          "Subject": "todo",
          "Relation": "has_one",
          "Object": "project"
        }
      ]
    }
    ```


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

Relation Type | Related Entity | Purpose
--- | --- | ---
belongs | user | owner of the object
has many | usergroup | belongs to usergroup


These relations help you precisely control the authorization for each user.

Read more about [authorization and permissions](/auth/authorization)


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


## Importing data

Upload one of these files:

| File        | Usage                              |
| ----------- | ---------------------------------- |
| Schema JSON | Create schema and apis             |
| CSV         | Auto create entity and upload data |
| XLSX        | Auto create entity and upload data |
| Data JSON   | Upload data from dumps             |



### Excel file upload

Excel upload provides an easy way to create entities. This takes away the complexity of writing each column type. Daptin uses a combination of rules to identify columns and their types based on the data in the excel.

You can upload data from XLS. Daptin will take care of going through your XLS file and identifying column types. This is one of the easiest and fastest ways to create entities and uploading data in daptin. You can specify relations among entities later from the online designer.

### CSV file upload

CSV upload provides an easy way to create entities. This takes away the complexity of writing each column type. Daptin uses a combination of rules to identify columns and their types based on the data in the csv.

You can upload data from CSV. Daptin will take care of going through your XLS file and identifying column types. This is one of the easiest and fastest ways to create entities and uploading data in daptin. You can specify relations among entities later from the online designer.


## Data conformations

Daptin uses the excellent [leebenson/conform](https://github.com/leebenson/conform) library to apply conformations on data before storing them in the database

- Conform: keep user input in check (go, golang)
- Trim, sanitize, and modify struct string fields in place, based on tags.

Use it for names, e-mail addresses, URL slugs, or any other form field where formatting matters.

Conform doesn't attempt any kind of validation on your fields.


## Data auditing

To enable recoding of all historical data for a particular entity, enable data audit for it in the worlds configuration.

Audits are ready only and cannot be manipulated over api. You can configure the permission for your use case.


All changes in daptin can be recorded by enabling **auditing**. History is maintained in separate audit tables which maintain a copy of all columns at each change. Audit table are entities just like regular entities. All Patch/Put/Delete calls to daptin will create an entry in the audit table if the entity is changed.

### Audit tables

For any entity named ```<X>```, another tables ```<X>_audit``` is added by daptin. Eg if you enable auditing of the `user_account` table, then a `user_account_audit` table will be created.

The audit table will contain all the columns which are present in the original table, plus an extra column ```is_audit_of``` is added, which contains the ID of the original row. The ```is_audit_of``` is a foreign key column to the parent tables ```id``` column.

### Audit row

Each row in the audit table is the copy of the original row just before it is being modified. The audit rows can be accessed just like any other relation.

### Audit table permissions

By default, everyone has the access to create audit row, and no one has the access to update or delete them. These permissions can be changed, but it is not recommended at present.

Type | Permission
--- | ---
Audit table permission | 007007007
Audit object permission | 003003003