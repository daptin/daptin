# Data storage

Daptin relies on a relational database for all data persistence requirements. As covered in the installation currently the following relational database are supported:

- MySQL
- PostgreSQL
- SQLite

This document goes into the detail of how the database is used and what are the tables created.

## Standard columns


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

## World table

The ```world``` table holds the structure for all the entities and relations (including for itself).

Each row contains the schema for the table in a "world_schema_json" column.
