# NATS Datastore

This package implements a NATS producer for the Fleet Telemetry system. NATS is a lightweight, high-performance messaging system ideal for cloud-native applications and microservices architectures.

## Overview

The NATS datastore allows the Fleet Telemetry system to publish vehicle data, alerts, errors, and connectivity events to a NATS server. It implements the `telemetry.Producer` interface and provides automatic reconnection handling.

## Key Design Decisions

1. **Subject-based routing**: Messages are published to subjects in the format `{namespace}.{vin}.{topic}`, enabling flexible subscription patterns using NATS wildcards.

2. **Automatic reconnection**: The producer is configured with unlimited reconnection attempts (`MaxReconnects(-1)`), ensuring resilience against temporary network issues.

3. **Connection lifecycle handling**: The producer registers handlers for various connection events (connect, reconnect, disconnect, close) for observability and debugging.

4. **Topic transformation**: The "V" record type is automatically transformed to "data" in the subject name for cleaner topic organization.

5. **Reliable acknowledgment support**: The producer supports reliable acknowledgment for specified transaction types, allowing critical data delivery confirmation.

## Configuration

The NATS producer is configured using a JSON object with the following fields:

- `url`: (string) The NATS server URL (e.g., "nats://localhost:4222")
- `name`: (string) A unique client name for identification in NATS server logs

Example configuration:

```json
{
  "nats": {
    "url": "nats://localhost:4222",
    "name": "fleet-telemetry"
  }
}
```

## Subject Structure

Messages are published to subjects following this pattern:

- Vehicle data (V records): `<namespace>.<VIN>.data`
- Alerts: `<namespace>.<VIN>.alerts`
- Errors: `<namespace>.<VIN>.errors`
- Connectivity: `<namespace>.<VIN>.connectivity`

### Examples

With namespace `tesla_telemetry` and VIN `5YJ3E1EA1NF123456`:

- Vehicle data: `tesla_telemetry.5YJ3E1EA1NF123456.data`
- Alerts: `tesla_telemetry.5YJ3E1EA1NF123456.alerts`
- Connectivity: `tesla_telemetry.5YJ3E1EA1NF123456.connectivity`

### Subscribing with Wildcards

NATS wildcards allow flexible subscription patterns:

- Subscribe to all data for a specific VIN: `tesla_telemetry.5YJ3E1EA1NF123456.>`
- Subscribe to all vehicle data: `tesla_telemetry.*.data`
- Subscribe to all messages: `tesla_telemetry.>`

## Payload Format

Payloads are sent as raw Protocol Buffer (protobuf) encoded bytes. Subscribers should decode using the appropriate protobuf message type:

- Vehicle data: `protos.Payload`
- Alerts: `protos.VehicleAlerts`
- Errors: `protos.VehicleErrors`
- Connectivity: `protos.VehicleConnectivity`

## Connection Handling

The producer implements robust connection handling:

- **Retry on failed connect**: Initial connection attempts are retried automatically
- **Unlimited reconnects**: After disconnection, the client will attempt to reconnect indefinitely
- **Panic on close**: If the connection is closed unexpectedly (e.g., server shutdown), the producer will panic to signal a critical failure

Connection events are logged:
- `nats_connected`: Successful initial connection
- `nats_reconnected`: Successful reconnection after disconnection
- `nats_disconnected`: Connection lost (with error details if available)
- `nats_closed`: Connection closed (triggers panic)
- `nats_error`: Subscription or other errors

## Metrics

When Prometheus metrics are enabled, the following metrics are exported:

| Metric | Type | Description |
|--------|------|-------------|
| `nats_produce_total` | Counter | Number of records produced to NATS |
| `nats_produce_total_bytes` | Counter | Number of bytes produced to NATS |
| `nats_produce_ack_total` | Counter | Number of records for which ACK was received |
| `nats_produce_ack_total_bytes` | Counter | Number of bytes for which ACK was received |
| `nats_reliable_ack_total` | Counter | Number of reliable ACKs sent |
| `nats_err` | Counter | Number of errors while producing to NATS |
| `nats_produce_queue_size` | Gauge | Total pending messages to produce |

All counters include a `record_type` label for filtering by message type.

## Error Handling and Reliability

- Publish errors are logged and reported to Airbrake (if configured)
- The error counter metric is incremented on publish failures
- Failed publishes do not trigger reliable acknowledgment

## Performance Considerations

- NATS is designed for high throughput with low latency
- Messages are published asynchronously using `nats.Conn.Publish()`
- The producer does not implement batching; each record is published individually
- For high-volume scenarios, ensure adequate NATS server capacity and network bandwidth

## NATS Server Requirements

- NATS Server 2.0 or later recommended
- No authentication is required by default (configure NATS server for production security)
- JetStream is not required for basic pub/sub functionality

## Example NATS Server Configuration

For development:

```bash
# Run NATS server
nats-server

# Or with Docker
docker run -p 4222:4222 nats:latest
```

For production, refer to the [NATS documentation](https://docs.nats.io/) for security configuration, clustering, and JetStream setup.
