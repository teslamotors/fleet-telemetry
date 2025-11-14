# Start by building the application.
FROM golang:1.25-trixie AS build

# build libsodium (dep of libzmq)
WORKDIR /build
RUN wget https://github.com/jedisct1/libsodium/releases/download/1.0.20-RELEASE/libsodium-1.0.20.tar.gz
RUN tar -xzvf libsodium-1.0.20.tar.gz
WORKDIR /build/libsodium-1.0.20
RUN ./configure --disable-shared --enable-static
RUN make -j`nproc`
RUN make install

# build libzmq (dep of zmq datastore)
WORKDIR /build
RUN wget https://github.com/zeromq/libzmq/releases/download/v4.3.5/zeromq-4.3.5.tar.gz
RUN tar -xvf zeromq-4.3.5.tar.gz
WORKDIR /build/zeromq-4.3.5
RUN ./configure --enable-static --disable-shared --disable-Werror
RUN make -j`nproc`
RUN make install

WORKDIR /go/src/fleet-telemetry

COPY . .
ENV CGO_ENABLED=1
ENV CGO_LDFLAGS="-lstdc++"

RUN make

# hadolint ignore=DL3006
FROM gcr.io/distroless/cc-debian13:nonroot
WORKDIR /
COPY --from=build /go/bin/fleet-telemetry /

CMD ["/fleet-telemetry", "-config", "/etc/fleet-telemetry/config.json"]
