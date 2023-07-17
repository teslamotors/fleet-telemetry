package monitoring

import (
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/teslamotors/fleet-telemetry/config"
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
func StartStatusServer(config *config.Config, logger *logrus.Logger) {
	statusServer := &statusServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/status", statusServer.Status())
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf(":%d", config.StatusPort), mux); err != nil {
			logger.Errorf("status %v", err)
		}
	}()
	logger.Infoln("status_server_configured")
}
