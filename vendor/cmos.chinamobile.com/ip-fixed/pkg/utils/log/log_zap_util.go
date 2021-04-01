package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// 初始化zap日志库
// logLevel 日志级别
//------日志切割归档相关配置（使用第三方库Lumberjack来实现）------
// logFilePath 日志文件绝对路径
// logMaxSize 日志文件的最大大小（以MB为单位）
// logMaxBackups 保留旧日志文件的最大个数
// logMaxAge 保留旧日志文件的最大天数
// logCompress 是否压缩/归档旧文件
func InitLogger(logLevel, logFilePath string, logMaxSize, logMaxBackups, logMaxAge int, logCompress bool) *zap.Logger {
	encoder := getLogEncoder()
	hook := lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    logMaxSize,
		MaxBackups: logMaxBackups,
		MaxAge:     logMaxAge,
		Compress:   logCompress,
	}
	writeSyncer := getLogWriter(hook)
	level := getLogLevel(logLevel)
	core := zapcore.NewCore(encoder, writeSyncer, level)
	//return zap.New(core)
	return zap.New(core, zap.AddCaller())
}

func getLogLevel(logLevel string) zapcore.Level {
	var level zapcore.Level
	switch logLevel {
	case "debug":
		level = zap.DebugLevel
	case "info":
		level = zap.InfoLevel
	case "error":
		level = zap.ErrorLevel
	default:
		level = zap.InfoLevel
	}
	return level
}

func getLogEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder   // ISO8601 UTC时间格式
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder // 大写编码器
	return zapcore.NewConsoleEncoder(encoderConfig)
}

func getLogWriter(hook lumberjack.Logger) zapcore.WriteSyncer {
	return zapcore.AddSync(&hook)
}
