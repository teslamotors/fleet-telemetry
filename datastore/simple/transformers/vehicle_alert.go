package transformers

import (
	"github.com/teslamotors/fleet-telemetry/protos"
)

// VehicleAlertToMap converts a VehicleAlert proto message to a map representation
func VehicleAlertToMap(alert *protos.VehicleAlert) map[string]interface{} {
	alertMap := map[string]interface{}{
		"Name": alert.Name,
	}

	if alert.StartedAt != nil {
		alertMap["StartedAt"] = alert.StartedAt.AsTime().Unix()
	}

	if alert.EndedAt != nil {
		alertMap["EndedAt"] = alert.EndedAt.AsTime().Unix()
	}

	if alert.Audiences == nil {
		alertMap["Audiences"] = nil
		return alertMap
	}

	audiences := make([]interface{}, len(alert.Audiences))
	for i, audience := range alert.Audiences {
		audiences[i] = audience.String()
	}
	alertMap["Audiences"] = audiences

	return alertMap
}
