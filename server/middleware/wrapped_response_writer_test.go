package middleware

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("WrappedResponseWriter", func() {
	var (
		recorder *httptest.ResponseRecorder
		wrapped  *WrappedResponseWriter
	)

	BeforeEach(func() {
		recorder = httptest.NewRecorder()
		wrapped = NewWrappedResponseWriter(recorder)
	})

	It("captures status, size, and body", func() {
		wrapped.WriteHeader(http.StatusBadGateway)
		_, err := wrapped.Write([]byte("bad "))
		Expect(err).NotTo(HaveOccurred())
		_, err = wrapped.Write([]byte("gateway"))
		Expect(err).NotTo(HaveOccurred())

		Expect(wrapped.Status()).To(Equal(http.StatusBadGateway))
		Expect(wrapped.Size()).To(Equal(len("bad gateway")))
		Expect(wrapped.Body()).To(Equal([]byte("bad gateway")))
		Expect(wrapped.ShouldReportOnAirbrake()).To(BeTrue())

		Expect(recorder.Code).To(Equal(http.StatusBadGateway))
		Expect(recorder.Body.Bytes()).To(Equal([]byte("bad gateway")))
	})

	It("defaults to 200 and does not report on airbrake", func() {
		_, err := wrapped.Write([]byte("ok"))
		Expect(err).NotTo(HaveOccurred())

		Expect(wrapped.Status()).To(Equal(http.StatusOK))
		Expect(wrapped.ShouldReportOnAirbrake()).To(BeFalse())
	})
})
