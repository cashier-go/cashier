FROM golang:latest as build
LABEL maintainer="nsheridan@gmail.com"
WORKDIR /build
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux make install-cashierd

FROM gcr.io/distroless/static
LABEL maintainer="nsheridan@gmail.com"
WORKDIR /cashier
COPY --from=build /go/bin/cashierd /
ENTRYPOINT ["/cashierd"]
