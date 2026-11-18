package mqtt_test

import (
    "bytes"
    "context"
    "testing"

    "github.com/teslamotors/fleet-telemetry/datastore/mqtt"
    logrus "github.com/teslamotors/fleet-telemetry/logger"
    "github.com/teslamotors/fleet-telemetry/messages"
    "github.com/teslamotors/fleet-telemetry/metrics"
    "github.com/teslamotors/fleet-telemetry/protos"
    "github.com/teslamotors/fleet-telemetry/server/airbrake"
    "github.com/teslamotors/fleet-telemetry/telemetry"
    "google.golang.org/protobuf/proto"
    "google.golang.org/protobuf/encoding/protowire"
)

func newVehicleFieldsTestProducer(t *testing.T) (telemetry.Producer, chan *telemetry.Record) {
    t.Helper()
    resetPublishedTopics()
    previous := mqtt.PahoNewClient
    mqtt.PahoNewClient = mockPahoNewClient
    t.Cleanup(func() { mqtt.PahoNewClient = previous })

    logger, _ := logrus.NoOpLogger()
    acks := make(chan *telemetry.Record, 2)
    producer, err := mqtt.NewProducer(context.Background(),
        &mqtt.Config{Broker: "tcp://localhost:1883", TopicBase: "tf03", ClientID: "synthetic"},
        metrics.NewCollector(nil, logger), "synthetic",
        airbrake.NewAirbrakeHandler(nil),
        acks, map[string]interface{}{"V": true}, logger)
    if err != nil { t.Fatal(err) }
    return producer, acks
}

func newVehicleFieldsTestRecord(t *testing.T, data []*protos.Datum) *telemetry.Record {
    t.Helper()
    dataBytes, err := proto.Marshal(&protos.Payload{Data: data})
    if err != nil { t.Fatal(err) }
    logger, _ := logrus.NoOpLogger()
    serializer := telemetry.NewBinarySerializer(
        &telemetry.RequestIdentity{DeviceID: "TEST123", SenderID: "vehicle_device.TEST123"},
        map[string][]telemetry.Producer{}, logger)
    raw, err := (&messages.StreamMessage{
        TXID: []byte("tf03-semantics"),
        SenderID: []byte("vehicle_device.TEST123"),
        MessageTopic: []byte("V"),
        Payload: dataBytes,
    }).ToBytes()
    if err != nil { t.Fatal(err) }
    rec, err := telemetry.NewRecord(serializer, raw, "synthetic-socket", false)
    if err != nil { t.Fatalf("unexpected protobuf/record rejection: %v", err) }
    return rec
}

func assertVehicleFieldsNoAck(t *testing.T, acks chan *telemetry.Record) {
    t.Helper()
    select { case <-acks: t.Fatal("unexpected reliable ACK for untransmitted record"); default: }
}

func assertVehicleFieldsAck(t *testing.T, acks chan *telemetry.Record, rec *telemetry.Record) {
    t.Helper()
    select {
    case got := <-acks:
        if got != rec { t.Fatal("ACK belongs to a different record") }
    default:
        t.Fatal("missing reliable ACK for successful record")
    }
}

// A single missing value invalidates the batch before any MQTT publish.
// Partial publish + reliable ACK would lose the omitted field silently.
func TestMQTTVehicleFieldsMixedMalformedFailsAtomically(t *testing.T) {
    p, acks := newVehicleFieldsTestProducer(t)
    rec := newVehicleFieldsTestRecord(t, []*protos.Datum{
        {Key: protos.Field_BatteryLevel, Value: &protos.Value{Value: &protos.Value_FloatValue{FloatValue: 75.5}}},
        {Key: protos.Field_Odometer},
        {Key: protos.Field_TimeToFullCharge, Value: &protos.Value{Value: &protos.Value_Invalid{Invalid: true}}},
    })
    defer func() {
        if r := recover(); r != nil { t.Fatalf("invalid field: mixed record panicked: %v", r) }
    }()
    p.Produce(rec)
    if len(publishedTopics) != 0 { t.Fatalf("partial MQTT publishes occurred: %v", publishedTopics) }
    assertVehicleFieldsNoAck(t, acks)
}

// Missing all fields must not be silently ACKed as successful delivery.
func TestMQTTVehicleFieldsAllMalformedNoAck(t *testing.T) {
    p, acks := newVehicleFieldsTestProducer(t)
    rec := newVehicleFieldsTestRecord(t, []*protos.Datum{{Key: protos.Field_BatteryLevel}})
    defer func() {
        if r := recover(); r != nil { t.Fatalf("invalid field: all-missing record panicked: %v", r) }
    }()
    p.Produce(rec)
    if len(publishedTopics) != 0 { t.Fatalf("malformed payload published values: %v", publishedTopics) }
    assertVehicleFieldsNoAck(t, acks)
}

