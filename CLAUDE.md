# CLAUDE.md

Guidance for [Claude Code](https://claude.com/claude-code) when working in this repository.

## What this repository is

Fleet Telemetry is the Go server that receives telemetry streamed by Tesla vehicles over mTLS/WebSocket, decodes the protobuf payloads, and dispatches them to configurable backends (Kafka, Kinesis, Google Pub/Sub, MQTT, Redis, ZMQ, or logs).

The critical thing to understand is the trust and version boundary: **this server does not decide what vehicles send.** Vehicle firmware does. A vehicle can run firmware newer than the deployed server, so the field numbers arriving on the wire are not bounded by the `Field` enum this binary was compiled with.

## Layout

| Path | Contents |
| --- | --- |
| `protos/` | Protobuf definitions and generated Go/Python/Ruby bindings. `vehicle_data.proto` holds the `Field` enum. |
| `datastore/` | One package per dispatch backend, plus `simple/transformers` which converts payloads to maps. |
| `telemetry/` | Record parsing and socket handling. |
| `server/streaming` | The mTLS WebSocket server vehicles connect to. |
| `config/` | Configuration parsing and validation. |
| `test/integration` | Docker-based integration tests, excluded from the unit test target. |

## Do not invent `Field` enum numbers

`Field` numbers in `protos/vehicle_data.proto` are **assigned by Tesla and tied to specific firmware releases**, which the proto records in comments:

```
// fields 260-269 are first available in firmware version 2026.32 (device client version 1.3.0)
```

Every commit adding field numbers in this repo's history is authored by Tesla staff. Adding a field number because an issue asks for a new signal does not make vehicles emit it, and picking the next free number is actively harmful: the number will eventually be assigned to something else, and any consumer that adopted the guess will silently mislabel real data. Field 260 is a concrete example — it became `GpsAccuracyMeters`.

If a request amounts to "please expose signal X", the repo-side change cannot be made correctly without Tesla assigning a number. Say so rather than guessing.

## Commands

```
go build -tags dynamic ./...
go vet -tags dynamic ./...
go test -tags dynamic -count=1 $(go list ./... | grep -v test/integration)
golangci-lint run --build-tags=dynamic ./...
make generate-protos      # regenerate Go/Python/Ruby bindings from .proto
make integration          # docker compose based, needs Docker running
```

**cgo dependencies are required to build at all.** The Kafka and ZMQ backends bind native libraries. CI installs `libsodium-dev libzmq3-dev`; on macOS use `brew install zeromq libsodium librdkafka`. Without them `go build ./...` fails in `pkg-config` before reaching any of this repo's code.

**The `dynamic` build tag is needed on macOS arm64.** The Makefile adds it automatically based on `go version`; when invoking `go` directly, pass `-tags dynamic` or the cgo backends will not link.

Generated protobuf files are checked in. Do not hand-edit `*.pb.go`, `protos/python/` or `protos/ruby/` — change the `.proto` and run `make generate-protos`, which needs `protoc` plus the Go, Python and Ruby plugins.

## Conventions

- **Linting is enforced in CI** via `.golangci.yml`: `errcheck`, `govet`, `staticcheck`, `unused`, `misspell` and `revive`. `revive`'s `exported` rule means every exported identifier needs a doc comment starting with its own name.
- **`goimports` is the formatter.** Run `make format` or `gofmt -l` before proposing changes.
- **Tests use Ginkgo v2 + Gomega**, with a `*_suite_test.go` per package. Dot-imports of ginkgo/gomega are explicitly allowed by the lint config; do not "fix" them.
- **Structured logging.** Use `logger.ActivityLog("snake_case_event", logrus.LogInfo{...})` rather than free-form messages, and never log payload contents that could identify a vehicle or customer — `CONTRIBUTING.md` is explicit that personal, account and vehicle information must not appear.

## Things to be careful about

- **Unrecognized field numbers must stay distinct.** `PayloadToMap` looks names up in `protos.Field_name`; a missing key yields the empty string. Any code that keys a map on a field name needs a fallback, or every unknown field in a payload collides on one key and all but the last are silently dropped. `UnknownFieldNamePrefix` exists for this.
- **A vehicle is an untrusted, version-skewed input.** Decoding logic should degrade rather than assume the payload matches this build's schema.
- **Integration tests need Docker and generated certs**; they are excluded from `UNIT_TEST_PACKAGES`. Do not claim they passed unless `make integration` actually ran.
- **Backends without local infrastructure report 0% coverage** (Kafka, Kinesis, Pub/Sub, ZMQ). That is expected and not a sign the build is broken.

## Commit and PR conventions

`PULL_REQUEST_TEMPLATE.md` asks for a description and confirmation that tests pass. `CONTRIBUTING.md` asks contributors not to work on issues still carrying a `triage` label. Keep unrelated changes in separate commits.
