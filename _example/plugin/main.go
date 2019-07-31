package main

// example of jrpc server (plugin). It provides a toy storage with 2 methods:
//  1. "store.save" - saves dataRecord and return ID
//  2. "store.load" - load dataRecord for given ID
import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/go-pkgz/jrpc"
)

// dataRecord to be stored and retrieved by a remote client
type dataRecord struct {
	TS    time.Time
	Value string
}

// jrpcServer wraps jrpc.Server and adds synced map to store data
type jrpcServer struct {
	*jrpc.Server

	sync.Mutex
	data map[string]dataRecord
}

func main() {

	// optional logger implementing a single func interface
	// Logf(format string, args ...interface{})
	logger := jrpc.LoggerFunc(func(format string, args ...interface{}) {
		log.Printf(format, args...)
	})

	// create rpcServer
	rpcServer := jrpcServer{
		Server: &jrpc.Server{
			API:        "/command",     // base url for rpc calls
			AuthUser:   "user",         // basic auth user name
			AuthPasswd: "password",     // basic auth password
			AppName:    "jrpc-example", // plugin name for headers
			Logger:     logger,
		},
		data: map[string]dataRecord{},
	}

	// add command handler in a group "store". Method name for client will be "store.save" and "store.load"
	rpcServer.Group("store", jrpc.HandlersGroup{
		"save": rpcServer.saveHndl,
		"load": rpcServer.loadHndl,
	})

	// activate jrpc server
	logger.Logf("failed with %v", rpcServer.Run(8080))
}

// saveHndl accept dataRecord as params, save it and returns record's ID
func (j *jrpcServer) saveHndl(id uint64, params json.RawMessage) (rr jrpc.Response) {

	// unmarshal request
	rec := dataRecord{}
	if err := json.Unmarshal(params, &rec); err != nil {
		return jrpc.Response{Error: err.Error()}
	}

	recID := fmt.Sprintf("%d", rand.Int63n(99999999999)) // make a random ID for stored record
	j.synced(func() {
		j.data[recID] = rec
	})

	// encode response (recID)
	return jrpc.EncodeResponse(id, recID, nil)
}

// loadHndl accepts record's ID (string value) as params, loads and returns corresponding dataRecord
func (j *jrpcServer) loadHndl(id uint64, params json.RawMessage) (rr jrpc.Response) {

	// unmarshal request
	var recID string
	if err := json.Unmarshal(params, &recID); err != nil {
		return jrpc.Response{Error: err.Error()}
	}

	var rec dataRecord
	var ok bool
	j.synced(func() {
		rec, ok = j.data[recID]
	})

	if !ok { // no record for given recID
		return jrpc.Response{Error: "not found"}
	}

	// encode response (dataRecord)
	return jrpc.EncodeResponse(id, rec, nil)
}

// run fn synced
func (j *jrpcServer) synced(fn func()) {
	j.Mutex.Lock()
	fn()
	j.Mutex.Unlock()
}
