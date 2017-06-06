FROM golang:alpine

MAINTAINER Parth Mudgal <artpar@gmail.com>

RUN apk add --update --no-cache git gcc musl-dev && rm -rf /var/cache/apk/*

RUN go get github.com/artpar/goms
ADD ./static /opt/gocms

EXPOSE 6336
RUN export
ENTRYPOINT ["goms"]