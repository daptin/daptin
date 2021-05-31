package resource

import (
	"fmt"
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	loopfsm "github.com/looplab/fsm"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type fsmManager struct {
	db    database.DatabaseConnection
	cruds map[string]*DbResource
}

type StateMachineInstance struct {
	CurrestState   string
	StateMachineId int64
	ObjectId       int64
}

func (fsm *fsmManager) getStateMachineInstance(objType string, objId int64, machineInstanceId string) (StateMachineInstance, error) {

	s, v, err := statementbuilder.Squirrel.Select("current_state", objType+"_smd", "is_state_of_"+objType, "id", "created_at", "permission").
		From(objType + "_state").
		Where(goqu.Ex{"reference_id": machineInstanceId}).
		Where(goqu.Ex{"is_state_of_" + objType: objId}).ToSQL()

	var res StateMachineInstance
	if err != nil {
		log.Errorf("Failed to create query for state select: %v", err)
		return res, err
	}

	responseMap := make(map[string]interface{})

	stmt1, err := fsm.db.Preparex(s)
	if err != nil {
		log.Errorf("[410] failed to prepare statment: %v", err)
	}

	err = stmt1.QueryRowx(v...).MapScan(responseMap)

	if err != nil {
		log.Errorf("Failed to map scan state row: %v", err)
		return res, err
	}

	currentStateString, ok := responseMap["current_state"].(string)
	if !ok {
		currentStateString = string(responseMap["current_state"].([]uint8))
	}
	res.CurrestState = currentStateString

	res.StateMachineId = responseMap[objType+"_smd"].(int64)
	res.ObjectId = responseMap["is_state_of_"+objType].(int64)

	return res, nil
}

type LoopbackEventDesc struct {
	// Name is the event name used when calling for a transition.
	Name  string
	Label string
	Color string

	// Src is a slice of source states that the FSM must be in to perform a
	// state transition.
	Src []string

	// Dst is the destination state that the FSM will be in if the transition
	// succeeds.
	Dst string
}

type LoopbookFsmDescription struct {
	InitialState string
	Name         string
	Label        string
	Events       []LoopbackEventDesc
}

func (fsm *fsmManager) stateMachineRunnerFor(currentState string, typeName string, machineId int64) (*loopfsm.FSM, error) {

	s, v, err := statementbuilder.Squirrel.Select("initial_state", "events").From("smd").Where(goqu.Ex{"id": machineId}).ToSQL()
	if err != nil {
		return nil, err
	}

	var jsonValue string
	var initialState string

	stmt1, err := fsm.db.Preparex(s)
	if err != nil {
		log.Errorf("[410] failed to prepare statment: %v", err)
	}

	err = stmt1.QueryRowx(v...).Scan(&initialState, &jsonValue)

	if currentState == "" {

		if err != nil {
			return nil, err
		}
		currentState = initialState
	}

	var events []LoopbackEventDesc
	err = json.Unmarshal([]byte(jsonValue), &events)
	if err != nil {
		return nil, err
	}

	listOfEvents := make([]loopfsm.EventDesc, 0)
	for _, e := range events {
		e1 := loopfsm.EventDesc{
			Name: e.Name,
			Src:  e.Src,
			Dst:  e.Dst,
		}
		listOfEvents = append(listOfEvents, e1)
	}

	fsmI := loopfsm.NewFSM(currentState, listOfEvents, map[string]loopfsm.Callback{})
	return fsmI, nil
}

func (fsm *fsmManager) ApplyEvent(subject map[string]interface{}, stateMachineEvent StateMachineEvent) (string, error) {

	objType := subject["__type"].(string)
	objReferenceId := subject["reference_id"].(string)

	objectIntegerId, err := ReferenceIdToIntegerId(objType, objReferenceId, fsm.db)
	if err != nil {
		log.Errorf("Failed to get object [%v] by reference id [%v]", objType, objReferenceId)
	}

	stateMachineInstance, err := fsm.getStateMachineInstance(objType, objectIntegerId, stateMachineEvent.GetStateMachineInstanceId())
	if err != nil {
		log.Errorf("Failed to get state machine instance: %v", err)
		return "", err
	}

	stateMachineRunner, err := fsm.stateMachineRunnerFor(stateMachineInstance.CurrestState, objType, stateMachineInstance.StateMachineId)
	if err != nil {
		return "", err
	}

	if stateMachineRunner.Can(stateMachineEvent.GetEventName()) {
		err := stateMachineRunner.Event(stateMachineEvent.GetEventName())
		nextState := stateMachineRunner.Current()
		if err == nil || err.Error() == "no transition" {
			return nextState, nil
		}
		return nextState, err
	} else {
		return stateMachineInstance.CurrestState,
			errors.New(fmt.Sprintf("Cannot apply event %s at this state [%v]",
				stateMachineEvent.GetEventName(), stateMachineInstance.CurrestState),
			)
	}

}
func ReferenceIdToIntegerId(typeName string, referenceId string, db database.DatabaseConnection) (int64, error) {

	s, v, err := statementbuilder.Squirrel.Select("id").From(typeName).Where(goqu.Ex{"reference_id": referenceId}).ToSQL()
	if err != nil {
		return 0, err
	}

	var intId int64

	stmt1, err := db.Preparex(s)
	if err != nil {
		log.Errorf("[410] failed to prepare statment: %v", err)
	}

	err = stmt1.QueryRowx(v...).Scan(&intId)
	return intId, err

}

func NewFsmManager(db database.DatabaseConnection, cruds map[string]*DbResource) FsmManager {

	fsm := fsmManager{
		db:    db,
		cruds: cruds,
	}

	return &fsm

}
