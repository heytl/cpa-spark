package main

/*
#include "abi.h"
*/
import "C"

import (
	"encoding/json"
	"errors"
	"sync"
	"unsafe"

	"cpa-spark/internal/warmup"
	"cpa-spark/web"
)

const (
	maxPluginRequest = 2 * 1024 * 1024
	maxHostReply     = 16 * 1024 * 1024
)

var runtime struct {
	sync.RWMutex
	application *warmup.App
}

func main() {}

//export cliproxy_plugin_init
func cliproxy_plugin_init(host *C.cliproxy_host_api, plugin *C.cliproxy_plugin_api) C.int {
	if code := C.spark_bind(host, plugin); code != 0 {
		return code
	}
	runtime.Lock()
	runtime.application = &warmup.App{Host: callbackHost{}, Assets: web.Assets}
	runtime.Unlock()
	return 0
}

//export cliproxyPluginCall
func cliproxyPluginCall(method *C.char, request *C.uint8_t, size C.size_t, reply *C.cliproxy_buffer) (status C.int) {
	if reply == nil {
		return 1
	}
	*reply = C.cliproxy_buffer{}
	defer func() {
		if recover() != nil {
			setReply(reply, []byte(`{"ok":false,"error":{"code":"panic","message":"plugin panic"}}`))
			status = 1
		}
	}()
	if method == nil || size > maxPluginRequest || (size != 0 && request == nil) {
		return 1
	}
	runtime.RLock()
	application := runtime.application
	runtime.RUnlock()
	if application == nil {
		return 1
	}
	payload := C.GoBytes(unsafe.Pointer(request), C.int(size))
	setReply(reply, application.Call(C.GoString(method), payload))
	return 0
}

//export cliproxyPluginFree
func cliproxyPluginFree(pointer unsafe.Pointer, _ C.size_t) {
	C.free(pointer)
}

//export cliproxyPluginShutdown
func cliproxyPluginShutdown() {
	runtime.RLock()
	application := runtime.application
	runtime.RUnlock()
	if application != nil {
		application.Close()
	}
}

func setReply(reply *C.cliproxy_buffer, payload []byte) {
	reply.ptr = C.CBytes(payload)
	reply.len = C.size_t(len(payload))
}

type callbackHost struct{}

// Call owns only the request buffer. The host owns and releases its reply.
func (callbackHost) Call(method string, request any, result any) error {
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}
	name := C.CString(method)
	body := C.CBytes(payload)
	defer C.free(unsafe.Pointer(name))
	defer C.free(body)
	var reply C.cliproxy_buffer
	status := C.spark_host_call(name, (*C.uint8_t)(body), C.size_t(len(payload)), &reply)
	defer C.spark_host_release(reply)
	if reply.ptr == nil || reply.len == 0 || reply.len > maxHostReply {
		return errors.New("invalid CPA response")
	}
	return decodeHostReply(C.GoBytes(reply.ptr, C.int(reply.len)), status != 0, result)
}

func decodeHostReply(payload []byte, transportFailed bool, result any) error {
	var reply struct {
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
		Error  struct {
			Code   string `json:"code"`
			Status int    `json:"http_status"`
		} `json:"error"`
	}
	if err := json.Unmarshal(payload, &reply); err != nil {
		return errors.New("invalid CPA envelope")
	}
	if !reply.OK {
		return &warmup.HostError{Code: reply.Error.Code, Status: reply.Error.Status}
	}
	if transportFailed {
		return errors.New("CPA callback transport failed")
	}
	if result == nil {
		return nil
	}
	return json.Unmarshal(reply.Result, result)
}
