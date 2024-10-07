package mqtt

import (
	"encoding/json"
	"fmt"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/teslamotors/fleet-telemetry/datastore/simple/transformers"
	"github.com/teslamotors/fleet-telemetry/protos"
	"github.com/teslamotors/fleet-telemetry/telemetry"
)

func (p *MQTTProducer) processVehicleFields(rec *telemetry.Record, payload *protos.Payload) ([]pahomqtt.Token, error) {
	var tokens []pahomqtt.Token
	convertedPayload := transformers.PayloadToMap(payload, false, p.logger)
	for key, value := range convertedPayload {
		if key == "Vin" || key == "CreatedAt" {
			continue
		}
		mqttTopicName := fmt.Sprintf("%s/%s/v/%s", p.config.TopicBase, rec.Vin, key)
		jsonValue, err := json.Marshal(map[string]interface{}{"value": value})
		if err != nil {
			return tokens, fmt.Errorf("failed to marshal JSON for MQTT topic %s: %v", mqttTopicName, err)
		}
		token := p.client.Publish(mqttTopicName, p.config.QoS, p.config.Retained, jsonValue)
		tokens = append(tokens, token)
		p.updateMetrics(rec.TxType, len(jsonValue))
	}
	return tokens, nil
}

func (p *MQTTProducer) processVehicleAlerts(rec *telemetry.Record, payload *protos.VehicleAlerts) ([]pahomqtt.Token, error) {
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

func (p *MQTTProducer) processVehicleErrors(rec *telemetry.Record, payload *protos.VehicleErrors) ([]pahomqtt.Token, error) {
	var tokens []pahomqtt.Token

	for _, vehicleError := range payload.Errors {
		topicName := fmt.Sprintf("%s/%s/errors/%s", p.config.TopicBase, rec.Vin, vehicleError.Name)
		errorMap := vehicleErrorToMqttMap(vehicleError)
		jsonValue, err := json.Marshal(errorMap)
		if err != nil {
			return tokens, fmt.Errorf("failed to marshal JSON for MQTT topic %s: %v", topicName, err)
		}

		token := p.client.Publish(topicName, p.config.QoS, p.config.Retained, jsonValue)
		tokens = append(tokens, token)
		p.updateMetrics(rec.TxType, len(jsonValue))
	}

	return tokens, nil
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

func vehicleErrorToMqttMap(vehicleError *protos.VehicleError) map[string]interface{} {
	errorMap := map[string]interface{}{
		"Body": vehicleError.Body,
		"Tags": vehicleError.Tags,
	}
	if vehicleError.CreatedAt != nil {
		errorMap["CreatedAt"] = vehicleError.CreatedAt.AsTime().Format(time.RFC3339)
	}
	return errorMap
}
