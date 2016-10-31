FROM golang:1.7-alpine

ADD . /go/src/github.com/nsheridan/cashier
RUN apk add --update build-base
RUN go install github.com/nsheridan/cashier/cmd/cashierd

VOLUME /cashier
WORKDIR /cashier
ENTRYPOINT /go/bin/cashierd
