package logrus

import (
	"errors"
	"os"

	githubLogrus "github.com/sirupsen/logrus"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("logger tests", func() {

	BeforeEach(func() {
		os.Clearenv()
	})

	It("works", func() {
		data := map[string]interface{}{"simple": "value", "complex": `has "quotes" and spaces`, "boolean": true}

		logger, hook := NoOpLogger()

		logger.Log(INFO, "http", data)
		Expect(hook.Entries).To(HaveLen(1))
		Expect(hook.LastEntry().Level).To(Equal(githubLogrus.InfoLevel))
		Expect(hook.LastEntry().Message).To(Equal("http"))

		logger.ActivityLog("sample_activity_log", data)
		Expect(hook.Entries).To(HaveLen(2))
		Expect(hook.LastEntry().Level).To(Equal(githubLogrus.InfoLevel))
		Expect(hook.LastEntry().Message).To(Equal("sample_activity_log"))

		logger.ErrorLog("sample_anomaly_log", nil, data)
		Expect(hook.Entries).To(HaveLen(3))
		Expect(hook.LastEntry().Level).To(Equal(githubLogrus.ErrorLevel))
		Expect(hook.LastEntry().Message).To(Equal("sample_anomaly_log"))

		logger.ErrorLog("sample_anomaly_log_2", errors.New("error message"), nil)
		Expect(hook.Entries).To(HaveLen(4))
		Expect(hook.LastEntry().Level).To(Equal(githubLogrus.ErrorLevel))
		Expect(hook.LastEntry().Message).To(Equal("sample_anomaly_log_2"))

		logger.Log(ERROR, "http: TLS handshake error from 0.0.0.0:1: EOF", nil)
		Expect(hook.Entries).To(HaveLen(5))
		Expect(hook.LastEntry().Level).To(Equal(githubLogrus.ErrorLevel))
		Expect(hook.LastEntry().Message).To(Equal("http: TLS handshake error from 0.0.0.0:1: EOF"))
	})

	Context("Tls handshake error", func() {
		DescribeTable("suppresses",
			func(envValue string) {
				_ = os.Setenv("SUPPRESS_TLS_HANDSHAKE_ERROR_LOGGING", envValue)
				logger, hook := NoOpLogger()
				logger.Log(ERROR, "http: TLS handshake error from 0.0.0.0:1: EOF", nil)
				Expect(hook.Entries).To(BeEmpty())
				logger.Log(ERROR, "Random error", nil)
				Expect(hook.Entries).To(HaveLen(1))
				Expect(hook.LastEntry().Message).To(Equal("Random error"))

			},
			Entry("for env variable true", "true"),
			Entry("for env variable TRUE", "TRUE"),
			Entry("for env variable 1", "1"),
		)
	})

})
