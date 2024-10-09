# MQTT Datastore

This package implements an MQTT (Message Queuing Telemetry Transport) producer for the Fleet Telemetry system. MQTT is particularly well-suited for fleet telemetry systems due to its lightweight, publish-subscribe architecture.

## Overview

The MQTT datastore allows the Fleet Telemetry system to publish vehicle data, alerts, and errors to an MQTT broker. It uses the Paho MQTT client library for Go and implements the `telemetry.Producer` interface.

## Key Design Decisions

1. **Separate topics for different data types**: We use distinct topic structures for metrics, alerts, and errors to allow easy filtering and processing by subscribers.

2. **Individual field publishing**: Each metric field is published as a separate MQTT message, allowing for granular updates and subscriptions.

3. **Current state and history for alerts**: We maintain both the current state and history of alerts, supporting both clients that require real-time monitoring and clients that require historical analysis.

4. **Configurable QoS and retention**: The MQTT QoS level and message retention can be configured to balance between performance and reliability.

5. **Reliable acknowledgment support**: The producer supports reliable acknowledgment for specified transaction types. However, it's important to note that the entire packet from the vehicle will be not be acknowledged if any of the related MQTT publish operations fail. This ensures data integrity by preventing partial updates and allows for retrying the complete set of data in case of any publishing issues.

## Configuration

The MQTT producer is configured using a JSON object with the following fields:

- `broker`: (string) The MQTT broker "host:port". (for example "localhost:1883")
- `client_id`: (string) A unique identifier for the MQTT client.
- `username`: (string) The username for MQTT broker authentication. (optional)
- `password`: (string) The password for MQTT broker authentication. (optional)
- `topic_base`: (string) The base topic for all MQTT messages.
- `qos`: (number) The Quality of Service level (0, 1, or 2). Default: 0
- `retained`: (boolean) Whether messages should be retained by the broker. Default: false
- `connect_timeout_ms`: (number) Connection timeout in milliseconds. Default: 30000
- `publish_timeout_ms`: (number) Publish operation timeout in milliseconds. Default: 2500
- `disconnect_timeout_ms`: (number) Disconnection timeout in milliseconds. Default: 250
- `connect_retry_interval_ms`: (number) Interval between connection retry attempts in milliseconds. Default: 10000
- `keep_alive_seconds`: (number) Keep-alive interval in seconds. Default: 30

Example configuration:

```json
{
  "mqtt": {
    "broker": "localhost:1883",
    "client_id": "fleet-telemetry",
    "username": "your_username",
    "password": "your_password",
    "topic_base": "telemetry",
    "qos": 1,
    "retained": false,
    "connect_timeout_ms": 30000,
    "publish_timeout_ms": 2500,
    "disconnect_timeout_ms": 250,
    "connect_retry_interval_ms": 10000,
    "keep_alive_seconds": 30
  }
}
```

The MQTT producer will use default values for any omitted fields as specified above.

## Topic Structure

- Metrics: `<topic_base>/<VIN>/v/<field_name>`
- Alerts (current state): `<topic_base>/<VIN>/alerts/<alert_name>/current`
- Alerts (history): `<topic_base>/<VIN>/alerts/<alert_name>/history`
- Errors: `<topic_base>/<VIN>/errors/<error_name>`

## Payload Formats

- Metrics: `{"value": <field_value>}`
- Alerts: `{"Name": <string>, "StartedAt": <timestamp>, "EndedAt": <timestamp>, "Audiences": [<string>]}`
- Errors: `{"Name": <string>, "Body": <string>, "Tags": {<string>: <string>}, "CreatedAt": <timestamp>}`

Note: The field contents and type are determined by the car. Fields may have their types updated with different software and vehicle versions to optimize for precision or space. For example, a float value like the vehicle's speed might be received as 12.3 (numeric) in one version and as "12.3" (string) in another version.

## Error Handling and Reliability

- The producer implements reconnection logic with configurable retry intervals.
- Publish operations have a configurable timeout to prevent blocking indefinitely.
- The producer supports reliable acknowledgment for specified transaction types, ensuring critical data is not lost.

## Performance Considerations

- Each field is published as a separate MQTT message, which can increase network traffic but allows for more granular subscriptions.
- QoS levels can be configured to balance between performance and reliability.
- The producer uses goroutines to handle message publishing asynchronously.

