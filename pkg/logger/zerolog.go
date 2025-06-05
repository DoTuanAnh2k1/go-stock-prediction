package logger

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

type LoggerInterface interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Fatal(msg string, args ...interface{})
	Panic(msg string, args ...interface{})

	Infof(msg string, args ...interface{})
	Errorf(msg string, args ...interface{})
	Debugf(msg string, args ...interface{})
	Warnf(msg string, args ...interface{})
	Fatalf(msg string, args ...interface{})
	Panicf(msg string, args ...interface{})
}

type zrlog struct {
	zerolog.Logger
}

var Logger LoggerInterface = &zrlog{}

func Init() {
	Logger = Newzrlog()
}

func Newzrlog() LoggerInterface {

	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	output.FormatLevel = func(i interface{}) string {
		level := strings.ToLower(fmt.Sprintf("%s", i))
		switch level {
		case "debug":
			return "\033[36m[debug]\033[0m" // Cyan
		case "info":
			return "\033[32m[info ]\033[0m" // Green
		case "error":
			return "\033[31m[error]\033[0m" // Red
		case "warn":
			return "\033[33m[warn ]\033[0m" // Yellow
		case "fatal":
			return "\033[35m[fatal]\033[0m" // Magenta
		case "panic":
			return "\033[1;31m[panic]\033[0m" // Bold Red
		default:
			return fmt.Sprintf("[%s]", level)
		}
	}
	zs := &zrlog{
		Logger: zerolog.New(output).With().Timestamp().Logger().With().CallerWithSkipFrameCount(3).Logger(),
	}
	return zs
}

func (l *zrlog) Info(msg string, args ...interface{}) {
	l.Logger.Info().Msgf(msg, args...)
}

func (l *zrlog) Error(msg string, args ...interface{}) {
	l.Logger.Error().Msgf(msg, args...)
}

func (l *zrlog) Debug(msg string, args ...interface{}) {
	l.Logger.Debug().Msgf(msg, args...)
}

func (l *zrlog) Warn(msg string, args ...interface{}) {
	l.Logger.Warn().Msgf(msg, args...)
}

func (l *zrlog) Fatal(msg string, args ...interface{}) {
	l.Logger.Fatal().Msgf(msg, args...)
}

func (l *zrlog) Panic(msg string, args ...interface{}) {
	l.Logger.Panic().Msgf(msg, args...)
}

func (l *zrlog) Infof(msg string, args ...interface{}) {
	l.Logger.Info().Msgf(msg, args...)
}

func (l *zrlog) Errorf(msg string, args ...interface{}) {
	l.Logger.Error().Msgf(msg, args...)
}

func (l *zrlog) Debugf(msg string, args ...interface{}) {
	l.Logger.Debug().Msgf(msg, args...)
}

func (l *zrlog) Warnf(msg string, args ...interface{}) {
	l.Logger.Warn().Msgf(msg, args...)
}

func (l *zrlog) Fatalf(msg string, args ...interface{}) {
	l.Logger.Fatal().Msgf(msg, args...)
}

func (l *zrlog) Panicf(msg string, args ...interface{}) {
	l.Logger.Panic().Msgf(msg, args...)
}
