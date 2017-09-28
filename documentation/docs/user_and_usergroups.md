# Users

Users are native objects in Goms. Every item in goms belongs to one user. A user which is not identified is a guest user.

A user belongs to one or more user groups.

# User groups

User groups is a really powerful concept that helps you manage "who" can interact with goms, and in what ways.

Objects can also belong to one or more user group.


# Permission

There are three type of interactions which we want to control

- Read access
- Write access - this includes creating/updating/deleting
- Actions - this includes actions which can be performed on the objects


## Access flow

Every "intercation" in goms goes through two levels of access

- Entity level access: does the user invoking the interaction has the appropriate permission to invoke this (So for sign up, the user table need to be writable by guests, for sign in the user table needs to be readable by guests)
- Instance level access: this is the second level, even if a user has access to "user" entity, not all "user" rows would be accessible by them


## Entity level permission

Entity level permission are set in the world table and can be updated from dashboard. This can be done by updating the "permission" column for the entity.

For these changes to take effect a restart is necessary.

## Instance level permission

Like we saw in the [entity documentation](entity.md), every table has a ```permission``` column.


## Permission column

The permission column contains a three digit number, which decides the access for guests, user groups and owner

Permission model is completely based on linux file system permission. No need to worry if you are not aware of that. Here is a brief overview

The three digits can be represented as follows:

```U G W```


U = User
G = Group
W = World

4 = Readable
2 = Writable
1 = Execute action
0 = No permission

Here is another way of looking at it:

Permissions:

400 read by owner
040 read by group
004 read by anybody (other)
200 write by owner
020 write by group
002 write by anybody
100 execute by owner
010 execute by group
001 execute by anybody

To get a combination, just add them up.

For example, to get

- read, write, execute by owner
- read, execute, by group
- execute by anybody

you would add 400+200+100+040+010+001 to give 751.