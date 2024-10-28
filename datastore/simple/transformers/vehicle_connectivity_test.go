package transformers_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/datastore/simple/transformers"
	"github.com/teslamotors/fleet-telemetry/protos"

	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ = Describe("VehicleConnectivity", func() {
	Describe("VehicleConnectivityToMap", func() {
		var (
			connectivity *protos.VehicleConnectivity
		)

		BeforeEach(func() {
			connectivity = &protos.VehicleConnectivity{
				Vin:          "Vin1",
				ConnectionId: "connection1",
				CreatedAt:    timestamppb.New(time.Now()),
				Status:       protos.ConnectivityEvent_CONNECTED,
			}
		})

		It("includes all expected data", func() {
			result := transformers.VehicleConnectivityToMap(connectivity)
			Expect(result).To(HaveLen(4))
			Expect(result["Vin"]).To(Equal("Vin1"))
			Expect(result["ConnectionID"]).To(Equal("connection1"))
			Expect(result["CreatedAt"]).To(BeNumerically("~", time.Now().Unix(), 1))
			Expect(result["Status"]).To(Equal("CONNECTED"))
		})

	})
})
