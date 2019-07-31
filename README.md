# jrpc - rpc with json [![Build Status](https://travis-ci.org/go-pkgz/jrpc.svg?branch=master)](https://travis-ci.org/go-pkgz/jrpc) [![Coverage Status](https://coveralls.io/repos/github/go-pkgz/jrpc/badge.svg?branch=master)](https://coveralls.io/github/go-pkgz/jrpc?branch=master) [![godoc](https://godoc.org/github.com/go-pkgz/jrpc?status.svg)](https://godoc.org/github.com/go-pkgz/jrpc)

jrpc library provides client and server for RPC-like communication over HTTP with json encoded messages.
The protocol is somewhat simplified version of json-rpc with a single POST call sending Request json 
(method name and the list of parameters) and receiving back json Response with result data and error string.

