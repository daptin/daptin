package resource


type CronjobExecutor interface {
	Execute(attributes map[string]interface{}) error
}