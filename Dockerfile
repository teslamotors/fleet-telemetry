# Start by building the application.
FROM golang:1.20.5-bullseye as build

WORKDIR /go/src/fleet-telemetry

COPY . .
ENV CGO_ENABLED=1

RUN make

# hadolint ignore=DL3006
FROM gcr.io/distroless/base-debian11
WORKDIR /
COPY --from=build /go/bin/fleet-telemetry /

CMD ["/fleet-telemetry", "-config", "/etc/fleet-telemetry/config.json"]
