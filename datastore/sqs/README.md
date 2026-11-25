# SQS Datastore

The SQS dispatcher publishes telemetry records to Standard queues. Create the queues before publishing; the dispatcher does not create AWS resources or change their policies. FIFO queues are not supported.

## Configuration

Use the same AWS credential and region configuration as [Kinesis](../../README.md#personalized-backendsdispatchers). Grant `sqs:GetQueueUrl` and `sqs:SendMessage` on the destination queues.

The following configuration fragment sends vehicle records to SQS and uses it as the reliable ACK source:

```json
{
  "sqs": {
    "queues": { "V": "vehicle-data" },
    "max_retries": 1,
    "publish_timeout_ms": 10000
  },
  "records": { "V": ["sqs"] },
  "reliable_ack_sources": { "V": "sqs" }
}
```

`SQS_QUEUE_<UPPERCASE_RECORD_TYPE>` overrides the configured queue name. When no name is configured, it defaults to `<namespace>_<record_type>`. Queue URLs are resolved on first use and cached; startup does not require `ListQueues` permission.

The defaults are one SDK retry and a 10,000 millisecond timeout. Zero retries disables SDK retries. The timeout includes queue lookup and SDK retries. `override_host` can select a local endpoint for testing.

## Message format

The queue body contains **base64-encoded payload bytes**, including when `transmit_decoded_records` is enabled. Consumers must base64-decode the body, then parse the resulting Protobuf or JSON payload. Nonempty `txtype`, `txid`, `vin`, and `device_client_version` values are supplied as string message attributes.

Base64 increases payload size. The destination service enforces its message size limit; rejected messages follow the normal publish error path and receive no success ACK.

## Reliable ACKs

A reliable ACK confirms successful `SendMessage` acceptance, not processing by a downstream consumer. Failed or timed-out sends produce no success ACK. Deliveries can be duplicated, so consumers should tolerate retries. When sending to multiple dispatchers, the selected ACK source does not confirm delivery to the other destinations.

Closing the dispatcher cancels active requests and releases pending ACK sends.

## Integration tests

Run `make integration` from the repository root. The SQS test uses LocalStack, generated test certificates, dummy credentials, and synthetic vehicle messages. It verifies payload delivery and ACKs, then deletes the queue to verify failure without a success ACK. No Tesla vehicle or AWS account is required.

LocalStack may not implement the latest AWS size limits or IAM enforcement. Local success does not establish those production guarantees.
