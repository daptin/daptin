# State tracking

State of an object can help you tracing any sort of progress while making sure you maintain the consistence of the state. For eg, you might want to track the status of a "blog post" in terms of "draft"/"edited"/"published" which pre-defined endpoints defining the flow of states.

Tracking the status of things is one of the most common operation in most business flows. Daptin has a native support for state tracking and allows a lot of convienent features.


## Defining a state machine

Define a state machine in YAML or JSON as follows:

!!! note "State machine description YAML"
    ```yaml
    StateMachineDescriptions:
    - Name: task_status
      Label: Task Status
      InitialState: to_be_done
      Events:
      - Name: start
        Label: Start
        Src:
        - to_be_done
        - delayed
        Dst: started
      - Name: delayed
        Label: Unable to pick up
        Src:
        - to_be_done
        Dst: delayed
      - Name: ongoing
        Label: Record progress
        Src:
        - started
        - ongoing
        Dst: ongoing
      - Name: interrupted
        Label: Interrupted
        Src:
        - started
        - ongoing
        Dst: interrupted
      - Name: resume
        Label: Resume from interruption
        Src:
        - interrupted
        Dst: ongoing
      - Name: completed
        Label: Mark as completed
        Src:
        - ongoing
        - started
        Dst: completed
    ```

Using state machine descriptions with daptin expose couple of super useful apis to manage state based data.

Enabling `task_status` state machine on `todo` entity will expose the following APIs


```bash
POST /track/start/:stateMachineId {"typeName": "todo", "referenceId": "objectId"} # Start tracking a particular object by id
```

This returns a state machine id.

```bash
POST /track/event/:typename/:objectStateMachineId/:eventName {} # Trigger event on current state
```

This either moves the `object state` to next state, or fails on invalid event name.



## State machine

A state machine is a description of "states" which the object can be in, and list of all valid transitions from one state to another. Let us begin with an example:

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


## REST API

### Start tracking an object by state machine reference id


Request
```
	POST  /track/start/:stateMachineId
	{"typeName": <entityTypeName>, "referenceId": <ReferenceIdOfTheObject> }
```

Response
```
		"current_state": <InitialStateOfTheStateMachine>
		"<typename>_smd": <ObjectStateInstanceReferenceId>
		"is_state_of_<typename>" = <ObjectInstanceId>
		"permission": <AuthPermission>
```

### Trigger an event by name in the state of an object

```
	POST  /track/event/:typename/:ObjectStateInstanceReferenceId/:eventName
```
Response
```
		"current_state": <NewStateAfterEvent>
		"<typename>_smd": <ObjectStateInstanceReferenceId>
		"is_state_of_<typename>" = <ObjectInstanceId>
```



## Enabling state tracking for entity

Begin with marking an entity as trackable. To do this,

- go to the world tables page and edit the an entity

- Check the "Is state tracking enabled" checkbox

This "is_state_tracking_enabled" options tells daptin to create the associated state table for the entity. Even though we have not yet specified which state machines are available for this entity.

To make a state machine available for an entity, go to the "SMD" tab of this entity on the same page and add the state machine by searching it by name and adding it.

It would not make a lot of sense if the above state machine was allowed for all type of entities.
