package streaming

import (
	"context"
	"crypto/x509"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/teslamotors/fleet-telemetry/config"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/messages"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/protos"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var (
	upgrader = websocket.Upgrader{
		// disable origin checking on the websocket.  we're not serving browsers
		CheckOrigin:     func(_ *http.Request) bool { return true },
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	serverMetricsRegistry ServerMetrics
	serverMetricsOnce     sync.Once
)

const (
	connectitivityTopic = "connectivity"
)

// ServerMetrics stores metrics reported from this package
type ServerMetrics struct {
	reliableAckCount     adapter.Counter
	reliableAckMissCount adapter.Counter
}

// Server stores server resources
type Server struct {
	// DispatchRules is a mapping of topics (records type) to their dispatching methods (loaded from Records json)
	DispatchRules map[string][]telemetry.Producer

	logger *logrus.Logger
	// Metrics collects metrics for the application
	metricsCollector metrics.MetricCollector

	airbrakeHandler *airbrake.Handler

	registry *SocketRegistry

	ackChan chan (*telemetry.Record)

	reliableAckSources map[string]telemetry.Dispatcher
}

// InitServer initializes the main server
func InitServer(c *config.Config, airbrakeHandler *airbrake.Handler, producerRules map[string][]telemetry.Producer, logger *logrus.Logger, registry *SocketRegistry) (*http.Server, *Server, error) {

	socketServer := &Server{
		DispatchRules:      producerRules,
		metricsCollector:   c.MetricCollector,
		logger:             logger,
		airbrakeHandler:    airbrakeHandler,
		registry:           registry,
		ackChan:            c.AckChan,
		reliableAckSources: c.ReliableAckSources,
	}
	registerServerMetricsOnce(socketServer.metricsCollector)

	mux := http.NewServeMux()
	mux.HandleFunc("/", socketServer.ServeBinaryWs(c))
	mux.Handle("/status", socketServer.airbrakeHandler.WithReporting(http.HandlerFunc(socketServer.Status())))

	server := &http.Server{Addr: fmt.Sprintf("%v:%v", c.Host, c.Port), Handler: serveHTTPWithLogs(mux, logger)}
	go socketServer.handleAcks()
	return server, socketServer, nil
}

func (s *Server) handleAcks() {
	for record := range s.ackChan {
		reliableAckSource := string(s.reliableAckSources[record.TxType])
		if record.Serializer != nil {
			if socket := s.registry.GetSocket(record.SocketID); socket != nil {
				serverMetricsRegistry.reliableAckCount.Inc(map[string]string{"record_type": record.TxType, "dispatcher": reliableAckSource})
				socket.respondToVehicle(record, nil)
			} else {
				serverMetricsRegistry.reliableAckMissCount.Inc(map[string]string{"record_type": record.TxType, "dispatcher": reliableAckSource})
			}
		}
	}
}

// serveHTTPWithLogs wraps a handler and logs the request
func serveHTTPWithLogs(h http.Handler, logger *logrus.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlPath := r.URL.Path
		start := time.Now()
		uuidStr := uuid.New().String()

		requestLogInfo := logrus.LogInfo{"uuid": uuidStr, "method": r.Method, "urlPath": urlPath, "remote_ip": r.RemoteAddr}
		logger.ActivityLog("request_start", requestLogInfo)

		h.ServeHTTP(w, r)

		requestLogInfo["duration_ms"] = int(time.Since(start).Milliseconds())
		logger.ActivityLog("request_end", requestLogInfo)
	})
}

// Status API shows server with mtls config is up
func (s *Server) Status() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "mtls ok")
	}
}

// ServeBinaryWs serves a http query and upgrades it to a websocket -- only serves binary data coming from the ws
func (s *Server) ServeBinaryWs(config *config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if ws := s.promoteToWebsocket(w, r); ws != nil {
			ctx := context.WithValue(context.Background(), SocketContext, map[string]interface{}{"request": r})
			requestIdentity, err := extractIdentityFromConnection(r)
			if err != nil {
				s.logger.ErrorLog("extract_sender_id_err", err, nil)
			}

			binarySerializer := telemetry.NewBinarySerializer(requestIdentity, s.DispatchRules, s.logger)
			socketManager := NewSocketManager(ctx, requestIdentity, ws, config, s.logger)
			s.registerSocket(socketManager, binarySerializer)
			defer s.deregisterSocket(socketManager, binarySerializer)

			socketManager.ProcessTelemetry(binarySerializer)
		}
	}
}

