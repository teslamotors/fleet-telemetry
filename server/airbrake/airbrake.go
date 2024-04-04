package airbrake

import (
	"net/http"

	githubairbrake "github.com/airbrake/gobrake/v5"
	"github.com/teslamotors/fleet-telemetry/server/middleware"
)

type AirbrakeHandler struct {
	airbrakeNotifier *githubairbrake.Notifier
}

func NewAirbrakeHandler(airbrakeNotifier *githubairbrake.Notifier) *AirbrakeHandler {
	return &AirbrakeHandler{
		airbrakeNotifier: airbrakeNotifier,
	}
}

func httpAirbrakeMessage(r *http.Request, w *middleware.WrappedResponseWriter) *githubairbrake.Notice {
	notice := githubairbrake.NewNotice(string(w.Body()), r, 1)
	notice.Params["status_code"] = w.Status()
	notice.Params["duration_ms"] = w.DurationMS()
	for responseHeaderKey, responseHeaderValue := range w.Header() {
		notice.Params[responseHeaderKey] = responseHeaderValue
	}
	return notice
}

// ReportError dispatches errors for incoming requests
func (a *AirbrakeHandler) ReportError(r *http.Request, err error) {
	if a.airbrakeNotifier == nil {
		return
	}
	notice := githubairbrake.NewNotice(err.Error(), r, 1)
	a.airbrakeNotifier.SendNoticeAsync(notice)
}

// WithReporting dispatches 5xx messages with some metadata to airbrake if notifier is configured
func (a *AirbrakeHandler) WithReporting(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := middleware.NewWrappedResponseWriter(w)
		next.ServeHTTP(recorder, r)
		if recorder.ShouldReportOnAirbrake() && a.airbrakeNotifier != nil {
			a.airbrakeNotifier.SendNoticeAsync(httpAirbrakeMessage(r, recorder))
		}
	})
}
