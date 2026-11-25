# SNS Datastore

The SNS dispatcher publishes telemetry records to Standard topics. Create the topics and any subscriptions before publishing; the dispatcher does not create AWS resources or change their policies. FIFO topics are not supported.

## Configuration

Use the same AWS credential and region configuration as [Kinesis](../../README.md#personalized-backendsdispatchers). Grant `sns:Publish` on the destination topics.

The following configuration fragment sends vehicle records to SNS and uses it as the reliable ACK source:

```json
{
  "sns": {
    "topics": { "V": "arn:aws:sns:us-west-2:123456789012:vehicle-data" },
    "max_retries": 1,
    "publish_timeout_ms": 10000
  },
  "records": { "V": ["sns"] },
  "reliable_ack_sources": { "V": "sns" }
}
```

`SNS_TOPIC_<UPPERCASE_RECORD_TYPE>` overrides the configured topic ARN. Each selected record type must have an explicit ARN; there is no namespace fallback. Startup does not require `ListTopics` permission.

The defaults are one SDK retry and a 10,000 millisecond timeout. Zero retries disables SDK retries. The timeout includes SDK retries. `override_host` can select a local endpoint for testing.

## Message format

The published message contains **base64-encoded payload bytes**, including when `transmit_decoded_records` is enabled. Consumers must base64-decode the message, then parse the resulting Protobuf or JSON payload. Nonempty `txtype`, `txid`, `vin`, and `device_client_version` values are supplied as string message attributes.

Base64 increases payload size. The destination service enforces its message size limit; rejected messages follow the normal publish error path and receive no success ACK.

## SQS subscriptions

An SNS subscription to SQS needs a queue policy allowing `sns.amazonaws.com` to call `sqs:SendMessage`, restricted to the topic's ARN; see the [AWS subscription instructions](https://docs.aws.amazon.com/sns/latest/dg/subscribe-sqs-queue-to-sns-topic.html).

With raw message delivery enabled, the queue body is the base64 payload directly. Otherwise, decode the payload from the SNS notification's `Message` field.

## Reliable ACKs

A reliable ACK confirms successful `Publish` acceptance, not delivery to a subscriber or processing by a downstream consumer. Failed or timed-out publications produce no success ACK. Deliveries can be duplicated, so consumers should tolerate retries. When sending to multiple dispatchers, the selected ACK source does not confirm delivery to the other destinations.

Closing the dispatcher cancels active requests and releases pending ACK sends.

## Integration tests

Run `make integration` from the repository root. The SNS test uses LocalStack, generated test certificates, dummy credentials, and synthetic vehicle messages. It receives notifications through a subscribed SQS queue, verifies payload delivery and ACKs, then deletes the topic to verify failure without a success ACK. No Tesla vehicle or AWS account is required.

LocalStack may not implement the latest AWS size limits or IAM enforcement. Local success does not establish those production guarantees.
