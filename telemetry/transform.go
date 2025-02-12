package telemetry

import (
	"fmt"
	"github.com/teslamotors/fleet-telemetry/protos"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// DatumTransformer is a function that transforms a Datum in place.
// The second argument is the version of the device that produced the Datum,
// allowing for version-specific transformations.
type DatumTransformer func(*protos.Datum, semver.Version)

// datumTransformations defines DataTransformers to execute on a per-field basis
var datumTransformations = map[protos.Field][]DatumTransformer{
	protos.Field_Location: {transformLocation},
}

// transformLocation does a best-effort attempt to convert the Location field to a proper protos.Location
// type if what we receive is a string that can be parsed. This should make the transition from strings to
// Locations easier to handle downstream.
func transformLocation(d *protos.Datum, _ semver.Version) {
	if strVal := d.GetValue().GetStringValue(); strVal != "" {
		if loc, err := parseLocation(strVal); err == nil {
			d.Value = &protos.Value{Value: &protos.Value_LocationValue{LocationValue: loc}}
		}
	}
}

// parseLocation parses a location string (such as "(37.412374 N, 122.145867 W)") into a *proto.Location type.
func parseLocation(s string) (*protos.LocationValue, error) {
	var lat, lon float64
	var latQ, lonQ string
	count, err := fmt.Sscanf(s, "(%f %1s, %f %1s)", &lat, &latQ, &lon, &lonQ)
	if err != nil {
		return nil, err
	}
	if count != 4 || !strings.Contains("NS", latQ) || !strings.Contains("EW", lonQ) {
		return nil, fmt.Errorf("invalid location format: %s", s)
	}
	if latQ == "S" {
		lat = -lat
	}
	if lonQ == "W" {
		lon = -lon
	}
	return &protos.LocationValue{
		Latitude:  lat,
		Longitude: lon,
	}, nil
}
