![Build Status](https://img.shields.io/github/actions/workflow/status/teslamotors/fleet-telemetry/build.yml?branch=main)
![Current Version](https://img.shields.io/github/v/tag/teslamotors/fleet-telemetry?label=latest%20tag)
![DockerHub Tags](https://img.shields.io/docker/v/tesla/fleet-telemetry?label=docker%20tags)

# Tesla Fleet Telemetry
---------------------------------

Fleet Telemetry is a simple, scalable, and secure data exchange for device fleets.

Clients establish a websocket connection to push configurable telemetry records. Telemetry provides clients with ack, error, or rate limit responses.


## Configuring and running the service
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
  "rate_limit": {
      "enabled": bool,
      "message_limit": int - ex.: 1000
  },
  "records": { list of records and their dispatchers
      "A": [
          "logger"
      ],
      "E": [
          "logger"
      ],
      "V": [
          "logger"
      ]
  },
  "tls": {
      "server_cert": string - server cert location,
      "server_key": string - server key location
  }
}
```
Example: [server_config.json](./examples/server_config.json)

5. Deploy and run the server.  This can be run as a binary via `./fleet-telemetry -config=/etc/fleet-telemetry/config.json` directly on a server, or as a kubernetes deployment.  Example snippet:
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
        image: fleet-telemetry:abc123
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


### With pubsub
- Along with the required pubsub config (See ./test/integration/config.json for example), be sure to set the environment variable `GOOGLE_APPLICATION_CREDENTIALS`


## Dependencies

Kafka: Telemetry publishes to Kafka as a backend store
Prometheus or a statsd interface supporting data store for metrics

# Protos

These represent different message types.  

To generate:
1. install protoc, currently on version 3.21.12: https://grpc.io/docs/protoc-installation/
2. install protoc-gen-go: `go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28`
3. Run make command
```sh
make generate-golang
```

## Run locally
```sh
export PUBSUB_EMULATOR_HOST=0.0.0.0:8085
go run ./cmd/main.go -config=./config-local.json
```

## Testing

### Unit Tests
To run the unit tests:-

 `make test`

Common Errors:

    ~/fleet-telemetry➜ git:(main) ✗  make test
    go build github.com/confluentinc/confluent-kafka-go/v2/kafka:
    # pkg-config --cflags  -- rdkafka
    Package rdkafka was not found in the pkg-config search path.
    Perhaps you should add the directory containing `rdkafka.pc'
    to the PKG_CONFIG_PATH environment variable
    No package 'rdkafka' found
    pkg-config: exit status 1
    make: *** [install] Error 1

librdkafka is missing, on MAC OS you can install it via `brew install librdkafka pkg-config` or follow instructions here https://github.com/confluentinc/confluent-kafka-go#getting-started

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

Reference to libcrypto is not set properly. To resolve find the reference to libcrypto by pkgconfig and set et the PKG_CONFIG_PATH accordingly.

### Integration Tests
(Optional): If you want to recreate fake certs for your test, run:-

`make generate-certs`

To run the integration tests:-

`make integration`

## Building binary for linux from mac arm64

```sh
DOCKER_BUILD_KIT=1 DOCKER_CLI_EXPERIMENTAL=enabled docker buildx version
docker buildx create --name go-builder --driver docker-container --driver-opt network=host --buildkitd-flags '--allow-insecure-entitlement network.host' --use
docker buildx inspect --bootstrap
docker buildx build --no-cache --progress=plain --platform linux/amd64 -t <name:tag>(e.x.: fleet-telemetry:local.1.1) -f Dockerfile . --load
container_id=$(docker create fleet-telemetry:local.1.1) docker cp $container_id:/fleet-telemetry /tmp/fleet-telemetry
```

## Security and privacy considerations

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
