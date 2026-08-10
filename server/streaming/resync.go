package streaming

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/messages"
)

const resyncTopic = "resync"

// ResyncRequest is the JSON body for POST /resync.
// An empty Fields list means "all configured fields" (vehicle interprets).
type ResyncRequest struct {
	VIN    string   `json:"vin"`
	Fields []string `json:"fields,omitempty"`
}

// ResyncResponse is returned by POST /resync.
type ResyncResponse struct {
	VIN            string `json:"vin"`
	SocketsNotified int    `json:"sockets_notified"`
	Fields         []string `json:"fields,omitempty"`
}

// HandleResync requests that a connected vehicle resend telemetry fields.
// This is the application-controlled true-up path discussed in #246: after a
// backend outage, the app asks fleet-telemetry to forward a resync request on
// the vehicle's open WebSocket(s).
func (s *Server) HandleResync() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ResyncRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("invalid json: %v", err), http.StatusBadRequest)
			return
		}
		if req.VIN == "" {
			http.Error(w, "vin is required", http.StatusBadRequest)
			return
		}

		notified, err := s.RequestVehicleResync(req.VIN, req.Fields)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ResyncResponse{
			VIN:             req.VIN,
			SocketsNotified: notified,
			Fields:          req.Fields,
		})
	}
}

// RequestVehicleResync sends a resync control message on every active socket for vin.
// Returns the number of sockets notified.
func (s *Server) RequestVehicleResync(vin string, fields []string) (int, error) {
	sockets := s.registry.GetSocketsForDevice(vin)
	if len(sockets) == 0 {
		return 0, fmt.Errorf("no active websocket for vin %s", vin)
	}

	payload, err := buildResyncMessage(vin, fields)
	if err != nil {
		return 0, err
	}

	for _, socket := range sockets {
		socket.Send(websocket.BinaryMessage, payload)
	}

	s.logger.ActivityLog("vehicle_resync_requested", logrus.LogInfo{
		"deviceID":         vin,
		"sockets_notified": len(sockets),
		"fields":           fields,
	})
	return len(sockets), nil
}

func buildResyncMessage(vin string, fields []string) ([]byte, error) {
	if fields == nil {
		fields = []string{}
	}
	body, err := json.Marshal(map[string]interface{}{
		"fields": fields,
	})
	if err != nil {
		return nil, err
	}

	msg := messages.StreamMessage{
		TXID:         []byte(uuid.New().String()),
		SenderID:     []byte("fleet-telemetry"),
		DeviceID:     []byte(vin),
		DeviceType:   []byte("vehicle_device"),
		MessageTopic: []byte(resyncTopic),
		Payload:      body,
		CreatedAt:    uint32(time.Now().Unix()),
	}
	return msg.ToBytes()
}
