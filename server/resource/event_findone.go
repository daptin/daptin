package resource

func NewFindOneEventHandler() DatabaseRequestInterceptor {
	return &eventHandlerMiddleware{}
}
