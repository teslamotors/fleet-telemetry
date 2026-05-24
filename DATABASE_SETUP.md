# Fleet Telemetry Database Connectors - Quick Reference

## Overview

Fleet Telemetry now supports **MySQL** and **PostgreSQL** as database dispatchers for storing vehicle telemetry data. Both connectors are fully integrated and ready to use.

---

## PostgreSQL (Recommended for Production)

### Why PostgreSQL?
✅ Superior JSONB support for flexible querying  
✅ Better performance for complex queries  
✅ Native partitioning and archiving  
✅ More mature replication features  
✅ Better for large-scale deployments  

### Minimal Environment Setup

```bash
# Using Docker Compose (Recommended)
docker-compose -f docker-compose.postgres.yml up -d
```

**What it includes:**
- PostgreSQL 15 Alpine (lightweight)
- Fleet Telemetry app
- Auto-networking and health checks
- Persistent data volumes
- Status and metrics endpoints

### Configuration Example

```json
{
  "host": "0.0.0.0",
  "port": 443,
  "postgres": {
    "host": "postgres",
    "port": 5432,
    "user": "telemetry_user",
    "password": "telemetry_password",
    "database": "fleet_telemetry",
    "ssl_mode": "disable"
  },
  "namespace": "tesla",
  "records": {
    "V": ["postgres"],
    "alerts": ["postgres"],
    "errors": ["postgres"],
    "connectivity": ["postgres"]
  }
}
```

### Quick Commands

```bash
# Check data in PostgreSQL
psql -h localhost -U telemetry_user -d fleet_telemetry
SELECT COUNT(*) FROM tesla_V;

# View metrics
curl http://localhost:8000/metrics | grep records_produced

# View app logs
docker-compose -f docker-compose.postgres.yml logs app

# Stop everything
docker-compose -f docker-compose.postgres.yml down
```

---

## MySQL

### Why MySQL?
✅ Simple setup and deployment  
✅ Wide hosting provider support  
✅ Lower resource footprint  
✅ Suitable for smaller deployments  

### Configuration Example

```json
{
  "host": "0.0.0.0",
  "port": 443,
  "mysql": {
    "host": "localhost",
    "port": 3306,
    "user": "telemetry_user",
    "password": "your_secure_password",
    "database": "fleet_telemetry",
    "max_open_conns": 25,
    "max_idle_conns": 5,
    "conn_max_lifetime": 300
  },
  "namespace": "tesla",
  "records": {
    "V": ["mysql"],
    "alerts": ["mysql"],
    "errors": ["mysql"],
    "connectivity": ["mysql"]
  }
}
```

### Quick Setup

```bash
# Create database and user
mysql -u root -p << EOF
CREATE DATABASE fleet_telemetry;
CREATE USER 'telemetry_user'@'%' IDENTIFIED BY 'password';
GRANT ALL PRIVILEGES ON fleet_telemetry.* TO 'telemetry_user'@'%';
FLUSH PRIVILEGES;
EOF

# Run Fleet Telemetry
./fleet-telemetry -config=config.json
```

---

## Combined Setup (PostgreSQL + Kafka)

Send data to both PostgreSQL and Kafka for distributed processing:

```json
{
  "kafka": {
    "bootstrap.servers": "kafka:9092"
  },
  "postgres": {
    "host": "postgres",
    "port": 5432,
    "user": "telemetry_user",
    "password": "telemetry_password",
    "database": "fleet_telemetry"
  },
  "records": {
    "V": ["postgres", "kafka"],
    "alerts": ["postgres", "kafka"],
    "errors": ["postgres"],
    "connectivity": ["postgres"]
  }
}
```

---

## File Structure

```
datastore/
├── mysql/
│   ├── mysql.go           # MySQL implementation
│   └── README.md          # Detailed MySQL docs
├── postgres/
│   ├── postgres.go        # PostgreSQL implementation
│   └── README.md          # Detailed PostgreSQL docs
├── kafka/
├── kinesis/
└── ... (other dispatchers)

examples/
├── mysql_config.json      # MySQL config example
└── postgres_config.json   # PostgreSQL config example (in test/integration/)

docker-compose.postgres.yml   # Minimal PostgreSQL + App setup
POSTGRES_SETUP.md            # Complete PostgreSQL setup guide
```

---

## Data Structure

Both connectors automatically create tables with the same schema:

```
Table: {namespace}_{type}
├── id (BIGINT PRIMARY KEY)
├── vin (VARCHAR(17))
├── tx_type (VARCHAR(50))
├── timestamp (TIMESTAMP)
├── payload (BYTEA/LONGBLOB)
├── metadata (JSONB/JSON)
└── received_at (TIMESTAMP)

Indexes:
├── idx_{namespace}_{type}_vin
├── idx_{namespace}_{type}_timestamp
└── idx_{namespace}_{type}_received_at
```

### Record Types

| Table | Purpose |
|-------|---------|
| `{ns}_V` | Vehicle telemetry data |
| `{ns}_alerts` | Vehicle alert messages |
| `{ns}_errors` | Vehicle error messages |
| `{ns}_connectivity` | Connectivity events |

---

## Common Queries

### PostgreSQL

