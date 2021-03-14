package logger

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
)

type Zerologger struct {
	log zerolog.Logger
}

func New() Logger {
	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339Nano}).With().Timestamp().Logger()
	log.GetLevel()
	return Zerologger{log: log}
}

func (l Zerologger) Fatal(args ...interface{}) {
	l.log.Fatal().Msg(fmt.Sprint(args...))
}

func (l Zerologger) Fatalf(format string, args ...interface{}) {
	l.log.Fatal().Msg(fmt.Sprintf(format, args...))
}

func (l Zerologger) Fatalln(args ...interface{}) {
	l.Fatal(args...)
}

func (l Zerologger) Error(args ...interface{}) {
	l.log.Error().Msg(fmt.Sprint(args...))
}

func (l Zerologger) Errorf(format string, args ...interface{}) {
	l.log.Error().Msg(fmt.Sprintf(format, args...))
}

func (l Zerologger) Errorln(args ...interface{}) {
	l.Error(args...)
}

func (l Zerologger) Info(args ...interface{}) {
	l.log.Info().Msg(fmt.Sprint(args...))
}

func (l Zerologger) Infof(format string, args ...interface{}) {
	l.log.Info().Msg(fmt.Sprintf(format, args...))
}

func (l Zerologger) Infoln(args ...interface{}) {
	l.Info(args...)
}

func (l Zerologger) Warning(args ...interface{}) {
	l.log.Warn().Msg(fmt.Sprint(args...))
}

func (l Zerologger) Warningf(format string, args ...interface{}) {
	l.log.Warn().Msg(fmt.Sprintf(format, args...))
}

func (l Zerologger) Warningln(args ...interface{}) {
	l.Warning(args...)
}

func (l Zerologger) Debug(args ...interface{}) {
	l.log.Debug().Msg(fmt.Sprint(args...))
}

func (l Zerologger) Debugln(args ...interface{}) {
	l.Debug(args...)
}

func (l Zerologger) Debugf(format string, args ...interface{}) {
	l.log.Debug().Msg(fmt.Sprintf(format, args...))
}

func (l Zerologger) Trace(args ...interface{}) {
	l.log.Trace().Msg(fmt.Sprint(args...))
}

func (l Zerologger) Traceln(args ...interface{}) {
	l.Trace(args...)
}

func (l Zerologger) Tracef(format string, args ...interface{}) {
	l.log.Trace().Msg(fmt.Sprintf(format, args...))
}

func (l Zerologger) NewLogHandler(host string) func(http.Handler) http.Handler {
	log := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("host", host).
		Logger()

	return hlog.NewHandler(log)
}

func (l Zerologger) V(level int) bool {
	return true
}
