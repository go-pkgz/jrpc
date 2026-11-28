package jrpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerPrimitiveTypes(t *testing.T) {
	s := NewServer("/v1/cmd")

	type respData struct {
		Res1 string
		Res2 bool
	}

	s.Add("test", func(id uint64, params json.RawMessage) Response {
		var args []any
		if err := json.Unmarshal(params, &args); err != nil {
			return Response{Error: err.Error()}
		}
		t.Logf("%+v", args)

		assert.Equal(t, 3, len(args))
		assert.Equal(t, "blah", args[0].(string))
		assert.Equal(t, 42., args[1].(float64))
		assert.Equal(t, true, args[2].(bool))

		return EncodeResponse(id, respData{"res blah", true}, nil)
	})

	url := startServer(t, s)

	// check with direct http call
	clientReq := Request{Method: "test", Params: []any{"blah", 42, true}, ID: 123}
	b := bytes.Buffer{}
	require.NoError(t, json.NewEncoder(&b).Encode(clientReq))
	resp, err := http.Post(url+"/v1/cmd", "application/json", &b)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, 200, resp.StatusCode)
	data, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, `{"result":{"Res1":"res blah","Res2":true},"id":123}`+"\n", string(data))

	// check with client call
	c := Client{API: url + "/v1/cmd", Client: http.Client{}}
	r, err := c.Call("test", "blah", 42, true)
	assert.NoError(t, err)
	assert.Equal(t, "", r.Error)

	res := respData{}
	err = json.Unmarshal(*r.Result, &res)
	assert.NoError(t, err)
	assert.Equal(t, respData{Res1: "res blah", Res2: true}, res)
	assert.Equal(t, uint64(1), r.ID)
}

func TestServerWithObject(t *testing.T) {
	s := NewServer("/v1/cmd")

	type respData struct {
		Res1 string
		Res2 bool
	}

	type reqData struct {
		Time time.Time
		F1   string
		F2   time.Duration
	}

	s.Add("test", func(id uint64, params json.RawMessage) Response {
		arg := reqData{}
		if err := json.Unmarshal(params, &arg); err != nil {
			return Response{Error: err.Error()}
		}
		t.Logf("%+v", arg)
		return EncodeResponse(id, respData{"res blah", true}, nil)
	})

	url := startServer(t, s)

	c := Client{API: url + "/v1/cmd", Client: http.Client{}}
	r, err := c.Call("test", reqData{Time: time.Now(), F1: "sawert", F2: time.Minute})
	assert.NoError(t, err)
	assert.Equal(t, "", r.Error)

	res := respData{}
	err = json.Unmarshal(*r.Result, &res)
	assert.NoError(t, err)
	assert.Equal(t, respData{Res1: "res blah", Res2: true}, res)
}

func TestServerMethodNotImplemented(t *testing.T) {
	s := NewServer("")

	ts := httptest.NewServer(http.HandlerFunc(s.handler))
	defer ts.Close()
	s.Add("test", func(_ uint64, params json.RawMessage) Response {
		return Response{}
	})

	r := Request{Method: "blah"}
	buf := bytes.Buffer{}
	assert.NoError(t, json.NewEncoder(&buf).Encode(r))
	resp, err := http.Post(ts.URL, "application/json", &buf)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusNotImplemented, resp.StatusCode)

	assert.EqualError(t, s.Shutdown(), "http server is not running")
}

func TestServerWithAuth(t *testing.T) {
	s := NewServer("/v1/cmd", Auth("user", "passwd"))

	s.Add("test", func(id uint64, params json.RawMessage) Response {
		var args []any
		if err := json.Unmarshal(params, &args); err != nil {
			return Response{Error: err.Error()}
		}
		t.Logf("%+v", args)

		assert.Equal(t, 3, len(args))
		assert.Equal(t, "blah", args[0].(string))
		assert.Equal(t, 42., args[1].(float64))
		assert.Equal(t, true, args[2].(bool))

		return EncodeResponse(id, "res blah", nil)
	})

	url := startServer(t, s)

	c := Client{API: url + "/v1/cmd", Client: http.Client{}, AuthUser: "user", AuthPasswd: "passwd"}
	r, err := c.Call("test", "blah", 42, true)
	assert.NoError(t, err)
	assert.Equal(t, "", r.Error)
	val := ""
	err = json.Unmarshal(*r.Result, &val)
	assert.NoError(t, err)
	assert.Equal(t, "res blah", val)

	c = Client{API: url + "/v1/cmd", Client: http.Client{}}
	_, err = c.Call("test", "blah", 42, true)
	assert.EqualError(t, err, "bad status 401 Unauthorized for test")
}

