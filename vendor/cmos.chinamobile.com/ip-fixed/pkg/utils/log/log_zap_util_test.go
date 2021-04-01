package log

import (
	"go.uber.org/zap"
	"testing"
)

func TestLogZapUtil(t *testing.T) {
	var (
		logLevel      = "info"
		logFilePath   = "/var/log/log-zap-utils.log"
		logMaxSize    = 1024
		logMaxBackups = 0
		logMaxAge     = 3
	)
	logger := InitLogger(logLevel, logFilePath, logMaxSize, logMaxBackups, logMaxAge, true)
	zap.ReplaceGlobals(logger)
	defer logger.Sync()

	logger.Warn("warn log")
	logger.Error("error log")
	logger.Info("info log")
	//logger.Panic("panic log")
}

func BenchmarkInitLogger(b *testing.B) {
	var (
		logLevel      = "info"
		logFilePath   = "/var/log/log-zap-utils.log"
		logMaxSize    = 1024
		logMaxBackups = 0
		logMaxAge     = 3
	)

	for i := 0; i < b.N; i++ {
		logger := InitLogger(logLevel, logFilePath, logMaxSize, logMaxBackups, logMaxAge, true)
		zap.ReplaceGlobals(logger)
		logger.Error("error log", zap.String("key", "value"))
		defer logger.Sync()
	}
}
