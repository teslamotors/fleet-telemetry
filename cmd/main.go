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

	cfg, logger, err := config.LoadApplicationConfiguration()
	if err != nil {
		// logger is not available yet
		panic(fmt.Sprintf("error=load_service_config value=\"%s\"", err.Error()))
	}

	if cfg.Monitoring != nil && cfg.Monitoring.ProfilingPath != "" {
		if cfg.Monitoring.ProfilerFile, err = os.Create(cfg.Monitoring.ProfilingPath); err != nil {
			logger.ErrorLog("profiling_file_error", err, nil)
			cfg.Monitoring.ProfilingPath = ""
		}

		defer func() {
			cfg.MetricCollector.Shutdown()
			_ = cfg.Monitoring.ProfilerFile.Close()
		}()
	}

	airbrakeNotifier, _, err := cfg.CreateAirbrakeNotifier(logger)
	if err != nil {
		panic(err)
	}
	if airbrakeNotifier != nil {
		defer airbrakeNotifier.NotifyOnPanic()
		defer func() {
			if err := airbrakeNotifier.Close(); err != nil {
				logger.ErrorLog("airbrake_close_error", err, nil)
			}
		}()
	}
	panic(startServer(cfg, airbrakeNotifier, logger))
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

	dispatchers, producerRules, err := config.ConfigureProducers(airbrakeHandler, logger, false)
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

	err = server.ListenAndServeTLS(config.TLS.ServerCert, config.TLS.ServerKey)
	for dispatcher, producer := range dispatchers {
		logger.ActivityLog("attempting_to_close", logrus.LogInfo{"dispatcher": dispatcher})
		// We don't care if this fails. If it does, we'll just continue on.
		if dispatcherCloseErr := producer.Close(); dispatcherCloseErr != nil {
			logger.ErrorLog("producer_close_error", dispatcherCloseErr, logrus.LogInfo{"dispatcher": dispatcher})
		}
	}
	logger.ActivityLog("stopped_server", nil)
	return err
}
