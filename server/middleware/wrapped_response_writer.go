package middleware

import (
	"net/http"
	"time"
)

// WrappedResponseWriter wraps a standard http.ResponseWriter
// so we can store the status code, track size and duration of response.
type WrappedResponseWriter struct {
	status int
	size   int
	start  time.Time
	http.ResponseWriter
	body [][]byte
}

// NewWrappedResponseWriter creates a new wrapped response writer
func NewWrappedResponseWriter(res http.ResponseWriter) *WrappedResponseWriter {
	return &WrappedResponseWriter{200, 0, time.Now(), res, make([][]byte, 0)}
}

// Status returns the captured status code
func (w *WrappedResponseWriter) Status() int {
	return w.status
}

// Size returns the total bytes written
func (w *WrappedResponseWriter) Size() int {
	return w.size
}

// Body flattens the chunks of byte arrays into a single byte array
func (w *WrappedResponseWriter) Body() []byte {
	var result []byte
	for _, body := range w.body {
		result = append(result, body...)
	}
	return result
}

// DurationMS returns the duration of this response in milliseconds
func (w *WrappedResponseWriter) DurationMS() int {
	return int(time.Since(w.start).Seconds() * 1000)
}

// Header passes through to the response writer
func (w *WrappedResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

// ShouldReportOnAirbrake returns true if response code is 5xx
func (w *WrappedResponseWriter) ShouldReportOnAirbrake() bool {
	return w.status >= 500 && w.status <= 599
}
