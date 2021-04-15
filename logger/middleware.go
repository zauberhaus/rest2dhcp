package logger

import (
	"net/http"
	"time"

	"github.com/justinas/alice"
	"github.com/rs/zerolog/hlog"
)

func NewLogMiddleware(host string, log Logger) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		chain := alice.New(log.NewLogHandler(host))

		chain = chain.Append(hlog.AccessHandler(func(r *http.Request, status, size int, duration time.Duration) {
			accept := r.Header.Get("Accept")

			evt := hlog.FromRequest(r).Info().
				Str("method", r.Method).
				Stringer("url", r.URL).
				Int("status", status).
				Int("size", size).
				Dur("duration", duration)

			if accept != "" {
				evt.Str("accept", accept)
			}

			evt.Msg("")
		}))

		chain = chain.Append(hlog.RemoteAddrHandler("ip"))
		chain = chain.Append(hlog.UserAgentHandler("user_agent"))
		chain = chain.Append(hlog.RefererHandler("referer"))
		chain = chain.Append(hlog.RemoteAddrHandler("remote_ip"))

		return chain.Then(h)

	}
}
