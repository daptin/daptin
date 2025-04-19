package resource

import (
	"context"
	"fmt"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/task"
	"github.com/daptin/daptin/server/task_scheduler"
	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
)

type DefaultTaskScheduler struct {
	//cmsConfig   *CmsConfig
	cruds       map[string]*DbResource
	configStore *ConfigStore
	cronService *cron.Cron
	activeTasks []*ActiveTaskInstance
}

func NewTaskScheduler(cmsConfig *CmsConfig, cruds map[string]*DbResource, configStore *ConfigStore) task_scheduler.TaskScheduler {
	cronService := cron.New()
	cronService.Start()
	dts := &DefaultTaskScheduler{
		//cmsConfig:   cmsConfig,
		cruds:       cruds,
		configStore: configStore,
		cronService: cronService,
		activeTasks: make([]*ActiveTaskInstance, 0),
	}
	return dts
}

func (dts *DefaultTaskScheduler) StopTasks() {
	dts.cronService.Stop()
}

func (dts *DefaultTaskScheduler) StartTasks() {

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
	dts.activeTasks = append(dts.activeTasks, at)
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
