package resource

func NewCreateEventHandler() DatabaseRequestInterceptor {
	return &eventHandlerMiddleware{}
}
