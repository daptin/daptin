FROM alpine:latest

MAINTAINER Parth Mudgal <artpar@gmail.com>

WORKDIR "/opt"

ADD .docker_build/gocms /opt/bin/gocms
ADD ./gocms/dist /opt/gocms
ADD ./static /opt/static

CMD ["/opt/bin/go-getting-started"]
