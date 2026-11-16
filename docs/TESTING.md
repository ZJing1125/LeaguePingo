# 测试与证据入口

先阅读 [ENVIRONMENTS.md](ENVIRONMENTS.md) 与根目录 [AGENTS.md](../AGENTS.md)，配置当前主机的工具和锁定依赖。下列入口不会自动安装工具或下载浏览器；默认测试不连接游戏。各层通过范围应分别报告。

## 自动验证

| 入口 | 范围 |
| --- | --- |
| `bash scripts/verify.sh portable` | 策略/工具脚本、组件样式/文案、Go AST函数长度、前端类型/22 项纯逻辑单测与生产构建、internal Go test/vet、diff |
| `bash scripts/verify.sh windows` | Windows amd64 完整验证，含绑定生成/一致性、原生 Go test/vet及全包竞态、前端浏览器回归 |
| `go run ./cmd/functioncheck` | 检查Git受管及未忽略的新生产Go声明/闭包不超过100物理行；包含平台专用文件，排除测试 |
| `node scripts/check-copy.mjs` | 检查含根应用的全部产品组件无中文硬编码（脚本、模板、属性及转义）；沿用uiText，无新依赖 |
| `node scripts/check-styles.mjs` | 所有产品Vue样式使用scoped/module，已迁移组件的私有选择器不能回流全局；portable/Windows共用门禁 |
| `bash scripts/build.sh` | 唯一 Windows 交付入口，内含完整验证、编译、登记、留存清理及只读审计 |
| `npm --prefix frontend run test:unit` | Vue 实际模块的 Node 环境纯逻辑测试，无浏览器/游戏 |
| `npm --prefix frontend run test:smoke` | 合成桥接的浏览器交互、错误/缓存/身份隔离和布局；先构建 frontend 并设置 `BROWSER_EXECUTABLE` |
| `node scripts/workflow.mjs check` | 构建/CI/依赖配置等工程策略 |
| `node --test scripts/ci-download.test.mjs` | 临时夹具核验下载包内容、提交/哈希、脏或篡改产物拒绝，不代表真实EXE运行 |

CI 位于 `.github/workflows/ci.yml`：按portable→quality→Windows顺序执行；quality强制运行Windows目标staticcheck/govulncheck及npm audit，每项实际结果独立记录，失败阻止Windows统一构建。具体结果核对提交号和 [Actions](https://github.com/ZJing1125/LeaguePingo/actions)，不能把另一个提交的成功套用到当前改动。静态分析范围和工具链更新见 [DEPENDENCIES.md](DEPENDENCIES.md)。

Windows全包竞态通过`CGO_ENABLED=1 go test -race ./...`执行，需要本机PATH已有兼容Go竞态运行库的MinGW-w64 GCC；缺失明确失败，不自动安装。该CGO设置只作用于竞态命令，正式产品仍沿用原构建设置。perfprobe两项Windows测试只采样测试进程自身并校验CSV/进程身份，不连接客户端，不代表F5整局性能实测。

浏览器使用现有 Edge/Chromium，通过 `BROWSER_EXECUTABLE` 指定；也可用 `PLAYWRIGHT_MODULE` 指向现有包。测试只生成合成账号；截图可显式设置 `UI_SCREENSHOT_PREFIX` / `WORKSPACE_SCREENSHOT`，不提交图片或程序。正式构建的证据登记见 [ARTIFACTS.md](ARTIFACTS.md)。组件工作台入口和限制见 [FRONTEND_GUIDE.md](FRONTEND_GUIDE.md)。

## Windows 隔离原生检查

以下双进程/隐藏窗口测试不连接游戏，但需要 Windows 桌面环境：

```bash
LEAGUEPINGO_DESKTOP_TEST=1 go test . -run '^TestNativeDesktopSingleInstance$' -count=1 -v
```

## 显式国服检查

仅在用户准备好当前游戏电脑、稳定登录和对应阶段后执行；不能在云开发机运行后宣称客户端通过，不保存鉴权、身份或完整响应。以下是从原 README 移来的入口，当前批次没有执行：

```bash
# 当前账号：完整 ID 到首批战绩；输出脱敏计数
LEAGUEPINGO_LIVE_LOOKUP_TEST=1 go test ./internal/source -run '^TestLiveManualLookup$' -count=1 -v
# 自账号复制 ID：仅身份清理，不读战绩
LEAGUEPINGO_LIVE_COPIED_ID_TEST=1 go test ./internal/source -run '^TestLiveCopiedRiotIDLookup$' -count=1 -v
# 仅选人阶段：名单/8 秒定向通知，不选英雄、不读战绩
LEAGUEPINGO_LIVE_SELECTION_TEST=1 go test ./internal/source -run '^TestLiveChampSelectNotifications$' -count=1 -v
# 基础资料/头像，不请求段位和历史、不保存图片
LEAGUEPINGO_LIVE_PROFILE_TEST=1 go test ./internal/source -run '^TestLiveBasicProfileAvatar$' -count=1 -v
# 非选人/非正式对局：真实后续历史页
LEAGUEPINGO_LIVE_PAGING_TEST=1 go test ./internal/source -run '^TestLiveHistoryPagination$' -count=1 -v
# 仅进行中的标准 5v5：本地名单，不读战绩
LEAGUEPINGO_LIVE_ROSTER_TEST=1 go test ./internal/source -run '^TestLiveRoster$' -count=1 -v
```

数据双源对照使用 [PLAYER_DATA_AUDIT.md](PLAYER_DATA_AUDIT.md)的有界入口，组队真实准确性使用 [PREMADE_GROUPS.md](PREMADE_GROUPS.md)的匿名对照。跳过、不在适用阶段或缺少客户端均不能记为通过。

S9严格证书候选使用[LCU_TLS.md](LCU_TLS.md)的显式测试，仅阶段HTTPS和定向WSS，无玩家读取或写动作；该测试默认跳过，生产连接方式未改。Riot的Game Client根不是已证明兼容国服LCU的根，必须保留NOT VERIFIED边界。

## 人工验收集中后置

每次发版的基础步骤和空白结果表见[R3固定回归](RELEASE_REGRESSION.md)。六项模板均为NOT_RUN；只有同候选的R1及实际步骤完成后才能记录R3通过。

按用户安排，当前优先完成非人工任务；[ACCEPTANCE.md](ACCEPTANCE.md)、[HOLD_INPUT.md](HOLD_INPUT.md)、[INSTALLATION.md](INSTALLATION.md)与[性能记录](PERFORMANCE.md)中的整局、焦点/DPI/反作弊、干净 Win10、发布证书及 FPS/延迟检查保留待验收。CI、单账号只读成功或离线资源采样均不能替代这些结论。
