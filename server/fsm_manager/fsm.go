package fsm_manager

type FsmManager interface {
  ApplyEvent(subject map[string]interface{}, stateMachineEvent StateMachineEvent) (string, error)
}

type StateMachineEvent interface {
  GetStateMachineInstanceId() string
  GetEventName() string
}


