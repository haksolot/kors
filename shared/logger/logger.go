package logger

import (
    "os"
    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
)

func Init(serviceName string) zerolog.Logger {
    level := zerolog.InfoLevel
    if os.Getenv("APP_ENV") == "development" {
        level = zerolog.DebugLevel
        log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
    }
    zerolog.SetGlobalLevel(level)
    return log.With().Str("service", serviceName).Logger()
}