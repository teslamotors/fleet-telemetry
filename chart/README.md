# Fleet Telemetry Helm Chart 
* Installs the fleet-telemetry system. [fleet-telemetry](https://github.com/teslamotors/fleet-telemetry)
## Get Repo Info
```console
helm repo add teslamotors https://teslamotors.github.io/helm-charts/
helm repo update
```
_See [helm repo](https://helm.sh/docs/helm/helm_repo/) for command documentation._

## Installing the Chart

To install the chart with the release name `fleet-telemetry`:

```console
helm install fleet-telemetry teslamotors/fleet-telemetry -n fleet-telemetry --create-namespace
```

## Uninstalling the Chart

To uninstall/delete the fleet-telemetry deployment:

```console
helm uninstall fleet-telemetry -n fleet-telemetry
```
The command removes all the Kubernetes components associated with the chart and deletes the release.

## Upgrade the Chart
To upgrade the chart with the release name `fleet-telemetry`:
```console
helm upgrade fleet-telemetry teslamotors/fleet-telemetry -n fleet-telemetry
```

## Configuration
| Parameter             | Description                                                                         | Default                 |
|-----------------------|-------------------------------------------------------------------------------------|-------------------------|
| `tlsSecret.name`      | Name of existing secret, if this value is set `tlsCrt` and `tlsKey` will be ignored | `nil`                   |
| `tlsSecret.tlsCrt`    | value of the certification                                                          | `nil`                   |
| `tlsSecret.tlsKey`    | value of the encryption key                                                         | `nil`                   |
| `image.repository`    | value of the docker image repo                                                      | `tesla/fleet-telemetry` |
| `image.tag`           | value of the docker image tag                                                       | `latest`                |
| `resources`           | CPU/Memory resource requests/limits                                                 | {}                      |
| `nodeSelector`        | Node labels for pod assignment                                                      | {}                      |
| `tolerations`         | Toleration labels for pod assignment                                                | {}                      |
| `replicas`            | Number of pods                                                                      | `1`                     |
| `service.annotations` | Service Annotations                                                                 | {}                      |
| `service.type`        | Service Type                                                                        | ClusterIP               |

## Example
* Set `config.data` in `values.yaml`
```yaml
service:
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-scheme: "internet-facing"
    service.beta.kubernetes.io/aws-load-balancer-type: "nlb-ip"
    service.beta.kubernetes.io/aws-load-balancer-nlb-target-type: "ip"
    service.beta.kubernetes.io/aws-load-balancer-subnets: subnet-1, subnet-2, subnet-3
  type: LoadBalancer
tlsSecret:
  tlsCrt: |
    value of the cert PEM
  tlsKey: |
    value the private key
config:
  data: |
    {
      "host": "0.0.0.0",
      "port": 8443,
      "status_port": 8080,
      "log_level": "info",
      "json_log_enable": true,
      "namespace": "tesla_telemetry",
      "monitoring": {
        "prometheus_metrics_port": 9273,
        "profiler_port": 4269,
        "profiling_path": "/tmp/trace.out"
      },
      "rate_limit": {
        "enabled": true,
        "message_interval_time": 30,
        "message_limit": 1000
      },
      "records": {
        "alerts": [
          "logger"
        ],
        "errors": [
          "logger"
        ],
        "V": [
          "logger"
        ]
      },
      "tls": {
        "server_cert": "/etc/certs/server/tls.crt",
        "server_key": "/etc/certs/server/tls.key"
      }
    }
```
```console
helm install fleet-telemetry teslamotors/fleet-telemetry -n fleet-telemetry -f values.yaml
```
* Set `config.data` by `--set-file`
```yaml
service:
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-scheme: "internet-facing"
    service.beta.kubernetes.io/aws-load-balancer-type: "nlb-ip"
    service.beta.kubernetes.io/aws-load-balancer-nlb-target-type: "ip"
    service.beta.kubernetes.io/aws-load-balancer-subnets: subnet-1, subnet-2, subnet-3
  type: LoadBalancer
tlsSecret:
  tlsCrt: |
    value of the cert PEM
  tlsKey: |
    value the private key
```
```json
{
  "host": "0.0.0.0",
  "port": 8443,
  "status_port": 8080,
  "log_level": "info",
  "json_log_enable": true,
  "namespace": "tesla_telemetry",
  "monitoring": {
    "prometheus_metrics_port": 9273,
    "profiler_port": 4269,
    "profiling_path": "/tmp/trace.out"
  },
  "rate_limit": {
    "enabled": true,
    "message_interval_time": 30,
    "message_limit": 1000
  },
  "records": {
    "alerts": [
      "logger"
    ],
    "errors": [
      "logger"
    ],
    "V": [
      "logger"
    ]
  },
  "tls": {
    "server_cert": "/etc/certs/server/tls.crt",
    "server_key": "/etc/certs/server/tls.key"
  }
}
```
```console
helm install fleet-telemetry teslamotors/fleet-telemetry -n fleet-telemetry -f values.yaml --set-file config.data=config.json
```
