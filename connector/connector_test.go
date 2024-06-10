package connector_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/fleet-telemetry/connector"
	"github.com/teslamotors/fleet-telemetry/connector/adapter/file"
	"github.com/teslamotors/fleet-telemetry/connector/adapter/grpc"
	httpConnector "github.com/teslamotors/fleet-telemetry/connector/adapter/http"
	"github.com/teslamotors/fleet-telemetry/connector/adapter/redis"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter/noop"
)

var _ = Describe("ConnectorProvider", func() {
	var (
		config       connector.Config
		logger       *logrus.Logger
		metrics      metrics.MetricCollector
		connProvider *connector.ConnectorProvider
		mockServer   *httptest.Server
	)

	BeforeEach(func() {
		mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"allowed": true}`))
		}))

		config = connector.Config{
			File:  &file.Config{},
			Grpc:  &grpc.Config{},
			Redis: &redis.Config{},
			Http: &httpConnector.Config{
				Host:         mockServer.URL,
				Capabilities: []string{"vin_allowed"},
			},
		}

		logger, _ = logrus.NoOpLogger()
		metrics = noop.NewCollector()

		connProvider = connector.NewConnectorProvider(config, metrics, logger)
		Expect(connProvider).NotTo(BeNil())
	})

	AfterEach(func() {
		mockServer.Close()
	})

	It("gets data using configured source", func() {
		allowed, err := connProvider.VinAllowed("testVin")
		Expect(err).To(BeNil())
		Expect(allowed).To(BeTrue())
	})

	Context("configuring sources", func() {
		It("should not initialize unconfigured sources", func() {
			Expect(connProvider.Connectors.File).To(BeNil())
			Expect(connProvider.Connectors.Grpc).To(BeNil())
			Expect(connProvider.Connectors.Redis).To(BeNil())
		})

		It("should initialize configured source", func() {
			Expect(connProvider.Connectors.Http).NotTo(BeNil())
		})
	})
})
