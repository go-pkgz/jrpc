# jrpc - rpc with json [![Build](https://github.com/go-pkgz/jrpc/actions/workflows/ci.yml/badge.svg)](https://github.com/go-pkgz/jrpc/actions/workflows/ci.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/go-pkgz/jrpc)](https://goreportcard.com/report/github.com/go-pkgz/jrpc) [![Coverage Status](https://coveralls.io/repos/github/go-pkgz/jrpc/badge.svg?branch=master)](https://coveralls.io/github/go-pkgz/jrpc?branch=master) [![godoc](https://godoc.org/github.com/go-pkgz/jrpc?status.svg)](https://godoc.org/github.com/go-pkgz/jrpc)

jrpc library provides client and server for RPC-like communication over HTTP with json encoded messages.
The protocol is a somewhat simplified version of json-rpc with a single POST call sending `Request` json 
(method name and the list of parameters) moreover, receiving json `Response` with result data and an error string.

## Usage

### Plugin (server)

```go
// create plugin (jrpc server) with NewServer where required param is a base url for rpc calls
plugin := jrpc.NewServer("/command")

// then add your function to map
plugin.Add("mycommand", func(id uint64, params json.RawMessage) jrpc.Response {
    return jrpc.EncodeResponse(id, "hello, it works", nil)
})

// and run server with port number value, it blocks until the server is stopped
if err := plugin.Run(8080); err != nil && !errors.Is(err, http.ErrServerClosed) {
    log.Fatal(err)
}
```

Handlers can also be added as a group with a common prefix, each one is called as `prefix.name`:

```go
plugin.Group("store", jrpc.HandlersGroup{
    "save": saveHandler, // called as "store.save"
    "load": loadHandler, // called as "store.load"
})
```

All handlers have to be added before `Run`: `Run` returns an error if none was added, and a method added after the
server started is ignored with a `[WARN]` line in the log. `Shutdown` stops the server: `Run` returns
`http.ErrServerClosed` at once, while `Shutdown` waits up to 5 seconds for the calls in progress to finish, so a program
should wait for `Shutdown` to return before it exits.

