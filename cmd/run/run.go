package run

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/teslamotors/fleet-telemetry/config"
	"github.com/teslamotors/fleet-telemetry/server/monitoring"
	"github.com/teslamotors/fleet-telemetry/server/streaming"
)

var (
	// ShutdownTimeout is the amount of time allotted for the http server to
	// shutdown.
	ShutdownTimeout = time.Second * 5
)

// RunServer runs a fleet telemetry server given its config.
func RunServer(ctx context.Context, config *config.Config, logger *logrus.Logger) (err error) {
	logger.Infoln("starting")
	mux := http.NewServeMux()
	registry := streaming.NewSocketRegistry()

	monitoring.StartProfilerServer(config, mux, logger)
	if config.StatusPort > 0 {
		monitoring.StartStatusServer(config, logger)
	}
	if config.Monitoring != nil {
		monitoring.StartServerMetrics(config, logger, registry)
	}

	producerRules, err := config.ConfigureProducers(logger)
	if err != nil {
		return err
	}

	server, streamingServer, err := streaming.InitServer(config, mux, producerRules, logger, registry)
	if err != nil {
		return err
	}
	defer streamingServer.Close()

	if server.TLSConfig, err = config.ExtractServiceTLSConfig(); err != nil {
		return err
	}

	// Create a new context in which to run our server.
	// This context will die when either:
	// 1. An os.Interrupt is received.
	// 2. The server encounters a fatal error.
	ctx, canceller := signal.NotifyContext(ctx, os.Interrupt)
	serverRunChan := make(chan error)

	// Run the server in the background.
	// When the server quits, signal the foreground goroutine by closing the
	// context. Send the error to the foreground via a channel.
	go func() {
		err := server.ListenAndServeTLS(config.TLS.ServerCert, config.TLS.ServerKey)
		canceller()
		serverRunChan <- err
	}()

	// Wait for either the server to encounter a fatal error or an interrupt.
	<-ctx.Done()
	logger.Infoln("Received interrupt, shutting down...")
	shutdownContext, canceller := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer canceller()
	servErr := server.Shutdown(shutdownContext)
	shutdownErr := <-serverRunChan
	return errors.Join(servErr, shutdownErr)
}
