# Fleet Telemetry + PostgreSQL Setup Guide

This guide provides step-by-step instructions to run Fleet Telemetry with PostgreSQL as the data store.

## Quick Start with Docker Compose

The simplest way to get started is using the provided `docker-compose.postgres.yml` file.

### Prerequisites

- Docker and Docker Compose installed
- Fleet Telemetry Docker image built: `fleet-telemetry-integration-tests:latest`
- TLS certificates generated in `test/integration/test-certs/`

### Steps

1. **Build the Fleet Telemetry Docker Image**

```bash
# From the fleet-telemetry root directory
docker build -t fleet-telemetry-integration-tests:latest .
```

2. **Generate TLS Certificates** (if not already present)

```bash
go run tools/main.go
```

3. **Start Services**

```bash
# Start PostgreSQL and Fleet Telemetry
docker-compose -f docker-compose.postgres.yml up -d

# Verify services are running
docker-compose -f docker-compose.postgres.yml ps
```

4. **Access Services**

- Fleet Telemetry: `https://localhost:443`
- Status endpoint: `http://localhost:8080`
- Metrics endpoint: `http://localhost:8000/metrics` (Prometheus)
- PostgreSQL: `localhost:5432` (Database: fleet_telemetry, User: telemetry_user)

5. **Stop Services**

```bash
docker-compose -f docker-compose.postgres.yml down
```

---

## Manual PostgreSQL Setup (Without Docker)

If you prefer to run PostgreSQL and Fleet Telemetry separately:

### 1. Install PostgreSQL

**On Ubuntu/Debian:**
```bash
sudo apt update
sudo apt install -y postgresql postgresql-contrib
sudo systemctl start postgresql
```

**On macOS (using Homebrew):**
```bash
brew install postgresql
brew services start postgresql
```

### 2. Create Database and User

```bash
# Connect to PostgreSQL
sudo -u postgres psql

# Run these SQL commands
CREATE DATABASE fleet_telemetry;
CREATE USER telemetry_user WITH PASSWORD 'telemetry_password';
GRANT ALL PRIVILEGES ON DATABASE fleet_telemetry TO telemetry_user;
\c fleet_telemetry
GRANT ALL PRIVILEGES ON SCHEMA public TO telemetry_user;
\q
```

### 3. Verify Connection

```bash
psql -h localhost -U telemetry_user -d fleet_telemetry
```

### 4. Create Configuration File

Create `config.json`:

```json
{
  "host": "0.0.0.0",
  "port": 443,
  "status_port": 8080,
  "log_level": "info",
  "json_log_enable": true,
  "namespace": "tesla",
  "tls": {
    "server_cert": "/path/to/server.crt",
    "server_key": "/path/to/server.key"
  },
  "postgres": {
    "host": "localhost",
    "port": 5432,
    "user": "telemetry_user",
    "password": "telemetry_password",
    "database": "fleet_telemetry",
    "ssl_mode": "disable",
    "max_open_conns": 25,
    "max_idle_conns": 5,
    "conn_max_lifetime": 300
  },
  "records": {
    "V": ["postgres"],
    "alerts": ["postgres"],
    "errors": ["postgres"],
    "connectivity": ["postgres"]
  },
  "monitoring": {
    "prometheus_metrics_port": 8000
  }
}
```

### 5. Build and Run Fleet Telemetry

```bash
# Build the binary
make build

# Run the service
./bin/fleet-telemetry -config=config.json
```

---

## Verifying the Setup

### 1. Check PostgreSQL Connectivity

```bash
psql -h localhost -U telemetry_user -d fleet_telemetry -c "\dt"
```

Expected output:
```
                  List of relations
 Schema |       Name       | Type  |     Owner
--------+------------------+-------+---------------
 public | tesla_V          | table | telemetry_user
 public | tesla_alerts     | table | telemetry_user
 public | tesla_errors     | table | telemetry_user
 public | tesla_connectivity | table | telemetry_user
```

### 2. Check Fleet Telemetry Status

```bash
curl -k https://localhost/status
# or
curl http://localhost:8080/status
```

### 3. View Metrics

```bash
curl http://localhost:8000/metrics | grep records_produced_total
```

### 4. Query Sample Data

```bash
psql -h localhost -U telemetry_user -d fleet_telemetry << EOF
SELECT COUNT(*) as total_records FROM tesla_V;
SELECT COUNT(*) as total_alerts FROM tesla_alerts;
SELECT COUNT(*) as total_errors FROM tesla_errors;
SELECT COUNT(*) as total_connectivity FROM tesla_connectivity;
EOF
```

---

## Database Administration

### Common Operations

**Check table sizes:**
```bash
psql -h localhost -U telemetry_user -d fleet_telemetry << EOF
SELECT 
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
EOF
```

**View recent records for a VIN:**
```bash
psql -h localhost -U telemetry_user -d fleet_telemetry << EOF
SELECT vin, tx_type, received_at 
FROM tesla_V 
WHERE vin = 'YOUR_VIN_HERE' 
ORDER BY received_at DESC 
LIMIT 10;
EOF
```

