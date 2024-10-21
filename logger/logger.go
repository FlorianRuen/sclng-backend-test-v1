package logger

import (
	"strings"

	"github.com/Scalingo/sclng-backend-test-v1/config"
	"github.com/sirupsen/logrus"
)

// Setup will configure logrus logger
func Setup(cfg config.Config) {
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	if cfg.Logs.OutputLogsAsJSON {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}

	logrus.SetLevel(StringToLogrusLogType(cfg.Logs.Level))
}

// StringToLogrusLogType will convert string to the right logrus level
func StringToLogrusLogType(logLevel string) logrus.Level {
	logLevelLowerCase := strings.ToLower(logLevel)
	switch logLevelLowerCase {
	case "error":
		return logrus.ErrorLevel
	case "warn":
		return logrus.WarnLevel
	case "info":
		return logrus.InfoLevel
	case "debug":
		return logrus.DebugLevel
	default:
		return logrus.ErrorLevel
	}
}
