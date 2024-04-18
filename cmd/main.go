package main

import (
	"fmt"
	"os"

	_ "go.uber.org/automaxprocs"

	"github.com/airbrake/gobrake/v5"
	"github.com/teslamotors/fleet-telemetry/config"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/server/airbrake"
	"github.com/teslamotors/fleet-telemetry/server/monitoring"
	"github.com/teslamotors/fleet-telemetry/server/streaming"
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
			logger.ErrorLog("profiling_file_error", err, nil)
			config.Monitoring.ProfilingPath = ""
		}

		defer func() {
			config.MetricCollector.Shutdown()
			_ = config.Monitoring.ProfilerFile.Close()
		}()
	}

	airbrakeNotifier, _, err := config.CreateAirbrakeNotifier(logger)
	if err != nil {
		panic(err)
	}
	if airbrakeNotifier != nil {
		defer airbrakeNotifier.NotifyOnPanic()
		defer airbrakeNotifier.Close()
	}
	panic(startServer(config, airbrakeNotifier, logger))
}

func startServer(config *config.Config, airbrakeNotifier *gobrake.Notifier, logger *logrus.Logger) (err error) {
	logger.ActivityLog("starting_server", nil)
	registry := streaming.NewSocketRegistry()

	airbrakeHandler := airbrake.NewAirbrakeHandler(airbrakeNotifier)

	if config.StatusPort > 0 {
		monitoring.StartStatusServer(config, logger, airbrakeHandler)
	}
	if config.Monitoring != nil {
		monitoring.StartServerMetrics(config, logger, registry)
	}

	producerRules, err := config.ConfigureProducers(airbrakeHandler, logger)
	if err != nil {
		return err
	}
	server, _, err := streaming.InitServer(config, airbrakeHandler, producerRules, logger, registry)
	if err != nil {
		return err
	}

	if server.TLSConfig, err = config.ExtractServiceTLSConfig(logger); err != nil {
		return err
	}
	return server.ListenAndServeTLS(config.TLS.ServerCert, config.TLS.ServerKey)
}
