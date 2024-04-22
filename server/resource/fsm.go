package resource

import daptinid "github.com/daptin/daptin/server/id"

type FsmManager interface {
	ApplyEvent(subject map[string]interface{}, stateMachineEvent StateMachineEvent) (string, error)
}

type simpleStateMachinEvent struct {
	machineReferenceId daptinid.DaptinReferenceId
	eventName          string
}

func NewStateMachineEvent(machineId daptinid.DaptinReferenceId, eventName string) StateMachineEvent {
	return &simpleStateMachinEvent{
		machineReferenceId: machineId,
		eventName:          eventName,
	}
}

func (f *simpleStateMachinEvent) GetStateMachineInstanceId() daptinid.DaptinReferenceId {
	return f.machineReferenceId
}
func (f *simpleStateMachinEvent) GetEventName() string {
	return f.eventName
}

type StateMachineEvent interface {
	GetStateMachineInstanceId() daptinid.DaptinReferenceId
	GetEventName() string
}
