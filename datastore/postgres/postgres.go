package postgres

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/lib/pq"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// Config for PostgreSQL connection
type Config struct {
	Host            string `json:"host,omitempty"`
	Port            int    `json:"port,omitempty"`
	Username        string `json:"user,omitempty"`
	Password        string `json:"password,omitempty"`
	Database        string `json:"database,omitempty"`
	SSLMode         string `json:"ssl_mode,omitempty"`
	MaxOpenConns    int    `json:"max_open_conns,omitempty"`
	MaxIdleConns    int    `json:"max_idle_conns,omitempty"`
	ConnMaxLifetime int    `json:"conn_max_lifetime,omitempty"`
}

// Producer client to handle PostgreSQL interactions
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

// NewProducer establishes the PostgreSQL connection and returns a producer
func NewProducer(config *Config, namespace string, prometheusEnabled bool, metricsCollector metrics.MetricCollector, airbrakeHandler *airbrake.Handler, ackChan chan (*telemetry.Record), reliableAckTxTypes map[string]interface{}, logger *logrus.Logger) (telemetry.Producer, error) {
	if config == nil {
		return nil, fmt.Errorf("PostgreSQL config is required")
	}

	if config.Database == "" {
		return nil, fmt.Errorf("database name is required")
	}

	registerMetricsOnce(metricsCollector)

	// Set default values
	if config.Port == 0 {
		config.Port = 5432
	}
	if config.SSLMode == "" {
		config.SSLMode = "disable"
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

	// Build DSN (Data Source Name)
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host,
		config.Port,
		config.Username,
		config.Password,
		config.Database,
		config.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open PostgreSQL connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(config.ConnMaxLifetime) * time.Second)

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping PostgreSQL database: %w", err)
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

	producer.logger.ActivityLog("postgres_registered", logrus.LogInfo{"namespace": namespace, "database": config.Database})
	return producer, nil
}

// createTables creates the necessary tables in PostgreSQL if they don't exist
func (p *Producer) createTables() error {
	// Table for vehicle data records
	createVTable := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS "%s_V" (
		id BIGSERIAL PRIMARY KEY,
		vin VARCHAR(17) NOT NULL,
		tx_type VARCHAR(50),
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		payload BYTEA,
		metadata JSONB,
		received_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_%s_V_vin ON "%s_V" (vin);
	CREATE INDEX IF NOT EXISTS idx_%s_V_timestamp ON "%s_V" (timestamp);
	CREATE INDEX IF NOT EXISTS idx_%s_V_received_at ON "%s_V" (received_at);
	`, p.namespace, p.namespace, p.namespace, p.namespace, p.namespace, p.namespace, p.namespace)

	// Table for alerts
	createAlertsTable := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS "%s_alerts" (
		id BIGSERIAL PRIMARY KEY,
		vin VARCHAR(17) NOT NULL,
		tx_type VARCHAR(50),
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		payload BYTEA,
		metadata JSONB,
		received_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_%s_alerts_vin ON "%s_alerts" (vin);
	CREATE INDEX IF NOT EXISTS idx_%s_alerts_timestamp ON "%s_alerts" (timestamp);
	CREATE INDEX IF NOT EXISTS idx_%s_alerts_received_at ON "%s_alerts" (received_at);
	`, p.namespace, p.namespace, p.namespace, p.namespace, p.namespace, p.namespace, p.namespace)

	// Table for errors
	createErrorsTable := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS "%s_errors" (
		id BIGSERIAL PRIMARY KEY,
		vin VARCHAR(17) NOT NULL,
		tx_type VARCHAR(50),
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		payload BYTEA,
		metadata JSONB,
		received_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_%s_errors_vin ON "%s_errors" (vin);
	CREATE INDEX IF NOT EXISTS idx_%s_errors_timestamp ON "%s_errors" (timestamp);
	CREATE INDEX IF NOT EXISTS idx_%s_errors_received_at ON "%s_errors" (received_at);
	`, p.namespace, p.namespace, p.namespace, p.namespace, p.namespace, p.namespace, p.namespace)

	// Table for connectivity
	createConnectivityTable := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS "%s_connectivity" (
		id BIGSERIAL PRIMARY KEY,
		vin VARCHAR(17) NOT NULL,
		tx_type VARCHAR(50),
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		payload BYTEA,
		metadata JSONB,
		received_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_%s_connectivity_vin ON "%s_connectivity" (vin);
	CREATE INDEX IF NOT EXISTS idx_%s_connectivity_timestamp ON "%s_connectivity" (timestamp);
	CREATE INDEX IF NOT EXISTS idx_%s_connectivity_received_at ON "%s_connectivity" (received_at);
	`, p.namespace, p.namespace, p.namespace, p.namespace, p.namespace, p.namespace, p.namespace)

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
	return fmt.Sprintf(`"%s_%s"`, p.namespace, recordType)
}

// Produce inserts the record into the appropriate PostgreSQL table
func (p *Producer) Produce(entry *telemetry.Record) {
	p.mu.Lock()
	defer p.mu.Unlock()

	table := p.tableName(entry.TxType)
	query := fmt.Sprintf(
		"INSERT INTO %s (vin, tx_type, payload, metadata) VALUES ($1, $2, $3, $4)",
		table,
	)

	metadata, err := entry.Metadata().MarshalJSON()
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

// ProcessReliableAck handles acknowledgments from PostgreSQL
func (p *Producer) ProcessReliableAck(entry *telemetry.Record) {
	// PostgreSQL inserts are synchronous, so we can confirm immediately
	if p.ackChan != nil {
		select {
		case p.ackChan <- entry:
			metricsRegistry.reliableAckCount.Inc(map[string]string{"record_type": entry.TxType})
		default:
			p.logger.WarningLog("ack_channel_full", logrus.LogInfo{"vin": entry.Vin, "txtype": entry.TxType})
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
	p.logger.ErrorLog("postgres_error", err, logrus.LogInfo{})
}

// registerMetricsOnce registers metrics on first use
func registerMetricsOnce(metricsCollector metrics.MetricCollector) {
	metricsOnce.Do(func() {
		metricsRegistry.producerCount = metricsCollector.RegisterCounter(
			"records_produced_total",
			"Total records produced to PostgreSQL",
			"record_type",
		)
		metricsRegistry.bytesTotal = metricsCollector.RegisterCounter(
			"record_bytes_produced_total",
			"Total bytes produced to PostgreSQL",
			"record_type",
		)
		metricsRegistry.producerAckCount = metricsCollector.RegisterCounter(
			"records_produced_ack_total",
			"Total records acked from PostgreSQL",
			"record_type",
		)
		metricsRegistry.bytesAckTotal = metricsCollector.RegisterCounter(
			"record_bytes_produced_ack_total",
			"Total bytes acked from PostgreSQL",
			"record_type",
		)
		metricsRegistry.errorCount = metricsCollector.RegisterCounter(
			"postgres_errors_total",
			"Total PostgreSQL errors",
		)
		metricsRegistry.reliableAckCount = metricsCollector.RegisterCounter(
			"records_reliable_ack_total",
			"Total records with reliable ack",
			"record_type",
		)
	})
}
