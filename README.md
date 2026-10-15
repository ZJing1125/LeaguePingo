# LeaguePingo

LeaguePingo 是面向《英雄联盟》腾讯国服的 Windows 桌面战绩助手原型，使用 Wails 3 beta、Go、Vue 3 和 TypeScript。当前仅在本地开发，未创建远程仓库或推送代码。

## 当前功能

- 对局外输入完整 `玩家名#标签`，通过本机 League 客户端查询该玩家最近至多 20 场可核验的战绩。
- 选人阶段自动读取客户端可提供的名单，单路串行准备已知 PUUID 的名称和战绩；无法获取身份的位置标为“身份不可获取”。
- 进入 `GameStart` 或 `InProgress` 等正式对局阶段后冻结现有资料，不再自动请求历史战绩；全局 `Ctrl+Shift+P` 显示或隐藏只读快览窗口。
- 不注入游戏进程、不读取游戏内存、不拦截或代理游戏流量。

当前是可构建原型，**不是已完成国服实机联调的正式产品**。尤其是国服客户端端点、隐私状态信号、不同模式的名单结构，以及悬浮窗在游戏中的兼容性仍需实测。详见 [待解决问题](docs/OPEN_ISSUES.md)。

## 运行与构建

在 Windows Git Bash 中进入 `E:/league-pingo`。需要 Go 1.25+、Node.js 22+、Wails 3 CLI beta.29 和依赖安装。当前机器上开发工具暂存在项目 `.tools/` 中（已忽略，不纳入源码）。

```bash
export PATH=/e/league-pingo/.tools/go/bin:/e/league-pingo/.tools/gopath/bin:/e/nodejs:$PATH
export GOPROXY=https://goproxy.cn
export GOSUMDB=sum.golang.google.cn
export GOPATH=/e/league-pingo/.tools/gopath
export GOCACHE=/e/league-pingo/.tools/gocache

wails3 generate bindings -clean=true -ts -i
npm --prefix frontend run build
go test ./...
go build -tags production -trimpath -buildvcs=false -ldflags='-H windowsgui' -o bin/leaguepingo.exe .
```

开发运行可用 `wails3 dev`。首次运行需已安装 Microsoft Edge WebView2。启动国服 League 客户端后，LeaguePingo 会自行发现 `LeagueClient.exe` 同目录的 `lockfile`；也可设置 `LEAGUEPINGO_LOCKFILE` 指向测试用 lockfile。切勿提交、发送或记录 lockfile 内容，其中包含本地客户端密码。

关闭主窗口后助手会留在系统托盘；点击托盘图标可重新打开，托盘菜单提供“退出”。

## 数据与隐私边界

“身份不可获取”仅表示当前客户端数据没有可用 PUUID，不能把它等同于玩家主动隐藏生涯。“查询失败”也不能作为生涯不公开的证据。界面把无法核验的生涯隐私状态标为“未核验”；识别“生涯不公开但身份已知”的可靠国服信号仍待实机验证，未完成前不会擅自贴标签。助手不持久保存查询到的战绩或玩家 ID，数据只保存在当前进程内。

架构与扩展边界见 [架构说明](docs/ARCHITECTURE.md)。
