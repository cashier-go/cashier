FROM --platform=${BUILDPLATFORM} golang:latest AS build
LABEL maintainer="nsheridan@gmail.com"

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

WORKDIR /build
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} make cashierd

FROM --platform=${TARGETPLATFORM} gcr.io/distroless/base
LABEL maintainer="nsheridan@gmail.com"
COPY --from=build /build/cashierd /
ENTRYPOINT ["/cashierd"]
