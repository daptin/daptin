FROM golang:alpine

MAINTAINER Parth Mudgal <artpar@gmail.com>


ADD main /
ADD ./static /opt/gocms

EXPOSE 6336
CMD ["/main"]