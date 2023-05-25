package streaming

import (
	"context"
	"crypto/x509"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	"github.com/teslamotors/fleet-telemetry/config"
	"github.com/teslamotors/fleet-telemetry/messages"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var (
	upgrader = websocket.Upgrader{
		// disable origin checking on the websocket.  we're not serving browsers
		CheckOrigin:     func(r *http.Request) bool { return true },
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

// Server stores server resources
type Server struct {
	// DispatchRules is a mapping of topics (records type) to their dispatching methods (loaded from Records json)
	DispatchRules map[string][]telemetry.Producer

	logger *logrus.Logger
	// Metrics collects metrics for the application
	statsCollector metrics.MetricCollector

	reliableAck bool
}

// InitServer initializes the main server
func InitServer(c *config.Config, mux *http.ServeMux, producerRules map[string][]telemetry.Producer, logger *logrus.Logger, registry *SocketRegistry) (*http.Server, *Server, error) {
	reliableAck := false
	if c.Kafka != nil {
		reliableAck = c.ReliableAck
	}

	socketServer := &Server{
		DispatchRules:  producerRules,
		statsCollector: c.MetricCollector,
		reliableAck:    reliableAck,
		logger:         logger,
	}

	mux.HandleFunc("/", socketServer.ServeBinaryWs(c, registry))
	mux.HandleFunc("/status", socketServer.Status())

	server := &http.Server{Addr: fmt.Sprintf("%v:%v", c.Host, c.Port), Handler: serveHTTPWithLogs(mux, logger)}
	return server, socketServer, nil
}

// serveHTTPWithLogs wraps a handler and logs the request
func serveHTTPWithLogs(h http.Handler, logger *logrus.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urlPath := r.URL.Path
		start := time.Now()
		uuidStr := uuid.New().String()
		logger.Infof("request_start uuid: %v, method: %v, path: %v, remote_ip: %v", uuidStr, r.Method, urlPath, r.RemoteAddr)

		h.ServeHTTP(w, r)

		durationMs := int(time.Since(start).Milliseconds())
		logger.Infof("request_end uuid: %v, method: %v, path: %v, remote_ip: %v, duration_ms: %d", uuidStr, r.Method, urlPath, r.RemoteAddr, durationMs)
	})
}

// Status API
func (v *Server) Status() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	}
}

// ServeBinaryWs serves a http query and upgrades it to a websocket -- only serves binary data coming from the ws
func (v *Server) ServeBinaryWs(config *config.Config, registry *SocketRegistry) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if ws := v.promoteToWebsocket(w, r); ws != nil {
			ctx := context.WithValue(context.Background(), SocketContext, map[string]interface{}{"request": r})
			requestIdentity, err := extractIdentityFromConnection(ctx, r)
			if err != nil {
				v.logger.Errorf("extract_sender_id err: %v", err)
			}

			socketManager := NewSocketManager(ctx, requestIdentity, ws, config, v.logger)
			registry.RegisterSocket(socketManager)
			defer registry.DeregisterSocket(socketManager)

			binarySerializer := telemetry.NewBinarySerializer(requestIdentity, v.DispatchRules, v.reliableAck, v.logger)
			socketManager.ProcessTelemetry(binarySerializer)
		}
	}
}

func (v *Server) promoteToWebsocket(w http.ResponseWriter, r *http.Request) *websocket.Conn {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			v.logger.Errorf("websocket_promotion_error err: %v", err)
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
