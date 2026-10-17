# LeaguePingo

LeaguePingo 是面向《英雄联盟》腾讯国服的 Windows 桌面战绩助手原型，使用 Wails 3 beta、Go、Vue 3 和 TypeScript。代码托管于私有 GitHub 仓库，仍处于原型开发与国服实机联调阶段。

## 当前功能

- 局内外均可输入完整 `玩家名#标签`，首次读取至多 20 场混合模式战绩；可显式加载更多至最多 1000 条，按比赛 ID 去重。筛选与统计只作用已加载样本。
- 大厅、排队、接受确认阶段准备客户端提供的房间成员；选人阶段读取可提供的本局名单，单路串行准备已知 PUUID。未知身份保留占位，不构造尚未确定的对手。
- 进入 `GameStart`、`InProgress` 或 `Reconnect` 时取消自动战绩预取，最多尝试三次本地名单刷新，补全客户端已提供的双方身份；取得完整十人名单和对局 ID 后停止自动名单刷新。
- 主窗口可“刷新对局名单”，也可选中已知身份玩家后“手动查询此玩家”。局内不自动补查新识别玩家的战绩，只有用户主动查询才发起请求；手动入口一次只处理一个操作，超时上限 12 秒。
- 全局 `Ctrl+Shift+P` 显示或隐藏快览。快览可拖动、隐藏、切换置顶，不提供鼠标穿透；资料查询和名单刷新在主窗口进行。
- 不注入游戏进程、不读取游戏内存、不拦截或代理游戏流量。
- 玩家战绩与当前对局分开；最多保留八个内存玩家标签、模式筛选和展开状态。未知字段显示 `—`，胜率/KDA 按有效样本统计，样本排除会提示；筛选不等同于该模式最近 20 场。
- 本地中文英雄/装备/技能资源；本玩家摘要包括时长、装备、技能、补刀、金币与伤害。可点击“读取双方详情”请求整场参与者，标准完整 5v5 且相关字段齐全时计算团队占比；接口兼容待国服实测。
- 可独立更新等级/段位/隐私，不重拉历史。仅身份匹配且客户端明确返回 PUBLIC/PRIVATE 时显示公开/生涯不公开；失败不推断隐私或未定级。真实头像尚未显示。
- 五分钟缓存、连接隔离、失败保留；选人整次会话最多 45 秒、每人两次，短暂阶段失败有界恢复，英雄变化不重置预算。分页/双方详情仅显式请求，与查询共享调度。
- 设置与诊断：窗口位置/大小和置顶记忆，可配置快览快捷键，显示版本/提交号；按需本地导出脱敏汇总，不上传。快览首次打开才创建第二个 WebView。

当前是可构建原型，**不是已完成国服实机联调的正式产品**。本机已验证客户端自动发现、阶段接口、自账号完整 ID 查询及 20 场战绩解析；曾在真实对局中确认本地 gameflow 接口提供双方各五个有效身份。新版从名单刷新到界面展示的整局流程、其他玩家的战绩可用性、隐私信号、不同模式，以及游戏内悬浮兼容性仍待实测。详见 [待解决问题](docs/OPEN_ISSUES.md) 与 [Bug 修复记录](BUGFIX_LOG.md)。

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

开发运行可用 `wails3 dev`。首次运行需已安装 Microsoft Edge WebView2。启动国服 League 客户端后，LeaguePingo 会发现 `LeagueClient.exe` / `LeagueClientUx.exe`，先尝试同目录的 `lockfile`，文件为空或不可用时改用 Windows 提供的客户端启动参数。也可设置 `LEAGUEPINGO_LOCKFILE` 指向测试用 lockfile。切勿提交、发送或记录 lockfile 内容或完整客户端启动参数，其中包含本地客户端密码。

本批独立测试版为 `bin/leaguepingo-dev-20261016-v3.exe`。使用前请从托盘菜单退出当前运行的旧版助手，包括 `leaguepingo-dev.exe`；不需要退出游戏。新版采用固定应用标识的单实例策略：如果旧版未退出，启动新版只会唤起旧实例，不能热替换旧进程。更早的 `leaguepingo.exe`、`leaguepingo-fixed.exe` 没有该约束，也需手动退出。实际版本/提交号见“设置与诊断”。已进入对局时，新版会有限刷新本地名单，不会自动补查战绩；缺少战绩时在主窗口主动查询。

