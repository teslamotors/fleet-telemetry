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

	"github.com/teslamotors/fleet-telemetry/config"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/messages"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var (
	upgrader = websocket.Upgrader{
		// disable origin checking on the websocket.  we're not serving browsers
		CheckOrigin:     func(r *http.Request) bool { return true },
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	serverMetricsRegistry ServerMetrics
	serverMetricsOnce     sync.Once
)

// Metrics stores metrics reported from this package
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

	airbrakeHandler *airbrake.AirbrakeHandler

	registry *SocketRegistry

	ackChan chan (*telemetry.Record)

	reliableAckSources map[string]telemetry.Dispatcher
}

// InitServer initializes the main server
func InitServer(c *config.Config, airbrakeHandler *airbrake.AirbrakeHandler, producerRules map[string][]telemetry.Producer, logger *logrus.Logger, registry *SocketRegistry) (*http.Server, *Server, error) {

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
	mux.Handle("/status", socketServer.airbrakeHandler.WithReporting(http.HandlerFunc(socketServer.Status(c))))

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
func (s *Server) Status(config *config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if config.DisableTLS {
			fmt.Fprint(w, "mtls disabled")
		} else {
			fmt.Fprint(w, "mtls ok")
		}
	}
}

// ServeBinaryWs serves a http query and upgrades it to a websocket -- only serves binary data coming from the ws
func (s *Server) ServeBinaryWs(config *config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if ws := s.promoteToWebsocket(w, r); ws != nil {
			ctx := context.WithValue(context.Background(), SocketContext, map[string]interface{}{"request": r})
			requestIdentity, err := extractIdentityFromConnection(ctx, r)
			if err != nil {
				s.logger.ErrorLog("extract_sender_id_err", err, nil)
			}

			socketManager := NewSocketManager(ctx, requestIdentity, ws, config, s.logger)
			s.registry.RegisterSocket(socketManager)
			defer s.registry.DeregisterSocket(socketManager)

			binarySerializer := telemetry.NewBinarySerializer(requestIdentity, s.DispatchRules, s.logger)
			socketManager.ProcessTelemetry(binarySerializer)
		}
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

func extractIdentityFromConnection(ctx context.Context, r *http.Request) (*telemetry.RequestIdentity, error) {
	cert, err := extractCertFromHeaders(ctx, r)
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

func extractCertFromHeaders(ctx context.Context, r *http.Request) (*x509.Certificate, error) {
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
