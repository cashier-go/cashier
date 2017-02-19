#! /bin/sh

# This can be used as a pre-commit script.  Just run
#   cp run_tests.sh .git/hooks/pre-commit
# and it will run before each commit.

set -xue

go install -race -v ./cmd/cashier ./cmd/cashierd
go list ./... |grep -v vendor/ |xargs go test -race
gofmt -d $(find * -type f -name '*.go' -not -path 'vendor/*')
go list ./... |grep -v vendor/ |xargs go vet
if ! type -f golint > /dev/null; then
  go get -u github.com/golang/lint/golint
fi
go list ./... |egrep -v 'vendor/|proto$' |xargs -L1 golint -set_exit_status
