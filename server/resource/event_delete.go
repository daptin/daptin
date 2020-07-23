package resource

func NewDeleteEventHandler() DatabaseRequestInterceptor {
	return &eventHandlerMiddleware{}
}
