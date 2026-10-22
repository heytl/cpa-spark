# GitHub 构建与 CPA 插件商店发布

## 发布结构

源码仓库保存 Go / 前端代码、生成脚本、测试、文档和 Actions 工作流。动态库、ZIP、数据库和日志不提交到源码仓库。

CPA 官方商店只维护 `registry.json`。插件的二进制与 `checksums.txt` 位于作者仓库的最新正式 GitHub Release。标签使用 `vX.Y.Z`，版本与 `internal/warmup/types.go` 一致。当前版本为 `0.1.7`。

## 推送代码后自动执行

每次推送 `main`、提交 PR 或手动运行 **CI**，会在以下 runner 上分别测试和原生构建：

| 平台 | 原生 runner | 动态库 |
| --- | --- | --- |
| Linux amd64 | ubuntu-22.04 | cpa-spark.so |
| Linux arm64 | ubuntu-22.04-arm | cpa-spark.so |
| macOS Intel | macos-15-intel | cpa-spark.dylib |
| macOS Apple Silicon | macos-15 | cpa-spark.dylib |
| Windows amd64 | windows-2022 + MinGW-w64 | cpa-spark.dll |

CI 先重新生成前端，并检查已提交生成结果与源码一致，再运行竞态检查、套餐解析测试、前端交互和打包测试。每个平台随后通过 C ABI 实际加载动态库，验证元数据、内嵌页面、账户绑定、配置、执行、记录清理和关闭过程。

原生验证成功才产生与库的 SHA-256 对应的 `abi.json`。打包任务核对凭据、架构、版本和 ZIP 布局，五个平台全部成功才产出 **store-packages**。交叉编译产物不能伪造这份原生加载凭据。

Actions 依赖固定到 commit SHA，Dependabot 定期更新。普通 CI 仅有仓库读取权限；创建 Release 草稿的单独 job 才有 `contents: write`。这些流程只使用 GitHub 自带的 `GITHUB_TOKEN`，不需要 CPA key 或个人访问令牌。

## 创建第一个正式版本

1. GitHub 上创建仓库并推送 `main`，等待 **CI** 的五个平台全部通过。
2. 源码 `Version` 和 CHANGELOG 使用同一版本；当前可使用 `0.1.7`。
3. 推送匹配标签：

   ```bash
   git tag -a v0.1.7 -m "CPA Spark v0.1.7"
   git push origin v0.1.7
   ```

4. **Build release draft** 验证标签、重新测试构建，并创建草稿，资产如下：

   ```text
   cpa-spark_0.1.7_linux_amd64.zip
   cpa-spark_0.1.7_linux_arm64.zip
   cpa-spark_0.1.7_darwin_amd64.zip
   cpa-spark_0.1.7_darwin_arm64.zip
   cpa-spark_0.1.7_windows_amd64.zip
   checksums.txt
   registry.json
   STORE_PR.md
   ```

   每个 ZIP 根目录只含一个同平台的动态库。`checksums.txt` 用 SHA-256 校验 ZIP 本身。
5. 可以对已有标签手动重跑草稿构建。已存在的草稿或正式版本不会被流程覆盖；先检查失败原因，再决定如何处理。
6. 先运行 **Verify published store packages**，手动输入草稿标签。它通过当前仓库 token 下载草稿的实际资产，并在各平台重新校验、加载；名称虽然包含 published，也可用于草稿验收。
7. 将 ARM64 包安装到实际 CPA，检查元数据注册、套餐、模型目录、一次性预热和调度。GitHub 原生加载测试使用假宿主，不替代真实 CPA 验收。
8. 发布草稿。发布事件会再次触发资产校验。草稿和 prerelease 不能作为商店 latest 的安装来源。

Linux CI 用 Ubuntu 22.04 原生 glibc 工具链，要求 glibc ≥ 2.35；不兼容 musl/Alpine。其他 Linux 构建基线以各发行说明为准。

## 先验证自定义商店源

`registry.json` 由 Actions 根据实际 GitHub 仓库和 owner 生成；不会发布占位作者或假地址。首次正式发布后，可在 CPA 插件商店添加：

```text
https://github.com/OWNER/cpa-spark/releases/latest/download/registry.json
```

这里的 `OWNER` 替换为实际仓库所有者；Release 正文会给出可直接复制的地址。该源跟随最新正式 Release 更新，无需每次修改商店版本。

## 申请官方商店收录

1. 确保仓库、正式 Release、五个平台 ZIP 和 `checksums.txt` 均可公开访问。
2. 等待发布资产验证成功，完成至少一台实际 CPA 服务器安装验收。
3. 下载 Release 的 `registry.json`，把 `plugins` 数组中的 `cpa-spark` 对象加入官方仓库的数组；不要覆盖官方已有条目。
4. 向 `router-for-me/CLIProxyAPI-Plugins-Store` 发起 PR，原则上只修改其 `registry.json`。
5. Release 中的 `STORE_PR.md` 已根据实际仓库和版本填写链接，可作 PR 正文草稿；提交前补充实际 CPA 验收结果。

正式登记的必填项为 `id`、`name`、`description`、`author`、`repository`；`logo`、`homepage`、`license`、`tags` 等为可选。项目许可由作者决定，未确定前不写入虚构的 license 值。后续更新只需发布新的数字版本，通常不必再次更新官方商店条目。

本轮任务仅准备源码、GitHub 构建与发布配置；连接 GitHub 并实际推送前，不宣称已建仓库、已运行远端 CI 或已上架。

## Docker 持久化

默认状态目录为 `./plugins/state/cpa-spark`。官方 Compose 工作目录 `/CLIProxyAPI` 下，对应 `/CLIProxyAPI/plugins/state/cpa-spark`，随插件目录挂载保留。使用其他工作目录或插件路径时，显式配置 `state_dir`。

## 规范来源

- 官方商店登记、标签、文件名、ZIP 和校验规范：https://github.com/router-for-me/CLIProxyAPI-Plugins-Store#readme
- CPA 插件 C ABI、管理资源和宿主回调：https://help.router-for.me/cn/plugin/development
- GitHub runner 标签：https://docs.github.com/en/actions/reference/runners/github-hosted-runners

核对日期：2026-10-21。
