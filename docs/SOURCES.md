# 官方接口与第三方来源

本文记录当前实现依据的上游源码版本，便于后续核验接口和更新兼容层。这里列出的官方管理界面存储格式属于兼容实现依据，并非 CPA 承诺长期不变的插件会话 API。

## 原生插件与宿主调用

来源：[CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI)，核验版本 `0db19025571684b198a2ef508f99b41c717337e8`。

- [Host Model Callback 示例](https://github.com/router-for-me/CLIProxyAPI/blob/0db19025571684b198a2ef508f99b41c717337e8/examples/plugin/host-model-callback/go/main.go)：C ABI 类型、入口及宿主回调机制。
- [pluginabi/types.go](https://github.com/router-for-me/CLIProxyAPI/blob/0db19025571684b198a2ef508f99b41c717337e8/sdk/pluginabi/types.go)：ABI 1、RPC schema 6 与生命周期方法。
- [pluginapi/types.go](https://github.com/router-for-me/CLIProxyAPI/blob/0db19025571684b198a2ef508f99b41c717337e8/sdk/pluginapi/types.go)：`host.auth.list` 的账号字段及 `host.model.execute` 的 `auth_id`、`forced_provider` 等请求字段。

项目对应文件：`cmd/plugin/abi.h`、`cmd/plugin/main.go`、`internal/warmup/types.go`、`internal/warmup/app.go`。ABI 定义依据官方示例；请求校验、缓冲区生命周期、错误解析和应用调度分别组织，保留宿主函数表的副本，避免初始化调用方的临时地址失效。

CPA 原生示例采用 MIT 许可，完整声明见 [cliproxyapi-LICENSE.txt](third-party/cliproxyapi-LICENSE.txt)。

## 登录会话兼容

来源：[CPA Management Center](https://github.com/router-for-me/Cli-Proxy-API-Management-Center)，核验版本 `714de0760a3ce86094b178fb46f834d34bc4a2a5`。

- [encryption.ts](https://github.com/router-for-me/Cli-Proxy-API-Management-Center/blob/714de0760a3ce86094b178fb46f834d34bc4a2a5/src/utils/encryption.ts)：`enc::v1::` 前缀、盐值及按主机和 User-Agent 生成的 XOR 混淆格式。
- [secureStorage.ts](https://github.com/router-for-me/Cli-Proxy-API-Management-Center/blob/714de0760a3ce86094b178fb46f834d34bc4a2a5/src/services/storage/secureStorage.ts)：JSON 编码及旧版字符串存储兼容。
- [useAuthStore.ts](https://github.com/router-for-me/Cli-Proxy-API-Management-Center/blob/714de0760a3ce86094b178fb46f834d34bc4a2a5/src/stores/useAuthStore.ts) 与 [constants.ts](https://github.com/router-for-me/Cli-Proxy-API-Management-Center/blob/714de0760a3ce86094b178fb46f834d34bc4a2a5/src/utils/constants.ts)：`cli-proxy-auth`、Zustand 的 `state` 包装、`rememberPassword`、`isLoggedIn` 和旧版 `apiBase` / `apiUrl` / `managementKey` 字段。

项目对应文件：`web/cpa-session.js`。适配器分为存储解码、会话结构读取、同源与挂载路径校验三部分，只读已有登录状态，不写入或迁移管理密钥。没有记住密码、数据损坏、来源不一致或已退出登录时，不发送受保护请求。未识别的存储编码不回退读取旧凭据。

官方管理界面采用 MIT 许可，完整声明见 [cpa-webui-LICENSE.txt](third-party/cpa-webui-LICENSE.txt)。

## 图标

设置图标使用 [Lucide settings-2](https://github.com/lucide-icons/lucide/blob/main/icons/settings-2.svg)，其许可及 Feather 来源声明见 [lucide-LICENSE.txt](third-party/lucide-LICENSE.txt)。

## 构建产物中的声明

`scripts/build_web.py` 将上述声明合并到 `web/third-party-notices.txt`，随前端资源嵌入动态库；通过插件资源路径 `third-party-notices.txt` 可读取。平台安装 ZIP 仍只包含一个根目录动态库。

任务、分组、SQLite 存储、账号队列与时间轴由本项目维护。接口字段、存储常量和 ABI 符号需要保持与 CPA 一致，不以更改协议名称来制造实现差异。本文记录当前代码依据，不改写既有 Git 历史，也不作未经证实的代码原创性保证。
