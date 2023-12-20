package http_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/sirupsen/logrus"
	"github.com/teslamotors/fleet-telemetry/datastore/http"
	"github.com/teslamotors/fleet-telemetry/metrics"
)

var _ = Describe("HTTP Producer", func() {
	var (
		mockLogger    *logrus.Logger
		mockCollector metrics.MetricCollector
		mockConfig    *http.Config
	)

	BeforeEach(func() {
		mockLogger = logrus.New()
		mockCollector = metrics.NewCollector(nil, mockLogger)
		mockConfig = &http.Config{
			WorkerCount: 2,
			Address:     "https://tesla.com",
			Timeout:     5,
		}
	})

	Context("NewProducer", func() {
		It("creates a new HTTP producer with valid config", func() {
			producer, err := http.NewProducer(mockConfig, false, mockCollector, "test", mockLogger)
			Expect(err).ToNot(HaveOccurred())
			Expect(producer).ToNot(BeNil())
		})

		It("returns an error for negative worker count", func() {
			mockConfig.WorkerCount = -1
			producer, err := http.NewProducer(mockConfig, false, mockCollector, "test", mockLogger)
			Expect(err).To(HaveOccurred())
			Expect(producer).To(BeNil())
		})

		It("returns an error for negative timeout", func() {
			mockConfig.Timeout = -1
			producer, err := http.NewProducer(mockConfig, false, mockCollector, "test", mockLogger)
			Expect(err).To(HaveOccurred())
			Expect(producer).To(BeNil())
		})
	})
})
