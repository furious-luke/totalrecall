FROM postgres:14.5-alpine3.16 AS mirror

RUN apk add --no-cache go musl-dev linux-headers make upx
ENV GOPATH=/go PATH=$PATH:/go/bin CGO_ENABLED=1 GOOS=linux

COPY . /go/src/totalrecall/
WORKDIR /go/src/totalrecall
RUN    make                    \
    && go install              \
    && upx /go/bin/totalrecall

FROM postgres:14.5-alpine3.16
COPY --from=mirror /go/bin/totalrecall /usr/bin/
COPY --from=mirror /go/src/totalrecall/worker/totalrecall_worker.so /usr/local/lib/postgresql/
RUN mkdir -p /etc/totalrecall
COPY postgres/totalrecall.yaml /etc/totalrecall/
COPY postgres/postgresql.conf /etc/postgresql/
COPY postgres/docker-entrypoint.sh /usr/local/bin/
RUN chown postgres:postgres -R /etc/postgresql /etc/totalrecall
