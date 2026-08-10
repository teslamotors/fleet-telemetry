package streaming_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func readSocketMessage(sm *streaming.SocketManager) streaming.SocketMessage {
	var msg streaming.SocketMessage
	done := make(chan struct{})
	go func() {
		msg = sm.ListenToWriteChannel()
		close(done)
	}()
	Eventually(done, time.Second).Should(BeClosed())
	return msg
}

var _ = Describe("Vehicle resync (#246)", func() {
	var (
		registry *streaming.SocketRegistry
		server   *streaming.Server
		logger   *logrus.Logger
	)

	BeforeEach(func() {
		logger, _ = logrus.NoOpLogger()
		registry = streaming.NewSocketRegistry()
		conf := &config.Config{MetricCollector: noop.NewCollector()}
		var err error
		_, server, err = streaming.InitServer(conf, airbrake.NewAirbrakeHandler(nil), map[string][]telemetry.Producer{}, logger, registry)
		Expect(err).NotTo(HaveOccurred())
	})

	newSocket := func(vin, id string) *streaming.SocketManager {
		identity := &telemetry.RequestIdentity{DeviceID: vin, SenderID: "vehicle_device." + vin}
		conf := &config.Config{MetricCollector: noop.NewCollector()}
		sm := streaming.NewSocketManager(context.Background(), identity, nil, conf, logger)
		sm.UUID = id
		registry.RegisterSocket(sm)
		return sm
	}

	It("returns 409 when the VIN has no active socket", func() {
		req := httptest.NewRequest(http.MethodPost, "/resync", bytes.NewBufferString(`{"vin":"MISSING"}`))
		rec := httptest.NewRecorder()
		server.HandleResync().ServeHTTP(rec, req)
		Expect(rec.Code).To(Equal(http.StatusConflict))
	})

	It("rejects missing vin and wrong method", func() {
		req := httptest.NewRequest(http.MethodPost, "/resync", bytes.NewBufferString(`{}`))
		rec := httptest.NewRecorder()
		server.HandleResync().ServeHTTP(rec, req)
		Expect(rec.Code).To(Equal(http.StatusBadRequest))

		req = httptest.NewRequest(http.MethodGet, "/resync", nil)
		rec = httptest.NewRecorder()
		server.HandleResync().ServeHTTP(rec, req)
		Expect(rec.Code).To(Equal(http.StatusMethodNotAllowed))
	})

	It("notifies every active socket for the VIN with a resync control message", func() {
		wifi := newSocket("VIN1", "wifi")
		cellular := newSocket("VIN1", "cellular")
		_ = newSocket("VIN2", "other")

		body := `{"vin":"VIN1","fields":["Gear","ChargeState"]}`
		req := httptest.NewRequest(http.MethodPost, "/resync", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		server.HandleResync().ServeHTTP(rec, req)
		Expect(rec.Code).To(Equal(http.StatusOK))

		var resp streaming.ResyncResponse
		Expect(json.Unmarshal(rec.Body.Bytes(), &resp)).To(Succeed())
		Expect(resp.VIN).To(Equal("VIN1"))
		Expect(resp.SocketsNotified).To(Equal(2))
		Expect(resp.Fields).To(Equal([]string{"Gear", "ChargeState"}))

		for _, sm := range []*streaming.SocketManager{wifi, cellular} {
			msg := readSocketMessage(sm)
			Expect(msg.MsgType).To(Equal(websocket.BinaryMessage))

			streamMsg, err := messages.StreamMessageFromBytes(msg.Msg)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(streamMsg.MessageTopic)).To(Equal("resync"))
			Expect(string(streamMsg.DeviceID)).To(Equal("VIN1"))

			var payload struct {
				Fields []string `json:"fields"`
			}
			Expect(json.Unmarshal(streamMsg.Payload, &payload)).To(Succeed())
			Expect(payload.Fields).To(Equal([]string{"Gear", "ChargeState"}))
		}
	})
})
