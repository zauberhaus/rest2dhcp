package background

import "golang.org/x/net/context"

// Server is a simple server interface
type Server interface {
	Init(ctx context.Context, args ...interface{})
	Start(ctx context.Context) chan bool
	Done() chan bool

	Hostname() string
	Port() uint16
}
