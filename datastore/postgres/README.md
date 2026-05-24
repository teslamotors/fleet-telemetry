# PostgreSQL Connector for Fleet Telemetry

The PostgreSQL connector allows Fleet Telemetry to persist vehicle data directly to a PostgreSQL database. It automatically creates and manages tables for different record types (V, alerts, errors, connectivity).

## Configuration

Add the PostgreSQL configuration to your `config.json` file:

```json
{
  "postgres": {
    "host": "localhost",
    "port": 5432,
    "user": "telemetry_user",
    "password": "your_secure_password",
    "database": "fleet_telemetry",
    "ssl_mode": "disable",
    "max_open_conns": 25,
    "max_idle_conns": 5,
    "conn_max_lifetime": 300
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

### Configuration Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `host` | string | (required) | PostgreSQL server hostname or IP address |
| `port` | int | 5432 | PostgreSQL server port |
| `user` | string | (required) | PostgreSQL username |
| `password` | string | (required) | PostgreSQL password |
| `database` | string | (required) | Database name to use |
| `ssl_mode` | string | disable | SSL connection mode (disable, allow, prefer, require, verify-ca, verify-full) |
| `max_open_conns` | int | 25 | Maximum number of open connections to the database |
| `max_idle_conns` | int | 5 | Maximum number of idle connections in the connection pool |
| `conn_max_lifetime` | int | 300 | Maximum lifetime of a connection in seconds |

## Database Setup

### 1. Create PostgreSQL Database and User

```sql
-- Create database
CREATE DATABASE fleet_telemetry;

-- Create user
CREATE USER telemetry_user WITH PASSWORD 'your_secure_password';

-- Grant permissions
GRANT ALL PRIVILEGES ON DATABASE fleet_telemetry TO telemetry_user;

-- Connect to the database
\c fleet_telemetry

