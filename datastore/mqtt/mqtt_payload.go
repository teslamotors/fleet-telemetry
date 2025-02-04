package mqtt

import (
	"encoding/json"
	"fmt"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/teslamotors/fleet-telemetry/protos"
	"github.com/teslamotors/fleet-telemetry/telemetry"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func (p *Producer) processVehicleFields(rec *telemetry.Record, payload *protos.Payload) ([]pahomqtt.Token, error) {
	var tokens []pahomqtt.Token
	convertedPayload := p.payloadToMap(payload)
	for key, value := range convertedPayload {
		mqttTopicName := fmt.Sprintf("%s/%s/v/%s", p.config.TopicBase, rec.Vin, key)
		jsonValue, err := json.Marshal(value)
		if err != nil {
			return tokens, fmt.Errorf("failed to marshal JSON for MQTT topic %s: %v", mqttTopicName, err)
		}
		token := p.client.Publish(mqttTopicName, p.config.QoS, p.config.Retained, jsonValue)
		tokens = append(tokens, token)
		p.updateMetrics(rec.TxType, len(jsonValue))
	}
	return tokens, nil
}

func (p *Producer) processVehicleAlerts(rec *telemetry.Record, payload *protos.VehicleAlerts) ([]pahomqtt.Token, error) {
	tokens := make([]pahomqtt.Token, 0, len(payload.Alerts)*2)
	alertsHistory := make(map[string][]*protos.VehicleAlert, len(payload.Alerts))
	alertsCurrentState := make(map[string]*protos.VehicleAlert, len(payload.Alerts))

	// Gather the history and the most current state for each alert name
	for _, alert := range payload.Alerts {
		// Gather alerts history per alert name
		alertsHistory[alert.Name] = append(alertsHistory[alert.Name], alert)

		// Alerts without a start time can not be current.
		if alert.StartedAt != nil {
			// Check if the alert is the most recent we have seen up to this point
			if mostCurrentAlert, exists := alertsCurrentState[alert.Name]; !exists || alert.StartedAt.AsTime().After(mostCurrentAlert.StartedAt.AsTime()) {
				alertsCurrentState[alert.Name] = alert
			}
		}
	}

	// Publish current state for each alert name
	for _, alert := range alertsCurrentState {
		topicName := fmt.Sprintf("%s/%s/alerts/%s/current", p.config.TopicBase, rec.Vin, alert.Name)
		alertMap := vehicleAlertToMqttMap(alert)
		jsonValue, err := json.Marshal(alertMap)
		if err != nil {
			return tokens, fmt.Errorf("failed to marshal JSON for MQTT topic %s: %v", topicName, err)
		}

		token := p.client.Publish(topicName, p.config.QoS, p.config.Retained, jsonValue)
		tokens = append(tokens, token)
		p.updateMetrics(rec.TxType, len(jsonValue))
	}

	// Publish historic states for each alert name
	for alertName, alerts := range alertsHistory {
		topicName := fmt.Sprintf("%s/%s/alerts/%s/history", p.config.TopicBase, rec.Vin, alertName)
		alertMaps := make([]map[string]interface{}, len(alerts))
		for i, alert := range alerts {
			alertMaps[i] = vehicleAlertToMqttMap(alert)
		}

		jsonArray, err := json.Marshal(alertMaps)
		if err != nil {
			return tokens, fmt.Errorf("failed to marshal JSON for MQTT topic %s: %v", topicName, err)
		}
		token := p.client.Publish(topicName, p.config.QoS, p.config.Retained, jsonArray)
		tokens = append(tokens, token)
		p.updateMetrics(rec.TxType, len(jsonArray))
	}

	return tokens, nil
}

func (p *Producer) processVehicleConnectivity(rec *telemetry.Record, payload *protos.VehicleConnectivity) ([]pahomqtt.Token, error) {
	topicName := fmt.Sprintf("%s/%s/connectivity", p.config.TopicBase, rec.Vin)
	value := map[string]interface{}{
		"ConnectionId": payload.GetConnectionId(),
		"Status":       payload.GetStatus().String(),
		"CreatedAt":    payload.GetCreatedAt().AsTime().Format(time.RFC3339),
	}
	jsonValue, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON for MQTT topic %s: %v", topicName, err)
	}
	return []pahomqtt.Token{p.client.Publish(topicName, p.config.QoS, p.config.Retained, jsonValue)}, nil
}

