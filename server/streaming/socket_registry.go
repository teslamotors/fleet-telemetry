package streaming

import (
	"sync"

	"github.com/google/uuid"
)

// SocketRegistry is a library to handle keeping track of connected sockets
type SocketRegistry struct {
	mutex         sync.RWMutex
	sockets       map[string]*SocketManager
	deviceSockets map[string]map[string]struct{}
	counter       int
}

// NewSocketRegistry returns an empty socket registry
func NewSocketRegistry() *SocketRegistry {
	return &SocketRegistry{
		sockets:       make(map[string]*SocketManager),
		deviceSockets: make(map[string]map[string]struct{}),
	}
}

// RegisterSocket registers a new socket.
// Returns true when this is the first active socket for the device (VIN),
// which is the signal consumers should treat as vehicle CONNECTED.
func (s *SocketRegistry) RegisterSocket(socket *SocketManager) (isFirstForDevice bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if existing, ok := s.sockets[socket.UUID]; ok && existing != socket {
		// X-TXID collisions would otherwise overwrite a live socket and break
		// VIN-level accounting / reliable acks for the surviving connection.
		socket.UUID = uuid.New().String()
	}

	if _, ok := s.sockets[socket.UUID]; !ok {
		s.counter++
	}
	s.sockets[socket.UUID] = socket

	deviceID := socket.requestIdentity.DeviceID
	deviceSet, ok := s.deviceSockets[deviceID]
	if !ok {
		deviceSet = make(map[string]struct{})
		s.deviceSockets[deviceID] = deviceSet
	}
	_, alreadyTracked := deviceSet[socket.UUID]
	deviceSet[socket.UUID] = struct{}{}
	return !alreadyTracked && len(deviceSet) == 1
}

// DeregisterSocket removes a disconnecting socket.
// Returns true when this was the last active socket for the device (VIN),
// which is the signal consumers should treat as vehicle DISCONNECTED.
func (s *SocketRegistry) DeregisterSocket(socket *SocketManager) (isLastForDevice bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if current, ok := s.sockets[socket.UUID]; ok {
		if current != socket {
			// A newer socket reused this UUID; leave registry state alone.
			return false
		}
		delete(s.sockets, socket.UUID)
		if s.counter > 0 {
			s.counter--
		}
	}

	deviceID := socket.requestIdentity.DeviceID
	deviceSet, ok := s.deviceSockets[deviceID]
	if !ok {
		return false
	}
	delete(deviceSet, socket.UUID)
	if len(deviceSet) == 0 {
		delete(s.deviceSockets, deviceID)
		return true
	}
	return false
}

// GetSocket returns a socket if connected
func (s *SocketRegistry) GetSocket(uuid string) *SocketManager {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.sockets[uuid]
}

// NumConnectedSockets returns the number of connected sockets
func (s *SocketRegistry) NumConnectedSockets() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.counter
}

// NumConnectedDevices returns the number of devices with at least one socket
func (s *SocketRegistry) NumConnectedDevices() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return len(s.deviceSockets)
}
