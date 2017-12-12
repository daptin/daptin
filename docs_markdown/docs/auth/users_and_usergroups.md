# User Management

Daptin natively manages users and usergroups so that it has no dependency on external user management services. Though it can be integrated with such services.

## Users

Users are native objects in Daptin. Every item in daptin belongs to one user. A user which is not identified is a guest user. User identification is based on the JWT token in the ```Authorization``` header

By default each user has one usergroup. A user can belong more user groups.

## User groups

User groups is a group concept that helps you manage "who" can interact with daptin, and in what ways.

Users and Objects belong to one or more user group.

