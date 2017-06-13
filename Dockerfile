FROM busybox

MAINTAINER Parth Mudgal <artpar@gmail.com>
WORKDIR /opt/goms

ADD main /opt/goms/goms
RUN chmod +x /opt/goms/goms
ADD gomsweb/dist /opt/goms/gomsweb/dist

EXPOSE 6336
RUN export
ENTRYPOINT ["/opt/goms/goms"]