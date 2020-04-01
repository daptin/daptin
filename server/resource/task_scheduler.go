package resource

import (
	"context"
	"fmt"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/auth"
	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type Task struct {
	Id             int64
	ReferenceId    string
	Schedule       string
	Active         bool
	Name           string
	Attributes     map[string]interface{}
	AsUserEmail    string
	ActionName     string
	EntityName     string
	AttributesJson string
}

type TaskScheduler interface {
	StartTasks()
	AddTask(task Task) error
	StopTasks()
}

type DefaultTaskScheduler struct {
	//cmsConfig   *CmsConfig
	cruds       map[string]*DbResource
	configStore *ConfigStore
	cronService *cron.Cron
	activeTasks []*ActiveTaskInstance
}

func NewTaskScheduler(cmsConfig *CmsConfig, cruds map[string]*DbResource, configStore *ConfigStore) TaskScheduler {
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
	Task          Task
	ActionRequest ActionRequest
	DbResource    *DbResource
}

func (ati *ActiveTaskInstance) Run() {
	log.Printf("Execute task [%v] as user [%v]", ati.Task.ActionName, ati.Task.AsUserEmail)

	sessionUser := &auth.SessionUser{}

	if ati.Task.AsUserEmail != "" {
		permission, err := ati.DbResource.GetObjectByWhereClause(USER_ACCOUNT_TABLE_NAME, "email", ati.Task.AsUserEmail)
		CheckErr(err, "Failed to load user by email [%v]", ati.Task.AsUserEmail)
		//log.Printf("Loaded user permission: %v", permission)
		refId := permission["reference_id"]
		if refId != nil {
			usergroups := ati.DbResource.GetObjectUserGroupsByWhere(USER_ACCOUNT_TABLE_NAME, "reference_id", refId.(string))
			sessionUser.UserReferenceId = permission["reference_id"].(string)
			sessionUser.UserId = permission["id"].(int64)
			sessionUser.Groups = usergroups
		}
	}

	pr1 := http.Request{
		Method: "EXECUTE",
	}

	pr := pr1.WithContext(context.WithValue(context.Background(), "user", sessionUser))
	req := api2go.Request{
		PlainRequest: pr,
	}
	res, err := ati.DbResource.Cruds[ati.ActionRequest.Type].HandleActionRequest(&ati.ActionRequest, req)
	//_, _, err := ati.ActionPerformer.DoAction(ati.ActionRequest, ati.Task.Attributes)

	if err != nil {
		log.Errorf("Errors while executing action: %v", err)
	} else {
		log.Printf("Response from action: %v", res)
	}

}

func (dts *DefaultTaskScheduler) AddTask(task Task) error {
	log.Printf("Register task [%v] at %v", task.ActionName, task.Schedule)
	at := dts.cruds["task"].NewActiveTaskInstance(task)
	dts.activeTasks = append(dts.activeTasks, at)
	_, err := dts.cronService.AddJob(task.Schedule, at)

	return err
}

func (db *DbResource) NewActiveTaskInstance(task Task) *ActiveTaskInstance {
	return &ActiveTaskInstance{
		Task: task,
		ActionRequest: ActionRequest{
			Action:     task.ActionName,
			Type:       task.EntityName,
			Attributes: task.Attributes,
		},
		DbResource: db,
	}
}
