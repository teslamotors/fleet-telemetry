package streaming

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/beefsack/go-rate"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/teslamotors/fleet-telemetry/config"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

type contextKeyType int

// SocketContext is the name of variable holding socket data in the context
const SocketContext contextKeyType = iota + 1

// ReadWriteExitDeadline is the deadline to set when existing the reader/writer
const ReadWriteExitDeadline = 50 * time.Millisecond

// WriteLoopDeadline is the read/write deadline in the main loop
const WriteLoopDeadline = 10 * time.Second

// SocketManager is a struct responsible for managing the socket connection with the clients
type SocketManager struct {
	Ws           *websocket.Conn
	MsgType      int
	RecordsStats map[string]int
	StartTime    time.Time
	UUID         string

	config                 *config.Config
	logger                 *logrus.Logger
	registry               *SocketRegistry
	requestIdentity        *telemetry.RequestIdentity
	requestInfo            map[string]interface{}
	metricsCollector       metrics.MetricCollector
	stopChan               chan struct{}
	writeChan              chan SocketMessage
	transmitDecodedRecords bool
}

// SocketMessage represents incoming socket connection
type SocketMessage struct {
	MsgType int
	Msg     []byte
}

// Metrics stores metrics reported from this package
type Metrics struct {
	rateLimitExceededCount       adapter.Counter
	recordTooBigCount            adapter.Counter
	unauthorizedSenderCount      adapter.Counter
	unknownMessageTypeErrorCount adapter.Counter
	dispatchCount                adapter.Counter
	unexpectedRecordErrorCount   adapter.Counter
	socketErrorCount             adapter.Counter
	recordSizeBytesTotal         adapter.Counter
	recordCount                  adapter.Counter
}

var (
	metricsRegistry Metrics
	metricsOnce     sync.Once
)

// NewSocketManager instantiates a SocketManager
func NewSocketManager(ctx context.Context, requestIdentity *telemetry.RequestIdentity, ws *websocket.Conn, config *config.Config, logger *logrus.Logger) *SocketManager {
	registerMetricsOnce(config.MetricCollector)

	requestLogInfo, socketUUID := buildRequestContext(ctx)

	return &SocketManager{
		Ws:           ws,
		MsgType:      websocket.BinaryMessage,
		RecordsStats: make(map[string]int),
		StartTime:    time.Now(),
		UUID:         socketUUID.String(),

		config:                 config,
		metricsCollector:       config.MetricCollector,
		logger:                 logger,
		requestInfo:            requestLogInfo,
		writeChan:              make(chan SocketMessage, 1000),
		stopChan:               make(chan struct{}),
		requestIdentity:        requestIdentity,
		transmitDecodedRecords: config.TransmitDecodedRecords,
	}
}

func buildRequestContext(ctx context.Context) (logInfo map[string]interface{}, socketUUID uuid.UUID) {
	socketUUID = uuid.New()
	logInfo = make(map[string]interface{})
	if ctx == nil {
		return
	}

	contextInfo, ok := ctx.Value(SocketContext).(map[string]interface{})
	if !ok {
		return
	}

	r, ok := contextInfo["request"].(*http.Request)
	if !ok {
		return
	}

	txid, err := uuid.Parse(r.Header.Get("X-TXID"))
	if err != nil {
		txid = socketUUID
		r.Header.Add("X-TXID", txid.String())
	} else {
		socketUUID = txid
	}

	logInfo["network_interface"] = r.Header.Get("X-Network-Interface")
	logInfo["txid"] = txid
	logInfo["method"] = r.Method
	logInfo["path"] = r.URL.Path
	logInfo["user_agent"] = r.Header.Get("User-Agent")
	logInfo["X-Forwarded-For"] = r.Header.Get("X-Forwarded-For")

	return
}

// ListenToWriteChannel to the write channel
func (sm *SocketManager) ListenToWriteChannel() SocketMessage {
	msg := <-sm.writeChan
	return msg
}

// Close shuts down a socket connection for a single client and log metrics
func (sm *SocketManager) Close() {
	if err := sm.Ws.Close(); err != nil {
		sm.logger.ErrorLog("websocket_close_err", err, nil)
	}

	socketMetrics := sm.RecordsStatsToLogInfo()
	socketMetrics["duration_sec"] = int(time.Since(sm.StartTime) / time.Second) // Result is in nanosecond, converting it to seconds
	sm.logger.ActivityLog("socket_disconnected", socketMetrics)
}

