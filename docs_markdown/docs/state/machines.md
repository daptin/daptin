# State machine

State of an object can help you tracing any sort of progress while making sure you maintain the consistence of the state. For eg, you might want to track the status of a "blog post" in terms of "draft"/"edited"/"published" which pre-defined endpoints defining the flow of states.

# Defining a state machine

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


