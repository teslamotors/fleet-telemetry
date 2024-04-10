package logrus

import (
	"github.com/sirupsen/logrus/hooks/test"
)

func NoOpLogger() (*Logger, *test.Hook) {
	log, hook := test.NewNullLogger()
	logger, _ := NewLogrusLogger("null_logger", map[string]interface{}{}, log.WithField("context", "test"))
	return logger, hook
}
