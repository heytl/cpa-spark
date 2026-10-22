# CPA Spark · 星火

[![CI](https://github.com/heytl/cpa-spark/actions/workflows/ci.yml/badge.svg)](https://github.com/heytl/cpa-spark/actions/workflows/ci.yml)

**智能账号预热与额度窗口调度 / Account Warmup & Scheduling**

CPA Spark 是 [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) 的原生插件，支持为单个账号或自定义分组安排独立预热任务。调度在服务端运行，关闭浏览器后仍继续执行。

当前开发版本为 **0.1.7**，尚未上架 CPA 官方插件商店。请求成功不等于额度窗口已开启；窗口可视化为计划估算，实际额度重置以上游为准。

## 功能

- 单账号、多账号与静态分组；独立设置星期、多个时间点、时区、模型及推理强度。
- 模型使用所选账号的并集；单个账号目录为空或读取失败不阻止选择其他账号支持的模型。
- 新任务优先选择 Codex Luna / Gemini Flash，默认最低可用推理消耗；能力信息来自 CPA。
- 支持内置 IANA 时区及搜索，默认 `Asia/Shanghai`。
- 同时刻、同账号、同请求配置合并执行；同账号串行，全局并发 1–8，每提供商最多 2 个。
- SQLite 保存配置、执行结果与幂等凭据；禁用、过期或不可用账号在执行时跳过。
- 账号与分组可立即预热一次，不新增定时任务。
- 窗口可视化支持 24/48 小时；账号详情显示过去 24 小时执行记录和未来 24 小时计划。
- 执行记录支持单条删除、按筛选清除和全部清除；执行中的记录保留。
- 页面跟随 CPA 的语言与主题；自定义名称和描述保留原文，缺译回退整句英文。

## 运行要求

| 项目 | 要求 |
| --- | --- |
| CPA | C ABI 1、RPC schema ≥ 6，支持 `management_api`、`host.auth.list` 和 `host.model.execute` |
| 提供商 | Codex、Antigravity；模型实际权限由 CPA 和上游校验 |
| 构建环境 | Go 1.24+、CGO C 编译器、Python 3.12+、Node.js 22+ |
| 状态目录 | 本机可写、持久化目录；单个插件实例使用，不支持 NFS / 多节点共享 |

普通运行无需 Python、Node 或外网 CDN，前端通过 `go:embed` 内嵌到动态库。`no-plugin` CPA 构建不能加载本插件。

## 安装到 Docker Compose 部署的 CPA

在 CPA 插件管理页导入**匹配服务器平台的动态库**并启用。ARM64 Linux 使用 `cpa-spark.so`，不要导入源码 ZIP。

若手动复制文件，在 CPA Compose 项目目录执行：

```bash
mkdir -p plugins/linux/arm64
cp /你的下载目录/cpa-spark.so plugins/linux/arm64/cpa-spark.so
```

确认 Compose 已将插件目录挂载到容器，并在 CPA 配置中启用：

```yaml
plugins:
  enabled: true
  dir: "./plugins"
  configs:
    cpa-spark:
      enabled: true
      priority: 10
      state_dir: "./plugins/state/cpa-spark"
```

`state_dir` 相对于容器内 CPA 工作目录，必须可写并持久化。重新加载插件或重启 CPA 后，从插件资源菜单打开 **CPA Spark**。默认资源地址为 `/v0/resource/plugins/cpa-spark/status`；反向代理有部署前缀时需要带上该前缀。

页面静默复用同源 CPA 已保存的登录信息，**不需要在插件内输入 Management key**。CPA 登录时需勾选“记住密码”；退出或登录过期后，返回 CPA 登录。插件不另存密钥，不将其写入 SQLite、URL 或日志。

完整接口及部署验收说明见 [集成文档](docs/INTEGRATION.md)。

## 使用

1. 在“账号与分组”检查 CPA 账号，按需要建立分组。
2. 创建任务，设置账号、星期、时间、时区、模型和推理强度。
3. 检查服务端返回的未来五次预览并保存。
4. 如需即时验证，使用账号或分组的“立即预热”；这会消耗正常账号额度。
5. 在执行记录中查看逐账号结果；在账号时间线查看前后各 24 小时安排。

首次启动不会创建示例账号、自动启用任务或主动发送模型请求。插件不负责账号登录或 OAuth 刷新。

## 从源码构建

```bash
npm ci --ignore-scripts
python3 scripts/build_web.py
go mod download
python3 scripts/ci.py
make build
```

产物为当前平台的 `dist/cpa-spark.so`、`.dylib` 或 `.dll`，安装通常只需动态库。必须为实际运行的 OS/CPU 构建；Linux 动态库不能直接在 macOS 或 Windows 使用。

也可以运行匹配当前平台的原生构建及 ABI 验证：

```bash
python3 scripts/build_native.py --target linux_arm64
```

其他目标为 `linux_amd64`、`darwin_amd64`、`darwin_arm64`、`windows_amd64`。脚本拒绝在不同平台冒充原生构建。

## 自动化构建与插件商店

- 推送 `main` 或提交 PR：运行检查，分别在五个平台原生构建并加载 C ABI，输出安装包。
- 推送与源码版本匹配的 `v0.1.7` 标签：生成 Release **草稿**，包含平台 ZIP、SHA-256 校验、`registry.json` 和商店 PR 材料。
- 发布前需完成真实 CPA 验收；草稿及 prerelease 不作为商店 latest 安装来源。

工作流固定第三方 Actions 的 commit SHA，普通 CI 仅授予读取权限。只有创建 Release 草稿的任务使用仓库写权限；不需要个人令牌、CPA key 或 OAuth 凭据。

详见 [发布与插件商店流程](docs/PLUGIN_STORE.md)。小型开发调整不自动递增版本；已经发布的标签和 Release 保持不可变。

## 开发与目录

```bash
make dev      # 本地假 CPA，不读取真实 OAuth、不调用上游
make preview  # 生成自包含静态预览并检查交互
make test     # 与 CI 相同的完整检查
```

| 目录 | 内容 |
| --- | --- |
| `cmd/plugin` | CPA C ABI 动态库入口 |
| `cmd/dev` | 本地假宿主服务 |
| `internal/warmup` | 数据模型、SQLite、调度、Host 回调与管理接口 |
| `web` | 前端源码、语言目录和内嵌生成资源 |
| `design` | 页面模板与离线预览生成器 |
| `scripts` | 前端生成、检查、构建、打包与发布材料 |
| `tests` | 前端规则、交互、构建及原生 ABI 检查 |
| `docs` | 接口、集成、国际化与发布说明 |

不要直接修改 `web/index.html`、`web/app.js`、`web/style.css`；请修改对应源码后运行前端生成脚本。静态预览、动态库、数据库、日志和依赖目录不提交到源码仓库。

- [参与开发](CONTRIBUTING.md)
- [国际化维护](docs/I18N.md)
- [管理 API](docs/API.md)
- [更新记录](CHANGELOG.md)

## 执行语义与验证范围

新启动从未来分钟开始，默认不补发停机期间的请求。夏令时缺失时刻跳过、重复时刻采用第一次。同 UTC 时刻、同账号和同配置去重；不同配置分别调用。

已发出的请求不会伪报为取消。重启后在途结果标记为未知，不自动重放；SQLite 写入失败时停止自动分发。CPA 自身路由与重试配置仍可能影响实际网络尝试次数。

执行历史默认保留 30 天，可设 7/30/90 天。清理展示历史不会清理防重复执行凭据；凭据有界归档、历史分页、配置导入、通知及多节点协调尚未实现。

Linux amd64 已通过本地假宿主 C ABI 加载验证；ARM64 已交叉编译。其他平台及 GitHub runner 结果以实际 Actions 为准，假宿主测试不替代真实 CPA 验收。套餐显示读取明确元数据及 Codex JWT claims；未知套餐和窗口不作猜测。

## 许可与致谢

项目源代码暂未指定开源许可证。公开可见不代表授予额外使用许可，正式许可由作者后续确定。第三方图标许可见 [第三方说明](docs/third-party/README.md)，Go / npm 依赖保留各自许可。

原生接口遵循 [CPA 插件规范](https://help.router-for.me/cn/plugin/development.html)，参考 [Host Model Callback 示例](https://github.com/router-for-me/CLIProxyAPI/tree/main/examples/plugin/host-model-callback) 和 [cpa-window-starter](https://github.com/ZenGeekLabs/cpa-window-starter)。本项目独立实现任务与调度逻辑。
