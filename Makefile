# enabling gofmt and golint since by default they're not enabled by golangci-lint
VERSION           = $(shell go version)
LINTER			  = golangci-lint run -v $(LINTER_FLAGS) --exclude-use-default=false --timeout $(LINTER_DEADLINE)
LINTER_DEADLINE	  = 30s
LINTER_FLAGS ?=
ALPHA_IMAGE_NAME=fleet-telemetry-server-aplha:v0.0.1
ALPHA_IMAGE_COMPRESSED_FILENAME := $(subst :,-, $(ALPHA_IMAGE_NAME))

WG_IMAGE_REPO := 464910097692.dkr.ecr.us-west-2.amazonaws.com/tesla_fleet

GO_FLAGS        ?=
GO_FLAGS        += --ldflags 'extldflags="-static"'

ifneq (,$(findstring darwin/arm,$(VERSION)))
    GO_FLAGS += -tags dynamic
endif
ifneq (,$(wildcard /etc/alpine-release))
    GO_FLAGS += -tags musl
LINTER_FLAGS += --build-tags=musl
endif

INTEGRATION_TEST_ROOT		= ./test/integration
UNIT_TEST_PACKAGES			= $(shell go list ./... | grep -v $(INTEGRATION_TEST_ROOT))

PROTO_DIR = protos
PROTO_FILES = $(wildcard $(PROTO_DIR)/*.proto)

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

integration: generate-certs
	@echo "** RUNNING INTEGRATION TESTS **"
	./test/integration/pretest.sh
	docker compose -p app -f docker-compose.yml build
	docker compose -p app -f docker-compose.yml up -d --remove-orphans
	./test/integration/test.sh
	docker compose -p app -f docker-compose.yml down
	@echo "** INTEGRATION TESTS FINISHED **"

generate-certs:
	go run tools/main.go

clean:
	find $(PROTO_DIR) -type f ! -name '*.proto' -delete

generate-golang:
	protoc --go_out=./ --go_opt=paths=source_relative $(PROTO_DIR)/*.proto

generate-python:
	protoc -I=$(PROTO_DIR) --python_out=$(PROTO_DIR)/python/ $(PROTO_DIR)/*.proto

generate-ruby:
	protoc --ruby_out=$(PROTO_DIR)/ruby/ --proto_path=$(PROTO_DIR) $(PROTO_FILES)

generate-protos: clean generate-golang generate-python generate-ruby

image-gen:
	docker build -t $(ALPHA_IMAGE_NAME) .
	docker save $(ALPHA_IMAGE_NAME) | gzip > $(ALPHA_IMAGE_COMPRESSED_FILENAME).tar.gz

.PHONY: test build vet linters install integration image-gen generate-protos generate-golang generate-python generate-ruby clean

wg-build-image:
	docker build --platform linux/amd64 -t $(WG_IMAGE_REPO):$(shell git rev-parse --short HEAD) .

wg-push-image: wg-build-image
	docker push $(WG_IMAGE_REPO):$(shell git rev-parse --short HEAD)
