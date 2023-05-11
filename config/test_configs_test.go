package config

const TestConfig = `{
	"host": "127.0.0.1",
	"port": 6942,
	"log_level": "info",
	"json_log_enable": true,
	"namespace": "tesla_telemetry",
	"reliable_ack": true,
	"reliable_ack_workers": 15,
	"kafka": {
		"bootstrap.servers": "some.broker1:9093,some.broker1:9093",
		"ssl.ca.location": "kafka.ca",
		"ssl.certificate.location": "kafka.crt",
		"ssl.key.location": "kafka.key",
		"queue.buffering.max.messages": 1000000
	},
	"monitoring": {
		"prometheus_metrics_port": 9090,
		"profiler_port": 4269,
		"profiling_path": "/tmp/tesla-telemetry/profile/"
	},
	"rate_limit": {
		"enabled": true,
		"message_interval_time": 30,
		"message_limit": 1000
	},
	"records": {
		"FS": ["kafka"]
	},
	"tls": {
		"ca_file": "tesla.ca",
		"server_cert": "your_own_cert.crt",
		"server_key": "your_own_key.key"
	}
}
`

const TestSmallConfig = `
{
	"host": "127.0.0.1",
	"port": 6942,
	"namespace": "tesla_telemetry",
	"kafka": {
		"bootstrap.servers": "some.broker1:9093,some.broker1:9093",
		"ssl.ca.location": "kafka.ca",
		"ssl.certificate.location": "kafka.crt",
		"ssl.key.location": "kafka.key",
		"queue.buffering.max.messages": 1000000
	},
	"records": {
		"FS": ["kafka"]
	},
	"tls": {
		"ca_file": "tesla.ca",
		"server_cert": "your_own_cert.crt",
		"server_key": "your_own_key.key"
	}
}
`

const TestPubsubConfig = `
{
	"host": "127.0.0.1",
	"port": 6942,
	"pubsub": {
        "gcp_project_id": "some-project-id",
		"reliable_ack": "true"
    },
	"records": {
		"FS": ["pubsub"]
	}
}
`

const BadTopicConfig = `
{
	"host": "127.0.0.1",
	"port": "",
}`
