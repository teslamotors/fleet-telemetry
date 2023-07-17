package monitoring

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/teslamotors/fleet-telemetry/config"
	"github.com/teslamotors/fleet-telemetry/metrics"
)

type profileServer struct {
}

// liveProfiler profiles https requests
func (p *profileServer) liveProfiler(config *config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.RemoteAddr, "127.0.0.1") { // enable this only locally
			w.WriteHeader(404)
			return
		}

		if config.Monitoring == nil || config.Monitoring.ProfilingPath == "" { // disabled by default
			w.WriteHeader(405)
			_, _ = w.Write([]byte(`{"error":"profiler not configured"}`))
			return
		}

		mode := r.URL.Query().Get("mode")
		if mode == "" {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"error":"mode not specified"}`))
			return
		}

		if metrics.EnableProfiler(mode) {
			_, _ = w.Write([]byte(fmt.Sprintf(`{"result":"mode %v"}`, mode)))
		} else {
			w.WriteHeader(304)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"result":"mode already %v"}`, mode)))
		}
	}
}

// gcStats display GC stats
func (p *profileServer) gcStats(config *config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		stats := &debug.GCStats{}
		debug.ReadGCStats(stats)

		json, err := json.Marshal(stats)

		if err != nil {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"error":"` + err.Error() + `"}`))
		}

		_, _ = w.Write(json)
	}
}

// StartProfilerServer initializes the profiler on http
func StartProfilerServer(config *config.Config, mux *http.ServeMux, logger *logrus.Logger) {
	profileServer := &profileServer{}
	mux.HandleFunc("/gc_stats", profileServer.gcStats(config))
	mux.HandleFunc("/live_profiler", profileServer.liveProfiler(config))
	logger.Infoln("profiler_started")
}