func vehicleAlertToMqttMap(alert *protos.VehicleAlert) map[string]interface{} {
	alertMap := make(map[string]interface{}, 3)
	if alert.StartedAt != nil {
		alertMap["StartedAt"] = alert.StartedAt.AsTime().Format(time.RFC3339)
	}

	if alert.EndedAt != nil {
		alertMap["EndedAt"] = alert.EndedAt.AsTime().Format(time.RFC3339)
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

// PayloadToMap transforms a Payload into a map for mqtt purposes
func (p *Producer) payloadToMap(payload *protos.Payload) map[string]interface{} {
	convertedPayload := make(map[string]interface{}, len(payload.Data))
	for _, datum := range payload.Data {
		convertedPayload[datum.Key.String()] = getDatumValue(datum.Value)
	}
	return convertedPayload
}

func getDatumValue(value *protos.Value) interface{} {
	// ordered by expected frequency (see payload.go transformValue)
	switch v := value.Value.(type) {
	case *protos.Value_StringValue:
		return v.StringValue
	case *protos.Value_LocationValue:
		return map[string]float64{
			"latitude":  v.LocationValue.Latitude,
			"longitude": v.LocationValue.Longitude,
		}
	case *protos.Value_FloatValue:
		return v.FloatValue
	case *protos.Value_IntValue:
		return v.IntValue
	case *protos.Value_DoubleValue:
		return v.DoubleValue
	case *protos.Value_LongValue:
		return v.LongValue
	case *protos.Value_BooleanValue:
		return v.BooleanValue
	case *protos.Value_Invalid:
		return nil
	case *protos.Value_ShiftStateValue:
		return v.ShiftStateValue.String()
	case *protos.Value_LaneAssistLevelValue:
		return v.LaneAssistLevelValue.String()
	case *protos.Value_ScheduledChargingModeValue:
		return v.ScheduledChargingModeValue.String()
	case *protos.Value_SentryModeStateValue:
		return v.SentryModeStateValue.String()
	case *protos.Value_SpeedAssistLevelValue:
		return v.SpeedAssistLevelValue.String()
	case *protos.Value_BmsStateValue:
		return v.BmsStateValue.String()
	case *protos.Value_BuckleStatusValue:
		return v.BuckleStatusValue.String()
	case *protos.Value_CarTypeValue:
		return v.CarTypeValue.String()
	case *protos.Value_ChargePortValue:
		return v.ChargePortValue.String()
	case *protos.Value_ChargePortLatchValue:
		return v.ChargePortLatchValue.String()
	case *protos.Value_DoorValue:
		return map[string]bool{
			"DriverFront":    v.DoorValue.DriverFront,
			"PassengerFront": v.DoorValue.PassengerFront,
			"DriverRear":     v.DoorValue.DriverRear,
			"PassengerRear":  v.DoorValue.PassengerRear,
			"TrunkFront":     v.DoorValue.TrunkFront,
			"TrunkRear":      v.DoorValue.TrunkRear,
		}
	case *protos.Value_DriveInverterStateValue:
		return v.DriveInverterStateValue.String()
	case *protos.Value_HvilStatusValue:
		return v.HvilStatusValue.String()
	case *protos.Value_WindowStateValue:
		return v.WindowStateValue.String()
	case *protos.Value_SeatFoldPositionValue:
		return v.SeatFoldPositionValue.String()
	case *protos.Value_TractorAirStatusValue:
		return v.TractorAirStatusValue.String()
	case *protos.Value_FollowDistanceValue:
		return v.FollowDistanceValue.String()
	case *protos.Value_ForwardCollisionSensitivityValue:
		return v.ForwardCollisionSensitivityValue.String()
	case *protos.Value_GuestModeMobileAccessValue:
		return v.GuestModeMobileAccessValue.String()
	case *protos.Value_TrailerAirStatusValue:
		return v.TrailerAirStatusValue.String()
	case *protos.Value_TimeValue:
		return fmt.Sprintf("%02d:%02d:%02d", v.TimeValue.Hour, v.TimeValue.Minute, v.TimeValue.Second)
	case *protos.Value_DetailedChargeStateValue:
		return v.DetailedChargeStateValue.String()
	default:
		// Any other value will be processed using the generic getProtoValue function
		return getProtoValue(value, true)
	}
}

// getProtoValue returns the value as an interface{} for consumption in a mqtt payload
func getProtoValue(protoMsg protoreflect.ProtoMessage, returnAsToplevel bool) interface{} {
	if protoMsg == nil || !protoMsg.ProtoReflect().IsValid() {
		return nil
	}

	m := protoMsg.ProtoReflect()
	result := make(map[string]interface{})

	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch fd.Kind() {
		case protoreflect.BoolKind:
			result[string(fd.Name())] = v.Bool()
		case protoreflect.StringKind:
			result[string(fd.Name())] = v.String()
		case protoreflect.Int32Kind, protoreflect.Int64Kind,
			protoreflect.Sint32Kind, protoreflect.Sint64Kind,
			protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
			result[string(fd.Name())] = v.Int()
		case protoreflect.Uint32Kind, protoreflect.Uint64Kind,
			protoreflect.Fixed32Kind, protoreflect.Fixed64Kind:
			result[string(fd.Name())] = v.Uint()
		case protoreflect.FloatKind, protoreflect.DoubleKind:
			result[string(fd.Name())] = v.Float()
		case protoreflect.BytesKind:
			result[string(fd.Name())] = v.Bytes()
		case protoreflect.EnumKind:
			if desc := fd.Enum().Values().ByNumber(v.Enum()); desc != nil {
				result[string(fd.Name())] = string(desc.Name())
			} else {
				result[string(fd.Name())] = int32(v.Enum())
			}
		case protoreflect.MessageKind, protoreflect.GroupKind:
			if msg := v.Message(); msg.IsValid() {
				result[string(fd.Name())] = getProtoValue(msg.Interface(), false)
			}
		}
		return true
	})

	// If there's only a single toplevel field, return its value directly
	if returnAsToplevel && len(result) == 1 {
		for _, v := range result {
			return v
		}
	}

	return result
}
