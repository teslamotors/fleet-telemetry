package streaming

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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
		// A client certificate must never reach a server that is configured to trust a
		// proxy header instead, because the two identities can disagree. Reject before
		// upgrading so the operator sees an HTTP error rather than a dropped websocket.
		if err := rejectClientCertWithPassThrough(r, config); err != nil {
			s.logger.ErrorLog("tls_pass_through_client_cert_rejected", err, nil)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if ws := s.promoteToWebsocket(w, r); ws != nil {
			ctx := context.WithValue(context.Background(), SocketContext, map[string]interface{}{"request": r})
			requestIdentity, err := extractIdentity(r, config)
			if err != nil {
				s.logger.ErrorLog("extract_sender_id_err", err, nil)
				_ = ws.Close()
				return
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
		Vin:              sm.requestIdentity.DeviceID,
		ConnectionId:     sm.UUID,
		NetworkInterface: sm.GetNetworkInterface(),
		CreatedAt:        timestamppb.Now(),
		Status:           event,
	}

	payload, err := proto.Marshal(connectivityMessage)
	if err != nil {
		return err
	}

	// creating streamMessage is hack to satisfy input reqirements for telemetry.NewRecord
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
		return err
	}
	record, err := telemetry.NewRecord(serializer, message, sm.UUID, sm.transmitDecodedRecords)
	if err != nil {
		return err
	}
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

// ErrClientCertWithPassThrough is returned when a request carries a TLS client certificate
// while the server is configured to read the certificate from a proxy header instead.
var ErrClientCertWithPassThrough = errors.New("received a TLS client certificate while tls_pass_through is enabled: refusing the connection because the proxy header and the TLS certificate could identify different vehicles. Either remove tls_pass_through from the server configuration, or stop presenting client certificates to this listener")

// rejectClientCertWithPassThrough enforces that header-based identity and TLS client
// certificates are never used together.
func rejectClientCertWithPassThrough(r *http.Request, conf *config.Config) error {
	if conf.TLSPassThrough == nil {
		return nil
	}
	if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
		return ErrClientCertWithPassThrough
	}
	return nil
}

func extractIdentity(r *http.Request, conf *config.Config) (*telemetry.RequestIdentity, error) {
	var cert *x509.Certificate
	var err error
	if conf.TLSPassThrough != nil {
		cert, err = extractCertFromProxyHeader(r, *conf.TLSPassThrough)
	} else {
		cert, err = extractCertFromTLS(r)
	}
	if err != nil {
		return nil, err
	}

	clientType, deviceID, err := messages.CreateIdentityFromCert(cert)
	if err != nil {
		return nil, fmt.Errorf("create_identity issuer: %s, common_name: %s, err: %v", cert.Issuer.CommonName, cert.Subject.CommonName, err)
	}
	return &telemetry.RequestIdentity{
		DeviceID:            deviceID,
		SenderID:            clientType + "." + deviceID,
		DeviceClientVersion: r.Header.Get("Version"),
	}, nil
}

// extractCertFromTLS returns the verified leaf certificate from the TLS connection. This is
// the default, and the only path where this server verifies the certificate itself.
func extractCertFromTLS(r *http.Request) (*x509.Certificate, error) {
	if r.TLS == nil {
		return nil, fmt.Errorf("missing_tls_state")
	}
	if len(r.TLS.VerifiedChains) == 0 || len(r.TLS.VerifiedChains[0]) == 0 {
		return nil, fmt.Errorf("missing_verified_client_certificate")
	}
	return r.TLS.VerifiedChains[0][0], nil
}

// extractCertFromProxyHeader returns the client certificate a trusted proxy forwarded.
//
// Unlike extractCertFromTLS, nothing here verifies the certificate: the proxy is trusted to
// have completed and validated the mTLS handshake. Anything able to reach this listener
// directly can therefore assert any identity, which is why the feature is off by default and
// the server must not be exposed outside the trusted network.
func extractCertFromProxyHeader(r *http.Request, mode config.TLSPassThrough) (*x509.Certificate, error) {
	switch mode {
	case config.RFC9440:
		return extractCertRFC9440(r)
	case config.AWSApplicationLoadBalancer:
		return extractCertAWSALB(r)
	default:
		return nil, fmt.Errorf("unsupported tls_pass_through mode: %s", mode)
	}
}

// extractCertRFC9440 reads the Client-Cert header defined by RFC 9440.
//
// Two details matter. The client's own certificate is carried in Client-Cert;
// Client-Cert-Chain carries the rest of the chain and explicitly excludes the end-entity
// certificate, so reading it would identify the issuing CA rather than the vehicle. The
// value is an RFC 8941 Byte Sequence: base64 of the DER certificate, delimited by colons,
// not PEM.
func extractCertRFC9440(r *http.Request) (*x509.Certificate, error) {
	raw := strings.TrimSpace(r.Header.Get("Client-Cert"))
	if raw == "" {
		return nil, fmt.Errorf("missing_certificate_header: Client-Cert")
	}
	// Tolerate a proxy that omits the Byte Sequence delimiters.
	encoded := strings.TrimSuffix(strings.TrimPrefix(raw, ":"), ":")
	der, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode Client-Cert header: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate from Client-Cert header: %w", err)
	}
	return cert, nil
}

// extractCertAWSALB reads the X-Amzn-Mtls-Clientcert header set by an AWS Application Load
// Balancer in mutual TLS passthrough mode. The value is the URL-encoded PEM of the whole
// chain, ordered leaf first, so the first block is the vehicle's certificate.
func extractCertAWSALB(r *http.Request) (*x509.Certificate, error) {
	raw := r.Header.Get("X-Amzn-Mtls-Clientcert")
	if raw == "" {
		return nil, fmt.Errorf("missing_certificate_header: X-Amzn-Mtls-Clientcert")
	}
	decoded, err := url.QueryUnescape(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to url decode X-Amzn-Mtls-Clientcert header: %w", err)
	}
	block, _ := pem.Decode([]byte(decoded))
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block from X-Amzn-Mtls-Clientcert header")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate from X-Amzn-Mtls-Clientcert header: %w", err)
	}
	return cert, nil
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
