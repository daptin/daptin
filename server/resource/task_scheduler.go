package resource

import (
	"fmt"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
)

type Task struct {
	Id             int64
	ReferenceId    string
	Schedule       string
	Active         bool
	Name           string
	Attributes     map[string]interface{}
	JobType        string
	AttributesJson string
}

type TaskScheduler interface {
	StartTasks()
	AddTask(task Task) error
}

type DefaultTaskScheduler struct {
	cmsConfig   *CmsConfig
	cruds       map[string]*DbResource
	configStore *ConfigStore
	cronService *cron.Cron
	activeTasks []*ActiveTaskInstance
}

func NewTaskScheduler(cmsConfig *CmsConfig, cruds map[string]*DbResource, configStore *ConfigStore) TaskScheduler {
	cronService := cron.New()
	cronService.Start()
	dts := &DefaultTaskScheduler{
		cmsConfig:   cmsConfig,
		cruds:       cruds,
		configStore: configStore,
		cronService: cronService,
		activeTasks: make([]*ActiveTaskInstance, 0),
	}
	return dts
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
	Task            Task
	ActionRequest   ActionRequest
	ActionPerformer ActionPerformerInterface
}

func (ati *ActiveTaskInstance) Run() {
	log.Printf("Execute task [%v]", ati.Task.JobType)
	_, _, err := ati.ActionPerformer.DoAction(ati.ActionRequest, ati.Task.Attributes)

	if len(err) > 0 {
		log.Errorf("Errors while executing action: %v", err)
	}

}

func (dts *DefaultTaskScheduler) AddTask(task Task) error {
	log.Printf("Register task [%v] at %v", task.JobType, task.Schedule)
	var actionPerformer ActionPerformerInterface

	for _, performer := range dts.cmsConfig.ActionPerformers {
		if performer.Name() == task.JobType {
			actionPerformer = performer
			break
		}
	}

	if actionPerformer == nil {
		return fmt.Errorf("invalid job type in task [%v] matched none of the available actions", task.JobType)
	}
	at := NewActiveTaskInstance(task, actionPerformer)
	dts.activeTasks = append(dts.activeTasks, at)
	err := dts.cronService.AddJob(task.Schedule, at)

	return err
}
func NewActiveTaskInstance(task Task, performerInterface ActionPerformerInterface) *ActiveTaskInstance {
	return &ActiveTaskInstance{
		Task:            task,
		ActionRequest:   ActionRequest{},
		ActionPerformer: performerInterface,
	}
}
