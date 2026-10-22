# 参与开发

需要 Go 1.24+、C 编译器、Python 3.12+ 和 Node.js 22+。

```bash
npm ci --ignore-scripts
python3 scripts/build_web.py
make test
```

前端生成入口为 `design/prototype.html`、`web/live-bridge.js`、`web/task-editor.js` 和 `web/product-ui.js`；样式覆盖在 `web/product-ui.css` 与 `web/host-theme.css`，窗口和账号时间线分别位于 `web/window-timeline.*`、`web/account-timeline.js`。语言词条见 `web/locales/messages.json`。修改后重新生成并提交 `web/index.html`、`web/app.js`、`web/style.css`。CI 会检查生成结果是否一致。

调度和账号回调代码位于 `internal/warmup/`。本地测试使用假 CPA Host，不读取个人 OAuth、不访问真实模型。手动调用真实 CPA 需要单独选择测试账号，并会消耗账号额度。

提交 PR 时描述行为变化和验证结果。不得提交 SQLite 数据、运行日志、OAuth 凭据、管理密钥、编译出的动态库或平台安装包。

平台安装包只通过 GitHub Actions 在匹配 OS/CPU 的 runner 上构建和原生加载验证。构建资产留在 Actions / Releases，源码仓库保留构建输入。

未正式发布前的小型界面、文案和样式调整不自动增加版本号，可更新当前开发构建；需要发布新版本或用户明确要求时再递增。已发布的 Git tag 和正式 Release 保持不可变。

版本号的唯一来源是 `internal/warmup/types.go` 的 `Version`。发布标签必须为 `vX.Y.Z` 并与该值一致。发布说明和商店流程见 [发布文档](docs/PLUGIN_STORE.md)。

当前尚未指定项目源代码许可；依赖库的许可按各依赖原有条款执行。项目作者确定许可后再添加 LICENSE，并同步商店元数据。
