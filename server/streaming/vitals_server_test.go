package streaming_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus/hooks/test"

	"github.com/teslamotors/fleet-telemetry/config"
	"github.com/teslamotors/fleet-telemetry/server/streaming"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

var _ = Describe("Socket handler test", func() {

	It("ServeBinaryWs test", func() {
		logger, hook := test.NewNullLogger()
		conf := &config.Config{
			RateLimit: &config.RateLimit{
				MessageLimit:              1,
				MessageIntervalTimeSecond: 1 * time.Second,
			},
		}

		registry := streaming.NewSocketRegistry()
		req := httptest.NewRequest("GET", "http://tel.vn.tesla.com", nil)

		producerRules := make(map[string][]telemetry.Producer)
		mux := http.NewServeMux()
		_, s, err := streaming.InitVitalsServer(conf, mux, producerRules, logger, registry)
		Expect(err).To(BeNil())

		srv := httptest.NewServer(http.HandlerFunc(s.ServeBinaryWs(conf, registry)))
		u, _ := url.Parse(srv.URL)
		u.Scheme = "ws"

		dialer := &websocket.Dialer{HandshakeTimeout: 1 * time.Second}
		conn, _, err := dialer.Dial(u.String(), req.Header)
		Expect(err).To(BeNil())

		err = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		Expect(err).To(BeNil())

		err = conn.WriteMessage(websocket.BinaryMessage, []byte(""))
		Expect(err).To(BeNil())

		_, _, _ = conn.ReadMessage()
		_ = conn.Close()

		Expect(hook.AllEntries()).To(BeEmpty())
	})
})
