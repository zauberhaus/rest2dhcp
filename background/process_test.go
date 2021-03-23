package background_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zauberhaus/rest2dhcp/background"
	"github.com/zauberhaus/rest2dhcp/mock"
)

func TestBackgroundProcessFinish(t *testing.T) {
	val := 0

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 4, 1, 0)

	process := background.Process{}
	process.Init(t.Name(), nil, nil, logger)

	p := func(ctx context.Context) bool {
		return exec(ctx, 10*time.Millisecond, func(p ...interface{}) bool {
			val := p[0].(*int)

			logger.Infof("tick %v", time.Now())
			*val = *val + 1

			return true
		}, &val)
	}

	c := process.Run(p)
	<-c

	process.Wait()

	assert.Equal(t, 1, val)
}

func TestBackgroundProcessFinishAndStop(t *testing.T) {
	val := 0

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 4, 2, 0)

	process := background.Process{}
	process.Init(t.Name(), nil, nil, logger)

	p := func(ctx context.Context) bool {
		return exec(ctx, 10*time.Millisecond, func(p ...interface{}) bool {
			val := p[0].(*int)

			logger.Infof("tick %v", time.Now())
			*val = *val + 1

			return true
		}, &val)
	}

	<-process.Run(p)

	time.Sleep(50 * time.Millisecond)

	process.Stop()

	assert.Equal(t, 1, val)
}

func TestBackgroundProcessCancel(t *testing.T) {
	val := 0
	started := make(chan bool)

	logger := mock.NewTestLogger()

	process := background.Process{}
	process.Init(t.Name(), nil, nil, logger)

	p := func(ctx context.Context) bool {
		close(started)
		return exec(ctx, 75*time.Millisecond, func(p ...interface{}) bool {
			val := p[0].(*int)

			logger.Infof("tick %v", time.Now())
			*val = *val + 1

			return false
		}, &val)
	}

	process.Run(p)
	<-started

	time.Sleep(220 * time.Millisecond)

	process.Stop()
}

func TestBackgroundProcessInitSchutdown(t *testing.T) {
	val := 0

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 0, 0, 5, 3, 0)

	i := func(ctx context.Context) error {
		val = 1

		return nil
	}

	s := func(ctx context.Context) error {
		val += 10

		return nil
	}

	process := background.Process{}
	process.Init(t.Name(), i, s, logger)

	p := func(ctx context.Context) bool {
		return exec(ctx, 10*time.Millisecond, func(p ...interface{}) bool {
			val := p[0].(*int)

			logger.Infof("tick %v", time.Now())
			*val = *val + 1

			return true
		}, &val)
	}

	c := process.Run(p)
	<-c

	assert.Equal(t, 1, val)

	process.Wait()

	assert.Equal(t, 12, val)
}

func TestBackgroundProcessInitFailed(t *testing.T) {
	val := 0

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 1, 0, 3, 0, 0)

	i := func(ctx context.Context) error {
		val = 99

		return fmt.Errorf("Init failed")
	}

	s := func(ctx context.Context) error {
		val += 10

		return nil
	}

	process := background.Process{}
	process.Init(t.Name(), i, s, logger)

	p := func(ctx context.Context) bool {
		return exec(ctx, 10*time.Millisecond, func(p ...interface{}) bool {
			val := p[0].(*int)

			logger.Infof("tick %v", time.Now())
			*val = *val + 1

			return true
		}, &val)
	}

	c := process.Run(p)
	<-c

	assert.Equal(t, 99, val)

	process.Wait()

	assert.Equal(t, 99, val)
}

func TestBackgroundProcessShutdownFailed(t *testing.T) {
	val := 0

	logger := mock.NewTestLogger()
	defer logger.Assert(t, 0, 1, 0, 5, 2, 0)

	i := func(ctx context.Context) error {
		val = 99
		return nil
	}

	s := func(ctx context.Context) error {
		val += 10

		return fmt.Errorf("Shutdown failed")
	}

	process := background.Process{}
	process.Init(t.Name(), i, s, logger)

	p := func(ctx context.Context) bool {
		return exec(ctx, 10*time.Millisecond, func(p ...interface{}) bool {
			val := p[0].(*int)

			logger.Infof("tick %v", time.Now())
			*val = *val + 1

			return true
		}, &val)
	}

	c := process.Run(p)
	<-c

	assert.Equal(t, 99, val)

	process.Wait()

	assert.Equal(t, 110, val)
}

func exec(ctx context.Context, timeout time.Duration, f func(...interface{}) bool, param ...interface{}) bool {
	timer := time.NewTimer(1 * time.Second)
	defer timer.Stop()

	for {
		timer.Reset(timeout)

		select {
		case <-ctx.Done():
			return false
		case <-timer.C:
			if f(param...) {
				return true
			}
		}
	}
}