// A present Value message without a oneof is also semantically absent.
// Do not turn this into an accepted MQTT '{}' payload.
func TestMQTTVehicleFieldsEmptyOneofNoAck(t *testing.T) {
    p, acks := newVehicleFieldsTestProducer(t)
    rec := newVehicleFieldsTestRecord(t, []*protos.Datum{{Key: protos.Field_BatteryLevel, Value: &protos.Value{}}})
    defer func() {
        if r := recover(); r != nil { t.Fatalf("empty oneof panicked: %v", r) }
    }()
    p.Produce(rec)
    if len(publishedTopics) != 0 { t.Fatalf("empty oneof was published: %v", publishedTopics) }
    assertVehicleFieldsNoAck(t, acks)
}

func TestMQTTVehicleFieldsNilDatumNoAck(t *testing.T) {
    p, acks := newVehicleFieldsTestProducer(t)
    rec := newVehicleFieldsTestRecord(t, []*protos.Datum{nil})
    defer func() {
        if r := recover(); r != nil { t.Fatalf("invalid field: nil datum panicked: %v", r) }
    }()
    p.Produce(rec)
    if len(publishedTopics) != 0 { t.Fatalf("nil datum was published: %v", publishedTopics) }
    assertVehicleFieldsNoAck(t, acks)
}

// Preserve the preexisting contract: a genuinely empty but well-formed
// telemetry frame has no fields to publish and is still acknowledged.
func TestMQTTVehicleFieldsEmptyPayloadLegacyAck(t *testing.T) {
    p, acks := newVehicleFieldsTestProducer(t)
    rec := newVehicleFieldsTestRecord(t, nil)
    p.Produce(rec)
    if len(publishedTopics) != 0 { t.Fatalf("empty payload published data: %v", publishedTopics) }
    assertVehicleFieldsAck(t, acks, rec)
}

func TestMQTTVehicleFieldsValidAndExplicitInvalidPublishAndAck(t *testing.T) {
    p, acks := newVehicleFieldsTestProducer(t)
    rec := newVehicleFieldsTestRecord(t, []*protos.Datum{
        {Key: protos.Field_BatteryLevel, Value: &protos.Value{Value: &protos.Value_FloatValue{FloatValue: 75.5}}},
        {Key: protos.Field_TimeToFullCharge, Value: &protos.Value{Value: &protos.Value_Invalid{Invalid: true}}},
    })
    p.Produce(rec)
    if len(publishedTopics) != 2 { t.Fatalf("expected two publishes, got %v", publishedTopics) }
    if got := string(publishedTopics["tf03/TEST123/v/BatteryLevel"]); got != "75.5" {
        t.Fatalf("numeric publish changed: %q", got)
    }
    if got := string(publishedTopics["tf03/TEST123/v/TimeToFullCharge"]); got != "null" {
        t.Fatalf("explicit invalid publish changed: %q", got)
    }
    assertVehicleFieldsAck(t, acks, rec)
}


// Future protobuf oneof alternatives are unknown to an older server, but their
// wire bytes must survive NewRecord and must not become a successful MQTT {}.
func TestMQTTVehicleFieldsUnknownWireValueNoAck(t *testing.T) {
    cases := []struct {
        name string
        raw []byte
    }{
        {
            name: "future_varint",
            raw: protowire.AppendVarint(
                protowire.AppendTag(nil, 127, protowire.VarintType), 42),
        },
        {
            name: "future_bytes",
            raw: protowire.AppendBytes(
                protowire.AppendTag(nil, 128, protowire.BytesType), []byte("future-value")),
        },
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            future := &protos.Value{}
            if err := proto.Unmarshal(tc.raw, future); err != nil {
                t.Fatalf("cannot decode unknown value wire: %v", err)
            }
            if future.Value != nil {
                t.Fatal("future wire alternative unexpectedly resolved to a known oneof")
            }
            p, acks := newVehicleFieldsTestProducer(t)
            rec := newVehicleFieldsTestRecord(t, []*protos.Datum{
                {Key: protos.Field_BatteryLevel, Value: &protos.Value{Value: &protos.Value_FloatValue{FloatValue: 75.5}}},
                {Key: protos.Field_Odometer, Value: future},
            })
            parsed, ok := rec.GetProtoMessage().(*protos.Payload)
            if !ok || len(parsed.Data) != 2 || parsed.Data[1] == nil || parsed.Data[1].Value == nil {
                t.Fatal("NewRecord lost future-value datum")
            }
            gotValue := parsed.Data[1].Value
            if gotValue.Value != nil || !bytes.Equal(gotValue.ProtoReflect().GetUnknown(), tc.raw) {
                t.Fatalf("unknown value bytes changed during NewRecord: %x", gotValue.ProtoReflect().GetUnknown())
            }
            defer func() {
                if r := recover(); r != nil {
                    t.Fatalf("future value panicked in MQTT producer: %v", r)
                }
            }()
            p.Produce(rec)
            if len(publishedTopics) != 0 {
                t.Fatalf("unknown value was published or caused partial publish: %v", publishedTopics)
            }
            assertVehicleFieldsNoAck(t, acks)
        })
    }
}
