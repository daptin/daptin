FROM alpine as certs
RUN apk update && apk add ca-certificates

FROM busybox

MAINTAINER Parth Mudgal <artpar@gmail.com>
WORKDIR /opt/daptin

COPY --from=certs /etc/ssl/certs /etc/ssl/certs

COPY main /opt/daptin/daptin
RUN chmod +x /opt/daptin/daptin
RUN ls -lah /opt/daptin/daptin

EXPOSE 8080
RUN export
ENTRYPOINT ["/opt/daptin/daptin", "-runtime", "release", "-port", ":8080"]