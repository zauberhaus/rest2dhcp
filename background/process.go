package background

import (
	"github.com/zauberhaus/rest2dhcp/logger"
	"golang.org/x/net/context"
)

type Process struct {
	name   string
	ctx    context.Context
	cancel context.CancelFunc
	done   chan bool
	init   func(ctx context.Context) error
	close  func(ctx context.Context) error
	logger logger.Logger
}

func (p *Process) Init(name string, init func(ctx context.Context) error, close func(ctx context.Context) error, logger logger.Logger) {
	ctx, cancel := context.WithCancel(context.Background())
	p.ctx = ctx
	p.cancel = cancel
	p.name = name
	p.done = make(chan bool, 1)
	p.init = init
	p.close = close
	p.logger = logger
}

func (p *Process) Run(process func(ctx context.Context) bool) chan bool {

	rc := make(chan bool, 1)

	go func() {
		if p.init != nil {
			p.logger.Infof("Init %s", p.name)
			if err := p.init(p.ctx); err != nil {
				p.logger.Errorf("Init %s failed: %v", p.name, err)
				close(p.done)
				close(rc)
				return
			}
		}
		p.logger.Infof("%s started", p.name)
		close(rc)
		if process != nil {
			if process(p.ctx) {
				p.logger.Debugf("%s finished", p.name)
			} else {
				p.logger.Debugf("%s cancelled", p.name)
			}

			if p.close != nil {
				p.logger.Debugf("%s wait for shutdown", p.name)

				if err := p.close(p.ctx); err == nil {
					p.logger.Debugf("%s shutdown finished.", p.name)

				} else {
					p.logger.Errorf("%s shutdown failed: %v", p.name, err)
				}
			}

			close(p.done)
		}
	}()

	return rc
}

func (p *Process) Wait() {
	p.logger.Infof("%s wait for done", p.name)
	<-p.done
	p.logger.Infof("%s done", p.name)
}

func (p *Process) Stop() {
	p.logger.Infof("%s shutdown start", p.name)
	p.cancel()
	p.logger.Debugf("%s wait for shutdown done", p.name)
	<-p.done
	p.logger.Infof("%s done", p.name)
}
