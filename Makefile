CASHIER_CMD := ./cmd/cashier
CASHIERD_CMD := ./cmd/cashierd
SRC_FILES = $(shell find * -type f -name '*.go' -not -path 'vendor/*' -not -name 'a_*-packr.go')
VERSION_PKG := github.com/nsheridan/cashier/lib.Version
VERSION := $(shell git describe --tags --always --dirty)

STATIC_LINKER_FLAGS ?= -linkmode external -extldflags -static -w
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
CGO_ENABLED ?= $(shell go env CGO_ENABLED)

all: test build

test:
	go test -coverprofile=coverage.txt -covermode=count ./...
	go install -race $(CASHIER_CMD) $(CASHIERD_CMD)

lint: dep
	go vet ./...
	go list ./... |xargs -L1 golint -set_exit_status
	gofmt -s -d -l -e $(SRC_FILES)
	$(MAKE) generate
	@[ -z "`git status --porcelain`" ] || (echo "unexpected files: `git status --porcelain`" && exit 1)

build: cashier cashierd
install: install-cashierd install-cashier
cashier: cashier-bin
cashierd: cashierd-bin

generate:
	go generate ./...

%-bin:
	CGO_ENABLED=$(CGO_ENABLED) GOARCH=$(GOARCH) GOOS=$(GOOS) go build -ldflags="-X $(VERSION_PKG)=$(VERSION) $(STATIC_LINKER_FLAGS)" ./cmd/$*
install-%: generate
	CGO_ENABLED=$(CGO_ENABLED) GOARCH=$(GOARCH) GOOS=$(GOOS) go install -ldflags="-X $(VERSION_PKG)=$(VERSION) $(STATIC_LINKER_FLAGS)" ./cmd/$*

docker-image:
	docker build -f Dockerfile .

clean:
	rm -f cashier cashierd

# usage: make migration name=whatever
migration:
	go run ./generate/migration/migration.go $(name)

dep:
	go install golang.org/x/lint/golint@latest

version:
	@echo $(VERSION)

.PHONY: all build dep generate test cashier cashierd clean migration