// RecordsStatsToLogInfo formats the stats map into a string
func (sm *SocketManager) RecordsStatsToLogInfo() map[string]interface{} {
	total := 0
	logInfo := make(map[string]interface{})
	for key, value := range sm.RecordsStats {
		logInfo[key] = strconv.Itoa(value)
		total += value
	}
	logInfo["total"] = strconv.Itoa(total)
	return logInfo
}

// ProcessTelemetry uses the serializer to dispatch telemetry records
func (sm *SocketManager) ProcessTelemetry(serializer *telemetry.BinarySerializer) {
	defer func() {
		sm.Close()
		close(sm.stopChan)
	}()

	sm.logger.ActivityLog("socket_connected", sm.requestInfo)
	go sm.writer()
	var rl *rate.RateLimiter

	if sm.config.RateLimit != nil && sm.config.RateLimit.Enabled {
		rl = rate.New(sm.config.RateLimit.MessageLimit, sm.config.RateLimit.MessageIntervalTimeSecond)
	} else {
		rl = rate.New(100, 60*time.Second)
	}

	var rateLimitStartTime time.Time
	messagesRateLimited := 0

	// infinite loop until the client disconnects (keep accepting new messages)
	for {
		msgType, message, err := sm.Ws.ReadMessage()
		if err != nil || msgType != sm.MsgType {
			return
		}

		// check rate limit
		if ok, _ := rl.Try(); !ok {
			if messagesRateLimited == 0 {
				rateLimitStartTime = time.Now()
			}
			// client exceeded the rate limit
			messagesRateLimited++
			record, _ := telemetry.NewRecord(serializer, message, sm.UUID, sm.transmitDecodedRecords)
			metricsRegistry.rateLimitExceededCount.Inc(map[string]string{"device_id": sm.requestIdentity.DeviceID, "txtype": record.TxType})
			if sm.config.RateLimit != nil && sm.config.RateLimit.Enabled {
				continue
			}
		}
		if messagesRateLimited > 0 {
			parts := bytes.Split(message, []byte(","))
			if len(parts) > 2 {
				duration := time.Since(rateLimitStartTime) / time.Second

				sm.logger.ErrorLog("rate_limit_exceeded", nil, logrus.LogInfo{"txid": parts[2], "duration_sec": duration, "messages_rate_limited": messagesRateLimited})
			}
			messagesRateLimited = 0
		}
		sm.ParseAndProcessRecord(serializer, message)
	}
}

// ParseAndProcessRecord reads incoming client message and dispatches to relevant producer
func (sm *SocketManager) ParseAndProcessRecord(serializer *telemetry.BinarySerializer, message []byte) {
	record, err := telemetry.NewRecord(serializer, message, sm.UUID, sm.transmitDecodedRecords)
	logInfo := logrus.LogInfo{"txid": record.Txid, "record_type": record.TxType}

	if err != nil {
		if err == telemetry.ErrMessageTooBig {
			sm.respondToVehicle(record, err)
			metricsRegistry.recordTooBigCount.Inc(map[string]string{})
			return
		}

		switch typedError := err.(type) {
		case *telemetry.UnauthorizedSenderIDError:
			logInfo["sender_id"] = typedError.ReceivedSenderID
			logInfo["expected_sender_id"] = typedError.ExpectedSenderID
			sm.logger.ErrorLog("unauthorized_sender_id", nil, logInfo)
			metricsRegistry.unauthorizedSenderCount.Inc(map[string]string{})
			sm.respondToVehicle(record, nil) // respond to the client message was accepted so they are not resending it over and over
			return
		case *telemetry.UnknownMessageType:
			logInfo["msg_txid"] = typedError.Txid
			logInfo["msg_type"] = string(typedError.GuessedType)
			sm.logger.ErrorLog("unknown_message_type_error", err, logInfo)
			metricsRegistry.unknownMessageTypeErrorCount.Inc(map[string]string{"msg_type": string(typedError.GuessedType)})
			sm.respondToVehicle(record, nil) // respond to the client message was accepted so they are not resending it over and over
		default:
			sm.respondToVehicle(record, err)
			return
		}
	}

	// write the record out to kafka
	sm.ReportMetricBytesPerRecords(record.TxType, record.Length())
	sm.processRecord(record)

	// respond instantly to the client if we are not doing reliable ACKs
	if !sm.reliableAck(record) {
		sm.respondToVehicle(record, nil)
	}
}

