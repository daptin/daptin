package task_scheduler

import "github.com/daptin/daptin/server/task"

type TaskScheduler interface {
	StartTasks()
	AddTask(task task.Task) error
	StopTasks()
}
