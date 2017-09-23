FROM alpine as certs
RUN apk update && apk add ca-certificates

FROM busybox

MAINTAINER Parth Mudgal <artpar@gmail.com>
WORKDIR /opt/goms

COPY --from=certs /etc/ssl/certs /etc/ssl/certs

ADD main /opt/goms/goms
RUN chmod +x /opt/goms/goms

EXPOSE 8080
RUN export
ENTRYPOINT ["/opt/goms/goms", "-runtime", "release", "-port", "8080"]