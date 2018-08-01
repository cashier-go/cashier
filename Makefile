CASHIER_CMD := ./cmd/cashier
CASHIER_BIN := ./cashier
CASHIERD_BIN := ./cashierd
CASHIERD_CMD := ./cmd/cashierd
SRC_FILES = $(shell find * -type f -name '*.go' -not -path 'vendor/*' -not -name 'a_*-packr.go')

all: test build

test: dep
	go test ./...
	go install -race $(CASHIER_CMD) $(CASHIERD_CMD)
	go vet ./...
	go list ./... |egrep -v 'vendor/|proto$$' |xargs -L1 golint -set_exit_status
	gofmt -d $(SRC_FILES)

build: cashier cashierd

generate:
	go generate -x ./...

cashier:
	go build -o cashier $(CASHIER_CMD)

cashierd: generate
	go build -o cashierd $(CASHIERD_CMD)

clean:
	rm -f cashier cashierd

dep:
	go get -u github.com/golang/lint/golint

.PHONY: dep generate test cashier cashierd