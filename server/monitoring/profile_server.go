package monitoring

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/teslamotors/fleet-telemetry/config"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
)

type profileServer struct {
}

// liveProfiler profiles https requests
func (p *profileServer) liveProfiler(config *config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
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
func (p *profileServer) gcStats() func(w http.ResponseWriter, _ *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
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
	mux.HandleFunc("/gc_stats", profileServer.gcStats())
	mux.HandleFunc("/live_profiler", profileServer.liveProfiler(config))

	logger.ActivityLog("profiler_started", logrus.LogInfo{"port": config.Monitoring.ProfilerPort})
}
