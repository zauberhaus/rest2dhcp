package mock

import (
	"net/http"
	"os"

	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	logger "github.com/zauberhaus/rest2dhcp/logger"
)

type Testlogger struct {
	log logger.Logger

	ErrorCount   int
	FatalCount   int
	InfoCount    int
	DebugCount   int
	TraceCount   int
	WarnCount    int
	TestCount    int
	RequestCount int
}

func NewTestLogger() *Testlogger {
	log := logger.New()
	return &Testlogger{log: log}
}

func (l *Testlogger) Assert(t gomock.TestReporter, val ...int) {
	if len(val) > 0 && val[0] >= 0 {
		assert.Equal(t, val[0], l.FatalCount, "Fatal count")
	}

	if len(val) > 1 && val[1] >= 0 {
		assert.Equal(t, val[1], l.ErrorCount, "Error count")
	}

	if len(val) > 2 && val[2] >= 0 {
		assert.Equal(t, val[2], l.WarnCount, "Warn count")
	}

	if len(val) > 3 && val[3] >= 0 {
		assert.Equal(t, val[3], l.InfoCount, "Info count")
	}

	if len(val) > 4 && val[4] >= 0 {
		assert.Equal(t, val[4], l.DebugCount, "Debug count")
	}

	if len(val) > 5 && val[5] >= 0 {
		assert.Equal(t, val[5], l.TraceCount, "Trace count")
	}

	if len(val) > 6 && val[6] >= 0 {
		assert.Equal(t, val[6], l.TestCount, "Test count")
	}

	if len(val) > 7 && val[7] >= 0 {
		assert.Equal(t, val[7], l.RequestCount, "Request count")
	}
}

func (l *Testlogger) Fatal(args ...interface{}) {
	l.FatalCount++
	l.log.Error(args...)
}

func (l *Testlogger) Fatalf(format string, args ...interface{}) {
	l.FatalCount++
	l.log.Errorf(format, args...)
}

func (l *Testlogger) Fatalln(args ...interface{}) {
	l.FatalCount++
	l.log.Error(args...)
}

func (l *Testlogger) Error(args ...interface{}) {
	l.ErrorCount++
	l.log.Error(args...)
}

func (l *Testlogger) Errorf(format string, args ...interface{}) {
	l.ErrorCount++
	l.log.Errorf(format, args...)
}

func (l *Testlogger) Errorln(args ...interface{}) {
	l.ErrorCount++
	l.log.Error(args...)
}

func (l *Testlogger) Info(args ...interface{}) {
	l.InfoCount++
	l.log.Info(args...)
}

func (l *Testlogger) Infof(format string, args ...interface{}) {
	l.InfoCount++
	l.log.Infof(format, args...)
}

func (l *Testlogger) Infoln(args ...interface{}) {
	l.InfoCount++
	l.log.Info(args...)
}

func (l *Testlogger) Warning(args ...interface{}) {
	l.WarnCount++
	l.log.Warning(args...)
}

func (l *Testlogger) Warningf(format string, args ...interface{}) {
	l.WarnCount++
	l.log.Warningf(format, args...)
}

func (l *Testlogger) Warningln(args ...interface{}) {
	l.WarnCount++
	l.log.Warning(args...)
}

func (l *Testlogger) Debug(args ...interface{}) {
	l.DebugCount++
	l.log.Debug(args...)
}

func (l *Testlogger) Debugln(args ...interface{}) {
	l.DebugCount++
	l.log.Debug(args...)
}

func (l *Testlogger) Debugf(format string, args ...interface{}) {
	l.DebugCount++
	l.log.Debugf(format, args...)
}

func (l *Testlogger) Trace(args ...interface{}) {
	l.TraceCount++
	l.log.Trace(args...)
}

func (l *Testlogger) Traceln(args ...interface{}) {
	l.TraceCount++
	l.log.Trace(args...)
}

func (l *Testlogger) Tracef(format string, args ...interface{}) {
	l.TraceCount++
	l.log.Tracef(format, args...)
}

func (l *Testlogger) Test(args ...interface{}) {
	l.TestCount++
	a := []interface{}{"----- "}
	a = append(a, args...)
	a = append(a, " ------")

	l.log.Trace(a...)
}

func (l *Testlogger) Testf(format string, args ...interface{}) {
	l.TestCount++
	l.log.Tracef("------ "+format+" ------", args...)
}

func (l *Testlogger) NewLogHandler(host string) func(http.Handler) http.Handler {
	log := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("host", host).
		Logger()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			l.RequestCount++
			r = r.WithContext(log.WithContext(r.Context()))
			next.ServeHTTP(w, r)
		})
	}
}

func (l Testlogger) V(level int) bool {
	return l.log.V(level)
}