**Archive old data:**
```bash
psql -h localhost -U telemetry_user -d fleet_telemetry << EOF
DELETE FROM tesla_V 
WHERE received_at < NOW() - INTERVAL '6 months';
VACUUM ANALYZE tesla_V;
EOF
```

---

## Troubleshooting

### PostgreSQL Connection Issues

**Error: "connection refused"**
```bash
# Check if PostgreSQL is running
sudo systemctl status postgresql

# Start PostgreSQL
sudo systemctl start postgresql
```

**Error: "password authentication failed"**
```bash
# Verify credentials in config.json match database user
psql -h localhost -U telemetry_user -d fleet_telemetry
```

### Fleet Telemetry Issues

**Error: "failed to create tables"**
- Check permissions: User must have CREATE TABLE privileges
- Verify database exists and user can access it

**Error: "failed to insert record"**
- Check PostgreSQL disk space: `df -h`
- Verify no table constraints violations
- Check PostgreSQL logs: `sudo -u postgres psql -d fleet_telemetry -c "SELECT * FROM pg_stat_statements LIMIT 10;"`

### Docker Issues

**Error: "postgres exited with code 127"**
```bash
# Pull the postgres image
docker pull postgres:15-alpine

# Recreate containers
docker-compose -f docker-compose.postgres.yml down
docker-compose -f docker-compose.postgres.yml up -d
```

**Error: "app cannot connect to postgres"**
```bash
# Check container network
docker network inspect fleet-telemetry-network

# Verify postgres health
docker-compose -f docker-compose.postgres.yml ps
```

---

## Performance Tuning

### 1. Connection Pool Settings

Adjust in `config.json`:
```json
{
  "postgres": {
    "max_open_conns": 50,      // Increase for high throughput
    "max_idle_conns": 10,      // More idle connections = lower latency
    "conn_max_lifetime": 300   // Career span of each connection
  }
}
```

### 2. PostgreSQL Configuration

Edit `/etc/postgresql/15/main/postgresql.conf`:
```ini
# Increase shared buffers for better caching
shared_buffers = 256MB

# Increase effective cache size
effective_cache_size = 1GB

# Enable query parallelization
max_parallel_workers_per_gather = 2
max_parallel_workers = 4

# Increase WAL buffer for better write performance
wal_buffers = 16MB
```

Restart PostgreSQL:
```bash
sudo systemctl restart postgresql
```

### 3. Add Indexes for Common Queries

```bash
psql -h localhost -U telemetry_user -d fleet_telemetry << EOF
-- Add GIN index for JSONB queries
CREATE INDEX idx_tesla_V_metadata_gin ON tesla_V USING GIN (metadata);

-- Add index for common time range queries
CREATE INDEX idx_tesla_V_received_at_vin ON tesla_V (received_at, vin);

-- Analyze query plans
ANALYZE;
EOF
```

---

## Backup and Recovery

### Backup Strategy

**Full database backup:**
```bash
pg_dump -h localhost -U telemetry_user fleet_telemetry > backup_$(date +%Y%m%d).sql
```

**Compressed backup:**
```bash
pg_dump -h localhost -U telemetry_user -F c fleet_telemetry > backup_$(date +%Y%m%d).dump
```

**Scheduled backups (cron):**
```bash
# Add to crontab -e
0 2 * * * pg_dump -h localhost -U telemetry_user fleet_telemetry > /backups/fleet_telemetry_$(date +\%Y\%m\%d).sql
```

### Restore from Backup

**From SQL backup:**
```bash
psql -h localhost -U telemetry_user fleet_telemetry < backup_20240101.sql
```

**From compressed backup:**
```bash
pg_restore -h localhost -U telemetry_user -d fleet_telemetry backup_20240101.dump
```

---

## Production Deployment Checklist

- [ ] Password changed from default `telemetry_password`
- [ ] SSL mode enabled (`ssl_mode: require`)
- [ ] Connection limits appropriate for workload
- [ ] Backup strategy in place
- [ ] Monitoring enabled (Prometheus metrics)
- [ ] Log collection configured
- [ ] Database disk space monitored
- [ ] Firewall configured to block unauthorized access
- [ ] Data retention policy established
- [ ] Performance baselines established

---

## Additional Resources

- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [pq PostgreSQL Driver](https://github.com/lib/pq)
- [Fleet Telemetry GitHub](https://github.com/teslamotors/fleet-telemetry)
- [PostgreSQL JSONB Guide](https://www.postgresql.org/docs/current/datatype-json.html)

---

## Support

For issues with:
- **PostgreSQL**: See [PostgreSQL Troubleshooting](https://www.postgresql.org/docs/current/admin.html)
- **Fleet Telemetry**: See [Fleet Telemetry Issues](https://github.com/teslamotors/fleet-telemetry/issues)
- **Docker**: See [Docker Documentation](https://docs.docker.com/)

