# 工程结构与维护约定

更新时间：2026-10-18。产品先完成 Windows，macOS 待 Windows 成熟后适配。本轮只整理工程，不扩展功能或开展实机验收。

## 目录与职责

```text
league-pingo/
├── main.go / appservice.go / overlay.go / preferences.go
│   └── 桌面启动、Wails 服务入口、快览和设置桥接
├── internal/
│   ├── core/          业务模型与战绩解析
│   ├── engine/        阶段机、调度与缓存
│   ├── lcu/           本机客户端发现、通信与阶段通知
│   ├── source/        腾讯数据源适配及未来数据源扩展接口
│   ├── settings/      本地非敏感设置
│   └── diagnostics/   脱敏诊断汇总
├── frontend/
│   ├── src/           Vue / TypeScript / CSS 和游戏资源
│   ├── bindings/      Wails 生成的接口与模型
│   └── tests/         合成数据浏览器回归
├── build/
│   ├── windows/       Windows 资源与打包任务
│   ├── darwin/        为后续 macOS 适配保留的构建资源
│   ├── docker/        桌面交叉编译辅助模板
│   └── Taskfile.yml / config.yml / 图标源文件
├── cmd/perfprobe/     Windows 离线资源采样工具
├── scripts/           统一验证、构建与游戏资源维护脚本
├── docs/              架构、设计、验收、性能与问题记录
└── BUGFIX_LOG.md       追加维护的修复与工程问题记录
```

根目录的桌面入口保持原位，避免为目录美观改变 Go 模块路径、嵌入资源和 Wails 绑定。测试与对应 Go 包同目录；前端工程说明见 [frontend/README.md](../frontend/README.md)。

## 平台和构建入口

| 入口 / 资源 | 当前用途 |
| --- | --- |
| `scripts/verify.sh` | 绑定、前端构建、Go 测试/静态检查与离线浏览器回归 |
| `scripts/build.sh <输出路径>` | Windows 本地测试程序；先统一验证，再嵌入提交号和构建时间 |
| 根目录 `Taskfile.yml` | Wails 桌面任务入口，仅引用 common、windows、darwin |
| `build/windows/` | 原生构建资源，保留 NSIS/MSIX/签名任务；正式安装器与签名仍待配置验收 |
| `build/darwin/` | 保留 macOS 模板；尚未完成客户端适配及实机验收 |
| `build/docker/Dockerfile.cross` | Windows/macOS 任务仍引用的可选交叉编译辅助工具；Windows 本机构建不依赖 Docker |

Docker 交叉编译模板内部沿用上游的通用编译器支持，不代表提供 Linux 产品目标；本轮没有安装 Docker、构建镜像或验证交叉编译。已移除服务端 Dockerfile 与服务端构建/运行入口。

## 源码、生成文件和本地文件

- 维护源码：Go 业务代码、前端源码、测试、构建任务及说明文档。
- 生成且跟踪：`frontend/bindings/` 通过固定版本 Wails CLI 生成，代码审查中核对接口变化；`.gitattributes` 标记为 `linguist-generated`，不手工修改。
- 本地且忽略：`.tools/`、`.task/`、`bin/`、`frontend/node_modules/`、`frontend/dist/`、Windows 临时 `.syso`、安装器输出及 WebView2 引导程序。
- 构建资源不是构建产物：`build/windows/` 的 manifest、安装脚本和 `build/darwin/` 的 plist/图标仍需纳入版本控制。
- 查询到的玩家身份、战绩、客户端凭据不属于工程文件，不落盘、不提交。

## 模板维护

本轮移除 Android、iOS、Linux 平台目录，移动端设备任务、Puppertino 下载任务、服务端部署任务及未使用的前端欢迎页资源。对应文件保存在 Git 历史中，可按路径恢复；没有清理本机工具缓存、已有测试程序或用户配置。

已移除会整套覆盖构建资源的 `common:update:build-assets` 快捷任务。升级 Wails 时，在独立临时目录生成模板并审查差异，只合入 Windows/macOS 所需变更；不要直接在当前工程整套重新生成，以免带回无关平台或覆盖维护中的任务。

Taskfile 的增量输入按源码目录声明：Go 入口、`internal/`、`cmd/` 和前端源码/绑定/配置。避免从仓库根目录递归匹配全部源码、或把 `node_modules`、`dist`、`.tools` 缓存纳入校验；新增源码目录时同步更新对应的 `sources`。

GitHub Languages 按默认分支的文件统计。当前默认分支为 `main`，本轮整理提交到 `dev`，因此主页语言栏在合并前可能仍显示原来的 Java 比例。仅自动生成绑定被标记；业务代码、测试和实际维护的 NSIS 脚本不为美化比例而隐藏。本轮不变更默认分支、不合并 `main`。
