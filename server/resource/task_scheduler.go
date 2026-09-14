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
	"github.com/jmoiron/sqlx"
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
	metering := NewMeteringService(&cruds)
	recovery := cron.FuncJob(func() {
		expired, recoveryErr := metering.recoverExpiredReservations(metering.now(), 100)
		if recoveryErr != nil {
			log.WithError(recoveryErr).Error("failed to recover expired metering reservations")
		} else if expired > 0 {
			log.WithField("expired", expired).Debug("recovered expired metering reservations")
		}
	})
	if _, err := scheduler.cronService.AddJob(meteringReservationRecoverySchedule,
		cron.SkipIfStillRunning(cron.DefaultLogger)(recovery)); err != nil {
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
	if err := ati.execute(); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"task_reference_id": ati.Task.ReferenceId.String(),
			"task_name":         ati.Task.Name,
			"entity_name":       ati.Task.EntityName,
			"action_name":       ati.Task.ActionName,
			"user_reference_id": ati.Task.AsUserReferenceId.String(),
		}).Error("scheduled task failed")
	}
}

func (ati *ActiveTaskInstance) execute() error {
	transaction, err := ati.DbResource.Connection().Beginx()
	if err != nil {
		return fmt.Errorf("begin scheduled task transaction: %w", err)
	}
	defer transaction.Rollback()

	sessionUser, err := ati.resolveSessionUser(transaction)
	if err != nil {
		return fmt.Errorf("resolve scheduled task user: %w", err)
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
		return fmt.Errorf("execute scheduled action: %w", err)
	}
	log.Debugf("Response from action: %v", res)
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit scheduled action: %w", err)
	}
	return nil
}

func (ati *ActiveTaskInstance) resolveSessionUser(transaction *sqlx.Tx) (*auth.SessionUser, error) {
	userReferenceId := ati.Task.AsUserReferenceId
	if userReferenceId == daptinid.NullReferenceId {
		return nil, fmt.Errorf("task has no as_user_id relationship")
	}
	user, _, err := ati.DbResource.Cruds[USER_ACCOUNT_TABLE_NAME].GetSingleRowByReferenceIdWithTransaction(
		USER_ACCOUNT_TABLE_NAME, userReferenceId, nil, transaction)
	if err != nil {
		return nil, fmt.Errorf("load user_account [%s]: %w", userReferenceId.String(), err)
	}
	userId, err := ResourceRowInt64(user["id"])
	if err != nil || userId <= 0 {
		return nil, fmt.Errorf("invalid user_account id for [%s]", userReferenceId.String())
	}
	groups := ati.DbResource.GetObjectUserGroupsByWhereWithTransaction(
		USER_ACCOUNT_TABLE_NAME, transaction, "id", userId)
	authVersion := int64(1)
	if user[auth.AuthVersionColumn] != nil {
		authVersion, err = ResourceRowInt64(user[auth.AuthVersionColumn])
		if err != nil {
			return nil, fmt.Errorf("invalid user_account auth_version for [%s]: %w", userReferenceId.String(), err)
		}
	}
	return &auth.SessionUser{
		UserId:          userId,
		UserReferenceId: userReferenceId,
		Groups:          groups,
		AuthVersion:     auth.AuthVersionOrDefault(authVersion),
	}, nil
}

func (dts *DefaultTaskScheduler) AddTask(task task.Task) error {
	if task.AsUserReferenceId == daptinid.NullReferenceId {
		return fmt.Errorf("task [%s] has no as_user_id relationship", task.Name)
	}
	if dts.cruds[task.EntityName] == nil {
		return fmt.Errorf("task [%s] targets unknown resource [%s]", task.Name, task.EntityName)
	}
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
