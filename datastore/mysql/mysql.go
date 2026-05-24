package mysql

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// Config for MySQL connection
type Config struct {
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Username string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
	Database string `json:"database,omitempty"`
	// MaxOpenConns is the maximum number of open connections to the database
	MaxOpenConns int `json:"max_open_conns,omitempty"`
	// MaxIdleConns is the maximum number of idle connections in the connection pool
	MaxIdleConns int `json:"max_idle_conns,omitempty"`
	// ConnMaxLifetime is the maximum amount of time a connection may be reused (in seconds)
	ConnMaxLifetime int `json:"conn_max_lifetime,omitempty"`
}

// Producer client to handle MySQL interactions
type Producer struct {
	db                 *sql.DB
	namespace          string
	prometheusEnabled  bool
	metricsCollector   metrics.MetricCollector
	logger             *logrus.Logger
	airbrakeHandler    *airbrake.Handler
	ackChan            chan (*telemetry.Record)
	reliableAckTxTypes map[string]interface{}
	mu                 sync.Mutex
}

// Metrics stores metrics reported from this package
type Metrics struct {
	producerCount    adapter.Counter
	bytesTotal       adapter.Counter
	producerAckCount adapter.Counter
	bytesAckTotal    adapter.Counter
	errorCount       adapter.Counter
	reliableAckCount adapter.Counter
}

var (
	metricsRegistry Metrics
	metricsOnce     sync.Once
)

// NewProducer establishes the MySQL connection and returns a producer
func NewProducer(config *Config, namespace string, prometheusEnabled bool, metricsCollector metrics.MetricCollector, airbrakeHandler *airbrake.Handler, ackChan chan (*telemetry.Record), reliableAckTxTypes map[string]interface{}, logger *logrus.Logger) (telemetry.Producer, error) {
	if config == nil {
		return nil, fmt.Errorf("MySQL config is required")
	}

	if config.Database == "" {
		return nil, fmt.Errorf("database name is required")
	}

	registerMetricsOnce(metricsCollector)

	// Build DSN (Data Source Name)
	if config.Port == 0 {
		config.Port = 3306
	}
	if config.MaxOpenConns == 0 {
		config.MaxOpenConns = 25
	}
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = 5
	}
	if config.ConnMaxLifetime == 0 {
		config.ConnMaxLifetime = 5 * 60 // 5 minutes
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=UTC",
		config.Username,
		config.Password,
		config.Host,
		config.Port,
		config.Database,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open MySQL connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(config.ConnMaxLifetime) * time.Second)

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping MySQL database: %w", err)
	}

	producer := &Producer{
		db:                 db,
		namespace:          namespace,
		metricsCollector:   metricsCollector,
		prometheusEnabled:  prometheusEnabled,
		logger:             logger,
		airbrakeHandler:    airbrakeHandler,
		ackChan:            ackChan,
		reliableAckTxTypes: reliableAckTxTypes,
	}

	// Create tables if they don't exist
	if err := producer.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	producer.logger.ActivityLog("mysql_registered", logrus.LogInfo{"namespace": namespace, "database": config.Database})
	return producer, nil
}

// createTables creates the necessary tables in MySQL if they don't exist
func (p *Producer) createTables() error {
	// Table for vehicle data records
	createVTable := `
	CREATE TABLE IF NOT EXISTS ` + p.tableName("V") + ` (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		vin VARCHAR(17) NOT NULL,
		tx_type VARCHAR(50),
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		payload LONGBLOB,
		metadata JSON,
		received_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_vin (vin),
		INDEX idx_timestamp (timestamp)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`

	// Table for alerts
	createAlertsTable := `
	CREATE TABLE IF NOT EXISTS ` + p.tableName("alerts") + ` (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		vin VARCHAR(17) NOT NULL,
		tx_type VARCHAR(50),
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		payload LONGBLOB,
		metadata JSON,
		received_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_vin (vin),
		INDEX idx_timestamp (timestamp)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`

	// Table for errors
	createErrorsTable := `
	CREATE TABLE IF NOT EXISTS ` + p.tableName("errors") + ` (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		vin VARCHAR(17) NOT NULL,
		tx_type VARCHAR(50),
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		payload LONGBLOB,
		metadata JSON,
		received_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_vin (vin),
		INDEX idx_timestamp (timestamp)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`

	// Table for connectivity
	createConnectivityTable := `
	CREATE TABLE IF NOT EXISTS ` + p.tableName("connectivity") + ` (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		vin VARCHAR(17) NOT NULL,
		tx_type VARCHAR(50),
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		payload LONGBLOB,
		metadata JSON,
		received_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_vin (vin),
		INDEX idx_timestamp (timestamp)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`

	tables := []string{createVTable, createAlertsTable, createErrorsTable, createConnectivityTable}
	for _, tableSQL := range tables {
		if _, err := p.db.Exec(tableSQL); err != nil {
			return err
		}
	}
	return nil
}

