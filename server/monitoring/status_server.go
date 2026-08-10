package monitoring

import (
	"fmt"
	"net/http"

	"github.com/teslamotors/fleet-telemetry/config"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/server/streaming"
)

type statusServer struct {
	socketServer *streaming.Server
}

// Status API
func (s *statusServer) Status() func(w http.ResponseWriter, _ *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "ok")
	}
}

// StartStatusServer initializes the status server on http.
// When socketServer is non-nil, POST /resync is mounted for application-controlled
// vehicle field true-ups after backend outages (see #246).
func StartStatusServer(config *config.Config, logger *logrus.Logger, airbrakeHandler *airbrake.Handler, socketServer *streaming.Server) {
	statusServer := &statusServer{socketServer: socketServer}
	mux := http.NewServeMux()
	mux.Handle("/status", airbrakeHandler.WithReporting(http.HandlerFunc(statusServer.Status())))
	if socketServer != nil {
		mux.Handle("/resync", airbrakeHandler.WithReporting(http.HandlerFunc(socketServer.HandleResync())))
	}
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf(":%d", config.StatusPort), mux); err != nil {
			logger.ErrorLog("status", err, nil)
		}
	}()
	logger.ActivityLog("status_server_configured", logrus.LogInfo{"resync_enabled": socketServer != nil})
}
