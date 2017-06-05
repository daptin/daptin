FROM ubuntu:16.04

MAINTAINER Parth Mudgal <artpar@gmail.com>

RUN ls
RUN pwd
RUN ls -lah
ADD main /
ADD ./static /opt/gocms

EXPOSE 6336
CMD ["/main"]