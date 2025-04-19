package task

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
