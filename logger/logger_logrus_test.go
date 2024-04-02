package logrus

import (
	"errors"

	githubLogrus "github.com/sirupsen/logrus"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("logger tests", func() {
	It("works", func() {
		data := map[string]interface{}{"simple": "value", "complex": `has "quotes" and spaces`, "boolean": true}

		logger, hook := NoOpLogger()

		logger.Log(INFO, "http", data)
		Expect(len(hook.Entries)).To(Equal(1))
		Expect(hook.LastEntry().Level).To(Equal(githubLogrus.InfoLevel))
		Expect(hook.LastEntry().Message).To(Equal("http"))

		logger.ActivityLog("sample_activity_log", data)
		Expect(len(hook.Entries)).To(Equal(2))
		Expect(hook.LastEntry().Level).To(Equal(githubLogrus.InfoLevel))
		Expect(hook.LastEntry().Message).To(Equal("sample_activity_log"))

		logger.AnomalyLog("sample_anomaly_log", data)
		Expect(len(hook.Entries)).To(Equal(3))
		Expect(hook.LastEntry().Level).To(Equal(githubLogrus.ErrorLevel))
		Expect(hook.LastEntry().Message).To(Equal("sample_anomaly_log"))

		logger.AnomalyLogError("sample_anomaly_log_2", errors.New("error message"), nil)
		Expect(len(hook.Entries)).To(Equal(4))
		Expect(hook.LastEntry().Level).To(Equal(githubLogrus.ErrorLevel))
		Expect(hook.LastEntry().Message).To(Equal("sample_anomaly_log_2"))
	})
})
