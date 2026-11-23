package jrpc

import (
	"net/http"
)

// Option func type
type Option func(s *Server)

// Auth sets basic auth credentials, optional.
// Auth enforced only if both user and password set to non-empty values, otherwise the server
// keeps serving every request unauthenticated. Setting just one of them doesn't enable auth.
func Auth(user, password string) Option {
	return func(s *Server) {
		s.authUser = user
		s.authPasswd = password
	}
}

// WithTimeouts sets server timeout values such as ReadHeader, Write and Idle timeout, optional.
// A zero field keeps the current value, by default 5s for ReadHeaderTimeout, 10s for WriteTimeout,
// 5s for IdleTimeout and no CallTimeout. A negative value turns the timeout off.
func WithTimeouts(timeouts Timeouts) Option {

	// this option sets only server limits values and exclude middlewares limits fields of Limits struct.
	return func(s *Server) {
		if timeouts.ReadHeaderTimeout != 0 {
			s.timeouts.ReadHeaderTimeout = timeouts.ReadHeaderTimeout
		}
		if timeouts.WriteTimeout != 0 {
			s.timeouts.WriteTimeout = timeouts.WriteTimeout
		}
		if timeouts.IdleTimeout != 0 {
			s.timeouts.IdleTimeout = timeouts.IdleTimeout
		}
		if timeouts.CallTimeout != 0 {
			s.timeouts.CallTimeout = timeouts.CallTimeout // it need for middleware with custom timeout value
		}
	}
}

// WithLimits sets value for client limit call/sec per client middleware
func WithLimits(limit float64) Option {

	// this option sets only server limits values and exclude middlewares limits fields of Limits struct.
	return func(s *Server) {
		s.limits.clientLimit = limit
	}
}

// WithThrottler sets throttler middleware with specify limit value, optional
func WithThrottler(limit int) Option {
	return func(s *Server) {
		s.limits.serverThrottle = limit
	}
}

// WithMiddlewares sets custom middlewares list, optional
func WithMiddlewares(middlewares ...func(http.Handler) http.Handler) Option {
	return func(s *Server) {
		s.customMiddlewares = append(s.customMiddlewares, middlewares...)
	}
}

// WithSignature sets signature data for server response headers
func WithSignature(appName, author, version string) Option {
	return func(s *Server) {
		s.signature = signaturePayload{
			appName: appName,
			author:  author,
			version: version,
		}
	}
}

// WithLogger sets custom logger, optional. A nil logger or a nil LoggerFunc is ignored and the server keeps
// the current one, NoOpLogger by default
func WithLogger(logger L) Option {
	return func(s *Server) {
		if fn, ok := logger.(LoggerFunc); logger == nil || (ok && fn == nil) {
			return
		}
		s.logger = logger
	}
}
