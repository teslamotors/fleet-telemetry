FROM golang:1.20.5-bullseye

COPY . /go/src/fleet-telemetry

WORKDIR /go/src/fleet-telemetry/test/integration

ARG it_target
ENV IT_TARGET=${it_target}

# hadolint ignore=DL3025
CMD go test -v ${IT_TARGET}
