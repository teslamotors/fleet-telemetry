package transformers_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/datastore/simple/transformers"
	"github.com/teslamotors/fleet-telemetry/protos"

	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ = Describe("VehicleError", func() {
	Describe("VehicleErrorToMap", func() {
		var (
			vehicleError *protos.VehicleError
		)

		BeforeEach(func() {
			vehicleError = &protos.VehicleError{
				Name:      "TestError",
				CreatedAt: timestamppb.New(time.Now()),
				Tags:      map[string]string{"tag1": "value1", "tag2": "value2"},
				Body:      "Error details",
			}
		})

		It("includes all expected data", func() {
			result := transformers.VehicleErrorToMap(vehicleError)

			Expect(result).To(HaveLen(4))
			Expect(result["Name"]).To(Equal("TestError"))
			Expect(result["CreatedAt"]).To(BeNumerically("~", time.Now().Unix(), 1))
			Expect(result["Tags"]).To(HaveKeyWithValue("tag1", "value1"))
			Expect(result["Tags"]).To(HaveKeyWithValue("tag2", "value2"))
			Expect(result["Body"]).To(Equal("Error details"))
		})

		It("handles missing fields", func() {
			vehicleError.Tags = nil
			vehicleError.Body = ""

			result := transformers.VehicleErrorToMap(vehicleError)

			Expect(result).To(HaveLen(4))
			Expect(result["Name"]).To(Equal("TestError"))
			Expect(result["CreatedAt"]).To(BeNumerically("~", time.Now().Unix(), 1))
			Expect(result["Body"]).To(Equal(""))
			Expect(result["Tags"]).To(BeEmpty())
		})
	})
})
