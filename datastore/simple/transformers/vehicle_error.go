package transformers

import (
	"github.com/teslamotors/fleet-telemetry/protos"
)

// VehicleErrorToMap converts a VehicleError proto message to a map representation
func VehicleErrorToMap(vehicleError *protos.VehicleError) map[string]interface{} {
	errorMap := map[string]interface{}{
		"Name": vehicleError.Name,
		"Body": vehicleError.Body,
		"Tags": vehicleError.Tags,
	}

	if vehicleError.CreatedAt != nil {
		errorMap["CreatedAt"] = vehicleError.CreatedAt.AsTime().Unix()
	}

	return errorMap
}
