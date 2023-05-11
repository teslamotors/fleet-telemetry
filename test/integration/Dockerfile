FROM golang:1.20.3

COPY . /go/src/vehicle-telemetry

WORKDIR /go/src/vehicle-telemetry/test/integration

ARG it_target
ENV IT_TARGET=${it_target}

# hadolint ignore=DL3025
CMD go test -v ${IT_TARGET}
