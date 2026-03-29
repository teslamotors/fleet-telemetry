package streaming_test

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gorilla/websocket"

	"github.com/teslamotors/fleet-telemetry/config"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/messages"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter/noop"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/server/streaming"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// withTLSState wraps a handler to inject a tls.ConnectionState into each request,
// simulating an mTLS connection for testing without actual TLS transport.
func withTLSState(handler http.Handler, tlsState *tls.ConnectionState) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.TLS = tlsState
		handler.ServeHTTP(w, r)
	})
}

func makeCert(commonName, issuerCommonName string) *x509.Certificate {
	return &x509.Certificate{
		Subject: pkix.Name{CommonName: commonName},
		Issuer:  pkix.Name{CommonName: issuerCommonName},
	}
}

type spyProducer struct {
	captured chan *telemetry.Record
}

func (p *spyProducer) Close() error                                    { return nil }
func (p *spyProducer) Produce(entry *telemetry.Record)                 { p.captured <- entry }
func (p *spyProducer) ProcessReliableAck(_ *telemetry.Record)          {}
func (p *spyProducer) ReportError(_ string, _ error, _ logrus.LogInfo) {}

var _ = Describe("Socket handler test", func() {

	var producerRules map[string][]telemetry.Producer

	AfterEach(func() {
		type Closer interface {
			Close() error
		}

		for _, typeProducers := range producerRules {
			for _, producer := range typeProducers {
				if closer, ok := producer.(Closer); ok {
					err := closer.Close()
					Expect(err).NotTo(HaveOccurred())
				}
			}
		}
	})

	It("ServeBinaryWs closes connection when TLS state is missing", func() {
		logger, _ := logrus.NoOpLogger()
		conf := &config.Config{
			RateLimit: &config.RateLimit{
				MessageLimit:              1,
				MessageIntervalTimeSecond: 1 * time.Second,
			},
			MetricCollector: noop.NewCollector(),
		}

		registry := streaming.NewSocketRegistry()
		producerRules = make(map[string][]telemetry.Producer)
		_, s, err := streaming.InitServer(conf, airbrake.NewAirbrakeHandler(nil), producerRules, logger, registry)
		Expect(err).NotTo(HaveOccurred())

		// No TLS state injected, simulates a non-mTLS connection
		srv := httptest.NewServer(http.HandlerFunc(s.ServeBinaryWs(conf)))
		defer srv.Close()
		u, _ := url.Parse(srv.URL)
		u.Scheme = "ws"

		dialer := &websocket.Dialer{HandshakeTimeout: 1 * time.Second}
		conn, _, err := dialer.Dial(u.String(), nil)
		Expect(err).NotTo(HaveOccurred())

		// Server should close the connection due to missing TLS state
		err = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		Expect(err).NotTo(HaveOccurred())
		_, _, err = conn.ReadMessage()
		Expect(err).To(HaveOccurred())
		_ = conn.Close()
	})

	It("ServeBinaryWs closes connection when VerifiedChains is empty", func() {
		logger, _ := logrus.NoOpLogger()
		conf := &config.Config{
			RateLimit: &config.RateLimit{
				MessageLimit:              1,
				MessageIntervalTimeSecond: 1 * time.Second,
			},
			MetricCollector: noop.NewCollector(),
		}

		registry := streaming.NewSocketRegistry()
		producerRules = make(map[string][]telemetry.Producer)
		_, s, err := streaming.InitServer(conf, airbrake.NewAirbrakeHandler(nil), producerRules, logger, registry)
		Expect(err).NotTo(HaveOccurred())

		// TLS state with PeerCertificates but no VerifiedChains
		tlsState := &tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{
				makeCert("device-1", "TeslaMotors"),
			},
			VerifiedChains: nil,
		}
		srv := httptest.NewServer(withTLSState(http.HandlerFunc(s.ServeBinaryWs(conf)), tlsState))
		defer srv.Close()
		u, _ := url.Parse(srv.URL)
		u.Scheme = "ws"

		dialer := &websocket.Dialer{HandshakeTimeout: 1 * time.Second}
		conn, _, err := dialer.Dial(u.String(), nil)
		Expect(err).NotTo(HaveOccurred())

		// Server should close the connection due to empty VerifiedChains
		err = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		Expect(err).NotTo(HaveOccurred())
		_, _, err = conn.ReadMessage()
		Expect(err).To(HaveOccurred())
		_ = conn.Close()
	})

	It("ServeBinaryWs uses verified leaf cert, not last PeerCertificate", func() {
		logger, _ := logrus.NoOpLogger()

		spy := &spyProducer{captured: make(chan *telemetry.Record, 1)}

		conf := &config.Config{
			RateLimit: &config.RateLimit{
				MessageLimit:              1,
				MessageIntervalTimeSecond: 1 * time.Second,
			},
			MetricCollector: noop.NewCollector(),
		}

		registry := streaming.NewSocketRegistry()
		producerRules = map[string][]telemetry.Producer{"V": {spy}}
		_, s, err := streaming.InitServer(conf, airbrake.NewAirbrakeHandler(nil), producerRules, logger, registry)
		Expect(err).NotTo(HaveOccurred())

		realCert := makeCert("device-1", "TeslaMotors")
		spoofedCert := makeCert("SPOOFED-VIN", "TeslaMotors")

		tlsState := &tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{realCert, spoofedCert},
			VerifiedChains:   [][]*x509.Certificate{{realCert}},
		}
		srv := httptest.NewServer(withTLSState(http.HandlerFunc(s.ServeBinaryWs(conf)), tlsState))
		defer srv.Close()
		u, _ := url.Parse(srv.URL)
		u.Scheme = "ws"

		dialer := &websocket.Dialer{HandshakeTimeout: 1 * time.Second}
		conn, _, err := dialer.Dial(u.String(), nil)
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = conn.Close() }()

		streamMsg := messages.StreamMessage{
			TXID:         []byte("test-txid"),
			SenderID:     []byte("vehicle_device.blah"),
			DeviceID:     []byte("blah"),
			DeviceType:   []byte("vehicle_device"),
			MessageTopic: []byte("V"),
			Payload:      []byte{},
		}
		msgBytes, err := streamMsg.ToBytes()
		Expect(err).NotTo(HaveOccurred())

		err = conn.WriteMessage(websocket.BinaryMessage, msgBytes)
		Expect(err).NotTo(HaveOccurred())

		// The record VIN must come from the verified cert ("device-1")
		var record *telemetry.Record
		Eventually(spy.captured).Should(Receive(&record))
		Expect(record.Vin).To(Equal("device-1"))
	})
})
