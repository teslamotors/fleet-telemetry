package logrus

import (
	"github.com/sirupsen/logrus/hooks/test"
)

// NoOpLogger creates a no-op logger for testing purposes.
func NoOpLogger() (*Logger, *test.Hook) {
	log, hook := test.NewNullLogger()
	logger, _ := NewLogrusLogger("null_logger", map[string]interface{}{}, log.WithField("context", "test"))
	return logger, hook
}
