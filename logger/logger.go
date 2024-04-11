package logrus

import (
	"os"
	"strconv"
	"strings"

	"github.com/mattn/go-colorable"
	"github.com/sirupsen/logrus"
)

// LogType is a typedef to represent the various log levels
type LogType int

// These are our list of log levels
const (
	DEBUG LogType = iota
	INFO
	WARN
	ERROR
	FATAL
)

const (
	tlsFilter = "http: TLS handshake error from"
)

// AllLogType is a map of LogType to string names
var AllLogType = map[LogType]string{
	DEBUG: "debug",
	INFO:  "info",
	WARN:  "warn",
	ERROR: "error",
	FATAL: "fatal",
}

// LogInfo is an alias to extra info to log
type LogInfo map[string]interface{}

// Logger a logrus implementer of the JSON logger interface
type Logger struct {
	logger *logrus.Entry

	suppressionFilter string
}

// NewLogrusLogger return a LogrusLogger
func NewLogrusLogger(context string, info LogInfo, logger *logrus.Entry) (*Logger, error) {
	if logger == nil {
		logger = logrus.WithField("context", context)
	}
	l := &Logger{logger: logger}
	l.logger = l.getEntry(info)
	value, ok := os.LookupEnv("SUPPRESS_TLS_HANDSHAKE_ERROR_LOGGING")
	if ok {
		b, err := strconv.ParseBool(value)
		if err != nil {
			return nil, err
		}
		if b {
			l.suppressionFilter = tlsFilter
		}
	}
	return l, nil
}

// NewBasicLogrusLogger creates a logrus logger with a context but no other options
func NewBasicLogrusLogger(context string) (*Logger, error) {
	return NewLogrusLogger(context, nil, nil)
}

// NewColorLogrusLogger creates a logrus logger with a context and colorized output support
func NewColorLogrusLogger(context string) (*Logger, error) {
	logger, err := NewLogrusLogger(context, nil, nil)
	if err != nil {
		return nil, err
	}
	logger.SetColorFormatter(true)
	return logger, nil
}

// Set minimum log level for messages
func SetLogLevel(name string) {
	level, err := logrus.ParseLevel(name)
	if err != nil {
		return
	}

	logrus.SetLevel(level)
}

func (l *Logger) shouldSuppress(message string) bool {
	if l.suppressionFilter == "" {
		return false
	}
	return strings.Contains(message, l.suppressionFilter)
}

// Log logs a message on a particular log level
func (l *Logger) Log(logType LogType, message string, info LogInfo) {
	if l.shouldSuppress(message) {
		return
	}
	entry := l.getEntry(info)

	switch logType {
	case DEBUG:
		entry.Debug(message)
	case INFO:
		entry.Info(message)
	case WARN:
		entry.Warn(message)
	case ERROR:
		entry.Error(message)
	case FATAL:
		entry.Fatal(message)
	}
}

// Write implements the Write method of the io.Writer interface
func (l *Logger) Write(p []byte) (n int, err error) {
	l.ActivityLog(string(p), nil)
	return len(p), nil
}

// Print allows Printing on the logger
func (l *Logger) Print(v ...interface{}) {
	l.logger.Print(v...)
}

// Printf allows Printf'ing on the logger
func (l *Logger) Printf(format string, v ...interface{}) {
	if l.shouldSuppress(format) {
		return
	}
	l.logger.Printf(format, v...)
}

// Println allows Println'ing on the logger
func (l *Logger) Println(v ...interface{}) {
	l.logger.Println(v...)
}

// Fatalf allows Fatalf'ing on the logger
func (l *Logger) Fatalf(format string, v ...interface{}) {
	if l.shouldSuppress(format) {
		return
	}
	l.logger.Fatalf(format, v...)
}

// ActivityLog is used for web activity logs
func (l *Logger) ActivityLog(message string, info LogInfo) {
	if l.shouldSuppress(message) {
		return
	}
	entry := l.getEntry(info)
	entry.WithField("activity", true).Info(message)
}

// ErrorLog log an error message
func (l *Logger) ErrorLog(message string, err error, info LogInfo) {
	if l.shouldSuppress(message) {
		return
	}
	entry := l.getEntry(info)
	entry.WithError(err).Error(message)
}

// SetJSONFormatter sets logger to emit JSON or false => TextFormatter
func (l *Logger) SetJSONFormatter(json bool) {
	if json {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{})
	}
}

// SetColorFormatter sets logger to show colors on Unix and Windows systems
func (l *Logger) SetColorFormatter(color bool) {
	if color {
		logrus.SetFormatter(&logrus.TextFormatter{ForceColors: true})
		logrus.SetOutput(colorable.NewColorableStdout())
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{DisableColors: true})
		logrus.SetOutput(os.Stdout)
	}
}

func (l *Logger) getEntry(info LogInfo) *logrus.Entry {
	if len(info) > 0 {
		entry := l.logger
		for k, v := range info {
			entry = entry.WithField(k, v)
		}
		return entry
	}

	return l.logger
}
