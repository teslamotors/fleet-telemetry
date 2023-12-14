[![Build and Test](https://github.com/teslamotors/fleet-telemetry/actions/workflows/build.yml/badge.svg?branch=main)](https://github.com/teslamotors/fleet-telemetry/actions/workflows/build.yml)
[![Current Version](https://img.shields.io/github/v/tag/teslamotors/fleet-telemetry?label=latest%20tag)](https://github.com/teslamotors/fleet-telemetry/tags)
[![DockerHub Tags](https://img.shields.io/docker/v/tesla/fleet-telemetry?label=docker%20tags)](https://hub.docker.com/r/tesla/fleet-telemetry/tags)

# Tesla Fleet Telemetry
---------------------------------

Fleet Telemetry is a server reference implementation for Tesla's telemetry protocol. Owners can allow registered applications to receive telemetry securely and directly from their vehicles. This reference implementation can be used by individual owners as is or by fleet operators who can extend it to aggregate data accross their fleet.

The service handles device connectivity as well as receiving and storing transmitted data. Once configured, devices establish a WebSocket connection to push configurable telemetry records. Fleet Telemetry provides clients with ack, error, or rate limit responses.

## Requirements
These are the minimum system requirements for a Fleet Telemetry server, with 10 active vehicles connected, running on a Debian-based server with regular day-to-day usage. The requirements will change based on vehicle usage and quantity of vehicles.

* Allocate at least 0.2 AWS vCPU, 256MB RAM, and 8GB storage
* Host on a publicly available server with an associated domain

### Recommendations
* Run on a Debian base image
* Allocate and configure a server certificate using best practices

### Dependencies
* Go 1.20+
* libzmq
* Docker*
* Kubernetes/Helm*

*Kubernetes/Helm is required when using the Helm Charts deployment method (recommended) and Docker is required for deploying Fleet Telemetry via a docker image.

### Vehicle Compatibility

Vehicles must be running firmware version 2023.20.6 or later. You may find this information under your VIN in the Tesla Mobile App or the Software tab in the vehicle's infotainment system. Some older model S/X are not supported.

## Configuring and running the service
You may generate [self-signed certificates](https://en.wikipedia.org/wiki/Self-signed_certificate) for your domain and the generated tls certificate + private key pair will be used to authenticate vehicles to your Fleet Telemetry server. Tesla vehicles rely on a mutual TLS (mTLS) WebSocket to create a connection with the backend.

You may generate a self-signed certificate using the OpenSSL command below, though we recommend using best practices for generating a server certificate from a trusted certificate authority (have your domain/CNAME ready):

```sh
openssl req -newkey rsa:2048 -nodes -keyout key.pem -x509 -days 365 -out certificate.pem
```

Before you deploy your Fleet Telemetry server, you will need to configure your server (the docker image will check the config mounted on `/etc/fleet-telemetry/config.json`). Server config template:
```
{
  "host": string - hostname,
  "port": int - port,
  "log_level": string - trace, debug, info, warn, error,
  "json_log_enable": bool,
  "namespace": string - kafka topic prefix,
  "reliable_ack": bool - for use with reliable datastores, recommend setting to true with kafka,
  "monitoring": {
    "prometheus_metrics_port": int,
    "profiler_port": int,
    "profiling_path": string - out path,
    "statsd": { if you are not using prometheus
      "host": string - host:port of the statsd server,
      "prefix": string - prefix for statsd metrics,
      "sample_rate": int - 0 to 100 percentage to sample stats,
      "flush_period": int - ms flush period
    }
  },
  "kafka": { // librdkafka kafka config, seen here: https://raw.githubusercontent.com/confluentinc/librdkafka/master/CONFIGURATION.md
    "bootstrap.servers": "kafka:9092",
    "queue.buffering.max.messages": 1000000
  },
  "kinesis": {
    "max_retries": 3,
    "streams": {
      "V": "custom_stream_name"
    }
  },
  "rate_limit": {
    "enabled": bool,
    "message_limit": int - ex.: 1000
  },
  "records": { // list of records and their dispatchers, currently: alerts, errors, and V(vehicle data)
    "alerts": [
        "logger"
    ],
    "errors": [
        "logger"
    ],
    "V": [
        "kinesis",
        "kafka"
    ]
  },
  "tls": { // Note: setting a ca_file here can be used for testing, but may reject vehicle mTLS connections
    "server_cert": string - server cert location,
    "server_key": string - server key location
  }
}
```
Example: [server_config.json](./examples/server_config.json)

### Deploy using Kubernetes with Helm Charts (recommended for large fleets)
Hosting on [Kubernetes](https://kubernetes.io/) will enable the service to scale for large fleets. Helm Charts help you define, install, and upgrade applications on Kubernetes. Reference Helm chart [here](https://github.com/teslamotors/helm-charts/blob/main/charts/fleet-telemetry/README.md).

You must have [Kubernetes](https://kubernetes.io/docs/setup/) amd [Helm](https://helm.sh/docs/intro/install/) installed for this deployment method.

### Deploy using Docker

1. Pull the Tesla Fleet Telemetry image (you may find all images in [docker hub](https://hub.docker.com/r/tesla/fleet-telemetry/tags)):
```sh
docker pull tesla/fleet-telemetry:v0.1.8
```
2. Run and deploy your docker image with your mTLS certificate, private key, and config.json locally mounted on `/etc/fleet-telemetry`:
```sh
sudo docker run -v /etc/fleet-telemetry:/etc/fleet-telemetry tesla/fleet-telemetry:v0.1.8
```
### Deploy manually
1. Build the server
```sh
make install
```

2. Deploy and run the server. This can be run as a binary via `$GOPATH/bin/fleet-telemetry -config=/etc/fleet-telemetry/config.json` directly on a server, or as a Kubernetes deployment. Example snippet:
```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: fleet-telemetry
spec:
  replicas: 1
  selector:
    matchLabels:
      app: fleet-telemetry
  template:
    metadata:
      labels:
        app: fleet-telemetry
    spec:
      containers:
      - name: fleet-telemetry
        image: tesla/fleet-telemetry:<tag>
        command: ["/fleet-telemetry", "-config=/etc/fleet-telemetry/config.json"]
        ports:
        - containerPort: 443
---
apiVersion: v1
kind: Service
metadata:
  name: fleet-telemetry
spec:
  selector:
    app: fleet-telemetry
  ports:
    - protocol: TCP
      port: 443
      targetPort: 443
  type: LoadBalancer
```

3. Register vehicles for streaming via [fleet-telemetry-config](https://developer.tesla.com/docs/fleet-api#fleet_telemetry_config) API.

## Backends/dispatchers
The following [dispatchers](./telemetry/producer.go#L10-L19) are supported
* Kafka (preferred): Configure with the config.json file.  See implementation here: [config/config.go](./config/config.go)
* Kinesis: Configure with standard [AWS env variables and config files](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html). The default AWS credentials and config files are: `~/.aws/credentials` and `~/.aws/config`.
  * By default, stream names will be \*configured namespace\*_\*topic_name\*  ex.: `tesla_V`, `tesla_errors`, `tesla_alerts`, etc
  * Configure stream names directly by setting the streams config `"kinesis": { "streams": { *topic_name*: stream_name } }`
  * Override stream names with env variables: KINESIS_STREAM_\*uppercase topic\* ex.: `KINESIS_STREAM_V`
* Google pubsub: Along with the required pubsub config (See ./test/integration/config.json for example), be sure to set the environment variable `GOOGLE_APPLICATION_CREDENTIALS`
* ZMQ: Configure with the config.json file.  See implementation here: [config/config.go](./config/config.go)
* Logger: This is a simple STDOUT logger that serializes the protos to json.
  
>NOTE: To add a new dispatcher, please provide integration tests and updated documentation.

## Metrics
Prometheus or a StatsD interface supporting data store for metrics. This is required for monitoring your applications.

## Protos
Data is encapsulated into protobuf messages of different types. We do not recommend making changes, but if you need to recompile them you can always do so with:

  1. Install protoc, currently on version 4.25.1: https://grpc.io/docs/protoc-installation/
  2. Install protoc-gen-go: `go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28`
  3. Run make command
  ```sh
  make generate-protos
  ```

# Testing

## Unit Tests
To run the unit tests: `make test`

Common Errors:

```
~/fleet-telemetry➜ git:(main) ✗  make test
go build github.com/confluentinc/confluent-kafka-go/v2/kafka:
# pkg-config --cflags  -- rdkafka
Package rdkafka was not found in the pkg-config search path.
Perhaps you should add the directory containing `rdkafka.pc'
to the PKG_CONFIG_PATH environment variable
No package 'rdkafka' found
pkg-config: exit status 1
make: *** [install] Error 1
```
librdkafka is missing, on macOS you can install it via `brew install librdkafka pkg-config` or follow instructions here https://github.com/confluentinc/confluent-kafka-go#getting-started

```
~/fleet-telemetry➜ git:(main) ✗  make test
go build github.com/confluentinc/confluent-kafka-go/v2/kafka:
# pkg-config --cflags  -- rdkafka
Package libcrypto was not found in the pkg-config search path.
Perhaps you should add the directory containing `libcrypto.pc'
to the PKG_CONFIG_PATH environment variable
Package 'libcrypto', required by 'rdkafka', not found
pkg-config: exit status 1
make: *** [install] Error 1

~/fleet-telemetry➜ git:(main) ✗  locate libcrypto.pc
/opt/homebrew/Cellar/openssl@3/3.0.8/lib/pkgconfig/libcrypto.pc

~/fleet-telemetry➜ git:(main) ✗  export PKG_CONFIG_PATH=$PKG_CONFIG_PATH:/opt/homebrew/Cellar/openssl@3/3.0.8/lib/pkgconfig/
```
A reference to libcrypto is not set properly. To resolve find the reference to libcrypto by pkgconfig and set et the PKG_CONFIG_PATH accordingly.

libzmq is missing. Install with:
```sh
sudo apt install -y libsodium-dev libzmq3-dev
```
Or for macOS:
```sh
brew install libsodium zmq
```

## Integration Tests

To run the integration tests: `make integration`

## Building the binary for Linux from Mac ARM64

```sh
DOCKER_BUILD_KIT=1 DOCKER_CLI_EXPERIMENTAL=enabled docker buildx version
docker buildx create --name go-builder --driver docker-container --driver-opt network=host --buildkitd-flags '--allow-insecure-entitlement network.host' --use
docker buildx inspect --bootstrap
docker buildx build --no-cache --progress=plain --platform linux/amd64 -t <name:tag>(e.x.: fleet-telemetry:local.1.1) -f Dockerfile . --load
container_id=$(docker create fleet-telemetry:local.1.1) docker cp $container_id:/fleet-telemetry /tmp/fleet-telemetry
```

## Security and Privacy considerations

System administrators should apply standard best practices, which are beyond
the scope of this README.

Moreover, the following application-specific considerations apply:

* Vehicles authenticate to the telemetry server with TLS client certificates
  and use a variety of security measures designed to prevent unauthorized
  access to the corresponding private key. However, as a defense-in-depth
  precaution, backend services should anticipate the possibility that a
  vehicle's TLS private key may be compromised. Therefore:
  * Backend systems should sanitize data before using it.
  * Users should consider threats from actors that may be incentivized to
    submit falsified data.
  * Users should filter by vehicle identification number (VIN) using an
    allowlist if possible.
* Configuration-signing private keys should be kept offline.
* Configuration-signing private keys should be kept in an HSM.
* If telemetry data is compromised, threat actors may be able to make
  inferences about driver behavior even if explicit location data is not
  collected. Security policies should be set accordingly.
* Tesla strongly encourages providers to only collect data they need, limited to
  the frequency they need.
* Providers agree to take full responsibility for privacy risks, as soon as data
  leave the devices (for more info read our privacy policies).
