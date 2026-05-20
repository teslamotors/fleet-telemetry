# Implementation Summary: MySQL & PostgreSQL Connectors for Fleet Telemetry

## ✅ Completed Implementation

### Database Connectors Added

1. **MySQL Connector** ✓
   - Full CRUD operations for telemetry data
   - Auto-creates normalized table schema
   - Connection pooling with configurable limits
   - Metrics integration (Prometheus)
   - Error handling and logging

2. **PostgreSQL Connector** ✓
   - Full CRUD operations for telemetry data
   - Auto-creates optimized table schema with indexes
   - Connection pooling with configurable limits
   - Advanced JSONB support for metadata
   - Metrics integration (Prometheus)
   - Error handling and logging

---

## Files Created

### 📁 Connector Implementation Files

```
datastore/
├── mysql/
│   ├── mysql.go                 # MySQL producer implementation (309 lines)
│   └── README.md                # Comprehensive MySQL documentation
│
└── postgres/
    ├── postgres.go              # PostgreSQL producer implementation (334 lines)
    └── README.md                # Comprehensive PostgreSQL documentation
```

### 📋 Configuration Files

```
examples/
└── mysql_config.json            # Example MySQL configuration

test/integration/
└── postgres_config.json         # Example PostgreSQL configuration for Docker

docker-compose.postgres.yml      # Minimal PostgreSQL + Fleet Telemetry setup
```

### 📖 Documentation Files

```
POSTGRES_SETUP.md                # Complete PostgreSQL setup guide (400+ lines)
DATABASE_SETUP.md                # Quick reference for both databases (300+ lines)
POSTGRES_VS_MYSQL.md             # Detailed comparison and decision guide (300+ lines)
```

### 🔧 Core Framework Updates

```
telemetry/producer.go            # Added MySQL & Postgres dispatcher constants
config/config.go                 # Added MySQL & Postgres configuration support
go.mod                           # Added MySQL & PostgreSQL driver dependencies
```

---

## Key Features

### MySQL Connector Features
- ✅ Automatic table creation for all record types (V, alerts, errors, connectivity)
- ✅ InnoDB with UTF8MB4 encoding for international character support
- ✅ Efficient indexing on VIN and timestamp columns
- ✅ Connection pooling (configurable)
- ✅ JSON column for flexible metadata storage
- ✅ Reliable acknowledgment support
- ✅ Prometheus metrics integration
- ✅ Comprehensive error handling and logging

### PostgreSQL Connector Features
- ✅ Automatic table creation with optimized schema
- ✅ JSONB columns for powerful metadata querying
- ✅ GIN-indexed JSONB for fast JSON queries
- ✅ Multiple indexes (VIN, timestamp, received_at)
- ✅ Connection pooling (configurable)
- ✅ BIGSERIAL auto-increment
- ✅ Reliable acknowledgment support
- ✅ Prometheus metrics integration
- ✅ Advanced PostgreSQL features support

---

## Configuration Examples

### Minimal MySQL Setup

```json
{
  "mysql": {
    "host": "localhost",
    "port": 3306,
    "user": "telemetry_user",
    "password": "password",
    "database": "fleet_telemetry"
  },
  "records": {
    "V": ["mysql"],
    "alerts": ["mysql"],
    "errors": ["mysql"],
    "connectivity": ["mysql"]
  }
}
```

### Minimal PostgreSQL Setup

```json
{
  "postgres": {
    "host": "postgres",
    "port": 5432,
    "user": "telemetry_user",
    "password": "telemetry_password",
    "database": "fleet_telemetry"
  },
  "records": {
    "V": ["postgres"],
    "alerts": ["postgres"],
    "errors": ["postgres"],
    "connectivity": ["postgres"]
  }
}
```

### Combined PostgreSQL + Kafka

```json
{
  "kafka": {
    "bootstrap.servers": "kafka:9092"
  },
  "postgres": {
    "host": "postgres",
    "user": "telemetry_user",
    "password": "secure_password",
    "database": "fleet_telemetry"
  },
  "records": {
    "V": ["postgres", "kafka"],
    "alerts": ["postgres", "kafka"],
    "errors": ["postgres"]
  }
}
```

---

## Quick Start

### PostgreSQL (Recommended)

```bash
# Option 1: Docker Compose (simplest)
docker-compose -f docker-compose.postgres.yml up -d

# Option 2: Manual setup
cd /home/user/share/fleet-telemetry
./bin/fleet-telemetry -config=test/integration/postgres_config.json
```

### MySQL

