FROM golang:latest as build
LABEL maintainer="nsheridan@gmail.com"
ARG SRC_DIR=/go/src/github.com/nsheridan/cashier
WORKDIR ${SRC_DIR}
ADD . ${SRC_DIR}
RUN CGO_ENABLED=0 GOOS=linux make install-cashierd

FROM scratch
LABEL maintainer="nsheridan@gmail.com"
WORKDIR /cashier
COPY --from=build /go/bin/cashierd /
ENTRYPOINT ["/cashierd"]
