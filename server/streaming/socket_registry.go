package streaming

import "sync"

// SocketRegistry is a library to handle keeping track of connected sockets
type SocketRegistry struct {
	mutex         sync.RWMutex
	sockets       map[string]*SocketManager
	deviceSockets map[string]map[string]*SocketManager
	counter       int
}

// NewSocketRegistry returns an empty socket registry
func NewSocketRegistry() *SocketRegistry {
	return &SocketRegistry{
		sockets:       make(map[string]*SocketManager),
		deviceSockets: make(map[string]map[string]*SocketManager),
	}
}

// RegisterSocket registers a new socket
func (s *SocketRegistry) RegisterSocket(socket *SocketManager) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, ok := s.sockets[socket.UUID]; !ok {
		s.counter++
	}
	s.sockets[socket.UUID] = socket

	deviceID := socket.requestIdentity.DeviceID
	deviceSet, ok := s.deviceSockets[deviceID]
	if !ok {
		deviceSet = make(map[string]*SocketManager)
		s.deviceSockets[deviceID] = deviceSet
	}
	deviceSet[socket.UUID] = socket
}

// DeregisterSocket removes a disconnecting socket
func (s *SocketRegistry) DeregisterSocket(socket *SocketManager) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if current, ok := s.sockets[socket.UUID]; ok && current == socket {
		delete(s.sockets, socket.UUID)
		if s.counter > 0 {
			s.counter--
		}
	}

	deviceID := socket.requestIdentity.DeviceID
	deviceSet, ok := s.deviceSockets[deviceID]
	if !ok {
		return
	}
	delete(deviceSet, socket.UUID)
	if len(deviceSet) == 0 {
		delete(s.deviceSockets, deviceID)
	}
}

// GetSocket returns a socket if connected
func (s *SocketRegistry) GetSocket(uuid string) *SocketManager {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.sockets[uuid]
}

// GetSocketsForDevice returns all active sockets for a VIN/device ID
func (s *SocketRegistry) GetSocketsForDevice(deviceID string) []*SocketManager {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	deviceSet := s.deviceSockets[deviceID]
	if len(deviceSet) == 0 {
		return nil
	}
	sockets := make([]*SocketManager, 0, len(deviceSet))
	for _, socket := range deviceSet {
		sockets = append(sockets, socket)
	}
	return sockets
}

// NumConnectedSockets returns the number of connected sockets
func (s *SocketRegistry) NumConnectedSockets() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.counter
}
