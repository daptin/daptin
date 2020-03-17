# What are actions

Actions are the most useful concept in Daptin and we will see that everything you have done in Daptin was an action api call.

Actions can be thought of as follows:

- A set of inputs (key value pair)
- A set of outcomes based on the inputs

## What are actions and why do I need this

Create/Read/Update/Delete (CRUD) APIs are only basic APIs exposed on the database, and you would rarely want to make those API available to your end user. Reasons could be multiple

- The end user doesn't (immediately) owe the data they create
- Creating a "row"/"data entry" entry doesn't signify completion of a process or a flow
- Usually a "set of entities" is to be created and not just a single entity (when you create a user, you also want to create a usergroup also and associate the user to usergroup)
- You could allow user to update only some fields of an entity and not all fields (eg user can change their name, but not email)
- Changes based on some entity (when you are going though a project, a new item should automatically belong to that project)


Actions provide a powerful abstraction over the CRUD and handle all of these use cases.

To quickly understand what actions are, lets see what happened when you "signed up" on Daptin.

Take a look at how "Sign up" action is defined in Daptin. We will go through each part of this definition
An action is performed on an entity. Let's also remember that ```world``` is an entity itself.

## Action schema

```golang
	{
		Name:             "signup",
		Label:            "Sign up",
		InstanceOptional: true,
		OnType:           "user_account",
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
				Type:      "user_account",
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



## Action Name

		Name:             "signup",

Name of the action, this should be unique for each actions. Actions are identified by this name

## Action Label

		Label:            "Sign up",

Label is for humans


## OnType

		OnType:           "user_account",

The primary type of entity on which the action happens. This is used to know where the actions should come up on the UI


## Action instance

		InstanceOptional: true,

If the action requires an "instance" of that type on which the action is defined (more about this below). So "Sign up" is defined on "user" table, but an instance of "user" is not required to initiate the action. This is why the "Sign up" doesnt ask you to select a user (which wouldn't make sense either)


## Input fields

        InFields: []api2go.ColumnInfo

This is a set of inputs which the user need to fill in to initiate that action. As we see here in case of "Sign up", we ask for four inputs

- Name
- Email
- Password
- Confirm password

Note that the ColumnInfo structure is the same one we used to [define tables](/setting-up/entities).


## Validations

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

## Conformations


        Conformations: []ColumnTag

Conformations help to clean the data before the action is carried out. The frequently one used are ```trim``` and ```email```.

- Trim: trim removes white spaces, which are sometimes accidently introduced when entering data
- Email: email conformation will normalize the email. Things like lowercase + trim


## OutFields


        OutFields: []Outcome

OutFields are the list of outcomes which the action will result in. The outcomes are evaluated in a top to bottom manner, and the result of one outcome is accessible when evaluating the next outcome.


We have defined three outcomes in our "Sign Up" action.

- Create a user


			{
				Type:      "user_account",
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


## Scripted fields - "!..."

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


## Referencing previous outcomes

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