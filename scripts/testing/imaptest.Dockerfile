FROM ghcr.io/icgood/docker-imaptest AS tests

FROM dovecot/imaptest
COPY --from=tests /tmp/imaptest-latest/src/tests /tests
COPY --from=tests /tmp/imaptest-latest/src/tests/default.mbox /default.mbox
