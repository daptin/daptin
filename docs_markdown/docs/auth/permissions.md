# Permission model

Every read/write to the system passes through two level of permission check.

- Type level: apply permission on all types of entities at the same time
- Data level: object level permission


The `world` table contains two columns:

- `Permission`: defines the entity level permission
- `Default permission`: defines the default permission for a new object of this entity type

The default permission for an object is picked from the default permission setting, and can be changed after the object creation (if the permission allows).

## Peek

**Peek** gives access to the user to read data in the system but not allow it in response as data. So while the query to read the data will execute and certain **actions** can be allowed over them, directly trying to read the data in response will fail.

## [C] Create

**Create** allows a new row to be created by using the POST api. Note: this doesn't apply over indirect creations using *actions**.

## [R] Read

**Read** allows the data to be served in the http response body. The response will usually follow the JSONAPI.org structure.

## [U] Update

**Update** allows the data fields to be updated using the PUT/PATCH http methods.

## [D] Delete

**Delete** gives permission to be delete a row or certain type of data using DELETE http method. Unless you have enabled **auditing**, you will permanently loose this data.

## [R] Refer

**Refer** gives permission to add data/users to usergroups. Note that you will also need certain permission on the **usergroup** as well.

## [X] Execute

**Execute** gives permission to invoke action over data (like export). Note that giving access to a **type of data** doesn't give access to all rows of that **entity type**.

