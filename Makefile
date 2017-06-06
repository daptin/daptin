GO_BUILD_ENV := GOOS=linux GOARCH=amd64
DOCKER_BUILD=$(shell pwd)/docker_dir
DOCKER_CMD=$(DOCKER_BUILD)/goms

$(DOCKER_CMD): clean
	mkdir -p $(DOCKER_BUILD)
	$(GO_BUILD_ENV) go build -v -o $(DOCKER_CMD) .

web:
      rm -rf

clean:
	rm -rf $(DOCKER_BUILD)

heroku: $(DOCKER_CMD)
	heroku container:push web