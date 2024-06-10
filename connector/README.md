# Data Connectors

Data connectors can be configured to fetch data from an external source to enhance server functionality.

Currently available capabilities:
- `vin_allowed`: Check if a VIN is allowed to connect to the server.

## Available Connectors

### File

Reads from a JSON file. Watches for file changes at runtime.

- `capabilities`: `[]string` capabilities to use the data connector for.
- `vin_allowed.path`: `string` path of the file to watch.

**Example File**:
```json
{
    "vins_allowed": ["VIN1"]
}
```

**Example Config**:
```json
{
    "data_connectors": {
        "file": {
            "capabilities": ["vin_allowed"],
            "path": "path/to/file"
        }
    }
}
```

### Redis

Obtains data from a Redis cache.

- `capabilities`: `[]string` capabilities to use the data connector for.
- `vin_allowed.prefix`: `string` prefix for all Redis keys when checking if a VIN is allowed to connect.
- `vin_allowed.allow_on_failure`: `bool` whether a VIN should be allowed to connect to the server if an error is encountered while fetching from Redis.

**Example Config**:

```json
{
    "data_connectors": {
        "redis": {
            "capabilities": ["vin_allowed"],
            "vin_allowed": {
                "prefix": "vin_allowed:",
                "allow_on_failure": true
            }
        }
    }
}
```

### HTTP

Obtains data from an REST API.

- `capabilities`: `[]string` capabilities to use the data connector for.
- `host`: `string` host of the remote server.
- `timeout_seconds`: `int` seconds to wait for a response.
- `transport`: `http.Transport` [golang transport options](https://pkg.go.dev/net/http#Transport).
- `vin_allowed.cache_results`: `bool` whether results from the API should be cached in memory.
- `vin_allowed.cache_ttl_minutes`: `int` how many minutes each result should be remembered in the cache. Defaults to no expiration.
- `vin_allowed.allow_on_failure`: `bool` whether a VIN should be allowed to connect to the server if an error is encountered while fetching from Redis.

**Example Config**:

```json
{
    "data_connectors": {
        "http": {
            "capabilities": ["vin_allowed"],
            "host": "localhost",
            "timeout_seconds": 10,
            "vin_allowed": {
                "cache_results": true,
                "cache_ttl_minutes": 60,
                "allow_on_failure": false
            }
        }
    }
}
```

### gRPC

Obtains data from a gRPC service.

- `capabilities`: capabilities to use the data connector for.
- `host`: host of the gRPC server.
- `tls.cert_file`: path to cert to use when connecting.
- `tls.key_file`: path to key to use when connecting.
- `vin_allowed.cache_results`: `bool` whether results from the API should be cached in memory.
- `vin_allowed.cache_ttl_minutes`: `int` how many minutes each result should be remembered in the cache. Defaults to no expiration.
- `vin_allowed.allow_on_failure`: `bool` whether a VIN should be allowed to connect to the server if an error is encountered while fetching from Redis.

**Example Config**:

```json
{
    "data_connectors": {
        "grpc": {
            "capabilities": ["vin_allowed"],
            "host": "grpc_host",
            "tls": {
                "cert_file": "path/to/cert",
                "key_file": "path/to/key"
            },
            "vin_allowed": {
                "cache_results": true,
                "cache_ttl_minutes": 60,
                "allow_on_failure": false
            }
        }
    }
}
```
