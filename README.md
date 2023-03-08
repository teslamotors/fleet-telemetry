# Tesla Telemetry
---------------------------------

Telemetry is a simple, scalable, and secure data exchange for device fleets.

Clients establish a websocket connection to push configurable telemetry records. Telemetry provides clients with ack, error, or rate limit responses.

# Messages

Messages are sent using a Flatbuffer wrapper, and are one of the proto types or raw bytes/strings

# Protos

These represent different message types.  Generate via protoc, ex.: `protoc --go_out=./ --go_opt=paths=source_relative protos/vehicle_data.proto`
