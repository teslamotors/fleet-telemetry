package streaming

import (
	"sync"
	"time"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
)

// SocketRegistry is a library to handle keeping track of connected sockets
type SocketRegistry struct {
	mutex   sync.RWMutex
	sockets map[string]*SocketManager
	counter int
	logger  *logrus.Logger
}

// NewSocketRegistry returns an empty socket registry
func NewSocketRegistry(logger *logrus.Logger) *SocketRegistry {
	s := &SocketRegistry{
		sockets: make(map[string]*SocketManager),
		logger:  logger,
	}
	go s.keepAliveCron()
	return s
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
	s.deregisterSocket(socket)
}

// deregisterSocket removes a disconnecting socket without a mutex lock
func (s *SocketRegistry) deregisterSocket(socket *SocketManager) {
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

func (s *SocketRegistry) keepAliveChecks() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for _, socket := range s.sockets {
		if !socket.IsConnectionActive() {
			s.logger.ActivityLog("active_check_false", logrus.LogInfo{"socket_id": socket.UUID})
			//s.deregisterSocket(socket)
		} else {
			s.logger.ActivityLog("active_check_true", logrus.LogInfo{"socket_id": socket.UUID})
		}
	}
}

func (s *SocketRegistry) keepAliveCron() {
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for range ticker.C {
			s.keepAliveChecks()
		}
	}()
}
