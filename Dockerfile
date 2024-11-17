FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:latest AS build

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

WORKDIR /build
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN if [ ${TARGETARCH} = "arm" ]; then export GOARM="${TARGETPLATFORM#v}"; fi
RUN go env
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} make cashierd

FROM --platform=${BUILDPLATFORM:-linux/amd64} gcr.io/distroless/base
LABEL maintainer="nsheridan@gmail.com"
COPY --from=build /build/cashierd /
ENTRYPOINT ["/cashierd"]
