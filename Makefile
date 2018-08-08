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
	go list ./... |egrep -v 'proto$$' |xargs -L1 golint -set_exit_status
	goimports -d $(SRC_FILES)
	$(MAKE) generate
	@[ -z "`git status --porcelain`" ] || (echo "unexpected files: `git status --porcelain`" && exit 1)

build: cashier cashierd

generate:
	go generate -x ./...

cashier:
	go build -o cashier $(CASHIER_CMD)

cashierd: generate
	go build -o cashierd $(CASHIERD_CMD)

clean:
	rm -f cashier cashierd

# usage: make migration name=whatever
migration:
	go run ./generate/migration/migration.go $(name)

dep:
	go get -u github.com/golang/lint/golint
	go get -u golang.org/x/tools/cmd/goimports

.PHONY: all build dep generate test cashier cashierd clean migration
