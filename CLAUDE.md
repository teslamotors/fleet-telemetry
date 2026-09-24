# CLAUDE.md

Guidance for AI coding agents working in this repository.

## Project overview

Tesla Fleet Telemetry is a reference server for Tesla's vehicle telemetry protocol.
Vehicles open a WebSocket connection and stream configurable fields; this service
acks records and forwards them to datastores (Kafka, Kinesis, MQTT, logger, etc.).

Upstream: https://github.com/teslamotors/fleet-telemetry

## Architecture (high level)

1. **Vehicle config** — owners/fleets set `fleet_telemetry_config` via Fleet API
   (fields + minimum intervals). Vehicles stream on change, rate-limited by interval.
2. **mTLS WebSocket** — `server/` terminates device connections and routes records.
3. **Protobuf schema** — `protos/vehicle_data.proto` defines the `Field` enum and
   typed `Value` oneof. Generated Go/Python/Ruby bindings live under `protos/`.
4. **Dispatch** — `datastore/` adapters publish decoded or raw payloads downstream.
5. **Observability** — Prometheus/statsd metrics and optional profiling ports.

## Common commands

```bash
# Unit tests (also runs go generate / install)
make test

# Format + lint (CI enforces both)
make format
make linters

# Regenerate protobuf bindings — keep committed output in sync
# CI uses protoc 28.3 and protoc-gen-go v1.28.1
make generate-protos

# Full integration suite (Docker Compose)
make integration

make build
```

## Adding a streamable field (Field enum)

Field IDs are assigned by Tesla and tied to vehicle firmware / device-client
versions. Do not invent IDs that collide with unpublished internal assignments
when an official ID is already known.

When implementing a new field (after an ID is agreed):

1. Add the enum value to `Field` in `protos/vehicle_data.proto`, with a short
   comment and firmware/device-client availability note when known.
2. If the field needs a new typed payload, extend the `Value` oneof and add any
   supporting enum/message types in the same proto.
3. Run `make generate-protos` and commit Go, Python, and Ruby outputs together.
4. Link the related GitHub issue in the PR; note that Fleet API must whitelist
   the field name before `fleet_telemetry_config` accepts it.

Existing HVAC-related fields for reference: `HvacACEnabled`, `HvacAutoMode`,
`HvacFanSpeed`, `HvacFanStatus`, `HvacPower`, temperature request fields, etc.

## Safety / contribution norms

- Follow `CONTRIBUTING.md`. Do not post VINs, tokens, account data, or other PII
  in issues or PRs.
- Prefer focused PRs; avoid drive-by refactors.
- This repo cannot by itself make vehicles emit a signal — firmware and Fleet API
  backend changes are required for new fields to appear on the wire.
- External contributors generally cannot merge to `teslamotors/fleet-telemetry`;
  open a PR for maintainer review.

## Issue context

Issue #524 requests `HvacCompressorRpm` so owners can see when the AC compressor
is running near its limits and take corrective action (shade, condenser cleaning)
to reduce thermal stress.
