package http_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	httpConnector "github.com/teslamotors/fleet-telemetry/connector/adapter/http"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/metrics/adapter/noop"
)

var _ = Describe("HttpConnector", func() {
	var (
		mockServer *httptest.Server
		connector  *httpConnector.Connector
		config     *httpConnector.Config
		err        error
		logger     *logrus.Logger

		vinAllowedCount  int
		vinRejectedCount int
		errorCount       int
	)

	BeforeEach(func() {
		logger, _ = logrus.NoOpLogger()
		mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/VIN-allowed/allowed" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"allowed": true}`))
				vinAllowedCount++
				return
			}

			if r.URL.Path == "/VIN-not-allowed/allowed" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"allowed": false}`))
				vinRejectedCount++
				return
			}

			errorCount++
			http.Error(w, "not found", http.StatusNotFound)
		}))

		vinAllowedCount = 0
		vinRejectedCount = 0
		errorCount = 0

		config = &httpConnector.Config{
			Host:           mockServer.URL,
			TimeoutSeconds: 5,
			VinAllowed: httpConnector.VinAllowed{
				AllowOnFailure: true,
				CacheResults:   true,
			},
		}

		connector, err = httpConnector.NewConnector(config, noop.NewCollector(), logger)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		mockServer.Close()
	})

	Describe("VinAllowed", func() {
		Context("when VIN is allowed", func() {
			It("should return true", func() {
				Expect(connector.VinAllowed("VIN-allowed")).To(BeTrue())
				Expect(vinAllowedCount).To(Equal(1))
			})
		})

		Context("when VIN is not allowed", func() {
			It("should return false", func() {
				Expect(connector.VinAllowed("VIN-not-allowed")).To(BeFalse())
				Expect(vinRejectedCount).To(Equal(1))
			})
		})

		Context("when there is an error", func() {
			BeforeEach(func() {
				connector, err = httpConnector.NewConnector(config, noop.NewCollector(), logger)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return AllowOnFailure value", func() {
				config.VinAllowed.AllowOnFailure = false
				Expect(connector.VinAllowed("VIN")).To(Equal(false))

				config.VinAllowed.AllowOnFailure = true
				Expect(connector.VinAllowed("VIN")).To(Equal(true))
				Expect(errorCount).To(Equal(2))
			})
		})

		It("should cache results", func() {
			Expect(connector.VinAllowed("VIN-allowed")).To(BeTrue())
			Expect(connector.VinAllowed("VIN-allowed")).To(BeTrue())
			Expect(vinAllowedCount).To(Equal(1))
		})
	})
})
