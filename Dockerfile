FROM alpine:latest

MAINTAINER Parth Mudgal <artpar@gmail.com>

WORKDIR "/opt"

ADD main /opt/bin/gocms
ADD ./dist /opt/gocms
ADD ./static /opt/static

CMD ["/opt/bin/gocms"]
