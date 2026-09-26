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

## Config eligibility vs vehicle client version (issue #510)

`fleet_status.fleet_telemetry_version` reports the on-vehicle client version. It does
**not** mean `fleet_telemetry_config` will succeed.

Configure skips (`unsupported_firmware`, `missing_key`, `unsupported_hardware`,
`max_configs`) are decided by **Fleet API**, not by this open-source server.

Legacy Intel Atom Model S/X (firmware 2025.20+):

- Use Safety → Allow Third-Party App Data Streaming (not Virtual Keys).
- `key_paired: false` is expected; that is not the same as `unsupported_firmware`.
- If prerequisites look met but configure still returns `unsupported_firmware`,
  document and escalate via developer.tesla.com Support Inquiry (region, firmware,
  redacted VIN, fleet_status + configure payloads). China-region configure has had
  prior backend defects (#132). Do not invent server workarounds.

## Safety / contribution norms

- Follow `CONTRIBUTING.md`. Do not post full VINs, tokens, or PII in issues/PRs.
- Prefer focused PRs; avoid drive-by refactors.
- External contributors generally cannot merge to `teslamotors/fleet-telemetry`;
  open a PR for maintainer review.