```sql
-- Recent records for a vehicle
SELECT * FROM tesla_V 
WHERE vin = 'VIN123' 
ORDER BY received_at DESC LIMIT 100;

-- Query JSONB metadata
SELECT vin, metadata->>'field_name' as value
FROM tesla_V 
WHERE metadata @> '{"key": "value"}';

-- Records per hour
SELECT DATE_TRUNC('hour', received_at) as hour, COUNT(*)
FROM tesla_V GROUP BY hour ORDER BY hour DESC;

-- Database size
SELECT pg_size_pretty(pg_database_size('fleet_telemetry'));
```

### MySQL

```sql
-- Recent records for a vehicle
SELECT * FROM tesla_V 
WHERE vin = 'VIN123' 
ORDER BY received_at DESC LIMIT 100;

-- Query JSON metadata
SELECT vin, JSON_EXTRACT(metadata, '$.field_name') as value
FROM tesla_V 
WHERE JSON_CONTAINS(metadata, JSON_OBJECT("key", "value"));

-- Records per hour
SELECT DATE_FORMAT(received_at, '%Y-%m-%d %H:00:00') as hour, COUNT(*)
FROM tesla_V GROUP BY hour ORDER BY hour DESC;

-- Database size
SELECT SUM(ROUND(((data_length + index_length) / 1024 / 1024), 2)) as 'Size in MB'
FROM information_schema.TABLES
WHERE table_schema = 'fleet_telemetry';
```

---

## Monitoring & Metrics

Both connectors report standard metrics:

```
records_produced_total{record_type="V"}        # Records written
record_bytes_produced_total{record_type="V"}   # Bytes written
postgres_errors_total                          # Connection/query errors
records_reliable_ack_total{record_type="V"}    # Acked records
```

**View metrics:**
```bash
curl http://localhost:8000/metrics | grep produced
```

---

## Docker Compose Quick Reference

### Start PostgreSQL + App
```bash
docker-compose -f docker-compose.postgres.yml up -d
```

### View Logs
```bash
docker-compose -f docker-compose.postgres.yml logs -f app
docker-compose -f docker-compose.postgres.yml logs -f postgres
```

### Execute Database Commands
```bash
docker exec fleet-telemetry-postgres psql -U telemetry_user -d fleet_telemetry -c "SELECT COUNT(*) FROM tesla_V;"
```

### Stop Everything
```bash
docker-compose -f docker-compose.postgres.yml down
```

### Keep Data, Stop Containers
```bash
docker-compose -f docker-compose.postgres.yml stop
```

### Clean Up Everything (Including Data!)
```bash
docker-compose -f docker-compose.postgres.yml down -v
```

---

## Configuration Parameters

### PostgreSQL Config

```json
"postgres": {
  "host": "localhost",              // Server hostname
  "port": 5432,                     // Default: 5432
  "user": "telemetry_user",         // Database user
  "password": "secure_password",    // User password
  "database": "fleet_telemetry",    // Database name
  "ssl_mode": "disable",            // disable|allow|prefer|require|verify-ca|verify-full
  "max_open_conns": 25,             // Default: 25
  "max_idle_conns": 5,              // Default: 5
  "conn_max_lifetime": 300          // Seconds, default: 300
}
```

### MySQL Config

```json
"mysql": {
  "host": "localhost",              // Server hostname
  "port": 3306,                     // Default: 3306
  "user": "telemetry_user",         // Database user
  "password": "secure_password",    // User password
  "database": "fleet_telemetry",    // Database name
  "max_open_conns": 25,             // Default: 25
  "max_idle_conns": 5,              // Default: 5
  "conn_max_lifetime": 300          // Seconds, default: 300
}
```

---

## Troubleshooting

### PostgreSQL Connection Error
```bash
# Check if PostgreSQL is running
docker-compose -f docker-compose.postgres.yml logs postgres

# Verify credentials
psql -h localhost -U telemetry_user -d fleet_telemetry
```

### MySQL Connection Error
```bash
# Check credentials
mysql -h localhost -u telemetry_user -p fleet_telemetry
```

### Tables Not Being Created
```bash
# Check app logs for errors
docker-compose -f docker-compose.postgres.yml logs app | grep "create table"

# Verify database permissions
docker exec fleet-telemetry-postgres psql -U telemetry_user -d fleet_telemetry -c "\dt"
```

### High Memory Usage
- Reduce `max_open_conns`
- Reduce `max_idle_conns`
- Check for slow queries in database logs

---

## Performance Guidelines

| Setting | Small | Medium | Large |
|---------|-------|--------|-------|
| max_open_conns | 10 | 25 | 50+ |
| max_idle_conns | 2 | 5 | 10+ |
| namespace | default | custom | custom |
| ssl_mode | disable | prefer | require |

---

## Additional Documentation

- **PostgreSQL Details**: See `/datastore/postgres/README.md`
- **MySQL Details**: See `/datastore/mysql/README.md`
- **PostgreSQL Setup**: See `/POSTGRES_SETUP.md`
- **Main README**: See `/README.md`

---

## Next Steps

1. Choose your database: PostgreSQL (recommended) or MySQL
2. Run quick setup (Docker Compose or manual)
3. Configure `config.json` with your database details
4. Start Fleet Telemetry
5. Verify data is being stored
6. Query your data using provided examples
7. Set up monitoring and backups

---

## Support Resources

- **GitHub Issues**: https://github.com/teslamotors/fleet-telemetry/issues
- **PostgreSQL Docs**: https://www.postgresql.org/docs/
- **MySQL Docs**: https://dev.mysql.com/doc/

