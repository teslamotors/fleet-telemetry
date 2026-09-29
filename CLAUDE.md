# CLAUDE.md

Guidance for AI coding agents working in this repository.

## Project overview

Tesla Fleet Telemetry is a reference server for Tesla's vehicle telemetry protocol.
Vehicles open a WebSocket connection and stream configurable fields; this service
acks records and forwards them to datastores (Kafka, Kinesis, MQTT, Redis, logger, etc.).

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
make test
make format
make linters
make generate-protos   # CI: protoc 28.3 + protoc-gen-go v1.28.1
make integration
make build
```

## Adding a streamable field (Field enum)

Field IDs are assigned by Tesla and tied to vehicle firmware / device-client
versions. When implementing a new field (after an ID is agreed):

1. Add the enum value to `Field` in `protos/vehicle_data.proto`.
2. If needed, extend the `Value` oneof and supporting types in the same proto.
3. Run `make generate-protos` and commit Go, Python, and Ruby outputs together.
4. Note that Fleet API must whitelist the field before config accepts it.

## Deprecated / absent vehicle signals (issue #459)

When production firmware stops emitting a field that remains in the proto:

1. **Document** the observed gap (firmware line, related vehicle_data behavior)
   next to the enum and in README — do not invent server-side values.
2. **Retain** the field ID for proto compatibility with older clients.
3. Point consumers at still-emitted alternatives when known
   (`EstBatteryRange` → `RatedRange` / `IdealBatteryRange` after ~2026.14.x).

Official Available Data pages may lag; this repo should still warn operators.

## Safety / contribution norms

- Follow `CONTRIBUTING.md`. Do not post full VINs, tokens, or PII in issues/PRs.
- Prefer focused PRs; avoid drive-by refactors.
- External contributors generally cannot merge to `teslamotors/fleet-telemetry`;
  open a PR for maintainer review.
