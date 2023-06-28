# build executable and run integration tests
FROM golang:1.20.5-bullseye

ENV CGO_ENABLED=1

WORKDIR /go/src/fleet-telemetry

COPY go.* ./
RUN go mod download

COPY . ./

RUN --mount=type=cache,target=/root/.cache/go-build \
go build -o /go/bin/fleet-telemetry cmd/main.go

WORKDIR /go/src/fleet-telemetry/test/integration

ARG it_target
ENV IT_TARGET=${it_target}

# hadolint ignore=DL3025
CMD go test -v ${IT_TARGET}
