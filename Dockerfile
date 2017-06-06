FROM busybox

MAINTAINER Parth Mudgal <artpar@gmail.com>
WORKDIR /opt/goms

ADD goms /opt/goms/goms
ADD ./static /opt/goms/gocms

EXPOSE 6336
RUN export
ENTRYPOINT ["/opt/goms/goms"]