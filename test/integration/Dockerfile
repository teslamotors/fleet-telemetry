# build executable and run integration tests
FROM golang:1.23.0-bullseye

# build libsodium (dep of libzmq)
WORKDIR /build
RUN wget https://github.com/jedisct1/libsodium/releases/download/1.0.19-RELEASE/libsodium-1.0.19.tar.gz
RUN tar -xzvf libsodium-1.0.19.tar.gz
WORKDIR /build/libsodium-stable
RUN ./configure --disable-shared --enable-static
RUN make -j`nproc`
RUN make install

# build libzmq (dep of zmq datastore)
WORKDIR /build
RUN wget https://github.com/zeromq/libzmq/releases/download/v4.3.4/zeromq-4.3.4.tar.gz
RUN tar -xvf zeromq-4.3.4.tar.gz
WORKDIR /build/zeromq-4.3.4
RUN ./configure --enable-static --disable-shared --disable-Werror
RUN make -j`nproc`
RUN make install

ENV CGO_ENABLED=1
ENV CGO_LDFLAGS="-lstdc++"

WORKDIR /go/src/fleet-telemetry

COPY go.* ./
RUN go mod download

COPY . ./


RUN --mount=type=cache,target=/root/.cache/go-build \
make build

WORKDIR /go/src/fleet-telemetry/test/integration

ARG it_target
ENV IT_TARGET=${it_target}

# hadolint ignore=DL3025
CMD go test -v ${IT_TARGET}
