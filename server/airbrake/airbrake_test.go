package airbrake

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AirbrakeMiddleware", func() {
	var (
		middleware *AirbrakeHandler
		recorder   *httptest.ResponseRecorder
		req        *http.Request
	)

	BeforeEach(func() {
		var err error
		req, err = http.NewRequest("GET", "/test", nil)
		Expect(err).NotTo(HaveOccurred())
		middleware = NewAirbrakeHandler(nil)
		recorder = httptest.NewRecorder()
	})

	DescribeTable("successfully handles request",
		func(statusCode int, body string) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(statusCode)
				w.Write([]byte(body))
			})
			middleware.WithReporting(handler).ServeHTTP(recorder, req)
			Expect(recorder.Code).To(Equal(statusCode))
			responseBody := recorder.Body.Bytes()
			Expect(responseBody).To(Equal([]byte(body)))
		},
		Entry("404", http.StatusNotFound, "not found"),
		Entry("400", http.StatusInternalServerError, "bad error"),
		Entry("200", http.StatusOK, "ok"),
	)
})