func TestServerErrReturn(t *testing.T) {
	s := NewServer("/v1/cmd", Auth("user", "passwd"))

	s.Add("test", func(id uint64, params json.RawMessage) Response {
		var args []any
		if err := json.Unmarshal(params, &args); err != nil {
			return Response{Error: err.Error()}
		}
		t.Logf("%+v", args)

		assert.Equal(t, 3, len(args))
		assert.Equal(t, "blah", args[0].(string))
		assert.Equal(t, 42., args[1].(float64))
		assert.Equal(t, true, args[2].(bool))

		return EncodeResponse(id, "res blah", fmt.Errorf("some error"))
	})

	url := startServer(t, s)

	c := Client{API: url + "/v1/cmd", Client: http.Client{}, AuthUser: "user", AuthPasswd: "passwd"}
	_, err := c.Call("test", "blah", 42, true)
	assert.EqualError(t, err, "some error")
}

func TestServerGroup(t *testing.T) {
	s := NewServer("/v1/cmd")

	s.Group("pre", HandlersGroup{
		"fn1": func(uint64, json.RawMessage) Response {
			return Response{}
		},
		"fn2": func(uint64, json.RawMessage) Response {
			return Response{}
		},
	})

	url := startServer(t, s)

	c := Client{API: url + "/v1/cmd", Client: http.Client{}}
	_, err := c.Call("fn1")
	assert.EqualError(t, err, "bad status 501 Not Implemented for fn1")

	_, err = c.Call("pre.fn1")
	assert.NoError(t, err)
	_, err = c.Call("pre.fn2")
	assert.NoError(t, err)
}

func TestServerAddLate(t *testing.T) {
	s := NewServer("/v1/cmd")

	s.Add("fn1", func(id uint64, params json.RawMessage) Response {
		return Response{}
	})
	url := startServer(t, s)

	// too late, ignored after run
	s.Add("fn2", func(id uint64, params json.RawMessage) Response {
		return Response{}
	})

	c := Client{API: url + "/v1/cmd", Client: http.Client{}}
	_, err := c.Call("fn1")
	assert.NoError(t, err)
	_, err = c.Call("fn2")
	assert.EqualError(t, err, "bad status 501 Not Implemented for fn2")
}

func TestServerNoHandlers(t *testing.T) {
	s := NewServer("/v1/cmd", Auth("user", "passwd"))
	assert.EqualError(t, s.Run(0), "nothing mapped for dispatch, Add has to be called prior to Run")
}

