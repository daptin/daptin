package resource

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/task"
	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
)

const meteringReservationRecoverySchedule = "@every 10s"

type DefaultTaskScheduler struct {
	cruds       map[string]*DbResource
	cronService *cron.Cron
	stopOnce    sync.Once
	stopped     context.Context
}

func NewTaskScheduler(cruds map[string]*DbResource) (*DefaultTaskScheduler, error) {
	scheduler := &DefaultTaskScheduler{
		cruds:       cruds,
		cronService: cron.New(),
	}
	if cruds["api_usage"] == nil {
		return nil, fmt.Errorf("task scheduler requires canonical api_usage resource")
	}
	metering := NewMeteringService(&cruds)
	if _, err := scheduler.cronService.AddFunc(meteringReservationRecoverySchedule, func() {
		expired, recoveryErr := metering.recoverExpiredReservations(metering.now(), 100)
		if recoveryErr != nil {
			log.WithError(recoveryErr).Error("failed to recover expired metering reservations")
		} else if expired > 0 {
			log.WithField("expired", expired).Debug("recovered expired metering reservations")
		}
	}); err != nil {
		return nil, fmt.Errorf("register metering reservation recovery: %w", err)
	}
	return scheduler, nil
}

func (dts *DefaultTaskScheduler) Start() {
	dts.cronService.Start()
}

// Quiesce prevents new scheduled jobs from starting.
func (dts *DefaultTaskScheduler) Quiesce() {
	dts.stopOnce.Do(func() {
		dts.stopped = dts.cronService.Stop()
	})
}

func (dts *DefaultTaskScheduler) Stop(ctx context.Context) error {
	dts.Quiesce()
	select {
	case <-dts.stopped.Done():
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (dts *DefaultTaskScheduler) LoadPersistedTasks() {

	tasks, err := dts.cruds["task"].GetAllTasks()
	if CheckErr(err, "Failed to fetch tasks from database") {
		return
	}
	for _, cronjob := range tasks {

		err := dts.AddTask(cronjob)
		if CheckErr(err, fmt.Sprintf("Failed to start scheduled job: %v", cronjob.Name)) {
			continue
		}

	}

}

type ActiveTaskInstance struct {
	Task          task.Task
	ActionRequest actionresponse.ActionRequest
	DbResource    *DbResource
}

func (ati *ActiveTaskInstance) Run() {
	log.Printf("[82] Execute task [%v][%v] as user [%v]", ati.Task.ReferenceId, ati.Task.ActionName, ati.Task.AsUserEmail)

	sessionUser := &auth.SessionUser{}
	transaction, err := ati.DbResource.Connection().Beginx()
	if err != nil {
		CheckErr(err, "Failed to begin transaction for ATI.run [88]")
	}
	if transaction == nil {
		return
	}
	defer transaction.Commit()

	if ati.Task.AsUserEmail != "" {

		permission, err := ati.DbResource.GetObjectByWhereClause(USER_ACCOUNT_TABLE_NAME, "email", ati.Task.AsUserEmail, transaction)
		CheckErr(err, "Failed to load user by email [%v]", ati.Task.AsUserEmail)
		//log.Printf("Loaded user permission: %v", permission)
		refId := permission["reference_id"]
		if refId != nil {
			dir := daptinid.InterfaceToDIR(refId)
			usergroups := ati.DbResource.GetObjectUserGroupsByWhereWithTransaction(USER_ACCOUNT_TABLE_NAME, transaction, "reference_id", dir[:])
			sessionUser.UserReferenceId = daptinid.InterfaceToDIR(permission["reference_id"])
			sessionUser.UserId = permission["id"].(int64)
			sessionUser.Groups = usergroups
		}
	}

	ur, _ := url.Parse("/action/" + ati.ActionRequest.Type)
	pr1 := http.Request{
		Method: "EXECUTE",
		URL:    ur,
	}

	pr := pr1.WithContext(context.WithValue(context.Background(), "user", sessionUser))
	req := api2go.Request{
		PlainRequest: pr,
	}
	res, err := ati.DbResource.Cruds[ati.ActionRequest.Type].HandleActionRequest(ati.ActionRequest, req, transaction)

	if err != nil {
		transaction.Rollback()
		log.Errorf("Errors while executing action 109: %v", err)
	} else {
		log.Debugf("Response from action: %v", res)
	}

}

func (dts *DefaultTaskScheduler) AddTask(task task.Task) error {
	log.Printf("Register task [%v] at %v", task.ActionName, task.Schedule)
	at := dts.cruds["task"].NewActiveTaskInstance(task)
	_, err := dts.cronService.AddJob(task.Schedule, at)

	return err
}

func (dbResource *DbResource) NewActiveTaskInstance(task task.Task) *ActiveTaskInstance {
	return &ActiveTaskInstance{
		Task: task,
		ActionRequest: actionresponse.ActionRequest{
			Action:     task.ActionName,
			Type:       task.EntityName,
			Attributes: task.Attributes,
		},
		DbResource: dbResource,
	}
}