```bash
# Create database (one time)
mysql -u root -p << EOF
CREATE DATABASE fleet_telemetry;
CREATE USER 'telemetry_user'@'%' IDENTIFIED BY 'password';
GRANT ALL PRIVILEGES ON fleet_telemetry.* TO 'telemetry_user'@'%';
FLUSH PRIVILEGES;
EOF

# Run Fleet Telemetry
./bin/fleet-telemetry -config=examples/mysql_config.json
```

---

## Database Schema

Both connectors create the following table structure:

### Table: `{namespace}_V`
```
Column         | Type          | Index
---------------|---------------|-------
id             | BIGINT PRIMARY| ✓
vin            | VARCHAR(17)   | ✓
tx_type        | VARCHAR(50)   | 
timestamp      | TIMESTAMP     | 
payload        | BYTEA/BLOB    | 
metadata       | JSONB/JSON    | 
received_at    | TIMESTAMP     | ✓
```

### Similar tables created for:
- `{namespace}_alerts`
- `{namespace}_errors`
- `{namespace}_connectivity`

Default namespace: `tesla`

---

## Configuration Parameters

### MySQL Specific

| Parameter | Default | Description |
|-----------|---------|-------------|
| `host` | - | MySQL server hostname |
| `port` | 3306 | MySQL server port |
| `user` | - | Database username |
| `password` | - | Database password |
| `database` | - | Database name (required) |
| `max_open_conns` | 25 | Max concurrent connections |
| `max_idle_conns` | 5 | Max idle connections |
| `conn_max_lifetime` | 300 | Connection lifetime (seconds) |

### PostgreSQL Specific

| Parameter | Default | Description |
|-----------|---------|-------------|
| `host` | - | PostgreSQL server hostname |
| `port` | 5432 | PostgreSQL server port |
| `user` | - | Database username |
| `password` | - | Database password |
| `database` | - | Database name (required) |
| `ssl_mode` | disable | SSL mode (disable\|allow\|prefer\|require\|verify-ca\|verify-full) |
| `max_open_conns` | 25 | Max concurrent connections |
| `max_idle_conns` | 5 | Max idle connections |
| `conn_max_lifetime` | 300 | Connection lifetime (seconds) |

---

## Dependencies Added

```go
// MySQL driver
github.com/go-sql-driver/mysql v1.7.1

// PostgreSQL driver
github.com/lib/pq v1.10.9
```

Both are pure Go drivers with no C dependencies.

---

## Metrics Reported

Both connectors report these metrics to Prometheus:

```
records_produced_total{record_type="V"}
record_bytes_produced_total{record_type="V"}
{mysql|postgres}_errors_total
records_reliable_ack_total{record_type="V"}
```

Access metrics at: `http://localhost:8000/metrics`

---

## Docker Compose Setup

The included `docker-compose.postgres.yml` provides:

```yaml
services:
  postgres:                        # PostgreSQL 15 Alpine
    - Auto-initialized database
    - Health checks
    - Persistent volumes
    
  app:                             # Fleet Telemetry
    - Configured to use PostgreSQL
    - TLS/HTTPS enabled (port 443)
    - Status endpoint (port 8080)
    - Metrics endpoint (port 8000)
    - Health checks
```

**Start:** `docker-compose -f docker-compose.postgres.yml up -d`  
**Stop:** `docker-compose -f docker-compose.postgres.yml down`  
**Logs:** `docker-compose -f docker-compose.postgres.yml logs -f`

---

## Testing Data Storage

### PostgreSQL

```bash
# Connect to database
docker exec fleet-telemetry-postgres psql -U telemetry_user -d fleet_telemetry

# Check tables
\dt

# Count records
SELECT COUNT(*) FROM tesla_V;

# View recent data
SELECT * FROM tesla_V ORDER BY received_at DESC LIMIT 5;
```

### MySQL

```bash
# Connect to database
mysql -h localhost -u telemetry_user -p fleet_telemetry

# Check tables
SHOW TABLES;

# Count records
SELECT COUNT(*) FROM tesla_V;

# View recent data
SELECT * FROM tesla_V ORDER BY received_at DESC LIMIT 5;
```

---

## Advanced Features

### PostgreSQL JSONB Querying

```sql
-- Query nested JSON
SELECT * FROM tesla_V 
WHERE metadata @> '{"alert_level": "critical"}';

-- Full-text search
SELECT * FROM tesla_alerts 
WHERE metadata::text ILIKE '%warning%';

-- Extract specific fields
SELECT vin, metadata->>'severity' as severity
FROM tesla_alerts;
```

### Partitioning (PostgreSQL)