func TestServer_getDefaultTimeouts(t *testing.T) {
	s := NewServer("")
	s.timeouts = getDefaultTimeouts()
	assert.Equal(t, Timeouts{ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 5 * time.Second}, s.timeouts)

	// test with custom timeouts
	s = NewServer("",
		WithTimeouts(Timeouts{ReadHeaderTimeout: 1 * time.Second, WriteTimeout: 4 * time.Second, IdleTimeout: 2 * time.Second}))

	assert.Equal(t, Timeouts{ReadHeaderTimeout: 1 * time.Second, WriteTimeout: 4 * time.Second, IdleTimeout: 2 * time.Second}, s.timeouts)

	// zero fields keep the defaults
	s = NewServer("/v1/cmd", WithTimeouts(Timeouts{CallTimeout: time.Second}))
	s.Add("fn1", func(id uint64, params json.RawMessage) Response { return Response{} })
	s.activate()
	assert.Equal(t, Timeouts{ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 5 * time.Second,
		CallTimeout: time.Second}, s.timeouts)
	assert.Equal(t, 5*time.Second, s.httpServer.ReadHeaderTimeout)
	assert.Equal(t, 10*time.Second, s.httpServer.WriteTimeout)
	assert.Equal(t, 5*time.Second, s.httpServer.IdleTimeout)

	// zero fields keep the value set by an earlier option
	s = NewServer("", WithTimeouts(Timeouts{WriteTimeout: 30 * time.Second}), WithTimeouts(Timeouts{CallTimeout: time.Second}),
		WithTimeouts(Timeouts{}))
	assert.Equal(t, Timeouts{ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 5 * time.Second,
		CallTimeout: time.Second}, s.timeouts)

	// negative values turn the server timeouts off
	s = NewServer("", WithTimeouts(Timeouts{ReadHeaderTimeout: -1, WriteTimeout: -1, IdleTimeout: -1}))
	assert.Equal(t, Timeouts{ReadHeaderTimeout: -1, WriteTimeout: -1, IdleTimeout: -1}, s.timeouts)
}

func TestServer_WithLimits(t *testing.T) {
	s := NewServer("")
	assert.Equal(t, limits{}, s.limits)

	// test with custom limit
	s = NewServer("",
		WithLimits(100))

	assert.Equal(t, limits{clientLimit: 100}, s.limits)

	// zero and negative limits turn the per-client limit off
	for _, limit := range []float64{0, -1} {
		s = NewServer("/v1/cmd", WithLimits(limit))
		s.Add("fn1", func(id uint64, params json.RawMessage) Response { return Response{} })
		url := startServer(t, s)
		client := &http.Client{Transport: &http.Transport{}}
		for range 5 {
			resp, err := client.Post(url+"/v1/cmd", "application/json", bytes.NewBufferString(`{"method":"fn1"}`))
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode, "limit %v", limit)
			_ = resp.Body.Close()
		}
		client.CloseIdleConnections()
	}
}

func TestServer_WithThrottler(t *testing.T) {
	s := NewServer("")
	assert.Equal(t, limits{}, s.limits)

	// test with custom limit
	s = NewServer("",
		WithThrottler(200))

	assert.Equal(t, limits{serverThrottle: 200}, s.limits)
}

func TestServer_WithSignature(t *testing.T) {

	checkSignatureMiddlewareFn := func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "", w.Header().Get("App-Name"))
			assert.Equal(t, "", w.Header().Get("App-Version"))
			assert.Equal(t, "", w.Header().Get("Author"))

			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
	s := NewServer("/v1/cmd", WithMiddlewares(checkSignatureMiddlewareFn))
	s.Add("fn1", func(id uint64, params json.RawMessage) Response {
		return Response{}
	})
	url := startServer(t, s)

	c := Client{API: url + "/v1/cmd", Client: http.Client{}}
	_, err := c.Call("fn1")
	assert.NoError(t, err)

	// checking signature with server response
	checkSignatureMiddlewareFn = func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "testApp", w.Header().Get("App-Name"))
			assert.Equal(t, "testAuthor", w.Header().Get("Author"))
			assert.Equal(t, "0.1.0", w.Header().Get("App-Version"))

			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}

	s = NewServer("/v1/cmd", WithSignature("testApp", "testAuthor", "0.1.0"), WithMiddlewares(checkSignatureMiddlewareFn))
	assert.Equal(t, signaturePayload{"testApp", "testAuthor", "0.1.0"}, s.signature)

	s.Add("fn1", func(id uint64, params json.RawMessage) Response {
		return Response{}
	})

	signedURL := startServer(t, s)

	c = Client{API: signedURL + "/v1/cmd", Client: http.Client{}}
	_, err = c.Call("fn1")
	assert.NoError(t, err)
}

