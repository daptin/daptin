FROM busybox

MAINTAINER Parth Mudgal <artpar@gmail.com>
WORKDIR /opt/goms

ADD main /opt/goms/goms
ADD webgoms/dist /opt/goms/webgoms/dist

EXPOSE 6336
RUN export
ENTRYPOINT ["/opt/goms/goms"]