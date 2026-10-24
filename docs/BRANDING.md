# CPA Spark 标志

根目录的 [`logo.svg`](../logo.svg) 是项目标志的唯一编辑源。正式标志采用 C「燃时」方案：浅暖色圆角底、橙色时钟与补全缺口的星火，表达准点唤醒和轻量预热；图形为本项目绘制，不依赖外部字体、图片或图标库。

- README 与插件商店使用该 SVG。商店登记生成器默认指向实际仓库的 `main/logo.svg`，支持 `--logo` 显式覆盖。
- 构建脚本生成 `web/logo.svg`，随动态库嵌入并以 `image/svg+xml` 提供；不要直接修改生成副本。
- 页头和设置页共用品牌组件，使用同一 SVG 的内嵌数据地址。浏览器图标使用同源 SVG 资源，离线预览会将其内嵌。
- CPA 导航与原生插件元数据使用 [`sidebar-icon.svg`](../sidebar-icon.svg)：从 C「燃时」提取的透明底灰色符号，线宽对齐宿主菜单；构建生成 `web/sidebar-icon.svg` 并内嵌，不依赖 GitHub。页面、README、浏览器图标及商店登记保留彩色品牌标志。

修改标志后运行 `python3 scripts/build_web.py` 和 `make test`，同时提交源文件及生成文件。默认方形 viewBox 为 `0 0 64 64`；不要拉伸比例或在图案外再加另一层有色底。