func TestServerCustomMiddlewares(t *testing.T) {

	testMiddlewareGetPathFn := func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Method, "POST")
			assert.Equal(t, r.URL.Path, "/v1/cmd")

			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}

	testMiddlewareGetValueFn := func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Method, "POST")
			assert.Equal(t, r.URL.Path, "/v1/cmd")

			value, isValue := r.URL.Query()["value"]
			assert.True(t, isValue)
			assert.Equal(t, value[0], "test")

			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
	s := NewServer("/v1/cmd", WithMiddlewares(testMiddlewareGetPathFn, testMiddlewareGetValueFn))

	s.Add("fn1", func(id uint64, params json.RawMessage) Response {
		return Response{}
	})
	url := startServer(t, s)

	c := Client{API: url + "/v1/cmd?value=test", Client: http.Client{}}
	_, err := c.Call("fn1")
	assert.NoError(t, err)
}

func TestServer_WithLogger(t *testing.T) {
	s := NewServer("")
	assert.Equal(t, reflect.TypeOf(s.logger), reflect.TypeFor[LoggerFunc]())

	s = NewServer("", WithLogger(testLogger{}))
	assert.Equal(t, reflect.TypeOf(s.logger), reflect.TypeFor[testLogger]())

	// nil logger and nil LoggerFunc ignored, the server keeps the default one and can log
	var nilFn LoggerFunc
	for _, lg := range []L{nil, nilFn} {
		s = NewServer("", WithLogger(lg))
		assert.Equal(t, reflect.TypeOf(s.logger), reflect.TypeFor[LoggerFunc]())
		assert.NotPanics(t, func() {
			s.Add("fn1", func(id uint64, params json.RawMessage) Response { return Response{} })
		})
	}

	// nil logger after a custom one keeps the custom one
	s = NewServer("", WithLogger(testLogger{}), WithLogger(nil))
	assert.Equal(t, reflect.TypeOf(s.logger), reflect.TypeFor[testLogger]())
}

type testLogger struct{}

func (l testLogger) Logf(format string, args ...any) {}

