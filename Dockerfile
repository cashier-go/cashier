FROM golang:1.6

ADD . /go/src/github.com/nsheridan/cashier
RUN go install github.com/nsheridan/cashier/cmd/cashierd

ENTRYPOINT /go/bin/cashierd
