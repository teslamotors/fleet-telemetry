[![Build and Test](https://github.com/teslamotors/fleet-telemetry/actions/workflows/build.yml/badge.svg?branch=main)](https://github.com/teslamotors/fleet-telemetry/actions/workflows/build.yml)
[![Current Version](https://img.shields.io/github/v/tag/teslamotors/fleet-telemetry?label=latest%20tag)](https://github.com/teslamotors/fleet-telemetry/tags)
[![DockerHub Tags](https://img.shields.io/docker/v/tesla/fleet-telemetry?label=docker%20tags)](https://hub.docker.com/r/tesla/fleet-telemetry/tags)

# Tesla Fleet Telemetry
---------------------------------

At Tesla we believe that security and privacy are core tenets of any modern technology. Customers should be able to decide what data they share with third parties, how they share it, and when it can be shared. We've developed a decentralized framework: "Fleet Telemetry" that allows customers to create a secure and direct bridge from their Tesla devices to any provider they authorize. Fleet Telemetry is a simple, scalable, and secure data exchange service for devices.

Fleet Telemetry is a server reference implementation. The service handles vehicle/device connectivity, receives and stores transmitted data. Once configured, devices establish a websocket connection to push configurable telemetry records. Fleet Telemetry provides clients with ack, error, or rate limit responses.


## Configuring and running the service

As a service provider you will need to register a publically available endpoint on the internet so vehicles (or other devices) can connect to it. Tesla devices will rely on mutual TLS (mTLS) websocket to accept creating a connection with the backend. Here are the steps you need to follow in order to get the service up and running. The application has been designed to operate on top of kubernetes but you can run it as a standalone binary if you prefer.

### Install on kubernetes with Helm Chart (recommended)
Please follow these [instructions](https://github.com/teslamotors/helm-charts/blob/main/charts/fleet-telemetry/README.md)

### Manual install (Skip this if you have installed with Helm on Kubernetes)
1. Allocate and assign a [FQDN](https://en.wikipedia.org/wiki/Fully_qualified_domain_name), this will be used in the server and client (vehicle) configuration.

2. Design a simple hosting architecture.  We recommend: Firewall/Loadbalancer -> Fleet Telemetry -> Kafka.

3. Ensure mTLS connections are terminated on the Fleet Telemetry service.

4. Configure the Server
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
  "kinesis" {
    "max_retries": 3,
    "streams": {
      "V": "custom_stream_name"
    }
  }
  "rate_limit": {
    "enabled": bool,
    "message_limit": int - ex.: 1000
  },
  "records": { list of records and their dispatchers, currently: alerts, errors, and V(vehicle data)
    "alerts": [
        "logger"
    ],
    "errors": [
        "logger"
    ],
    "V": [
        "kinesis"
    ]
  },
  "tls": {
    "server_cert": string - server cert location,
    "server_key": string - server key location
  }
}
```
Example: [server_config.json](./examples/server_config.json)

5. Deploy and run the server.  Get the latest docker image information from [docker hub](https://hub.docker.com/r/tesla/fleet-telemetry/tags). This can be run as a binary via `./fleet-telemetry -config=/etc/fleet-telemetry/config.json` directly on a server, or as a kubernetes deployment.  Example snippet:
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

6. Create and share a vehicle configuration with Tesla
```
{
  "hostname": string - server hostname,
  "ca": string - pem format ca certificate(s),
  "fields": { map of field configurations
    name (string) -> {
        "interval_seconds": int - data polling interval in seconds
    }...
  },
  "alert_types": [ string list - alerts audiences that should be pushed to the server, recommendation is to use only "service" ]
}
```
Example: [client_config.json](./examples/client_config.json)

## Backends/dispatchers
The following [dispatchers](./telemetry/producer.go#L10-L19) are supported
1. Kafka (preferred): Configure with the config.json file.  See implementation here: [config/config.go](./config/config.go)
2. Kinesis: Configure with standard [AWS env variables and config files](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html). The default aws credentials and config files are: `~/.aws/credentials` and `~/.aws/config`.
  a. By default stream names will be *configured namespace*_*topic_name*  ex.: tesla_V, tesla_errors, tesla_alerts, etc
  b. Configure stream names directly by setting the streams config `"kinesis": { "streams": { *topic_name*: stream_name } }`
  c. Override stream names with env variables: KINESIS_STREAM_*uppercase topic* ex.: `KINESIS_STREAM_V`
3. Google pubsub: Along with the required pubsub config (See ./test/integration/config.json for example), be sure to set the environment variable `GOOGLE_APPLICATION_CREDENTIALS`
4. Logger: This is a simple STDOUT logger that serializes the protos to json.

## Install with Helm Chart
Please follow these [instructions](https://github.com/teslamotors/helm-charts/blob/main/charts/fleet-telemetry/README.md)

## Metrics
Prometheus or a statsd interface supporting data store for metrics, this is required you should always monitor your applications.

## Protos
Data is encapsulated into protobuf messages of different types. We do not recommend making changes but if you need to recompile them you can always do so with:

  1. Install protoc, currently on version 3.21.12: https://grpc.io/docs/protoc-installation/
  2. Install protoc-gen-go: `go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28`
  3. Run make command
  ```sh
  make generate-golang
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
librdkafka is missing, on MAC OS you can install it via `brew install librdkafka pkg-config` or follow instructions here https://github.com/confluentinc/confluent-kafka-go#getting-started

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

## Integration Tests
(Optional): If you want to recreate fake certs for your test: `make generate-certs`

To run the integration tests: `make integration`

## Building the binary for linux from mac arm64

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
  frequency that they need.
* Providers agree to take full responsibility for privacy risks, as soon as data
  leave the devices (for more info read our privacy policies).
