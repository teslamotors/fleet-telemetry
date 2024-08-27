package transformers_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/datastore/simple/transformers"
	"github.com/teslamotors/fleet-telemetry/protos"

	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ = Describe("VehicleAlert", func() {
	Describe("VehicleAlertToMap", func() {
		var (
			alert *protos.VehicleAlert
		)

		BeforeEach(func() {
			alert = &protos.VehicleAlert{
				Name:      "TestAlert",
				StartedAt: timestamppb.New(time.Now().Add(-1 * time.Hour)),
				EndedAt:   timestamppb.New(time.Now()),
				Audiences: []protos.Audience{protos.Audience_Customer, protos.Audience_Service},
			}
		})

		It("includes all expected data", func() {
			result := transformers.VehicleAlertToMap(alert)

			Expect(result).To(HaveLen(4))
			Expect(result["Name"]).To(Equal("TestAlert"))
			Expect(result["StartedAt"]).To(BeNumerically("~", time.Now().Add(-1*time.Hour).Unix(), 1))
			Expect(result["EndedAt"]).To(BeNumerically("~", time.Now().Unix(), 1))
			Expect(result["Audiences"]).To(ConsistOf("Customer", "Service"))
		})

		It("handles missing fields", func() {
			alert.EndedAt = nil
			alert.Audiences = nil
			result := transformers.VehicleAlertToMap(alert)

			Expect(result).To(HaveLen(3))
			Expect(result["Name"]).To(Equal("TestAlert"))
			Expect(result["StartedAt"]).To(BeNumerically("~", time.Now().Add(-1*time.Hour).Unix(), 1))
			Expect(result).NotTo(HaveKey("EndedAt"))
			Expect(result["Audiences"]).To(BeNil())
		})
	})
})
