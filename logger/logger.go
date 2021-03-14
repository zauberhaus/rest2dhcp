package logger

import (
	"net/http"

	"google.golang.org/grpc/grpclog"
)

type Logger interface {
	grpclog.LoggerV2

	// Debug logs to DEBUG log. Arguments are handled in the manner of fmt.Print.
	Debug(args ...interface{})
	// Debugln logs to DEBUG log. Arguments are handled in the manner of fmt.Println.
	Debugln(args ...interface{})
	// Debugf logs to DEBUG log. Arguments are handled in the manner of fmt.Printf.
	Debugf(format string, args ...interface{})

	// Trace logs to TRACE log. Arguments are handled in the manner of fmt.Print.
	Trace(args ...interface{})
	// Traceln logs to TRACE log. Arguments are handled in the manner of fmt.Println.
	Traceln(args ...interface{})
	// Tracef logs to TRACE log. Arguments are handled in the manner of fmt.Printf.
	Tracef(format string, args ...interface{})

	NewLogHandler(host string) func(http.Handler) http.Handler

	V(level int) bool
}
