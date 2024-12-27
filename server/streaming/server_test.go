package streaming_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter/noop"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gorilla/websocket"

	"github.com/teslamotors/fleet-telemetry/config"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/server/streaming"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var _ = Describe("Extract certificate from header test", func() {

	It("No error", func() {
		logger, hook := logrus.NoOpLogger()
		producerRules := make(map[string][]telemetry.Producer)
		registry := streaming.NewSocketRegistry()
		conf := &config.Config{
			DisableTLS:      true,
			TLS:             nil,
			MetricCollector: noop.NewCollector(),
		}
		_, s, err := streaming.InitServer(conf, airbrake.NewAirbrakeHandler(nil), producerRules, logger, registry)
		Expect(err).NotTo(HaveOccurred())
		srv := httptest.NewServer(http.HandlerFunc(s.ServeBinaryWs(conf)))
		u, err := url.Parse(srv.URL)
		u.Scheme = "ws"

		dialer := &websocket.Dialer{HandshakeTimeout: 1 * time.Second}
		req := httptest.NewRequest("GET", "http://test-server.example.com", nil)

		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		Expect(err).NotTo(HaveOccurred())

		issuerTemplate := x509.Certificate{
			SerialNumber: big.NewInt(1),
			Subject: pkix.Name{
				CommonName:   "Tesla Motors Products CA",
				Organization: []string{"Test Co"},
			},
			NotBefore:             time.Now(),
			NotAfter:              time.Now().Add(24 * time.Hour),
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			BasicConstraintsValid: true,
			IsCA:                  true,
		}

		template := x509.Certificate{
			SerialNumber: big.NewInt(2),
			Subject: pkix.Name{
				CommonName:   "device-1",
				Organization: []string{"Test Co"},
			},
			NotBefore:             time.Now(),
			NotAfter:              time.Now().Add(24 * time.Hour),
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			BasicConstraintsValid: true,
			IsCA:                  false,
		}

		certBytes, err := x509.CreateCertificate(
			rand.Reader,
			&template,
			&issuerTemplate,
			&priv.PublicKey,
			priv,
		)
		Expect(err).NotTo(HaveOccurred())

		var caCertPEM bytes.Buffer
		err = pem.Encode(&caCertPEM, &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certBytes,
		})
		Expect(err).NotTo(HaveOccurred())

		req.Header.Set("Client-Cert-Chain", base64.StdEncoding.EncodeToString(caCertPEM.Bytes()))
		conn, _, err := dialer.Dial(u.String(), req.Header)
		Expect(err).NotTo(HaveOccurred())

		err = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		Expect(err).NotTo(HaveOccurred())

		err = conn.WriteMessage(websocket.BinaryMessage, []byte(""))
		Expect(err).NotTo(HaveOccurred())

		_, _, _ = conn.ReadMessage()
		_ = conn.Close()

		for _, e := range hook.AllEntries() {
			Expect(e.Level).NotTo(Equal(logrus.ERROR))
		}
	})
})

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

	It("ServeBinaryWs test", func() {
		logger, hook := logrus.NoOpLogger()
		conf := &config.Config{
			RateLimit: &config.RateLimit{
				MessageLimit:              1,
				MessageIntervalTimeSecond: 1 * time.Second,
			},
			MetricCollector: noop.NewCollector(),
		}

		registry := streaming.NewSocketRegistry()
		req := httptest.NewRequest("GET", "http://test-server.example.com", nil)

		producerRules := make(map[string][]telemetry.Producer)
		_, s, err := streaming.InitServer(conf, airbrake.NewAirbrakeHandler(nil), producerRules, logger, registry)
		Expect(err).NotTo(HaveOccurred())

		srv := httptest.NewServer(http.HandlerFunc(s.ServeBinaryWs(conf)))
		u, _ := url.Parse(srv.URL)
		u.Scheme = "ws"

		dialer := &websocket.Dialer{HandshakeTimeout: 1 * time.Second}
		conn, _, err := dialer.Dial(u.String(), req.Header)
		Expect(err).NotTo(HaveOccurred())

		err = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		Expect(err).NotTo(HaveOccurred())

		err = conn.WriteMessage(websocket.BinaryMessage, []byte(""))
		Expect(err).NotTo(HaveOccurred())

		_, _, _ = conn.ReadMessage()
		_ = conn.Close()

		Expect(hook.AllEntries()).To(BeEmpty())
	})
})
