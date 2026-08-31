FROM ubuntu:24.04

ARG TARGETARCH
ARG VERSION=dev
ARG VCS_REF=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.title="Daptin" \
      org.opencontainers.image.description="Open source backend and data platform" \
      org.opencontainers.image.url="https://dapt.in" \
      org.opencontainers.image.source="https://github.com/daptin/daptin" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.licenses="LGPL-3.0-only"

RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends \
        ca-certificates \
        curl \
        tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10001 daptin \
    && useradd --uid 10001 --gid daptin --no-create-home --home-dir /var/lib/daptin daptin \
    && install --directory --owner=daptin --group=daptin --mode=0750 \
        /var/lib/daptin /var/lib/daptin/storage /var/cache/daptin

COPY --chmod=0755 build/daptin-linux-${TARGETARCH} /usr/local/bin/daptin

USER 10001:10001
WORKDIR /var/lib/daptin

ENV DAPTIN_PORT=:8080 \
    DAPTIN_RUNTIME=release \
    DAPTIN_LOCAL_STORAGE_PATH=/var/lib/daptin/storage \
    DAPTIN_CACHE_FOLDER=/var/cache/daptin

EXPOSE 8080 5336 5337
STOPSIGNAL SIGTERM

HEALTHCHECK --interval=10s --timeout=3s --start-period=30s --retries=6 \
    CMD ["curl", "--fail", "--silent", "--show-error", "http://127.0.0.1:8080/ping"]

ENTRYPOINT ["/usr/local/bin/daptin"]
CMD []
