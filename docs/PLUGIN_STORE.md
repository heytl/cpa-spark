# GitHub 构建与 CPA 插件商店发布

## 发布结构

源码仓库保存 Go / 前端代码、生成脚本、测试、文档和 Actions 工作流。动态库、ZIP、数据库和日志不提交到源码仓库。

CPA 官方商店只维护 `registry.json`。插件的二进制与 `checksums.txt` 位于作者仓库的最新正式 GitHub Release。标签使用 `vX.Y.Z`，版本与 `internal/warmup/types.go` 一致。当前版本为 `0.1.8`。

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

原生验证成功才产生与库的 SHA-256 对应的 `abi.json`。打包任务核对凭据、架构、版本和 ZIP 布局，五个平台全部成功才产出带版本号的汇总下载，例如 **cpa-spark_0.1.8-dev.a976b6b_packages.zip**。开发安装包同样带开发版本与提交短码，且不生成商店登记文件；正式包保持 `cpa-spark_0.1.8_linux_arm64.zip` 形式。交叉编译产物不能伪造这份原生加载凭据。

Actions 依赖固定到 commit SHA，Dependabot 定期更新。普通 CI 仅有仓库读取权限；创建和发布 Release 的单独 job 才有 `contents: write`。这些流程只使用 GitHub 自带的 `GITHUB_TOKEN`，不需要 CPA key 或个人访问令牌。

## 创建第一个正式版本

1. GitHub 上创建仓库并推送 `main`，等待 **CI** 的五个平台全部通过。
2. 源码 `Version`、`CHANGELOG.md` 与 `CHANGELOG.en.md` 使用同一版本；当前可使用 `0.1.8`。
3. 推送匹配标签：

   ```bash
   git tag -a v0.1.8 -m "CPA Spark v0.1.8"
   git push origin v0.1.8
   ```

4. **Release** 验证标签和双语日志、重新测试构建，并创建草稿，资产如下：

   ```text
   cpa-spark_0.1.8_linux_amd64.zip
   cpa-spark_0.1.8_linux_arm64.zip
   cpa-spark_0.1.8_darwin_amd64.zip
   cpa-spark_0.1.8_darwin_arm64.zip
   cpa-spark_0.1.8_windows_amd64.zip
   checksums.txt
   registry.json
   STORE_PR.md
   ```

   每个 ZIP 根目录只含一个同平台的动态库。`checksums.txt` 用 SHA-256 校验 ZIP 本身。
5. 工作流自动调用 **Verify Release packages**，由现有 Release 创建任务下载草稿的实际资产，再通过同一次运行的 artifact 传递给只读验证任务，在各平台重新校验、加载。不依赖 Release 事件间接触发验证。
6. 将 ARM64 包安装到实际 CPA，检查元数据注册、套餐、模型目录、一次性预热和调度。GitHub 原生加载测试使用假宿主，不替代真实 CPA 验收。
7. 可以在 GitHub 手动发布已验证的草稿；也可进入 **Actions → Release → Run workflow**，输入已有 Tag，将 `mode` 选择为 `publish`。已有草稿会复用，不替换其资产和说明；已经正式发布的版本会拒绝覆盖。
8. `publish` 模式在草稿下载验证通过后发布为最新稳定版，再显式执行一次发布后下载验证。默认 `draft` 只生成并验证草稿。手动在 GitHub 页面发布也保留 `release.published` 验证入口。草稿不会作为商店 latest 的安装来源；当前发布流程仅支持稳定数字版本，不发布 prerelease。

Release 正文由两份更新日志中的对应版本生成，中文在前、英文在后，包含语言跳转、五平台下载、校验文件及“最新稳定版 / 全部版本 / Tags”入口。新增版本必须同步填写两种语言，缺失、重复或空条目会阻止发布。历史版本在 GitHub Releases 页面选择，不维护容易过期的硬编码版本下拉列表。

Linux CI 用 Ubuntu 22.04 原生 glibc 工具链，要求 glibc ≥ 2.35；不兼容 musl/Alpine。其他 Linux 构建基线以各发行说明为准。

推送到 main 后，CI 在五个平台检查全部成功时为新版本补建标签，并显式调用 Release 草稿构建和验证；已有标签保持原样，不覆盖资产；未完成的草稿可以继续验证。也保留上面的手动标签及手动 Release 入口。

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

构建与发布结果以对应提交的 Actions 为准。真实 CPA 验收与官方商店收录需分别确认。

## Docker 持久化

默认状态目录为 `./plugins/state/cpa-spark`。官方 Compose 工作目录 `/CLIProxyAPI` 下，对应 `/CLIProxyAPI/plugins/state/cpa-spark`，随插件目录挂载保留。使用其他工作目录或插件路径时，显式配置 `state_dir`。

## 规范来源

- 官方商店登记、标签、文件名、ZIP 和校验规范：https://github.com/router-for-me/CLIProxyAPI-Plugins-Store#readme
- CPA 插件 C ABI、管理资源和宿主回调：https://help.router-for.me/cn/plugin/development
- GitHub runner 标签：https://docs.github.com/en/actions/reference/runners/github-hosted-runners

核对日期：2026-10-22。

### 手动安装文件名

动态库可以使用 `cpa-spark.so` 或 `cpa-spark-v0.1.8.so`；带版本号时必须使用 `-v` 分隔，不能把 ZIP 的版本和平台命名直接套用到动态库。平台由目录 `plugins/linux/arm64/` 区分。商店安装器同样使用 `插件ID-v版本号.扩展名`，无需上架才能采用该格式。同一版本只保留一份动态库，替换后重启 CPA。
