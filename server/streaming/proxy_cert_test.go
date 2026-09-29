package streaming_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
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
	"github.com/teslamotors/fleet-telemetry/metrics/adapter/noop"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/server/streaming"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

// signedLeafCert returns a leaf certificate signed by a CA whose common name is a known
// Tesla issuer, so identity extraction resolves the way it would for a real vehicle.
func signedLeafCert(deviceCommonName, issuerCommonName string) (leafDER []byte, chainPEM []byte) {
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: issuerCommonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	Expect(err).NotTo(HaveOccurred())
	caCert, err := x509.ParseCertificate(caDER)
	Expect(err).NotTo(HaveOccurred())

	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: deviceCommonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	leafDER, err = x509.CreateCertificate(rand.Reader, leafTemplate, caCert, &leafKey.PublicKey, caKey)
	Expect(err).NotTo(HaveOccurred())

	// Leaf first, then CA, matching the order an ALB forwards the chain in.
	chainPEM = append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})...)
	return leafDER, chainPEM
}

var _ = Describe("Trusted proxy client certificate", func() {
	var (
		producerRules map[string][]telemetry.Producer
		leafDER       []byte
		chainPEM      []byte
	)

	BeforeEach(func() {
		producerRules = make(map[string][]telemetry.Producer)
		leafDER, chainPEM = signedLeafCert("device-1", "TeslaMotors")
	})

	// dial connects to a server built with the given config and request headers. The
	// server and connection stay alive until the end of the spec so assertions can settle;
	// identity extraction happens after the websocket upgrade, so it must be polled rather
	// than read once.
	type dialResult struct {
		registry       *streaming.SocketRegistry
		identityFailed func() bool
		dialErr        error
		status         int
	}

	dial := func(conf *config.Config, headers http.Header, tlsState *tls.ConnectionState) dialResult {
		logger, hook := logrus.NoOpLogger()
		registry := streaming.NewSocketRegistry()
		_, s, err := streaming.InitServer(conf, airbrake.NewAirbrakeHandler(nil), producerRules, logger, registry)
		Expect(err).NotTo(HaveOccurred())

		handler := http.Handler(http.HandlerFunc(s.ServeBinaryWs(conf)))
		if tlsState != nil {
			handler = withTLSState(handler, tlsState)
		}
		srv := httptest.NewServer(handler)
		DeferCleanup(srv.Close)

		u, _ := url.Parse(srv.URL)
		u.Scheme = "ws"
		dialer := &websocket.Dialer{HandshakeTimeout: 2 * time.Second}
		conn, resp, dialErr := dialer.Dial(u.String(), headers)
		if conn != nil {
			DeferCleanup(func() { _ = conn.Close() })
		}

		status := 0
		if resp != nil {
			status = resp.StatusCode
		}

		return dialResult{
			registry: registry,
			identityFailed: func() bool {
				for _, entry := range hook.AllEntries() {
					if entry.Message == "extract_sender_id_err" {
						return true
					}
				}
				return false
			},
			dialErr: dialErr,
			status:  status,
		}
	}

	// expectAccepted asserts the vehicle was identified and its socket registered.
	expectAccepted := func(result dialResult) {
		Expect(result.dialErr).NotTo(HaveOccurred())
		Eventually(result.registry.NumConnectedSockets, 2*time.Second, 10*time.Millisecond).Should(Equal(1))
		Expect(result.identityFailed()).To(BeFalse())
	}

	// expectRejected asserts identity extraction failed and no socket was registered.
	expectRejected := func(result dialResult) {
		Expect(result.dialErr).NotTo(HaveOccurred())
		Eventually(result.identityFailed, 2*time.Second, 10*time.Millisecond).Should(BeTrue())
		Expect(result.registry.NumConnectedSockets()).To(Equal(0))
	}

	baseConfig := func(mode *config.TLSPassThrough) *config.Config {
		return &config.Config{
			RateLimit:       &config.RateLimit{MessageLimit: 1, MessageIntervalTimeSecond: time.Second},
			MetricCollector: noop.NewCollector(),
			TLSPassThrough:  mode,
		}
	}

	Describe("RFC 9440", func() {
		mode := config.RFC9440

		It("extracts the certificate from the Client-Cert header", func() {
			headers := http.Header{}
			// RFC 8941 Byte Sequence: base64 DER delimited by colons.
			headers.Set("Client-Cert", ":"+base64.StdEncoding.EncodeToString(leafDER)+":")

			expectAccepted(dial(baseConfig(&mode), headers, nil))
		})

		It("tolerates a proxy that omits the byte sequence delimiters", func() {
			headers := http.Header{}
			headers.Set("Client-Cert", base64.StdEncoding.EncodeToString(leafDER))

			expectAccepted(dial(baseConfig(&mode), headers, nil))
		})

		It("does not identify the vehicle from Client-Cert-Chain", func() {
			// Client-Cert-Chain excludes the end-entity certificate (RFC 9440 section 2.2),
			// so it must never be used as the client identity.
			headers := http.Header{}
			headers.Set("Client-Cert-Chain", ":"+base64.StdEncoding.EncodeToString(leafDER)+":")

			expectRejected(dial(baseConfig(&mode), headers, nil))
		})

		It("fails when the header is absent", func() {
			expectRejected(dial(baseConfig(&mode), http.Header{}, nil))
		})

		It("fails when the header is not valid base64", func() {
			headers := http.Header{}
			headers.Set("Client-Cert", ":not-base64!:")

			expectRejected(dial(baseConfig(&mode), headers, nil))
		})

		It("fails when the header is PEM rather than base64 DER", func() {
			headers := http.Header{}
			headers.Set("Client-Cert", string(chainPEM))

			expectRejected(dial(baseConfig(&mode), headers, nil))
		})
	})

	Describe("AWS ALB", func() {
		mode := config.AWSApplicationLoadBalancer

		It("extracts the leaf from the url encoded PEM chain", func() {
			headers := http.Header{}
			headers.Set("X-Amzn-Mtls-Clientcert", url.QueryEscape(string(chainPEM)))

			expectAccepted(dial(baseConfig(&mode), headers, nil))
		})

		It("fails when the header is absent", func() {
			expectRejected(dial(baseConfig(&mode), http.Header{}, nil))
		})

		It("fails when the header is not a PEM block", func() {
			headers := http.Header{}
			headers.Set("X-Amzn-Mtls-Clientcert", url.QueryEscape("not a certificate"))

			expectRejected(dial(baseConfig(&mode), headers, nil))
		})
	})

	Describe("mixing pass through with a TLS client certificate", func() {
		It("rejects the connection before upgrading it", func() {
			mode := config.RFC9440
			leaf, err := x509.ParseCertificate(leafDER)
			Expect(err).NotTo(HaveOccurred())

			headers := http.Header{}
			headers.Set("Client-Cert", ":"+base64.StdEncoding.EncodeToString(leafDER)+":")
			tlsState := &tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{leaf},
				VerifiedChains:   [][]*x509.Certificate{{leaf}},
			}

			result := dial(baseConfig(&mode), headers, tlsState)
			Expect(result.dialErr).To(HaveOccurred())
			Expect(result.status).To(Equal(http.StatusBadRequest))
			Expect(result.registry.NumConnectedSockets()).To(Equal(0))
		})

		It("allows a TLS client certificate when pass through is disabled", func() {
			leaf, err := x509.ParseCertificate(leafDER)
			Expect(err).NotTo(HaveOccurred())
			tlsState := &tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{leaf},
				VerifiedChains:   [][]*x509.Certificate{{leaf}},
			}

			expectAccepted(dial(baseConfig(nil), http.Header{}, tlsState))
		})

		It("ignores proxy headers entirely when pass through is disabled", func() {
			// Without the setting, a forged header must not be able to assert an identity.
			headers := http.Header{}
			headers.Set("Client-Cert", ":"+base64.StdEncoding.EncodeToString(leafDER)+":")

			expectRejected(dial(baseConfig(nil), headers, nil))
		})
	})
})
