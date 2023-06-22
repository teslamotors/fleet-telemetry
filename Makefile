# enabling gofmt and golint since by default they're not enabled by golangci-lint
VERSION           = $(shell go version)
LINTER			  = golangci-lint run -v $(LINTER_FLAGS) --exclude-use-default=false --enable gofmt,golint --timeout $(LINTER_DEADLINE)
LINTER_DEADLINE	  = 30s
LINTER_FLAGS ?=
ALPHA_IMAGE_NAME=fleet-telemetry-server-aplha:v0.0.1
ALPHA_IMAGE_COMPRESSED_FILENAME := $(subst :,-, $(ALPHA_IMAGE_NAME))

GO_FLAGS        ?=

ifneq (,$(findstring darwin/arm,$(VERSION)))
    GO_FLAGS += -tags dynamic
endif
ifneq (,$(wildcard /etc/alpine-release))
    GO_FLAGS += -tags musl
LINTER_FLAGS += --build-tags=musl
endif

INTEGRATION_TEST_ROOT		= ./test/integration
UNIT_TEST_PACKAGES			= $(shell go list ./... | grep -v $(INTEGRATION_TEST_ROOT))

build:
	go build $(GO_FLAGS) -v -o $(GOPATH)/bin/fleet-telemetry cmd/main.go
	@echo "** build complete: $(GOPATH)/bin/fleet-telemetry **"

linters: install
	@echo "** Running linters...**"
	$(LINTER)
	@echo "** SUCCESS **"

format:
	go fmt ./...

vet:
	go vet ./...

install:
	@$(RUNTIME_FLAG) go generate $(GO_FLAGS) ./...
	@$(RUNTIME_FLAG) go install $(GO_FLAGS) ./...

test: install
	go test -cover $(TEST_OPTIONS) $(GO_FLAGS) $(UNIT_TEST_PACKAGES) || exit 1;

test-race: TEST_OPTIONS = -race
test-race: test

integration:
	@echo "** RUNNING INTEGRATION TESTS **"
	./test/integration/pretest.sh
	docker-compose -p app -f docker-compose.yml build
	docker-compose -p app -f docker-compose.yml up -d --remove-orphans
	./test/integration/test.sh
	docker-compose -p app -f docker-compose.yml down
	@echo "** INTEGRATION TESTS FINISHED **"

generate-certs:
	go run tools/main.go

generate-golang:
	protoc --go_out=./ --go_opt=paths=source_relative protos/*.proto

generate-python:
	protoc -I=protos --python_out=protos/python/ protos/*.proto

generate-protos: generate-golang generate-python

image-gen:
	docker build -t $(ALPHA_IMAGE_NAME) .
	docker save $(ALPHA_IMAGE_NAME) | gzip > $(ALPHA_IMAGE_COMPRESSED_FILENAME).tar.gz

.PHONY: test build vet linters install integration image-gen generate-protos generate-golang generate-python