```sql
-- Partition by month
ALTER TABLE tesla_V 
PARTITION BY RANGE (DATE_TRUNC('month', received_at));

-- Create partitions
CREATE TABLE tesla_V_2024_01 PARTITION OF tesla_V
  FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');
```

### Performance Optimization

```bash
# PostgreSQL: Analyze query plans
EXPLAIN ANALYZE
SELECT * FROM tesla_V 
WHERE vin = 'VIN123' AND received_at > NOW() - INTERVAL '1 day';

# Add GIN index for JSONB
CREATE INDEX idx_metadata_gin ON tesla_V USING GIN (metadata);
```

---

## Comparison Summary

| Aspect | MySQL | PostgreSQL |
|--------|-------|-----------|
| **Setup** | ⭐⭐⭐ | ⭐⭐ |
| **JSONB Querying** | ⭐ | ⭐⭐⭐ |
| **Scalability** | ⭐⭐ | ⭐⭐⭐ |
| **Performance** | ⭐⭐ | ⭐⭐⭐ |
| **Features** | ⭐⭐ | ⭐⭐⭐ |
| **Learning Curve** | ⭐⭐⭐ | ⭐⭐ |

**Recommendation:** **PostgreSQL** for production, **MySQL** for simplicity.

---

## Documentation References

- **Complete PostgreSQL Guide:** `/POSTGRES_SETUP.md`
- **Quick Reference:** `/DATABASE_SETUP.md`
- **Comparison:** `/POSTGRES_VS_MYSQL.md`
- **MySQL Details:** `/datastore/mysql/README.md`
- **PostgreSQL Details:** `/datastore/postgres/README.md`
- **Docker Setup:** `/docker-compose.postgres.yml`

---

## Next Steps

1. ✅ Choose your database (PostgreSQL recommended)
2. ✅ Run Docker Compose or manual setup
3. ✅ Verify data is being stored
4. ✅ Query your data
5. ✅ Set up monitoring
6. ✅ Configure backups
7. ✅ Scale as needed

---

## Support & Troubleshooting

### Common Issues

| Issue | Solution |
|-------|----------|
| "connection refused" | Check if database is running: `docker-compose logs postgres` |
| "password auth failed" | Verify credentials in config.json match database |
| "table already exists" | Normal - app checks before creating tables |
| "disk space full" | Check database size and implement data archival |

### Health Checks

```bash
# Fleet Telemetry status
curl http://localhost:8080/status

# Database connectivity
docker-compose logs app | grep "postgres_registered"

# Metrics
curl http://localhost:8000/metrics | grep produced_total
```

---

## Performance Tuning

### For Small Deployments (< 10K records/day)
```json
"max_open_conns": 5,
"max_idle_conns": 2
```

### For Medium Deployments (10K - 100K records/day)
```json
"max_open_conns": 25,
"max_idle_conns": 5
```

### For Large Deployments (> 100K records/day)
```json
"max_open_conns": 50,
"max_idle_conns": 10
```

---

## Backup Strategies

### PostgreSQL
```bash
# Daily backup
pg_dump -h localhost -U telemetry_user fleet_telemetry > backup_$(date +%Y%m%d).sql

# Continuous archiving (for PITR)
archive_command = 'cp %p /archive/%f'
```

### MySQL
```bash
# Daily backup
mysqldump -h localhost -u telemetry_user -p fleet_telemetry > backup_$(date +%Y%m%d).sql

# Enable binlog for point-in-time recovery
log_bin = mysql-bin
```

---

## Version Information

- **Fleet Telemetry:** Current main branch
- **Go:** 1.23.0+
- **MySQL Driver:** v1.7.1 (pure Go)
- **PostgreSQL Driver:** v1.10.9 (pure Go)
- **Compatible Databases:**
  - PostgreSQL 12+
  - MySQL 5.7+ / MariaDB 10.2+

---

## Credits & References

- Fleet Telemetry: https://github.com/teslamotors/fleet-telemetry
- PostgreSQL: https://www.postgresql.org/docs/
- MySQL: https://dev.mysql.com/doc/
- pq Driver: https://github.com/lib/pq
- MySQL Driver: https://github.com/go-sql-driver/mysql

---

## Summary

✅ **Complete implementation of MySQL and PostgreSQL connectors for Fleet Telemetry**

Both databases are fully integrated with:
- Automatic schema creation
- Connection pooling
- Metrics integration
- Error handling
- Reliable acknowledgments
- Comprehensive documentation

**To get started:** See `/DATABASE_SETUP.md` or `/POSTGRES_SETUP.md`

**To run immediately:** `docker-compose -f docker-compose.postgres.yml up -d`

