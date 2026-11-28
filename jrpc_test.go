package jrpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeResponse(t *testing.T) {
	resp := EncodeResponse(42, "value", nil)
	assert.Equal(t, uint64(42), resp.ID)
	assert.Empty(t, resp.Error)
	assert.JSONEq(t, `"value"`, string(*resp.Result))

	resp = EncodeResponse(42, "value", assert.AnError)
	assert.Equal(t, Response{ID: 42, Error: assert.AnError.Error()}, resp)

	// a value json can't encode keeps the request id
	resp = EncodeResponse(42, make(chan int), nil)
	assert.Equal(t, Response{ID: 42, Error: "json: unsupported type: chan int"}, resp)
}