// tableName returns the full table name with namespace prefix
func (p *Producer) tableName(recordType string) string {
	return fmt.Sprintf("`%s_%s`", p.namespace, recordType)
}

// Produce inserts the record into the appropriate MySQL table
func (p *Producer) Produce(entry *telemetry.Record) {
	p.mu.Lock()
	defer p.mu.Unlock()

	table := p.tableName(entry.TxType)
	query := fmt.Sprintf(
		"INSERT INTO %s (vin, tx_type, payload, metadata) VALUES (?, ?, ?, ?)",
		table,
	)

	metadata, err := entry.MetadataJSON()
	if err != nil {
		p.logError(fmt.Errorf("failed to marshal metadata: %w", err))
		metricsRegistry.errorCount.Inc(map[string]string{"record_type": entry.TxType})
		return
	}

	_, err = p.db.Exec(query, entry.Vin, entry.TxType, entry.Payload(), string(metadata))
	if err != nil {
		p.logError(fmt.Errorf("failed to insert record into %s: %w", table, err))
		metricsRegistry.errorCount.Inc(map[string]string{"record_type": entry.TxType})
		return
	}

	metricsRegistry.producerCount.Inc(map[string]string{"record_type": entry.TxType})
	metricsRegistry.bytesTotal.Add(int64(entry.Length()), map[string]string{"record_type": entry.TxType})

	// Handle reliable acks
	if p.reliableAckTxTypes[entry.TxType] != nil && p.ackChan != nil {
		p.ackChan <- entry
		metricsRegistry.reliableAckCount.Inc(map[string]string{"record_type": entry.TxType})
	}
}

// ProcessReliableAck handles acknowledgments from MySQL
func (p *Producer) ProcessReliableAck(entry *telemetry.Record) {
	// MySQL inserts are synchronous, so we can confirm immediately
	if p.ackChan != nil {
		select {
		case p.ackChan <- entry:
			metricsRegistry.reliableAckCount.Inc(map[string]string{"record_type": entry.TxType})
		default:
			p.logger.Log(logrus.WARN, "ack_channel_full", logrus.LogInfo{"vin": entry.Vin, "txtype": entry.TxType})
		}
	}
}

// ReportError logs errors to airbrake and logger
func (p *Producer) ReportError(message string, err error, logInfo logrus.LogInfo) {
	p.airbrakeHandler.ReportLogMessage(logrus.ERROR, message, err, logInfo)
	p.logger.ErrorLog(message, err, logInfo)
	metricsRegistry.errorCount.Inc(map[string]string{})
}

// Close closes the database connection
func (p *Producer) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// logError logs an error
func (p *Producer) logError(err error) {
	p.logger.ErrorLog("mysql_error", err, logrus.LogInfo{})
}

// registerMetricsOnce registers metrics on first use
func registerMetricsOnce(metricsCollector metrics.MetricCollector) {
	metricsOnce.Do(func() {
		metricsRegistry.producerCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
			Name:   "records_produced_total",
			Help:   "Total records produced to MySQL",
			Labels: []string{"record_type"},
		})
		metricsRegistry.bytesTotal = metricsCollector.RegisterCounter(adapter.CollectorOptions{
			Name:   "record_bytes_produced_total",
			Help:   "Total bytes produced to MySQL",
			Labels: []string{"record_type"},
		})
		metricsRegistry.producerAckCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
			Name:   "records_produced_ack_total",
			Help:   "Total records acked from MySQL",
			Labels: []string{"record_type"},
		})
		metricsRegistry.bytesAckTotal = metricsCollector.RegisterCounter(adapter.CollectorOptions{
			Name:   "record_bytes_produced_ack_total",
			Help:   "Total bytes acked from MySQL",
			Labels: []string{"record_type"},
		})
		metricsRegistry.errorCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
			Name: "mysql_errors_total",
			Help: "Total MySQL errors",
		})
		metricsRegistry.reliableAckCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
			Name:   "records_reliable_ack_total",
			Help:   "Total records with reliable ack",
			Labels: []string{"record_type"},
		})
	})
}
