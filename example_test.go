package jrpc_test

import (
	"encoding/json"
	"log"
	"time"

	"github.com/go-pkgz/rest"

	"github.com/go-pkgz/jrpc"
)

// ExampleNewServer shows the minimal server setup with a single method registered.
func ExampleNewServer() {
	plugin := jrpc.NewServer("/command")

	plugin.Add("mycommand", func(id uint64, params json.RawMessage) jrpc.Response {
		return jrpc.EncodeResponse(id, "hello, it works", nil)
	})

	// blocks until Shutdown called or the server failed
	_ = plugin.Run(8080)
}

// ExampleNewServer_options shows the server setup with all the available options.
func ExampleNewServer_options() {
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
		jrpc.WithLogger(jrpc.LoggerFunc(log.Printf)),
		jrpc.WithMiddlewares(rest.Trace),
	)

	plugin.Add("mycommand", func(id uint64, params json.RawMessage) jrpc.Response {
		return jrpc.EncodeResponse(id, "hello, it works", nil)
	})

	_ = plugin.Run(8080)
}

// ExampleClient_Call shows how an application calls the remote method.
func ExampleClient_Call() {
	rpcClient := jrpc.Client{
		API:        "http://127.0.0.1:8080/command",
		AuthUser:   "user",
		AuthPasswd: "password",
	}

	resp, err := rpcClient.Call("mycommand")
	if err != nil {
		log.Fatalf("call failed: %v", err)
	}

	var message string
	if err = json.Unmarshal(*resp.Result, &message); err != nil {
		log.Fatalf("failed to decode result: %v", err)
	}
	log.Printf("got %q", message)
}
