package mock

import (
	"net/http"
	"os"
	"sync/atomic"

	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	logger "github.com/zauberhaus/rest2dhcp/logger"
)

type Testlogger struct {
	log logger.Logger

	ErrorCount   int64
	FatalCount   int64
	InfoCount    int64
	DebugCount   int64
	TraceCount   int64
	WarnCount    int64
	TestCount    int64
	RequestCount int64
}

func NewTestLogger() *Testlogger {
	log := logger.New()
	return &Testlogger{log: log}
}

func (l *Testlogger) Assert(t gomock.TestReporter, val ...int64) {
	if len(val) > 0 && val[0] >= 0 {
		v := atomic.LoadInt64(&l.FatalCount)
		assert.Equal(t, val[0], v, "Fatal count")
	}

	if len(val) > 1 && val[1] >= 0 {
		v := atomic.LoadInt64(&l.ErrorCount)
		assert.Equal(t, val[1], v, "Error count")
	}

	if len(val) > 2 && val[2] >= 0 {
		v := atomic.LoadInt64(&l.WarnCount)
		assert.Equal(t, val[2], v, "Warn count")
	}

	if len(val) > 3 && val[3] >= 0 {
		v := atomic.LoadInt64(&l.InfoCount)
		assert.Equal(t, val[3], v, "Info count")
	}

	if len(val) > 4 && val[4] >= 0 {
		v := atomic.LoadInt64(&l.DebugCount)
		assert.Equal(t, val[4], v, "Debug count")
	}

	if len(val) > 5 && val[5] >= 0 {
		v := atomic.LoadInt64(&l.TraceCount)
		assert.Equal(t, val[5], v, "Trace count")
	}

	if len(val) > 6 && val[6] >= 0 {
		v := atomic.LoadInt64(&l.TestCount)
		assert.Equal(t, val[6], v, "Test count")
	}

	if len(val) > 7 && val[7] >= 0 {
		v := atomic.LoadInt64(&l.RequestCount)
		assert.Equal(t, val[7], v, "Request count")
	}
}

func (l *Testlogger) Fatal(args ...interface{}) {
	atomic.AddInt64(&l.FatalCount, 1)
	l.log.Error(args...)
}

func (l *Testlogger) Fatalf(format string, args ...interface{}) {
	atomic.AddInt64(&l.FatalCount, 1)
	l.log.Errorf(format, args...)
}

func (l *Testlogger) Fatalln(args ...interface{}) {
	atomic.AddInt64(&l.FatalCount, 1)
	l.log.Error(args...)
}

func (l *Testlogger) Error(args ...interface{}) {
	atomic.AddInt64(&l.ErrorCount, 1)
	l.log.Error(args...)
}

func (l *Testlogger) Errorf(format string, args ...interface{}) {
	atomic.AddInt64(&l.ErrorCount, 1)
	l.log.Errorf(format, args...)
}

func (l *Testlogger) Errorln(args ...interface{}) {
	atomic.AddInt64(&l.ErrorCount, 1)
	l.log.Error(args...)
}

func (l *Testlogger) Info(args ...interface{}) {
	atomic.AddInt64(&l.InfoCount, 1)
	l.log.Info(args...)
}

func (l *Testlogger) Infof(format string, args ...interface{}) {
	atomic.AddInt64(&l.InfoCount, 1)
	l.log.Infof(format, args...)
}

func (l *Testlogger) Infoln(args ...interface{}) {
	atomic.AddInt64(&l.InfoCount, 1)
	l.log.Info(args...)
}

func (l *Testlogger) Warning(args ...interface{}) {
	l.WarnCount++
	l.log.Warning(args...)
}

func (l *Testlogger) Warningf(format string, args ...interface{}) {
	atomic.AddInt64(&l.WarnCount, 1)
	l.log.Warningf(format, args...)
}

func (l *Testlogger) Warningln(args ...interface{}) {
	atomic.AddInt64(&l.WarnCount, 1)
	l.log.Warning(args...)
}

func (l *Testlogger) Debug(args ...interface{}) {
	atomic.AddInt64(&l.DebugCount, 1)
	l.log.Debug(args...)
}

func (l *Testlogger) Debugln(args ...interface{}) {
	atomic.AddInt64(&l.DebugCount, 1)
	l.log.Debug(args...)
}

func (l *Testlogger) Debugf(format string, args ...interface{}) {
	atomic.AddInt64(&l.DebugCount, 1)
	l.log.Debugf(format, args...)
}

func (l *Testlogger) Trace(args ...interface{}) {
	atomic.AddInt64(&l.TraceCount, 1)
	l.log.Trace(args...)
}

func (l *Testlogger) Traceln(args ...interface{}) {
	atomic.AddInt64(&l.TraceCount, 1)
	l.log.Trace(args...)
}

func (l *Testlogger) Tracef(format string, args ...interface{}) {
	atomic.AddInt64(&l.TraceCount, 1)
	l.log.Tracef(format, args...)
}

func (l *Testlogger) Test(args ...interface{}) {
	atomic.AddInt64(&l.TestCount, 1)
	a := []interface{}{"----- "}
	a = append(a, args...)
	a = append(a, " ------")

	l.log.Trace(a...)
}

func (l *Testlogger) Testf(format string, args ...interface{}) {
	atomic.AddInt64(&l.TestCount, 1)
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
