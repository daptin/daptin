FROM phusion/baseimage

MAINTAINER Parth Mudgal <artpar@gmail.com>


ADD goms /bin
ADD ./static /opt/gocms

EXPOSE 6336
RUN export
ENTRYPOINT ["goms"]