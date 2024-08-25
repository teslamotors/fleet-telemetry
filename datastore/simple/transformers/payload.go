package transformers

import (
	"time"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/protos"
)

func PayloadToMap(payload *protos.Payload, includeTypes bool, logger *logrus.Logger) map[string]interface{} {
	convertedPayload := make(map[string]interface{}, len(payload.Data)+2)
	convertedPayload["Vin"] = payload.Vin
	convertedPayload["CreatedAt"] = payload.CreatedAt.AsTime().Format(time.RFC3339)

	for _, datum := range payload.Data {
		if datum == nil || datum.Value == nil {
			logger.ActivityLog("unknown_payload_data_type", logrus.LogInfo{"vin": payload.Vin})
			continue
		}
		name := protos.Field_name[int32(datum.Key.Number())]
		value, ok := transformValue(datum.Value.Value, includeTypes)
		if !ok {
			logger.ActivityLog("unknown_payload_value_data_type", logrus.LogInfo{"name": name, "vin": payload.Vin})
			continue
		}
		convertedPayload[name] = value
	}

	return convertedPayload
}

func transformValue(value interface{}, includeTypes bool) (interface{}, bool) {
	var outputValue interface{}
	var outputType string

	// ordered by expected frequency
	switch v := value.(type) {
	case *protos.Value_StringValue:
		outputType = "stringValue"
		outputValue = v.StringValue
	case *protos.Value_LocationValue:
		outputType = "locationValue"
		outputValue = map[string]float64{
			"latitude":  v.LocationValue.Latitude,
			"longitude": v.LocationValue.Longitude,
		}
	case *protos.Value_FloatValue:
		outputType = "floatValue"
		outputValue = v.FloatValue
	case *protos.Value_IntValue:
		outputType = "intValue"
		outputValue = v.IntValue
	case *protos.Value_DoubleValue:
		outputType = "doubleValue"
		outputValue = v.DoubleValue
	case *protos.Value_LongValue:
		outputType = "longValue"
		outputValue = v.LongValue
	case *protos.Value_BooleanValue:
		outputType = "booleanValue"
		outputValue = v.BooleanValue
	case *protos.Value_Invalid:
		outputType = "invalid"
		outputValue = "<invalid>"
		if includeTypes {
			outputValue = true
		}
	case *protos.Value_ShiftStateValue:
		outputType = "shiftStateValue"
		outputValue = v.ShiftStateValue.String()
	case *protos.Value_ChargingValue:
		outputType = "chargingValue"
		outputValue = v.ChargingValue.String()
	default:
		return nil, false
	}

	if includeTypes {
		return map[string]interface{}{outputType: outputValue}, true
	}

	return outputValue, true
}
