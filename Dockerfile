FROM golang:latest as build
LABEL maintainer="nsheridan@gmail.com"
WORKDIR /build
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux make install-cashierd

FROM gcr.io/distroless/base
LABEL maintainer="nsheridan@gmail.com"
WORKDIR /cashier
COPY --from=build /go/bin/cashierd /
ENTRYPOINT ["/cashierd"]