func (sm *SocketManager) reliableAck(record *telemetry.Record) bool {
	_, ok := sm.config.ReliableAckSources[record.TxType]
	return ok
}

func (sm *SocketManager) processRecord(record *telemetry.Record) {
	record.Dispatch()
	metricsRegistry.dispatchCount.Inc(map[string]string{"record_type": record.TxType})
}

// respondToVehicle sends an ack message to the client to acknowledge that the records have been transmitted
func (sm *SocketManager) respondToVehicle(record *telemetry.Record, err error) {
	var response []byte

	logInfo := logrus.LogInfo{"txid": record.Txid, "record_type": record.TxType}

	if err != nil {
		logInfo["client_id"] = sm.requestIdentity.DeviceID
		sm.logger.ErrorLog("unexpected_record", err, logInfo)
		metricsRegistry.unexpectedRecordErrorCount.Inc(map[string]string{})
		response = record.Error(errors.New("incorrect message format"))
		logInfo["response_type"] = "error"
	} else {
		logInfo["response_type"] = "ack"
		response = record.Ack()
	}

	sm.logger.Log(logrus.DEBUG, "message_respond", logInfo)
	sm.writeChan <- SocketMessage{sm.MsgType, response}
}

func (sm *SocketManager) writer() {
	defer func() {

		sm.logger.Log(logrus.DEBUG, "writer_done", nil)
		_ = sm.Ws.SetReadDeadline(time.Now().Add(ReadWriteExitDeadline))
	}()

	for {
		select {
		case <-sm.stopChan:
			sm.logger.Log(logrus.DEBUG, "return_stop_chan", nil)
			return
		case msg := <-sm.writeChan:
			err := sm.writeMessage(msg.MsgType, msg.Msg)
			if err != nil {
				metricsRegistry.socketErrorCount.Inc(map[string]string{})
				sm.logger.ErrorLog("socket_err", err, nil)
				return
			}
		}
	}
}

func (sm *SocketManager) writeMessage(msgType int, msg []byte) error {
	_ = sm.Ws.SetWriteDeadline(time.Now().Add(WriteLoopDeadline))
	return sm.Ws.WriteMessage(msgType, msg)
}

// ReportMetricBytesPerRecords records metrics for metric size
func (sm SocketManager) ReportMetricBytesPerRecords(recordType string, byteSize int) {
	sm.RecordsStats[recordType] += byteSize

	metricsRegistry.recordSizeBytesTotal.Add(int64(byteSize), map[string]string{"record_type": recordType})
	metricsRegistry.recordCount.Inc(map[string]string{"record_type": recordType})
}

func registerMetricsOnce(metricsCollector metrics.MetricCollector) {
	metricsOnce.Do(func() { registerMetrics(metricsCollector) })
}

func registerMetrics(metricsCollector metrics.MetricCollector) {
	metricsRegistry.rateLimitExceededCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "rate_limit_exceeded_total",
		Help:   "The number of times a client has been rate limited.",
		Labels: []string{"device_id", "txtype"},
	})

	metricsRegistry.recordTooBigCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "record_too_big_total",
		Help:   "The number of times the record was too large.",
		Labels: []string{},
	})

	metricsRegistry.unauthorizedSenderCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "unauthorized_sender_id_total",
		Help:   "The number of times the sender was not authorized.",
		Labels: []string{},
	})

	metricsRegistry.unknownMessageTypeErrorCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "unknown_message_type_error_total",
		Help:   "The number of times the message type was not known.",
		Labels: []string{"msg_type"},
	})

	metricsRegistry.dispatchCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "dispatch_total",
		Help:   "The number of records dispatched.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.unexpectedRecordErrorCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "unexpected_record_err_total",
		Help:   "The number of unexpected records received.",
		Labels: []string{},
	})

	metricsRegistry.socketErrorCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "socket_err_total",
		Help:   "The number of socket errors.",
		Labels: []string{},
	})

	metricsRegistry.recordSizeBytesTotal = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "record_size_bytes_total",
		Help:   "The total number of record bytes processed.",
		Labels: []string{"record_type"},
	})

	metricsRegistry.recordCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "record_total",
		Help:   "The number of records processed.",
		Labels: []string{"record_type"},
	})

}