func TestRateLimitByIP(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("allows requests within limit", func(t *testing.T) {
		limiter := rateLimitByIP(5) // 5 req/sec
		ts := httptest.NewServer(limiter(handler))
		defer ts.Close()

		for range 5 {
			resp, err := http.Get(ts.URL)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			_ = resp.Body.Close()
		}
	})

	t.Run("rejects requests over limit", func(t *testing.T) {
		limiter := rateLimitByIP(2) // 2 req/sec
		ts := httptest.NewServer(limiter(handler))
		defer ts.Close()

		var rejected bool
		for range 10 {
			resp, err := http.Get(ts.URL)
			require.NoError(t, err)
			if resp.StatusCode == http.StatusTooManyRequests {
				rejected = true
			}
			_ = resp.Body.Close()
		}
		assert.True(t, rejected, "expected at least one request to be rate limited")
	})

	t.Run("keys on remote address without port, ignores X-Real-IP header", func(t *testing.T) {
		now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		limiter := newRateLimiter(1, func() time.Time { return now }).handler(handler) // 1 req/sec per client

		call := func(remoteAddr, realIP string) int {
			req := httptest.NewRequest("POST", "/", http.NoBody)
			req.RemoteAddr = remoteAddr
			if realIP != "" {
				req.Header.Set("X-Real-IP", realIP)
			}
			rec := httptest.NewRecorder()
			limiter.ServeHTTP(rec, req)
			return rec.Code
		}

		assert.Equal(t, http.StatusOK, call("1.1.1.1:1000", "spoof-1"), "first request from 1.1.1.1")
		assert.Equal(t, http.StatusTooManyRequests, call("1.1.1.1:1000", "spoof-2"), "header changed, same client")
		assert.Equal(t, http.StatusTooManyRequests, call("1.1.1.1:2000", ""), "port changed, same client")
		assert.Equal(t, http.StatusOK, call("2.2.2.2:1000", ""), "different client")
		assert.Equal(t, http.StatusOK, call("3.3.3.3", ""), "remote address without port")
		assert.Equal(t, http.StatusTooManyRequests, call("[::ffff:1.1.1.1]:3000", ""), "IPv4-mapped IPv6, same client")
		assert.Equal(t, http.StatusOK, call("2001:db8::1", ""), "IPv6 client")
		assert.Equal(t, http.StatusTooManyRequests, call("[2001:DB8:0::1]:1000", ""), "other spelling, same client")
	})

	t.Run("removes idle clients", func(t *testing.T) {
		now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		l := newRateLimiter(2, func() time.Time { return now }) // bucket of 2 is full again after 1s idle

		assert.True(t, l.allow("1.1.1.1"))
		assert.True(t, l.allow("2.2.2.2"))
		assert.True(t, l.allow("2.2.2.2"))
		assert.False(t, l.allow("2.2.2.2"), "2.2.2.2 used both tokens")
		assert.Len(t, l.clients, 2)

		now = now.Add(500 * time.Millisecond)
		assert.True(t, l.allow("2.2.2.2"), "one token back after 500ms")
		assert.Len(t, l.clients, 2, "nobody idle for 1s yet")

		now = now.Add(500 * time.Millisecond)
		assert.True(t, l.allow("3.3.3.3"))
		assert.Len(t, l.clients, 2, "1.1.1.1 idle for 1s removed")
		assert.NotContains(t, l.clients, "1.1.1.1")

		// 2.2.2.2 was active 500ms ago, its state kept: one token back, not a full bucket of two
		assert.True(t, l.allow("2.2.2.2"))
		assert.False(t, l.allow("2.2.2.2"))
	})

	t.Run("sweep keeps a bucket short of a whole token", func(t *testing.T) {
		now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		l := newRateLimiter(0.75, func() time.Time { return now }) // a call every 1.333333333...s
		assert.Equal(t, time.Duration(1333333333), l.sweepEvery)

		assert.True(t, l.allow("1.1.1.1"))
		now = now.Add(l.sweepEvery) // the sweep is due, the bucket holds 0.99999999975 tokens
		assert.False(t, l.allow("1.1.1.1"), "not removed and refilled by the sweep")
		now = now.Add(time.Nanosecond)
		assert.True(t, l.allow("1.1.1.1"), "a whole token after 4/3s")
	})

	t.Run("limit below 1/sec", func(t *testing.T) {
		now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		limiter := newRateLimiter(0.5, func() time.Time { return now }).handler(handler) // a call every 2 seconds

		call := func() int {
			rec := httptest.NewRecorder()
			limiter.ServeHTTP(rec, httptest.NewRequest("POST", "/", http.NoBody))
			return rec.Code
		}

		assert.Equal(t, http.StatusOK, call(), "first call allowed")
		assert.Equal(t, http.StatusTooManyRequests, call(), "second call at once rejected")
		now = now.Add(time.Second)
		assert.Equal(t, http.StatusTooManyRequests, call(), "half a token after 1s, rejected")
		now = now.Add(time.Second)
		assert.Equal(t, http.StatusOK, call(), "a whole token after 2s, allowed")
	})

	t.Run("rejected calls don't delay the refill", func(t *testing.T) {
		now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		l := newRateLimiter(0.1, func() time.Time { return now }) // a call every 10 seconds

		assert.True(t, l.allow("1.1.1.1"))
		for i := range 9 {
			now = now.Add(time.Second)
			assert.False(t, l.allow("1.1.1.1"), "rejected after %ds", i+1)
		}
		now = now.Add(time.Second)
		assert.True(t, l.allow("1.1.1.1"), "allowed 10s after the first call")
	})

	t.Run("sweep interval", func(t *testing.T) {
		assert.Equal(t, time.Second, newRateLimiter(100, time.Now).sweepEvery)
		assert.Equal(t, 10*time.Second, newRateLimiter(0.1, time.Now).sweepEvery)
		// the fill time doesn't fit in a Duration, a conversion would overflow and sweep on every call
		assert.Equal(t, time.Duration(math.MaxInt64), newRateLimiter(1e-12, time.Now).sweepEvery)
	})

	t.Run("refills tokens over time", func(t *testing.T) {
		limiter := rateLimitByIP(10) // 10 req/sec
		ts := httptest.NewServer(limiter(handler))
		defer ts.Close()

		// exhaust all tokens
		for range 10 {
			resp, err := http.Get(ts.URL)
			require.NoError(t, err)
			_ = resp.Body.Close()
		}

		// should be rate limited
		resp, err := http.Get(ts.URL)
		require.NoError(t, err)
		assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
		_ = resp.Body.Close()

		// wait for tokens to refill
		time.Sleep(200 * time.Millisecond)

		// should succeed after refill
		resp, err = http.Get(ts.URL)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		_ = resp.Body.Close()
	})
}

