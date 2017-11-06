# Access Authorization

Authorization is the part where daptin decides if the caller has enough permission to execute the call. Currently daptin has the following permissions.

## Entity level permission check

The world table has the list of all entities. Consider the scenario where we created a todo list. The world table would have a row to represent this entity

Entity | Permission
--- | ---
todo | 112000006 |

Here:

- 112 is for owners, which basically means 64 + 32 + 16 = Refer/Execute/Delete
- 000 is for group users, no permission allowed in this case
- 006 is for guest users, which is 2 + 4 = Read/Create


## Object level permission check

Once the call clears the entity level check, an object level permission check is applied. This happens in cases where the action is going to affect/read an existing row. The permission is stored in the same way. Each table has a permission column which stores the permission in ```OOOGGGXXX``` format.

## Order of permission check

The permission is checked in order of:

- Check if the user is owner, if yes, check if permission allows the current action, if yes do action
- Check if the user belongs to a group to which this object also belongs, if yes, check if permisison allows the current action, if yes do action
- User is guest, check if guest permission allows this actions, if yes do action, if no, unauthorized

Things to note here:

- There is no negative permission (this may be introduced in the future)
  - eg, you cannot say owner is 'not allowed' to read but read by guest is allowed. 
- Permission check is done in a hierarchy type order

## Access flow

Every "interaction" in daptin goes through two levels of access. Each level has a ```before``` and ```after``` check.

- Entity level access: does the user invoking the interaction has the appropriate permission to invoke this (So for sign up, the user table need to be writable by guests, for sign in the user table needs to be peakable by guests)
- Instance level access: this is the second level, even if a user has access to "user" entity, not all "user" rows would be accessible by them


So the actual checks happen in following order:

- "Before check" for entity
- "Before check" for instance
- "After check" for instance
- "After check" for entity

Each of these checks can filter out objects where the user does not have enough permission.

## Entity level permission

Entity level permission are set in the world table and can be updated from dashboard. This can be done by updating the "permission" column for the entity.

For these changes to take effect a restart is necessary.

## Instance level permission

Like we saw in the [entity documentation](/setting-up/entities.md), every table has a ```permission``` column. No restart is necessary for changes in these permission.

## Permission column

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