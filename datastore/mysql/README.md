# MySQL Connector for Fleet Telemetry

The MySQL connector allows Fleet Telemetry to persist vehicle data directly to a MySQL database. It automatically creates and manages tables for different record types (V, alerts, errors, connectivity).

## Configuration

Add the MySQL configuration to your `config.json` file:

```json
{
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

### Configuration Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `host` | string | (required) | MySQL server hostname or IP address |
| `port` | int | 3306 | MySQL server port |
| `user` | string | (required) | MySQL username |
| `password` | string | (required) | MySQL password |
| `database` | string | (required) | Database name to use |
| `max_open_conns` | int | 25 | Maximum number of open connections to the database |
| `max_idle_conns` | int | 5 | Maximum number of idle connections in the connection pool |
| `conn_max_lifetime` | int | 300 | Maximum lifetime of a connection in seconds |

## Database Setup

### 1. Create MySQL Database and User

```sql
-- Create database
CREATE DATABASE fleet_telemetry;

-- Create user
CREATE USER 'telemetry_user'@'%' IDENTIFIED BY 'your_secure_password';

-- Grant permissions
GRANT ALL PRIVILEGES ON fleet_telemetry.* TO 'telemetry_user'@'%';
FLUSH PRIVILEGES;
```

### 2. Tables Created Automatically

The MySQL connector automatically creates the following tables on startup:

#### `tesla_V` (Vehicle Data)
- Stores vehicle telemetry data
- Columns: id, vin, tx_type, timestamp, payload, metadata, received_at
- Indexed: vin, timestamp

#### `tesla_alerts` (Vehicle Alerts)
- Stores vehicle alert messages
- Same schema as tesla_V

#### `tesla_errors` (Vehicle Errors)
- Stores vehicle error messages
- Same schema as tesla_V

#### `tesla_connectivity` (Vehicle Connectivity)
- Stores vehicle connectivity events
- Same schema as tesla_V

**Note:** Table names are prefixed with your configured namespace (default: `tesla`). Use InnoDB engine with utf8mb4 encoding.

## Usage Examples

### Example Configuration with MySQL Only

```json
{
  "host": "0.0.0.0",
  "port": 443,
  "tls": {
    "server_cert": "/path/to/server.crt",
    "server_key": "/path/to/server.key"
  },
  "mysql": {
    "host": "db.example.com",
    "port": 3306,
    "user": "telemetry_user",
    "password": "secure_password",
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
  },
  "monitoring": {
    "prometheus_metrics_port": 8000
  }
}
```

### Example Configuration with MySQL and Kafka

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
  "mysql": {
    "host": "db.example.com",
    "port": 3306,
    "user": "telemetry_user",
    "password": "secure_password",
    "database": "fleet_telemetry"
  },
  "namespace": "tesla",
  "records": {
    "V": ["mysql", "kafka"],
    "alerts": ["mysql", "kafka"],
    "errors": ["mysql", "kafka"],
    "connectivity": ["mysql"]
  }
}
```

### Example Configuration with Reliable Acks

```json
{
  "mysql": {
    "host": "localhost",
    "port": 3306,
    "user": "telemetry_user",
    "password": "password",
    "database": "fleet_telemetry"
  },
  "namespace": "tesla",
  "records": {
    "V": ["mysql"],
    "alerts": ["mysql"],
    "errors": ["mysql"]
  },
  "reliable_ack_sources": {
    "V": "mysql",
    "alerts": "mysql",
    "errors": "mysql"
  }
}
```

## Querying the Data

### Check Table Structure

```sql
DESCRIBE tesla_V;
DESCRIBE tesla_alerts;
DESCRIBE tesla_errors;
DESCRIBE tesla_connectivity;
```

### Get Recent Records for a Vehicle

```sql
SELECT * FROM tesla_V 
WHERE vin = 'VEHICLE_VIN_HERE' 
ORDER BY received_at DESC 
LIMIT 100;
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
    DATE_FORMAT(received_at, '%Y-%m-%d %H:00:00') AS hour,
    COUNT(*) as record_count,
    SUM(LENGTH(payload)) as bytes_total
FROM tesla_V
GROUP BY hour
ORDER BY hour DESC;
```

### Check Database Size

```sql
SELECT 
    table_name,
    ROUND(((data_length + index_length) / 1024 / 1024), 2) AS 'Size in MB'
FROM information_schema.TABLES
WHERE table_schema = 'fleet_telemetry'
ORDER BY (data_length + index_length) DESC;
```

## Performance Considerations

1. **Connection Pool Settings**
   - `max_open_conns`: Increase for high-throughput scenarios (default: 25)
   - `max_idle_conns`: Keep connections warm (default: 5)
   - `conn_max_lifetime`: Helps prevent connection leaks (default: 300 seconds)

2. **Indexes**
   - VIN and timestamp indexes are created by default
   - Consider adding more indexes based on your query patterns

3. **Table Partitioning**
   - For large datasets, consider partitioning tables by date or VIN
   - Example: `PARTITION BY RANGE (YEAR(received_at))`

4. **Backup Strategy**
   - Regularly backup your MySQL database
   - Example: `mysqldump -u user -p database > backup.sql`

## Monitoring

The MySQL connector reports the following metrics to Prometheus (if enabled):

- `records_produced_total`: Total records written to MySQL
- `record_bytes_produced_total`: Total bytes written to MySQL
- `mysql_errors_total`: Total MySQL errors encountered
- `records_reliable_ack_total`: Total reliable acknowledgments sent

View metrics at: `http://localhost:8000/metrics` (if Prometheus port is configured)

## Troubleshooting

### Connection Errors

```
failed to ping MySQL database: invalid DSN
```

Check your connection parameters (host, port, user, password, database).

### Table Creation Errors

```
failed to create tables: Table 'fleet_telemetry.tesla_V' already exists
```

This is normal - the connector checks if tables exist before creating them.

### Performance Issues

1. Monitor query performance
2. Check indexes: `SHOW INDEX FROM tesla_V;`
3. Review slow query log
4. Consider partitioning for large tables

### High Memory Usage

- Reduce `max_open_conns` to limit connections
- Monitor `max_idle_conns` setting
- Check for connection leaks in application logs

## Database Cleanup and Maintenance

### Archive Old Data

```sql
-- Create archive table
CREATE TABLE tesla_V_archive AS 
SELECT * FROM tesla_V 
WHERE received_at < DATE_SUB(NOW(), INTERVAL 6 MONTH);

-- Delete from main table
DELETE FROM tesla_V 
WHERE received_at < DATE_SUB(NOW(), INTERVAL 6 MONTH);

-- Optimize table
OPTIMIZE TABLE tesla_V;
```

### Regular Maintenance

```sql
-- Check table integrity
CHECK TABLE tesla_V;

-- Optimize tables
OPTIMIZE TABLE tesla_V, tesla_alerts, tesla_errors, tesla_connectivity;

-- Analyze table statistics
ANALYZE TABLE tesla_V;
```

## Security Considerations

1. **Use Strong Passwords**: Generate secure passwords for database users
2. **Network Security**: Restrict database access to application servers only
3. **SSL/TLS**: Enable SSL for MySQL connections in production
4. **User Permissions**: Grant minimal necessary privileges
5. **Data Encryption**: Consider encrypting sensitive data at rest
6. **Backup Security**: Protect backup files with appropriate permissions

## References

- [MySQL Documentation](https://dev.mysql.com/doc/)
- [Go MySQL Driver](https://github.com/go-sql-driver/mysql)
- [Fleet Telemetry Documentation](https://github.com/teslamotors/fleet-telemetry)

