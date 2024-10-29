package transformers

import (
	"fmt"
	"time"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/protos"
)

// PayloadToMap transforms a Payload into a human readable map for logging purposes
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
	case *protos.Value_LaneAssistLevelValue:
		outputType = "laneAssistLevel"
		outputValue = v.LaneAssistLevelValue.String()
	case *protos.Value_ScheduledChargingModeValue:
		outputType = "scheduledChargingMode"
		outputValue = v.ScheduledChargingModeValue.String()
	case *protos.Value_SentryModeStateValue:
		outputType = "sentryModeState"
		outputValue = v.SentryModeStateValue.String()
	case *protos.Value_SpeedAssistLevelValue:
		outputType = "speedAssistLevel"
		outputValue = v.SpeedAssistLevelValue.String()
	case *protos.Value_BmsStateValue:
		outputType = "bmsState"
		outputValue = v.BmsStateValue.String()
	case *protos.Value_BuckleStatusValue:
		outputType = "buckleStatus"
		outputValue = v.BuckleStatusValue.String()
	case *protos.Value_CarTypeValue:
		outputType = "carType"
		outputValue = v.CarTypeValue.String()
	case *protos.Value_ChargePortValue:
		outputType = "chargePort"
		outputValue = v.ChargePortValue.String()
	case *protos.Value_ChargePortLatchValue:
		outputType = "chargePortLatch"
		outputValue = v.ChargePortLatchValue.String()
	case *protos.Value_CruiseStateValue:
		outputType = "cruiseState"
		outputValue = v.CruiseStateValue.String()
	case *protos.Value_DoorValue:
		outputType = "doorValue"
		outputValue = map[string]bool{
			"DriverFront":    v.DoorValue.DriverFront,
			"PassengerFront": v.DoorValue.PassengerFront,
			"DriverRear":     v.DoorValue.DriverRear,
			"PassengerRear":  v.DoorValue.PassengerRear,
			"TrunkFront":     v.DoorValue.TrunkFront,
			"TrunkRear":      v.DoorValue.TrunkRear,
		}
	case *protos.Value_DriveInverterStateValue:
		outputType = "driveInverterState"
		outputValue = v.DriveInverterStateValue.String()
	case *protos.Value_HvilStatusValue:
		outputType = "hvilStatus"
		outputValue = v.HvilStatusValue.String()
	case *protos.Value_WindowStateValue:
		outputType = "windowState"
		outputValue = v.WindowStateValue.String()
	case *protos.Value_SeatFoldPositionValue:
		outputType = "seatFoldPosition"
		outputValue = v.SeatFoldPositionValue.String()
	case *protos.Value_TractorAirStatusValue:
		outputType = "tractorAirStatus"
		outputValue = v.TractorAirStatusValue.String()
	case *protos.Value_FollowDistanceValue:
		outputType = "followDistance"
		outputValue = v.FollowDistanceValue.String()
	case *protos.Value_ForwardCollisionSensitivityValue:
		outputType = "forwardCollisionSensitivity"
		outputValue = v.ForwardCollisionSensitivityValue.String()
	case *protos.Value_GuestModeMobileAccessValue:
		outputType = "guestModeMobileAccess"
		outputValue = v.GuestModeMobileAccessValue.String()
	case *protos.Value_TrailerAirStatusValue:
		outputType = "trailerAirStatus"
		outputValue = v.TrailerAirStatusValue.String()
	case *protos.Value_TimeValue:
		outputType = "time"
		outputValue = fmt.Sprintf("%02d:%02d:%02d", v.TimeValue.Hour, v.TimeValue.Minute, v.TimeValue.Second)
	case *protos.Value_DetailedChargeStateValue:
		outputType = "detailedChargeState"
		outputValue = v.DetailedChargeStateValue.String()
	default:
		return nil, false
	}

	if includeTypes {
		return map[string]interface{}{outputType: outputValue}, true
	}

	return outputValue, true
}
