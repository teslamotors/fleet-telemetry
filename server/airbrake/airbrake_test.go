package airbrake

import (
	"errors"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	logrus "github.com/teslamotors/fleet-telemetry/logger"
)

var _ = Describe("AirbrakeMiddleware", func() {
	var (
		handler  *AirbrakeHandler
		recorder *httptest.ResponseRecorder
		req      *http.Request
	)

	BeforeEach(func() {
		var err error
		req, err = http.NewRequest("GET", "/test", nil)
		Expect(err).NotTo(HaveOccurred())
		handler = NewAirbrakeHandler(nil)
		recorder = httptest.NewRecorder()
	})

	DescribeTable("successfully handles request",
		func(statusCode int, body string) {
			httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(statusCode)
				w.Write([]byte(body))
			})
			handler.WithReporting(httpHandler).ServeHTTP(recorder, req)
			Expect(recorder.Code).To(Equal(statusCode))
			responseBody := recorder.Body.Bytes()
			Expect(responseBody).To(Equal([]byte(body)))
		},
		Entry("404", http.StatusNotFound, "not found"),
		Entry("400", http.StatusInternalServerError, "bad error"),
		Entry("200", http.StatusOK, "ok"),
	)

	Context("logMessage", func() {

		It("for error", func() {
			notice := handler.logMessage(logrus.ERROR, "test_err", errors.New("sample error"), logrus.LogInfo{"key1": "value1", "key2": "value2"})
			Expect(len(notice.Errors)).Should(Equal(1))
			Expect(notice.Errors[0].Message).Should(Equal("test_err"))
			Expect(notice.Params).Should(Equal(map[string]interface{}{"log_type": "error", "error": "sample error", "key1": "value1", "key2": "value2"}))
		})

		It("for error with logInfo", func() {
			notice := handler.logMessage(logrus.ERROR, "test_err", errors.New("sample error"), nil)
			Expect(len(notice.Errors)).Should(Equal(1))
			Expect(notice.Errors[0].Message).Should(Equal("test_err"))
			Expect(notice.Params).Should(Equal(map[string]interface{}{"log_type": "error", "error": "sample error"}))
		})

		It("for info", func() {
			notice := handler.logMessage(logrus.INFO, "test_info", nil, logrus.LogInfo{"key1": "value1", "key2": "value2"})
			Expect(len(notice.Errors)).Should(Equal(1))
			Expect(notice.Errors[0].Message).Should(Equal("test_info"))
			Expect(notice.Params).Should(Equal(map[string]interface{}{"log_type": "info", "key1": "value1", "key2": "value2"}))
		})
	})

})
