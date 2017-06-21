package fsm_manager

import (
  "github.com/jmoiron/sqlx"
  "github.com/artpar/goms/server/resource"
  "gopkg.in/Masterminds/squirrel.v1"
  log "github.com/sirupsen/logrus"
  loopfsm "github.com/looplab/fsm"
  "encoding/json"
  "github.com/pkg/errors"
  "fmt"
)

type fsmManager struct {
  db    *sqlx.DB
  cruds map[string]*resource.DbResource
}

type StateMachineInstance struct {
  CurrestState   string
  StateMachineId int64
  ObjectId       int64
}

func (fsm *fsmManager) getStateMachineInstance(objType string, objId int64, machineInstanceId string) (StateMachineInstance, error) {

  s, v, err := squirrel.Select("current_state", objType+"_smd", "is_state_of_"+objType, "id", "created_at", "permission").
      From(objType + "_state").
      Where(squirrel.Eq{"deleted_at": nil}).
      Where(squirrel.Eq{"reference_id": machineInstanceId}).
      Where(squirrel.Eq{"is_state_of_" + objType: objId}).ToSql()

  var res StateMachineInstance
  if err != nil {
    log.Errorf("Failed to create query for state select: %v", err)
    return res, err
  }

  responseMap := make(map[string]interface{})
  err = fsm.db.QueryRowx(s, v...).MapScan(responseMap)

  if err != nil {
    log.Errorf("Failed to map scan state row: %v", err)
    return res, err
  }

  res.CurrestState = string(responseMap["current_state"].([]uint8))
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
  // succeds.
  Dst string
}

type LoopbookFsmDescription struct {
  InitialState string `json:"initial_state"`
  Name         string
  Label        string
  Events       []LoopbackEventDesc `json:"events"`
}

func (fsm *fsmManager) stateMachineRunnerFor(currentState string, typeName string, machineId int64) (*loopfsm.FSM, error) {

  s, v, err := squirrel.Select("initial_state", "events").From("smd").Where(squirrel.Eq{"id": machineId}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
  if err != nil {
    return nil, err
  }

  var jsonValue string
  var initialState string
  err = fsm.db.QueryRowx(s, v...).Scan(&initialState, &jsonValue)

  if (currentState == "") {

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

  if (stateMachineRunner.Can(stateMachineEvent.GetEventName())) {
    err := stateMachineRunner.Event(stateMachineEvent.GetEventName())
    nextState := stateMachineRunner.Current()
    return nextState, err
  } else {
    return stateMachineInstance.CurrestState,
        errors.New(fmt.Sprintf("Cannot apply event %s at this state [%v]",
          stateMachineEvent.GetEventName(), stateMachineInstance.CurrestState),
        )
  }

}
func ReferenceIdToIntegerId(typeName string, referenceId string, db *sqlx.DB) (int64, error) {

  s, v, err := squirrel.Select("id").From(typeName).Where(squirrel.Eq{"reference_id": referenceId}).Where(squirrel.Eq{"deleted_at": nil}).ToSql()
  if err != nil {
    return 0, err
  }

  var intId int64

  err = db.QueryRowx(s, v...).Scan(&intId)
  return intId, err

}

func NewFsmManager(db *sqlx.DB, cruds map[string]*resource.DbResource) FsmManager {

  fsm := fsmManager{
    db:    db,
    cruds: cruds,
  }

  return &fsm

}
