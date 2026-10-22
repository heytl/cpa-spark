package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef struct {
	void* ptr;
	size_t len;
} cliproxy_buffer;

typedef int (*cliproxy_host_call_fn)(void*, const char*, const uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_host_free_fn)(void*, size_t);

typedef struct {
	uint32_t abi_version;
	void* host_ctx;
	cliproxy_host_call_fn call;
	cliproxy_host_free_fn free_buffer;
} cliproxy_host_api;

typedef int (*cliproxy_plugin_call_fn)(char*, uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_plugin_free_fn)(void*, size_t);
typedef void (*cliproxy_plugin_shutdown_fn)(void);

typedef struct {
	uint32_t abi_version;
	cliproxy_plugin_call_fn call;
	cliproxy_plugin_free_fn free_buffer;
	cliproxy_plugin_shutdown_fn shutdown;
} cliproxy_plugin_api;

extern int cliproxyPluginCall(char*, uint8_t*, size_t, cliproxy_buffer*);
extern void cliproxyPluginFree(void*, size_t);
extern void cliproxyPluginShutdown(void);

static cliproxy_host_api host_copy;
static const cliproxy_host_api* stored_host;

static void store_host_api(const cliproxy_host_api* host) {
	host_copy = *host; stored_host = &host_copy;
}

static int call_host_api(const char* method, const uint8_t* request, size_t request_len, cliproxy_buffer* response) {
	if (stored_host == NULL || stored_host->call == NULL) {
		return 1;
	}
	return stored_host->call(stored_host->host_ctx, method, request, request_len, response);
}

static void free_host_buffer(void* ptr, size_t len) {
	if (stored_host != NULL && stored_host->free_buffer != NULL && ptr != NULL) {
		stored_host->free_buffer(ptr, len);
	}
}
*/
import "C"
import (
	"cpa-spark/internal/warmup"
	"cpa-spark/web"
	"encoding/json"
	"errors"
	"sync"
	"unsafe"
)

var mu sync.RWMutex
var app *warmup.App

func main() {}

//export cliproxy_plugin_init
func cliproxy_plugin_init(host *C.cliproxy_host_api, plugin *C.cliproxy_plugin_api) C.int {
	if host == nil || plugin == nil || host.abi_version != 1 || host.call == nil || host.free_buffer == nil {
		return 1
	}
	C.store_host_api(host)
	mu.Lock()
	app = &warmup.App{Host: callbackHost{}, Assets: web.Assets}
	mu.Unlock()
	plugin.abi_version = 1
	plugin.call = C.cliproxy_plugin_call_fn(C.cliproxyPluginCall)
	plugin.free_buffer = C.cliproxy_plugin_free_fn(C.cliproxyPluginFree)
	plugin.shutdown = C.cliproxy_plugin_shutdown_fn(C.cliproxyPluginShutdown)
	return 0
}

//export cliproxyPluginCall
func cliproxyPluginCall(method *C.char, request *C.uint8_t, n C.size_t, response *C.cliproxy_buffer) (code C.int) {
	if response == nil {
		return 1
	}
	response.ptr = nil
	response.len = 0
	defer func() {
		if recover() != nil {
			writeResponse(response, []byte(`{"ok":false,"error":{"code":"panic","message":"plugin panic"}}`))
			code = 1
		}
	}()
	if method == nil || n > 2*1024*1024 || (n > 0 && request == nil) {
		return 1
	}
	mu.RLock()
	current := app
	mu.RUnlock()
	if current == nil {
		return 1
	}
	var raw []byte
	if n > 0 {
		raw = C.GoBytes(unsafe.Pointer(request), C.int(n))
	}
	writeResponse(response, current.Call(C.GoString(method), raw))
	return 0
}

//export cliproxyPluginFree
func cliproxyPluginFree(ptr unsafe.Pointer, n C.size_t) {
	if ptr != nil {
		C.free(ptr)
	}
}

//export cliproxyPluginShutdown
func cliproxyPluginShutdown() {
	mu.RLock()
	current := app
	mu.RUnlock()
	if current != nil {
		current.Close()
	}
}
func writeResponse(out *C.cliproxy_buffer, b []byte) {
	out.ptr = C.CBytes(b)
	out.len = C.size_t(len(b))
}

type callbackHost struct{}

func (callbackHost) Call(method string, request any, out any) error {
	b, err := json.Marshal(request)
	if err != nil {
		return err
	}
	m := C.CString(method)
	defer C.free(unsafe.Pointer(m))
	data := C.CBytes(b)
	defer C.free(data)
	var response C.cliproxy_buffer
	code := C.call_host_api(m, (*C.uint8_t)(data), C.size_t(len(b)), &response)
	if response.ptr != nil {
		defer C.free_host_buffer(response.ptr, response.len)
	}
	if response.ptr == nil || response.len == 0 || response.len > 16*1024*1024 {
		return errors.New("invalid CPA response")
	}
	var env struct {
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code   string `json:"code"`
			Status int    `json:"http_status"`
		} `json:"error"`
	}
	if json.Unmarshal(C.GoBytes(response.ptr, C.int(response.len)), &env) != nil {
		return errors.New("invalid CPA envelope")
	}
	if !env.OK {
		e := &warmup.HostError{}
		if env.Error != nil {
			e.Code = env.Error.Code
			e.Status = env.Error.Status
		}
		return e
	}
	if code != 0 {
		return errors.New("CPA callback transport failed")
	}
	if out != nil {
		return json.Unmarshal(env.Result, out)
	}
	return nil
}
