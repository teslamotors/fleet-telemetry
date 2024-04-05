package monitoring

import (
	"fmt"
	"net/http"

	"github.com/teslamotors/fleet-telemetry/config"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
)

type statusServer struct {
}

// Status API
func (s *statusServer) Status() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	}
}

// StartStatusServer initializes the status server on http
func StartStatusServer(config *config.Config, logger *logrus.Logger, airbrakeHandler *airbrake.AirbrakeHandler) {
	statusServer := &statusServer{}
	mux := http.NewServeMux()
	mux.Handle("/status", airbrakeHandler.WithReporting(http.HandlerFunc(statusServer.Status())))
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf(":%d", config.StatusPort), mux); err != nil {
			logger.ErrorLog("status", err, nil)
		}
	}()
	logger.ActivityLog("status_server_configured", nil)
}
