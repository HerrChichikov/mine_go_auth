package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Log  *zap.Logger
	once sync.Once
)

func LoggerInit(env string) {
	once.Do(func() {
		var cfg zap.Config
		var opts []zap.Option

		if env == "production" {
			cfg = zap.NewProductionConfig()
			cfg.EncoderConfig.TimeKey = "timestamp"
			cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

			core := zapcore.NewCore(
				zapcore.NewJSONEncoder(cfg.EncoderConfig),
				zapcore.NewMultiWriteSyncer(
					zapcore.AddSync(os.Stdout),
				),
				zap.InfoLevel,
			)
			Log = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel))
		} else {
			cfg = zap.NewDevelopmentConfig()
			cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
			cfg.EncoderConfig.TimeKey = "time"
			cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
			cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)

			opts = append(opts, zap.AddCaller(), zap.AddStacktrace(zap.WarnLevel))
			Log, _ = cfg.Build(opts...)
		}

		zap.ReplaceGlobals(Log)
	})
}

func LoggerSync() {
	if Log != nil {
		_ = Log.Sync()
	}
}