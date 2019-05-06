.PHONY: container publish serve serve-container clean

app        := daptin
static-app := build/linux-amd64/$(app)
docker-tag := daptin/daptin

bin/$(app): *.go
	go build -o $@

docker: docker-daptin-binary
	cd docker_dir && cp ../Dockerfile Dockerfile && cp ../daptin-linux-amd64 main && docker build -t daptin/daptin  . && cd ..


docker-daptin-binary: daptin-linux-amd64
	rm -rf rice-box.go && rice embed-go && xgo --targets='linux/amd64' -ldflags='-extldflags "-static"'  .

daptin-linux-amd64:
    rm -rf rice-box.go && rice embed-go && xgo --targets='linux/amd64' -ldflags='-extldflags "-static"'  .

dashboard:


$(static-app): *.go
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 \
		go build  -ldflags='-extldflags "-static"' -a -installsuffix cgo -o $(static-app)

container: $(static-app)
	docker build -t $(docker-tag) .

publish: container
	docker push $(docker-tag)

serve: bin/$(app)
	env PATH=$(PATH):./bin forego start web

serve-container:
	docker run -it --rm --env-file=.env -p 8081:8080 $(docker-tag)

clean:
	rm -rf bin build daptin-linux-amd64