package main

import (
	"context"
	"fmt"
	_ "net/http/pprof"
	"os"

	_ "go.uber.org/automaxprocs"

	"github.com/teslamotors/fleet-telemetry/cmd/run"
	"github.com/teslamotors/fleet-telemetry/config"
)

func main() {
	var err error

	config, logger, err := config.LoadApplicationConfiguration()
	if err != nil {
		// logger is not available yet
		panic(fmt.Sprintf("error=load_service_config value=\"%s\"", err.Error()))
	}

	if config.Monitoring != nil && config.Monitoring.ProfilingPath != "" {
		if config.Monitoring.ProfilerFile, err = os.Create(config.Monitoring.ProfilingPath); err != nil {
			logger.Errorf("profiling_file_error %v", err)
			config.Monitoring.ProfilingPath = ""
		}

		defer func() {
			config.MetricCollector.Shutdown()
			_ = config.Monitoring.ProfilerFile.Close()
		}()
	}

	panic(run.RunServer(context.Background(), config, logger))
}
