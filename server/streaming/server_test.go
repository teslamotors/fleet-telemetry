package streaming_test

import (
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
		}

		registry := streaming.NewSocketRegistry()
		req := httptest.NewRequest("GET", "http://test-server.example.com", nil)

		producerRules := make(map[string][]telemetry.Producer)
		_, s, err := streaming.InitServer(conf, airbrake.NewAirbrakeHandler(nil), producerRules, logger, registry)
		Expect(err).NotTo(HaveOccurred())

		srv := httptest.NewServer(http.HandlerFunc(s.ServeBinaryWs(conf, registry)))
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
