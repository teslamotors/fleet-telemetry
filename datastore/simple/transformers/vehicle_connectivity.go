package transformers

import (
	"github.com/teslamotors/fleet-telemetry/protos"
)

// VehicleConnectivityToMap converts a VehicleConnectivity proto message to a map representation
func VehicleConnectivityToMap(vehicleConnectivity *protos.VehicleConnectivity) map[string]interface{} {
	return map[string]interface{}{
		"Vin":              vehicleConnectivity.GetVin(),
		"ConnectionID":     vehicleConnectivity.GetConnectionId(),
		"NetworkInterface": vehicleConnectivity.GetNetworkInterface(),
		"Status":           vehicleConnectivity.GetStatus().String(),
		"CreatedAt":        vehicleConnectivity.CreatedAt.AsTime().Unix(),
	}
}
