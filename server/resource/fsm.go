package resource

type FsmManager interface {
	ApplyEvent(subject map[string]interface{}, stateMachineEvent StateMachineEvent) (string, error)
}

type simpleStateMachinEvent struct {
	machineReferenceId string
	eventName          string
}

func NewStateMachineEvent(machineId string, eventName string) StateMachineEvent {
	return &simpleStateMachinEvent{
		machineReferenceId: machineId,
		eventName:          eventName,
	}
}

func (f *simpleStateMachinEvent) GetStateMachineInstanceId() string {
	return f.machineReferenceId
}
func (f *simpleStateMachinEvent) GetEventName() string {
	return f.eventName
}

type StateMachineEvent interface {
	GetStateMachineInstanceId() string
	GetEventName() string
}
