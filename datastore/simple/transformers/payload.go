package transformers

import (
	"fmt"
	"time"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/protos"
)

const (
	// SemiModelLetter is the 4th character in VIN representing Tesla semi
	SemiModelLetter = "T"
)

// PayloadToMap transforms a Payload into a human readable map for logging purposes
func PayloadToMap(payload *protos.Payload, includeTypes bool, vin string, logger *logrus.Logger) map[string]interface{} {
	convertedPayload := make(map[string]interface{}, len(payload.Data)+2)
	convertedPayload["Vin"] = payload.Vin
	convertedPayload["CreatedAt"] = payload.CreatedAt.AsTime().Format(time.RFC3339)

	for _, datum := range payload.Data {
		if datum == nil || datum.Value == nil {
			logger.ActivityLog("unknown_payload_data_type", logrus.LogInfo{"vin": payload.Vin})
			continue
		}
		name := protos.Field_name[int32(datum.Key.Number())]
		value, ok := transformValue(datum.Value.Value, includeTypes, vin)
		if !ok {
			logger.ActivityLog("unknown_payload_value_data_type", logrus.LogInfo{"name": name, "vin": payload.Vin})
			continue
		}
		convertedPayload[name] = value
	}

	return convertedPayload
}

func transformValue(value interface{}, includeTypes bool, vin string) (interface{}, bool) {
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
	case *protos.Value_HvacAutoModeValue:
		outputType = "hvacAutoMode"
		outputValue = v.HvacAutoModeValue.String()
	case *protos.Value_CabinOverheatProtectionModeValue:
		outputType = "cabinOverheatProtectionMode"
		outputValue = v.CabinOverheatProtectionModeValue.String()
	case *protos.Value_CabinOverheatProtectionTemperatureLimitValue:
		outputType = "cabinOverheatProtectionTemperatureLimit"
		outputValue = v.CabinOverheatProtectionTemperatureLimitValue.String()
	case *protos.Value_DefrostModeValue:
		outputType = "defrostMode"
		outputValue = v.DefrostModeValue.String()
	case *protos.Value_ClimateKeeperModeValue:
		outputType = "climateKeeperMode"
		outputValue = v.ClimateKeeperModeValue.String()
	case *protos.Value_HvacPowerValue:
		outputType = "hvacPower"
		outputValue = v.HvacPowerValue.String()
	case *protos.Value_TireLocationValue:
		outputType = "tireLocation"
		if getModelFromVIN(vin) == SemiModelLetter {
			outputValue = map[string]bool{
				"FrontLeft":            v.TireLocationValue.FrontLeft,
				"FrontRight":           v.TireLocationValue.FrontRight,
				"SemiMiddleAxleLeft":   v.TireLocationValue.RearLeft,
				"SemiMiddleAxleRight":  v.TireLocationValue.RearRight,
				"SemiMiddleAxleLeft2":  v.TireLocationValue.SemiMiddleAxleLeft_2,
				"SemiMiddleAxleRight2": v.TireLocationValue.SemiMiddleAxleRight_2,
				"SemiRearAxleLeft":     v.TireLocationValue.SemiRearAxleLeft,
				"SemiRearAxleRight":    v.TireLocationValue.SemiRearAxleRight,
				"SemiRearAxleLeft2":    v.TireLocationValue.SemiRearAxleLeft_2,
				"SemiRearAxleRight2":   v.TireLocationValue.SemiRearAxleRight_2,
			}
		} else {
			outputValue = map[string]bool{
				"FrontLeft":  v.TireLocationValue.FrontLeft,
				"FrontRight": v.TireLocationValue.FrontRight,
				"RearLeft":   v.TireLocationValue.RearLeft,
				"RearRight":  v.TireLocationValue.RearRight,
			}
		}
	case *protos.Value_FastChargerValue:
		outputType = "fastChargerType"
		outputValue = v.FastChargerValue.String()
	case *protos.Value_CableTypeValue:
		outputType = "chargingCableType"
		outputValue = v.CableTypeValue.String()
	case *protos.Value_TonneauTentModeValue:
		outputType = "tonneauTentMode"
		outputValue = v.TonneauTentModeValue.String()
	case *protos.Value_TonneauPositionValue:
		outputType = "tonneauPosition"
		outputValue = v.TonneauPositionValue.String()
	case *protos.Value_PowershareStateValue:
		outputType = "powershareStatus"
		outputValue = v.PowershareStateValue.String()
	case *protos.Value_PowershareStopReasonValue:
		outputType = "powershareStopReason"
		outputValue = v.PowershareStopReasonValue.String()
	case *protos.Value_PowershareTypeValue:
		outputType = "powershareType"
		outputValue = v.PowershareTypeValue.String()
	case *protos.Value_DisplayStateValue:
		outputType = "displayState"
		outputValue = v.DisplayStateValue.String()
	default:
		return nil, false
	}

	if includeTypes {
		return map[string]interface{}{outputType: outputValue}, true
	}

	return outputValue, true
}

func getModelFromVIN(vin string) string {
	return string(vin[3])
}
