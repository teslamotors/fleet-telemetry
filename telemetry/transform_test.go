package telemetry

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/protos"

	"github.com/Masterminds/semver/v3"
)

var _ = Describe("transformers", func() {
	var daylightSavingsOffset int
	pacificTimeLoc, err := time.LoadLocation("America/Los_Angeles")
	Expect(err).NotTo(HaveOccurred())
	_, offset := time.Now().In(pacificTimeLoc).Zone()
	if offset != -8*60*60 {
		// during daylight savings, the offset is -7 hours
		daylightSavingsOffset = 60 * 60
	}

	DescribeTable("transformTimestamp",
		func(timestamp float64, expected float64) {
			d := &protos.Datum{Value: &protos.Value{Value: &protos.Value_DoubleValue{DoubleValue: timestamp}}}
			transformTimestamp(d, semver.Version{})
			Expect(d.Value.GetDoubleValue()).To(Equal(expected))
		},
		Entry("zero timestamp", 0.0, 0.0),
		Entry("regular timestamp", 1736756100.0, float64(1736727300.0+daylightSavingsOffset)),
	)

	DescribeTable("transformLocation",
		func(locStr string, expected *protos.LocationValue, errRegex string) {
			d := &protos.Datum{Value: &protos.Value{Value: &protos.Value_StringValue{StringValue: locStr}}}
			transformLocation(d, semver.Version{})
			Expect(d.Value.GetLocationValue()).To(Equal(expected))
		},
		Entry("for a bogus string", "Abhishek is NOT a location", nil, "input does not match format"),
		Entry("for a broken location", "(37.412374 Q, 122.145867 W)", nil, "invalid location format.*"),
		Entry("for a valid loc, NW", "(37.412374 N, 122.145867 W)", &protos.LocationValue{Latitude: 37.412374, Longitude: -122.145867}, ""),
		Entry("for a valid loc, NE", "(37.412374 N, 122.145867 E)", &protos.LocationValue{Latitude: 37.412374, Longitude: 122.145867}, ""),
		Entry("for a valid loc, SW", "(37.412374 S, 122.145867 W)", &protos.LocationValue{Latitude: -37.412374, Longitude: -122.145867}, ""),
		Entry("for a valid loc, SE", "(37.412374 S, 122.145867 E)", &protos.LocationValue{Latitude: -37.412374, Longitude: 122.145867}, ""),
	)
})
