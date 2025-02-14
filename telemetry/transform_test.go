package telemetry

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/protos"

	"github.com/Masterminds/semver/v3"
)

var _ = Describe("transformers", func() {
	DescribeTable("transformLocation",
		func(locStr string, expected *protos.LocationValue) {
			d := &protos.Datum{Value: &protos.Value{Value: &protos.Value_StringValue{StringValue: locStr}}}
			transformLocation(d, semver.Version{})
			Expect(d.Value.GetLocationValue()).To(Equal(expected))
		},
		Entry("for a bogus string", "Abhishek is NOT a location", nil),
		Entry("for a broken location", "(37.412374 Q, 122.145867 W)", nil),
		Entry("for a valid loc, NW", "(37.412374 N, 122.145867 W)", &protos.LocationValue{Latitude: 37.412374, Longitude: -122.145867}),
		Entry("for a valid loc, NE", "(37.412374 N, 122.145867 E)", &protos.LocationValue{Latitude: 37.412374, Longitude: 122.145867}),
		Entry("for a valid loc, SW", "(37.412374 S, 122.145867 W)", &protos.LocationValue{Latitude: -37.412374, Longitude: -122.145867}),
		Entry("for a valid loc, SE", "(37.412374 S, 122.145867 E)", &protos.LocationValue{Latitude: -37.412374, Longitude: 122.145867}),
	)
})
