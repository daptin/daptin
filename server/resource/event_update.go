package resource

func NewUpdateEventHandler() DatabaseRequestInterceptor {
	return &eventHandlerMiddleware{}
}