// startServer runs s on a random port and returns its base url. the listener is bound and the server
// activated before serve starts, so no readiness wait needed. shutdown and serve error checked on cleanup.
func startServer(t *testing.T, s *Server) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	s.activate()
	done := make(chan error, 1)
	go func() { done <- s.serve(l) }()

	t.Cleanup(func() {
		// a spare connection the default transport pooled unused blocks Shutdown until its 5s deadline
		http.DefaultTransport.(*http.Transport).CloseIdleConnections()
		assert.NoError(t, s.Shutdown())
		assert.ErrorIs(t, <-done, http.ErrServerClosed)
	})

	return "http://" + l.Addr().String()
}

func TestServerRunFailedToListen(t *testing.T) {
	l, err := net.Listen("tcp", ":0") //nolint:gosec // has to bind the same way Run does to collide with it
	require.NoError(t, err)
	defer func() { assert.NoError(t, l.Close()) }()
	port := l.Addr().(*net.TCPAddr).Port

	s := NewServer("/v1/cmd")
	s.Add("test", func(uint64, json.RawMessage) Response { return Response{} })

	done := make(chan error, 1)
	go func() { done <- s.Run(port) }()

	select {
	case err = <-done:
		require.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("can't listen on port %d", port))
	case <-time.After(time.Second):
		assert.NoError(t, s.Shutdown())
		t.Fatal("expected Run to fail on the busy port")
	}
}

func TestServerServeNotActivated(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { assert.NoError(t, l.Close()) }()

	s := NewServer("/v1/cmd")
	assert.EqualError(t, s.serve(l), "server is not activated")
}

func TestServerCallTimeout(t *testing.T) {
	s := NewServer("/v1/cmd", WithTimeouts(Timeouts{
		ReadHeaderTimeout: time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       time.Second,
		CallTimeout:       50 * time.Millisecond,
	}))

	s.Add("fast", func(id uint64, _ json.RawMessage) Response {
		return EncodeResponse(id, "done", nil)
	})
	s.Add("slow", func(id uint64, _ json.RawMessage) Response {
		time.Sleep(500 * time.Millisecond)
		return EncodeResponse(id, "too late", nil)
	})

	url := startServer(t, s)

	t.Run("call within timeout", func(t *testing.T) {
		c := Client{API: url + "/v1/cmd", Client: http.Client{}}
		r, err := c.Call("fast")
		require.NoError(t, err)
		val := ""
		require.NoError(t, json.Unmarshal(*r.Result, &val))
		assert.Equal(t, "done", val)
	})

	t.Run("call over timeout aborted with 503", func(t *testing.T) {
		b := bytes.Buffer{}
		require.NoError(t, json.NewEncoder(&b).Encode(Request{Method: "slow", ID: 1}))

		st := time.Now()
		resp, err := http.Post(url+"/v1/cmd", "application/json", &b)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
		assert.Less(t, time.Since(st), 300*time.Millisecond, "response should not wait for the handler to finish")

		data, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, `{"error":"call timeout"}`, string(data))
	})
}

