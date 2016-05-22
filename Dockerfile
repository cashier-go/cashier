FROM golang:1.6

ADD . /go/src/github.com/nsheridan/cashier
WORKDIR /go/src/github.com/nsheridan/cashier

RUN go install -v

ENTRYPOINT /go/bin/cashier
