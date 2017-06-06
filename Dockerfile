FROM golang:1.7-alpine

ADD . /go/src/github.com/nsheridan/cashier
RUN apk --update add --virtual build-dependencies build-base && \
    go install -ldflags="-s -w" github.com/nsheridan/cashier/cmd/cashierd && \
    apk del build-dependencies && \
    rm -rf /go/src

VOLUME /cashier
WORKDIR /cashier
ENTRYPOINT /go/bin/cashierd
