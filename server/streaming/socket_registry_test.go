package streaming_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/config"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter/noop"
	"github.com/teslamotors/fleet-telemetry/server/streaming"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

func newTestSocketManager(deviceID, socketUUID string) *streaming.SocketManager {
	logger, _ := logrus.NoOpLogger()
	conf := &config.Config{MetricCollector: noop.NewCollector()}
	identity := &telemetry.RequestIdentity{
		DeviceID: deviceID,
		SenderID: "vehicle_device." + deviceID,
	}
	sm := streaming.NewSocketManager(context.Background(), identity, nil, conf, logger)
	sm.UUID = socketUUID
	return sm
}

var _ = Describe("SocketRegistry VIN-level connectivity", func() {
	var registry *streaming.SocketRegistry

	BeforeEach(func() {
		registry = streaming.NewSocketRegistry()
	})

	It("emits first-connect / last-disconnect transitions for a single socket", func() {
		socket := newTestSocketManager("VIN1", "conn-a")

		Expect(registry.RegisterSocket(socket)).To(BeTrue())
		Expect(registry.NumConnectedSockets()).To(Equal(1))
		Expect(registry.NumConnectedDevices()).To(Equal(1))

		Expect(registry.DeregisterSocket(socket)).To(BeTrue())
		Expect(registry.NumConnectedSockets()).To(Equal(0))
		Expect(registry.NumConnectedDevices()).To(Equal(0))
	})

	It("does not treat an overlapping second socket as VIN disconnect", func() {
		// Reproduces #244: wifi + cellular (or reconnect overlap) previously
		// published DISCONNECTED when only one of the sockets closed, even
		// though the vehicle was still online and streaming on the other.
		wifi := newTestSocketManager("VIN1", "wifi")
		cellular := newTestSocketManager("VIN1", "cellular")

		Expect(registry.RegisterSocket(wifi)).To(BeTrue(), "first socket should be VIN CONNECTED")
		Expect(registry.RegisterSocket(cellular)).To(BeFalse(), "second socket must not emit another CONNECTED")
		Expect(registry.NumConnectedSockets()).To(Equal(2))
		Expect(registry.NumConnectedDevices()).To(Equal(1))

		Expect(registry.DeregisterSocket(wifi)).To(BeFalse(), "closing one of two sockets must not emit DISCONNECTED")
		Expect(registry.NumConnectedSockets()).To(Equal(1))
		Expect(registry.NumConnectedDevices()).To(Equal(1))
		Expect(registry.GetSocket("cellular")).NotTo(BeNil())

		Expect(registry.DeregisterSocket(cellular)).To(BeTrue(), "last socket should be VIN DISCONNECTED")
		Expect(registry.NumConnectedSockets()).To(Equal(0))
		Expect(registry.NumConnectedDevices()).To(Equal(0))
	})

	It("tracks connectivity independently per VIN", func() {
		a := newTestSocketManager("VIN-A", "a1")
		b := newTestSocketManager("VIN-B", "b1")

		Expect(registry.RegisterSocket(a)).To(BeTrue())
		Expect(registry.RegisterSocket(b)).To(BeTrue())
		Expect(registry.NumConnectedDevices()).To(Equal(2))

		Expect(registry.DeregisterSocket(a)).To(BeTrue())
		Expect(registry.NumConnectedDevices()).To(Equal(1))
		Expect(registry.GetSocket("b1")).NotTo(BeNil())
	})

	It("uniquifies colliding X-TXID values so a live socket is not overwritten", func() {
		first := newTestSocketManager("VIN1", "shared-txid")
		second := newTestSocketManager("VIN1", "shared-txid")

		Expect(registry.RegisterSocket(first)).To(BeTrue())
		Expect(registry.RegisterSocket(second)).To(BeFalse())
		Expect(first.UUID).To(Equal("shared-txid"))
		Expect(second.UUID).NotTo(Equal("shared-txid"))
		Expect(registry.NumConnectedSockets()).To(Equal(2))

		Expect(registry.DeregisterSocket(first)).To(BeFalse())
		Expect(registry.GetSocket(second.UUID)).NotTo(BeNil())
		Expect(registry.DeregisterSocket(second)).To(BeTrue())
	})
})
