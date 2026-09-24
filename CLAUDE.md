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

## Legacy Model S / Model X streaming (Intel Atom)

These vehicles are a frequent support false-alarm (see issue #537):

- Firmware **2025.20+** is required; `fleet_status.fleet_telemetry_version` becomes non-null.
- They do **not** use Virtual Keys for Fleet Telemetry.
- Owners must enable **Controls → Safety → Allow Third-Party App Data Streaming**.
- Diagnose with `fleet_status.safety_screen_streaming_toggle_enabled`.
- `updated_vehicles: 1` on config create only means config was accepted, not that
  the WebSocket is connected.
- Pre-2018 MCU1 Model S/X are unsupported (`unsupported_hardware`).

Prefer documenting / linking official Fleet API behavior over inventing server-side
workarounds. The open-source server cannot force a vehicle to stream if the Safety
toggle is off or firmware is too old.

## Safety / contribution norms

- Follow `CONTRIBUTING.md`. Do not post VINs, tokens, account data, or other PII
  in issues or PRs (redact VINs in logs).
- Prefer focused PRs; avoid drive-by refactors.
- External contributors generally cannot merge to `teslamotors/fleet-telemetry`;
  open a PR for maintainer review.
