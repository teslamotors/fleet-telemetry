package airbrake

import (
	"net/http"

	"github.com/airbrake/gobrake/v5"

	logrus "github.com/teslamotors/fleet-telemetry/logger"
	"github.com/teslamotors/fleet-telemetry/server/middleware"
)

// Handler reports errors to airbrake
type Handler struct {
	airbrakeNotifier *gobrake.Notifier
}

// NewAirbrakeHandler returns a new instance of AirbrakeHandler
func NewAirbrakeHandler(airbrakeNotifier *gobrake.Notifier) *Handler {
	return &Handler{
		airbrakeNotifier: airbrakeNotifier,
	}
}

func httpAirbrakeMessage(r *http.Request, w *middleware.WrappedResponseWriter) *gobrake.Notice {
	notice := gobrake.NewNotice(string(w.Body()), r, 1)
	notice.Params["status_code"] = w.Status()
	notice.Params["duration_ms"] = w.DurationMS()
	for responseHeaderKey, responseHeaderValue := range w.Header() {
		notice.Params[responseHeaderKey] = responseHeaderValue
	}
	return notice
}

// ReportError dispatches errors for incoming requests
func (a *Handler) ReportError(r *http.Request, err error) {
	if a.airbrakeNotifier == nil {
		return
	}
	notice := gobrake.NewNotice(err.Error(), r, 1)
	a.airbrakeNotifier.SendNoticeAsync(notice)
}

// WithReporting dispatches 5xx messages with some metadata to airbrake if notifier is configured
func (a *Handler) WithReporting(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := middleware.NewWrappedResponseWriter(w)
		next.ServeHTTP(recorder, r)
		if recorder.ShouldReportOnAirbrake() && a.airbrakeNotifier != nil {
			a.airbrakeNotifier.SendNoticeAsync(httpAirbrakeMessage(r, recorder))
		}
	})
}

func (a *Handler) logMessage(logType logrus.LogType, message string, err error, logInfo logrus.LogInfo) *gobrake.Notice {
	notice := gobrake.NewNotice(message, nil, 1)
	notice.Params["log_type"] = logrus.AllLogType[logType]
	if err != nil {
		notice.Params["error"] = err.Error()
	}
	//nolint:gosimple
	if logInfo != nil {
		for logKey, logValue := range logInfo {
			notice.Params[logKey] = logValue
		}
	}
	return notice
}

// ReportLogMessage log message to airbrake
func (a *Handler) ReportLogMessage(logType logrus.LogType, message string, err error, logInfo logrus.LogInfo) {
	if a.airbrakeNotifier == nil {
		return
	}
	a.airbrakeNotifier.SendNoticeAsync(a.logMessage(logType, message, err, logInfo))
}