func (s *Server) dispatchConnectivityEvent(sm *SocketManager, serializer *telemetry.BinarySerializer, event protos.ConnectivityEvent) error {
	connectivityDispatcher, ok := s.DispatchRules[connectitivityTopic]
	if !ok {
		return nil
	}

	connectivityMessage := &protos.VehicleConnectivity{
		Vin:          sm.requestIdentity.DeviceID,
		ConnectionId: sm.UUID,
		CreatedAt:    timestamppb.Now(),
		Status:       event,
	}

	payload, err := proto.Marshal(connectivityMessage)
	if err != nil {
		return nil
	}

	// creating streamMessage is hack to satify input reqirements for telemetry.NewRecord
	streamMessage := messages.StreamMessage{
		TXID:         []byte(sm.UUID),
		SenderID:     []byte(sm.requestIdentity.SenderID),
		DeviceID:     []byte(sm.requestIdentity.DeviceID),
		DeviceType:   []byte("vehicle_device"),
		MessageTopic: []byte(connectitivityTopic),
		Payload:      payload,
		CreatedAt:    uint32(connectivityMessage.CreatedAt.AsTime().Unix()),
	}

	message, err := streamMessage.ToBytes()
	if err != nil {
		return nil
	}
	record, _ := telemetry.NewRecord(serializer, message, sm.UUID, sm.transmitDecodedRecords)
	for _, dispatcher := range connectivityDispatcher {
		dispatcher.Produce(record)
	}
	return nil
}

func (s *Server) registerSocket(sm *SocketManager, serializer *telemetry.BinarySerializer) {
	s.registry.RegisterSocket(sm)
	event := protos.ConnectivityEvent_CONNECTED
	if err := s.dispatchConnectivityEvent(sm, serializer, event); err != nil {
		s.logger.ErrorLog("connectivity_registeration_error", err, logrus.LogInfo{"deviceID": sm.requestIdentity.DeviceID, "event": event})
	}

}

func (s *Server) deregisterSocket(sm *SocketManager, serializer *telemetry.BinarySerializer) {
	s.registry.DeregisterSocket(sm)
	event := protos.ConnectivityEvent_DISCONNECTED
	if err := s.dispatchConnectivityEvent(sm, serializer, event); err != nil {
		s.logger.ErrorLog("connectivity_deregisteration_error", err, logrus.LogInfo{"deviceID": sm.requestIdentity.DeviceID, "event": event})
	}
}

func (s *Server) promoteToWebsocket(w http.ResponseWriter, r *http.Request) *websocket.Conn {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.airbrakeHandler.ReportError(r, err)
		if _, ok := err.(websocket.HandshakeError); !ok {
			s.logger.ErrorLog("websocket_promotion_error", err, nil)
		}
		return nil
	}

	return ws
}

func extractIdentityFromConnection(r *http.Request) (*telemetry.RequestIdentity, error) {
	cert, err := extractCertFromHeaders(r)
	if err != nil {
		return nil, err
	}

	clientType, deviceID, err := messages.CreateIdentityFromCert(cert)
	if err != nil {
		return nil, fmt.Errorf("create_identity issuer: %s, common_name: %s, err: %v", cert.Issuer.CommonName, cert.Subject.CommonName, err)
	}
	return &telemetry.RequestIdentity{
		DeviceID: deviceID,
		SenderID: clientType + "." + deviceID,
	}, nil
}

func extractCertFromHeaders(r *http.Request) (*x509.Certificate, error) {
	nbCerts := len(r.TLS.PeerCertificates)
	if nbCerts == 0 {
		return nil, fmt.Errorf("missing_certificate_error")
	}

	return r.TLS.PeerCertificates[nbCerts-1], nil
}

func registerServerMetricsOnce(metricsCollector metrics.MetricCollector) {
	serverMetricsOnce.Do(func() { registerServerMetrics(metricsCollector) })
}

func registerServerMetrics(metricsCollector metrics.MetricCollector) {

	serverMetricsRegistry.reliableAckCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "reliable_ack",
		Help:   "The number of reliable acknowledgements.",
		Labels: []string{"record_type", "dispatcher"},
	})

	serverMetricsRegistry.reliableAckMissCount = metricsCollector.RegisterCounter(adapter.CollectorOptions{
		Name:   "reliable_ack_miss",
		Help:   "The number of missing reliable acknowledgements.",
		Labels: []string{"record_type", "dispatcher"},
	})
}
