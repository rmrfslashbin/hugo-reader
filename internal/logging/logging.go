package logging

import (
	"log/slog"
	"os"

	"github.com/spf13/viper"
)

func New() *slog.Logger {
	logLevel := &slog.LevelVar{}
	loggerOps := &slog.HandlerOptions{
		Level: logLevel,
	}

	switch viper.GetString("log_level") {
	case "debug":
		logLevel.Set(slog.LevelDebug)
	case "info":
		logLevel.Set(slog.LevelInfo)
	case "warn":
		logLevel.Set(slog.LevelWarn)
	case "error":
		logLevel.Set(slog.LevelError)
	default:
		logLevel.Set(slog.LevelInfo)
	}

	servername := viper.GetString("server_name")
	if servername == "" {
		servername = "omdb"
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, loggerOps))
	slog.SetDefault(logger)
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}
	return logger.With(
		slog.Group("process_info",
			slog.Int("pid", os.Getpid()),
			slog.String("hostname", hostname),
			slog.String("servername", servername),
		),
	)
}