The constructor `NewServer` accepts two parameters:
* `API` - a base url for rpc calls
* `Options` - optional parameters such as timeouts, logger, limits, middlewares and so on.
  * `Auth` - sets basic auth credentials, accepts `username` and `password`. Auth is enforced only if both of them
    set to non-empty values; setting just one leaves the server serving every request unauthenticated
  * `WithTimeouts` - sets server timeouts, accepts a `Timeouts` struct with `ReadHeaderTimeout`, `WriteTimeout`,
    `IdleTimeout` and `CallTimeout`. The defaults are 5s for `ReadHeaderTimeout`, 10s for `WriteTimeout`, 5s for
    `IdleTimeout` and no `CallTimeout`; a zero field keeps the current value and a negative one turns the timeout off.
    `CallTimeout` limits the time allowed for a single call and responds with `503` if exceeded, and has to be set
    below `WriteTimeout`, otherwise the write deadline kills the connection before the `503` can be sent. Calls that may
    run longer than 10s need a larger `WriteTimeout` as well
  * `WithLimits` - defines a limit of calls/sec per client, accepts limit value in `float64` type. A limit between 0 and
    1 allows one call every `1/limit` seconds, e.g. `0.5` allows a call every 2 seconds; zero or a negative value sets
    no limit.
    A client is the IP address [rest.RealIP](https://github.com/go-pkgz/rest#realip-middleware) derives from the request:
    a public IP from the `X-Real-IP`, `CF-Connecting-IP` or `X-Forwarded-For` header, or the address of the connection
    otherwise. The headers are taken as sent, so the limit holds per client only behind a proxy that overwrites them;
    a caller connecting directly can send a different public IP with each call and bypass the limit
  * `WithThrottler` - sets throttler middleware limiting the number of parallel calls to the server
  * `WithSignature` - sets server signature, accepts appName, author and version. Disabled by default
  * `WithLogger` - defines custom logger (e.g. [lgr](https://github.com/go-pkgz/lgr)). The logger gets a `[DEBUG]` line
    with the first 1024 bytes of the request body for every request except `/ping` and the ones the throttler rejects,
    so requests rejected by auth or by the per-client limit are logged too. A nil logger is ignored
  * `WithMiddlewares` - sets custom middlewares list to server, accepts list of handlers with idiomatic type `func(http.Handler) http.Handler`.
    Custom middlewares run last, after auth and the limits, so they see only the requests that passed them

Example with options:
```go
import (
	"time"

	"github.com/go-pkgz/jrpc"
	"github.com/go-pkgz/rest"
)

plugin := jrpc.NewServer("/command",
	jrpc.Auth("user", "password"),
	jrpc.WithTimeouts(jrpc.Timeouts{
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       10 * time.Second,
		CallTimeout:       25 * time.Second,
	}),
	jrpc.WithThrottler(120),
	jrpc.WithLimits(100),
	jrpc.WithSignature("the best plugin ever", "author", "1.0.0"),
	jrpc.WithMiddlewares(rest.Trace),
)
```

### Application (client)

```go
// Client makes jrpc.Client and invoke remote call
rpcClient := jrpc.Client{
    API:        "http://127.0.0.1:8080/command",
    Client:     http.Client{Timeout: 10 * time.Second},
    AuthUser:   "user",
    AuthPasswd: "password",
}

resp, err := rpcClient.Call("mycommand")
if err != nil {
    panic(err)
}
var message string
if err = json.Unmarshal(*resp.Result, &message); err != nil {
    panic(err)
}
```

### Running the example

[_example](https://github.com/go-pkgz/jrpc/tree/master/_example) has a working pair of a plugin and an application.
Both are separate go modules pointing to the local jrpc with a `replace` directive, so no extra setup is needed
beyond go 1.24 or later and a free local port 8080. Start the plugin first, in one terminal:

```sh
cd _example/plugin
go run .
```

It registers two handlers and listens on port 8080, printing output similar to this (the two handlers can be
listed in either order):

```
2025/01/12 12:00:00 [INFO] add handler for store.save
2025/01/12 12:00:00 [INFO] add handler for store.load
2025/01/12 12:00:00 [INFO] listen on [::]:8080
```

Then run the application in another terminal:

```sh
cd _example/application
go run .
```

It calls the plugin three times and prints the results, similar to this:

```
2025/01/12 12:00:05 stored {TS:2025-01-12 12:00:05.123456 +0100 CET m=+0.001234567 Value:12345} with id=54118548792
2025/01/12 12:00:05 loaded {TS:2025-01-12 12:00:05.123456 +0100 CET Value:12345} from id=54118548792
2025/01/12 12:00:05 can't load for id=something, not found
```

The application exits on its own, the plugin keeps listening until stopped with Ctrl-C.

## Technical details
 
 * `jrpc.Server` runs on user-defined port as a regular http server
 * Server accepts a single POST request on user-defined url with [Request](https://github.com/go-pkgz/jrpc/blob/master/jrpc.go#L12) sent as json payload
 <details><summary>request details and an example:</summary>
 
     ```go
     type Request struct {
     	Method string      `json:"method"`
     	Params interface{} `json:"params,omitempty"`
     	ID     uint64      `json:"id"`
     }
     ```
     example: 
     
     ```json
       {
        "method":"test",
        "params":[123,"abc"],
        "id":1
        }
     ```
 </details>
 
* Params can be a struct, primitive type or slice of values, even with different types.
* Server also answers `GET` and `HEAD` requests to any path ending with `/ping` with `200` (and `pong` in the body of a
  `GET`), without auth, to be used as a health check. The throttler applies to it, so a server at its `WithThrottler`
  limit answers it with `503`.
* Server defines `ServerFn` handler function to react on a POST request. The handler provided by the user.
* Communication between the server and the caller can be protected with basic auth. The protection is on only if
  both user and password set with the `Auth` option; with either of them empty the server responds to every request
  without asking for credentials.
* [Client](https://github.com/go-pkgz/jrpc/blob/master/client.go) provides a single method `Call` and return `Response`
  or an error. With an error the returned `Response` is nil, and a failure on the server side comes back as error text
  only, with no status code or error type to check:
  * an error the handler sets in `Response.Error` (e.g. with `EncodeResponse(id, nil, err)`) is returned as its text
  * any HTTP status other than `200` is returned as `bad status <status> for <method>`, without the response body:
    `400` for a malformed request, `401` for missing or wrong credentials, `429` if the per-client limit is exceeded,
    `500` if the handler panics, `501` for an unknown method, and `503` both if `CallTimeout` is exceeded and if the
    throttler rejects the call
  * a failed http call or a response that can't be decoded is returned with the method name added, wrapping the
    underlying error

  The client doesn't retry, and a call waits as long as `http.Client` allows, so set its `Timeout`.

 <details><summary>response details:</summary>
 
   ```go
    // Response encloses result and error received from remote server
    type Response struct {
    	Result *json.RawMessage `json:"result,omitempty"`
    	Error  string           `json:"error,omitempty"`
    	ID     uint64           `json:"id"`
    }
   ```
 </details>
 
* User should encode and decode json payloads on the application level, see provided [examples](https://github.com/go-pkgz/jrpc/tree/master/_example)
* `jrpc.Server` doesn't support https internally (yet). If used on exposed or non-private networks, should be proxied with something providing https termination (nginx and others). 
* `Run(port)` listens on all network interfaces and has no option to bind to a single address, so with a local proxy
  in front a firewall has to keep other hosts from reaching the port directly.

## Status

The code was extracted from [remark42](https://github.com/umputun/remark) and still under development. Until v1.x released the
 API & protocol may change.
 
