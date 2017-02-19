#!/bin/bash
# Run this script from the root of the repo to regenerate static content.

set -xue

go get -u github.com/mjibson/esc
${GOPATH}/bin/esc -ignore '\.go' -prefix 'server' \
  -o 'server/static/static.go' -pkg 'static' 'server/static'

go get -u github.com/golang/protobuf/protoc-gen-go
protoc --go_out=plugins=grpc:. proto/signer.proto
