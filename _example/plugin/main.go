package main

// example of jrpc server (plugin). It provides a toy storage with 2 methods:
// 1. "store.save" - saves dataRecord and return ID
// 2. "store.load" - load dataRecord for given ID
import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/go-pkgz/lgr"

	"github.com/go-pkgz/jrpc"
)

type dataRecord struct {
	TS    time.Time
	Value string
}

type jrpcServer struct {
	*jrpc.Server

	sync.Mutex
	data map[string]dataRecord
}

func main() {

	logger := lgr.Default()
	rpcServer := jrpcServer{
		Server: &jrpc.Server{
			API:        "/command",
			AuthUser:   "user",
			AuthPasswd: "password",
			AppName:    "jrpc-example",
			Logger:     logger,
		},
		data: map[string]dataRecord{},
	}

	rpcServer.Group("store", jrpc.HandlersGroup{
		"save": rpcServer.saveHndl,
		"load": rpcServer.loadHndl,
	})

	logger.Logf("failed with %v", rpcServer.Run(8080))
}

func (j *jrpcServer) saveHndl(id uint64, params json.RawMessage) (rr jrpc.Response) {

	// unmarshal request
	rec := dataRecord{}
	if err := json.Unmarshal(params, &rec); err != nil {
		return jrpc.Response{Error: err.Error()}
	}

	recID := fmt.Sprintf("%d", rand.Int63n(99999999999))
	j.synced(func() {
		j.data[recID] = rec
	})

	var err error
	if rr, err = j.EncodeResponse(id, recID, nil); err != nil {
		return jrpc.Response{Error: err.Error()}
	}
	return rr
}

func (j *jrpcServer) loadHndl(id uint64, params json.RawMessage) (rr jrpc.Response) {

	// unmarshal request
	args := []interface{}{}
	if err := json.Unmarshal(params, &args); err != nil {
		return jrpc.Response{Error: err.Error()}
	}

	recID, ok := args[0].(string)
	if !ok {
		return jrpc.Response{Error: "incompatible argument"}
	}
	var rec dataRecord
	j.synced(func() {
		rec, ok = j.data[recID]
	})

	if !ok {
		return jrpc.Response{Error: "not found"}
	}

	var err error
	if rr, err = j.EncodeResponse(id, rec, nil); err != nil {
		return jrpc.Response{Error: err.Error()}
	}
	return rr
}

func (j *jrpcServer) synced(fn func()) {
	j.Mutex.Lock()
	fn()
	j.Mutex.Unlock()
}