统一验证：`bash scripts/verify.sh`。推荐构建：`bash scripts/build.sh bin/leaguepingo-dev-20261016-v3.exe`，运行绑定生成/构建/测试/静态检查与浏览器回归，嵌入当前提交号和 UTC 时间并输出 SHA-256。若系统 npm 缓存不可写，可用 `npm --cache E:/league-pingo/.tools/npm-cache --prefix frontend ci`。避免覆盖被占用的旧版程序。可执行文件、诊断文件和测试截图不提交 Git。`main` 不改动，各改进批次单独提交推送 `dev`。

关闭主窗口后助手会留在系统托盘；点击托盘图标可重新打开，托盘菜单提供“退出”。

## 快览窗口操作

- 拖动标题栏移动窗口；× 隐藏快览，窗口获焦时也可按 Esc；标题栏可切换置顶。
- 默认 `Ctrl+Shift+P` 显示/隐藏，可在“设置与诊断”更换；主窗口与托盘也可隐藏快览，不依赖快捷键。
- 已删除鼠标穿透、恢复鼠标的按钮/托盘项和 `Ctrl+Shift+O` 注册；快览一直接受鼠标操作。
- 快捷键注册失败会提示，可继续使用主窗口、关闭按钮和托盘。

## 本批验证

除 `go test ./...`、`go vet ./...` 与前端构建外，可按需执行：

```bash
# 隔离的隐藏原生窗口与双进程验证，不连接 League 客户端
LEAGUEPINGO_DESKTOP_TEST=1 go test . -run '^TestNativeDesktopSingleInstance$' -count=1 -v

# 显式选择执行：用当前账号验证完整 ID 到战绩，不输出账号数据
LEAGUEPINGO_LIVE_LOOKUP_TEST=1 go test ./internal/source -run '^TestLiveManualLookup$' -count=1 -v

# 仅处于正在进行的标准 5v5 对局时执行：检查本地名单数量，不请求战绩
LEAGUEPINGO_LIVE_ROSTER_TEST=1 go test ./internal/source -run '^TestLiveRoster$' -count=1 -v
```

浏览器交互测试：在 frontend 目录执行 `npm ci` 安装锁定的依赖（包含 Playwright），设置 `BROWSER_EXECUTABLE` 为本地 Edge/Chromium 路径后，运行 `npm run test:smoke`。也可用 `PLAYWRIGHT_MODULE` 指定已有 Playwright 包；不自动下载浏览器。需先构建前端，使用模拟绑定及合成玩家，不等同于真实游戏。`WORKSPACE_SCREENSHOT` 可保存玩家工作区截图。

默认 `go test ./...` 不连接真实客户端；上述实机检查必须显式启用。完整 ID 自账号查询已返回 20 场战绩；新版 `TestLiveRoster` 当时因对局结束而跳过，仍需下一局验证，不能将其计为通过。已经采集包含 WebView2 的短时离线内存/CPU 基线，详见 [性能验证](docs/PERFORMANCE.md)；未测游戏 FPS/延迟，不承诺零开销。

## 数据与隐私边界

“身份不可获取”仅表示当前客户端数据没有可用 PUUID，不能把它等同于玩家主动隐藏生涯。“查询失败”也不能作为生涯不公开的证据。界面把无法核验的生涯隐私状态标为“未核验”；识别“生涯不公开但身份已知”的可靠国服信号仍待实机验证，未完成前不会擅自贴标签。助手不持久保存查询到的战绩或玩家 ID，数据只保存在当前进程内。

非敏感窗口/快捷键设置存储在 Windows 用户配置目录的 `LeaguePingo/settings.json`；手动导出的诊断位于其 `diagnostics` 子目录，只包含固定类别计数/耗时/状态码与运行时信息。无自动遥测、上传或战绩落盘。静态游戏资源来源及发布前政策核验要求见 [第三方声明](THIRD_PARTY_NOTICES.md)。

架构与扩展边界见 [架构说明](docs/ARCHITECTURE.md)。