-- Grant schema permissions
GRANT ALL PRIVILEGES ON SCHEMA public TO telemetry_user;
```

### 2. Tables Created Automatically

The PostgreSQL connector automatically creates the following tables on startup:

#### `{namespace}_V` (Vehicle Data)
- Stores vehicle telemetry data
- Columns: id, vin, tx_type, timestamp, payload, metadata, received_at
- Indexed: vin, timestamp, received_at

#### `{namespace}_alerts` (Vehicle Alerts)
- Stores vehicle alert messages
- Same schema as {namespace}_V

#### `{namespace}_errors` (Vehicle Errors)
- Stores vehicle error messages
- Same schema as {namespace}_V

#### `{namespace}_connectivity` (Vehicle Connectivity)
- Stores vehicle connectivity events
- Same schema as {namespace}_V

**Note:** Table names are prefixed with your configured namespace (default: `tesla`). JSONB column for metadata provides excellent querying capabilities.

## Usage Examples

### Example Configuration with PostgreSQL Only

```json
{
  "host": "0.0.0.0",
  "port": 443,
  "tls": {
    "server_cert": "/path/to/server.crt",
    "server_key": "/path/to/server.key"
  },
  "postgres": {
    "host": "db.example.com",
    "port": 5432,
    "user": "telemetry_user",
    "password": "secure_password",
    "database": "fleet_telemetry",
    "ssl_mode": "require",
    "max_open_conns": 25,
    "max_idle_conns": 5,
    "conn_max_lifetime": 300
  },
  "namespace": "tesla",
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

### Example Configuration with PostgreSQL and Kafka

```json
{
  "host": "0.0.0.0",
  "port": 443,
  "tls": {
    "server_cert": "/path/to/server.crt",
    "server_key": "/path/to/server.key"
  },
  "kafka": {
    "bootstrap.servers": "kafka:9092"
  },
  "postgres": {
    "host": "db.example.com",
    "port": 5432,
    "user": "telemetry_user",
    "password": "secure_password",
    "database": "fleet_telemetry"
  },
  "namespace": "tesla",
  "records": {
    "V": ["postgres", "kafka"],
    "alerts": ["postgres", "kafka"],
    "errors": ["postgres", "kafka"],
    "connectivity": ["postgres"]
  }
}
```

### Example Configuration with Reliable Acks

```json
{
  "postgres": {
    "host": "localhost",
    "port": 5432,
    "user": "telemetry_user",
    "password": "password",
    "database": "fleet_telemetry"
  },
  "namespace": "tesla",
  "records": {
    "V": ["postgres"],
    "alerts": ["postgres"],
    "errors": ["postgres"]
  },
  "reliable_ack_sources": {
    "V": "postgres",
    "alerts": "postgres",
    "errors": "postgres"
  }
}
```

## Querying the Data

### Check Table Structure

```sql
\d tesla_V
\d tesla_alerts
\d tesla_errors
\d tesla_connectivity
```

### Get Recent Records for a Vehicle

```sql
SELECT * FROM tesla_V 
WHERE vin = 'VEHICLE_VIN_HERE' 
ORDER BY received_at DESC 
LIMIT 100;
```

### Query JSONB Metadata

```sql
-- Get records with specific metadata
SELECT vin, tx_type, metadata, received_at 
FROM tesla_V 
WHERE metadata @> '{"key": "value"}'
ORDER BY received_at DESC;

-- Extract specific fields from metadata
SELECT vin, metadata->>'field_name' as field_value, received_at 
FROM tesla_V 
WHERE vin = 'VEHICLE_VIN_HERE'
ORDER BY received_at DESC;
```

### Get All Alerts for a VIN

```sql
SELECT vin, tx_type, metadata, received_at 
FROM tesla_alerts 
WHERE vin = 'VEHICLE_VIN_HERE' 
ORDER BY received_at DESC;
```

### Get Records per Hour

```sql
SELECT 
    DATE_TRUNC('hour', received_at) AS hour,
    COUNT(*) as record_count,
    SUM(LENGTH(payload)) as bytes_total
FROM tesla_V
GROUP BY DATE_TRUNC('hour', received_at)
ORDER BY hour DESC;
```

### Check Database Size

```sql
SELECT 
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
```

## Performance Considerations

1. **Connection Pool Settings**
   - `max_open_conns`: Increase for high-throughput scenarios (default: 25)
   - `max_idle_conns`: Keep connections warm (default: 5)
   - `conn_max_lifetime`: Helps prevent connection leaks (default: 300 seconds)

2. **Indexes**
   - VIN, timestamp, and received_at indexes are created by default
   - Consider adding GIN index on metadata for complex queries
   - Example: `CREATE INDEX idx_tesla_V_metadata ON tesla_V USING GIN (metadata);`

3. **Table Partitioning**
   - For very large datasets, consider partitioning tables by date
   - Example: `PARTITION BY RANGE (DATE_TRUNC('month', received_at))`

4. **Backup Strategy**
   - Use pg_dump for backups: `pg_dump -h host -U user fleet_telemetry > backup.sql`
   - Consider incremental backups for large databases

## Monitoring

The PostgreSQL connector reports the following metrics to Prometheus (if enabled):

- `records_produced_total`: Total records written to PostgreSQL
- `record_bytes_produced_total`: Total bytes written to PostgreSQL
- `postgres_errors_total`: Total PostgreSQL errors encountered
- `records_reliable_ack_total`: Total reliable acknowledgments sent

View metrics at: `http://localhost:8000/metrics` (if Prometheus port is configured)

## Troubleshooting

### Connection Errors

```
failed to ping PostgreSQL database: pq: password authentication failed
```

Check your connection parameters (host, port, user, password, database).

### Table Creation Errors

```
failed to create tables: relation "tesla_V" already exists
```

This is normal - the connector checks if tables exist before creating them.

### SSL Connection Issues

If you need SSL connections, set `ssl_mode` to one of:
- `disable` - No SSL
- `allow` - OK to not use SSL
- `prefer` - Prefer SSL (default)
- `require` - SSL required
- `verify-ca` - SSL required and CA verified
- `verify-full` - SSL required, CA verified, hostname verified

### Performance Issues

1. Check active connections: `SELECT * FROM pg_stat_activity;`
2. Check slow queries: Enable `log_min_duration_statement`
3. Analyze query plans: `EXPLAIN ANALYZE SELECT ...`
4. Check index usage: `SELECT * FROM pg_stat_user_indexes;`

## Database Maintenance

### Analyzing Query Performance

```sql
-- Analyze table statistics
ANALYZE tesla_V;

-- Check index usage
SELECT schemaname, tablename, indexname, idx_scan 
FROM pg_stat_user_indexes 
WHERE schemaname = 'public' 
ORDER BY idx_scan DESC;
```

### Vacuum and Cleanup

```sql
-- Standard vacuum
VACUUM tesla_V;

-- Aggressive vacuum with index reorg
VACUUM FULL ANALYZE tesla_V;
```

### Archive Old Data

```sql
-- Create a separate schema for archives
CREATE SCHEMA IF NOT EXISTS archive;

-- Create archive table
CREATE TABLE archive.tesla_V AS 
SELECT * FROM tesla_V 
WHERE received_at < CURRENT_DATE - INTERVAL '6 months';

-- Delete from main table
DELETE FROM tesla_V 
WHERE received_at < CURRENT_DATE - INTERVAL '6 months';

-- Vacuum after delete
VACUUM ANALYZE tesla_V;
```

## Security Considerations

1. **Strong Passwords**: Always use strong, unique passwords for database users
2. **SSL/TLS**: Enable SSL for PostgreSQL connections in production (`ssl_mode: require`)
3. **User Permissions**: Grant minimal necessary privileges
   ```sql
   -- More restrictive setup
   GRANT CONNECT ON DATABASE fleet_telemetry TO telemetry_user;
   GRANT USAGE ON SCHEMA public TO telemetry_user;
   GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA public TO telemetry_user;
   ```
4. **Network Security**: Restrict database access to application servers only
5. **Data Encryption**: Consider encrypting sensitive data at rest
6. **Backup Security**: Protect backup files with appropriate permissions
7. **Audit Logging**: Enable PostgreSQL audit logging for compliance
   ```sql
   -- Enable statement logging
   ALTER SYSTEM SET log_statement = 'all';
   ALTER SYSTEM SET log_duration = ON;
   SELECT pg_reload_conf();
   ```

## Advantages of PostgreSQL

- **JSONB**: Superior JSON querying capabilities compared to MySQL
- **Advanced indexing**: GIN and GIST indexes for complex queries
- **Full-text search**: Built-in full-text search capabilities
- **Partitioning**: Native table partitioning for large datasets
- **Replication**: Robust replication and failover options
- **Performance**: Generally superior performance for complex queries

## References

- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [JSONB Documentation](https://www.postgresql.org/docs/current/datatype-json.html)
- [pq Driver](https://github.com/lib/pq)
- [Fleet Telemetry Documentation](https://github.com/teslamotors/fleet-telemetry)

