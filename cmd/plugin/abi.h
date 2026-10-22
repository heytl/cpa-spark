/* Native declarations follow CPA's official host-model-callback example.
 * Source revision: docs/SOURCES.md. MIT notice: docs/third-party/cliproxyapi-LICENSE.txt.
 * The copied host table outlives the caller's initialization stack frame.
 */
#ifndef CPA_SPARK_ABI_H
#define CPA_SPARK_ABI_H
#include <stdint.h>
#include <stdlib.h>

typedef struct { void* ptr; size_t len; } cliproxy_buffer;
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

static cliproxy_host_api spark_host;
static int spark_bind(const cliproxy_host_api* host, cliproxy_plugin_api* plugin) {
    if (!host || !plugin || host->abi_version != 1 || !host->call || !host->free_buffer) return 1;
    spark_host = *host;
    plugin->abi_version = 1;
    plugin->call = cliproxyPluginCall;
    plugin->free_buffer = cliproxyPluginFree;
    plugin->shutdown = cliproxyPluginShutdown;
    return 0;
}
static int spark_host_call(const char* method, const uint8_t* body, size_t size, cliproxy_buffer* reply) {
    if (!spark_host.call) return 1;
    return spark_host.call(spark_host.host_ctx, method, body, size, reply);
}
static void spark_host_release(cliproxy_buffer reply) {
    if (reply.ptr && spark_host.free_buffer) spark_host.free_buffer(reply.ptr, reply.len);
}
#endif
