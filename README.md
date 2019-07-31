# jrpc - rpc with json [![Build Status](https://travis-ci.org/go-pkgz/jrpc.svg?branch=master)](https://travis-ci.org/go-pkgz/jrpc) [![Coverage Status](https://coveralls.io/repos/github/go-pkgz/jrpc/badge.svg?branch=master)](https://coveralls.io/github/go-pkgz/jrpc?branch=master) [![godoc](https://godoc.org/github.com/go-pkgz/jrpc?status.svg)](https://godoc.org/github.com/go-pkgz/jrpc)

jrpc library provides client and server for RPC-like communication over HTTP with json encoded messages.
The protocol is somewhat simplified version of json-rpc with a single POST call sending Request json 
(method name and the list of parameters) and receiving back json Response with result data and error string.

## Usage

### Plugin (server)

```go
// Server wraps jrpc.Server and adds synced map to store data
type Puglin struct {
	*jrpc.Server
}

    // create plugin (jrpc server)
	plugin := jrpcServer{
		Server: &jrpc.Server{
			API:        "/command",     // base url for rpc calls
			AuthUser:   "user",         // basic auth user name
			AuthPasswd: "password",     // basic auth password
			AppName:    "jrpc-example", // plugin name for headers
			Logger:     logger,
		},
	}
    
    plugin.Add("mycommand", func(id uint64, params json.RawMessage) Response {
        return jrpc.EncodeResponse(id, "hello, it works", nil)
    })
```

### Application (client)

```go
// Client makes jrpc.Client and invoke remote call
rpcClient := jrpc.Client{
    API:        "http://127.0.0.1:8080/command",
    Client:     http.Client{},
    AuthUser:   "user",
    AuthPasswd: "password",
}

resp, err := rpcClient.Call("mycommand")
var message string
if err = json.Unmarshal(*resp.Result, &message); err != nil {
    panic(err)
}
```

_for functional examples for both plugin and application see [_example](https://github.com/go-pkgz/jrpc/tree/master/_example)_
 