FROM alpine

MAINTAINER Parth Mudgal <artpar@gmail.com>

ADD goms /bin/goms
ADD ./static /opt/gocms

EXPOSE 6336
RUN export
ENTRYPOINT ["goms"]