func TestServerLimitsForwardedHeaders(t *testing.T) {
	rotate := func(format string) func(int) string { return func(i int) string { return fmt.Sprintf(format, i) } }
	ipv6 := []string{"2001:db8::1", "2001:DB8::1", "2001:db8:0::1", "2001:0db8::0001", " 2001:db8::1 "}

	tbl := []struct {
		name, header string
		value        func(i int) string
		wantOK       int
	}{
		{name: "invalid X-Real-IP", header: "X-Real-IP", value: rotate("spoof-%d"), wantOK: 1},
		{name: "private X-Real-IP", header: "X-Real-IP", value: rotate("10.0.0.%d"), wantOK: 1},
		{name: "private X-Forwarded-For", header: "X-Forwarded-For", value: rotate("192.168.1.%d"), wantOK: 1},
		{name: "spellings of one public IPv6", header: "X-Real-IP", value: func(i int) string { return ipv6[i%len(ipv6)] }, wantOK: 1},
		{name: "public X-Real-IP, taken as sent", header: "X-Real-IP", value: rotate("8.8.8.%d"), wantOK: 10},
		{name: "public CF-Connecting-IP, taken as sent", header: "CF-Connecting-IP", value: rotate("8.8.8.%d"), wantOK: 10},
		{name: "public X-Forwarded-For, taken as sent", header: "X-Forwarded-For", value: rotate("8.8.8.%d"), wantOK: 10},
	}
	for _, tt := range tbl {
		t.Run(tt.name, func(t *testing.T) {
			s := NewServer("/v1/cmd", WithLimits(0.001)) // one call, then one more after 1000s
			s.Add("fn1", func(id uint64, params json.RawMessage) Response { return Response{} })
			url := startServer(t, s)
			client := &http.Client{Transport: &http.Transport{}}
			t.Cleanup(client.CloseIdleConnections) // runs before the server shutdown, which waits on idle connections otherwise

			codes := map[int]int{}
			for i := range 10 {
				req, err := http.NewRequest("POST", url+"/v1/cmd", bytes.NewBufferString(`{"method":"fn1"}`))
				require.NoError(t, err)
				req.Header.Set(tt.header, tt.value(i))
				resp, err := client.Do(req)
				require.NoError(t, err)
				codes[resp.StatusCode]++
				_ = resp.Body.Close()
			}
			assert.Equal(t, tt.wantOK, codes[http.StatusOK], "accepted")
			assert.Equal(t, 10-tt.wantOK, codes[http.StatusTooManyRequests], "rejected by the limit, got %v", codes)
		})
	}
}

func TestServerErrorBodies(t *testing.T) {
	var logs bytes.Buffer
	var mu sync.Mutex
	lg := LoggerFunc(func(format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		_, _ = fmt.Fprintf(&logs, format+"\n", args...)
	})
	s := NewServer("/v1/cmd", WithLogger(lg))
	s.Add("fn1", func(id uint64, params json.RawMessage) Response { return Response{} })
	url := startServer(t, s)
	client := &http.Client{Transport: &http.Transport{}}
	t.Cleanup(client.CloseIdleConnections) // runs before the server shutdown, which waits on idle connections otherwise

	tbl := []struct {
		name, body, wantBody, wantLog string
		wantCode                      int
	}{
		{name: "unknown method", body: `{"method":"nosuch","id":1}`, wantCode: http.StatusNotImplemented,
			wantBody: `{"error":"unsupported method"}`, wantLog: `unsupported method "nosuch"`},
		{name: "malformed request", body: `not json`, wantCode: http.StatusBadRequest,
			wantBody: `{"error":"invalid request"}`, wantLog: "invalid character"},
		{name: "malformed request with a method", body: `{"method":"fn1","id":"x"}`, wantCode: http.StatusBadRequest,
			wantBody: `{"error":"invalid request"}`, wantLog: "cannot unmarshal string"},
	}
	for _, tt := range tbl {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.Post(url+"/v1/cmd", "application/json", bytes.NewBufferString(tt.body))
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()
			assert.Equal(t, tt.wantCode, resp.StatusCode)
			data, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, tt.wantBody, strings.TrimSpace(string(data)))
			mu.Lock()
			defer mu.Unlock()
			assert.Contains(t, logs.String(), tt.wantLog, "the reason is logged")
		})
	}
}
