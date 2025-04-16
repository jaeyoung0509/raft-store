package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger

func InitLogger(debug bool) {
	var cfg zap.Config
	if debug {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		cfg = zap.NewProductionConfig()
	}

	var err error
	logger, err = cfg.Build()
	if err != nil {

	}
}

func Sync() error {
	if logger != nil {
		return logger.Sync()
	}
	return nil
}

func L() *zap.Logger {
	return logger
}
