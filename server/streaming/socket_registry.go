package streaming

import "sync"

// SocketRegistry is a library to handle keeping track of connected sockets
type SocketRegistry struct {
	mutex   sync.RWMutex
	sockets map[string]*SocketManager
	counter int
}

// NewSocketRegistry returns an empty socket registry
func NewSocketRegistry() *SocketRegistry {
	return &SocketRegistry{
		sockets: make(map[string]*SocketManager),
	}
}

// RegisterSocket registers a new socket
func (s *SocketRegistry) RegisterSocket(socket *SocketManager) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.sockets[socket.UUID] = socket
	s.counter++
}

// DeregisterSocket removes a disconnecting socket
func (s *SocketRegistry) DeregisterSocket(socket *SocketManager) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	delete(s.sockets, socket.UUID)
	if s.counter > 0 {
		s.counter--
	}
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
