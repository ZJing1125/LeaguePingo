# 测试与证据入口

先阅读 [ENVIRONMENTS.md](ENVIRONMENTS.md) 与根目录 [AGENTS.md](../AGENTS.md)，配置当前主机的工具和锁定依赖。下列入口不会自动安装工具或下载浏览器；默认测试不连接游戏。各层通过范围应分别报告。

## 自动验证

| 入口 | 范围 |
| --- | --- |
| `bash scripts/verify.sh portable` | 策略/工具脚本、前端类型/19 项纯逻辑单测与生产构建、internal Go test/vet、diff |
| `bash scripts/verify.sh windows` | Windows amd64 完整验证，含绑定生成/一致性、原生 Go、前端浏览器回归 |
| `bash scripts/build.sh` | 唯一 Windows 交付入口，内含完整验证、编译、登记、留存清理及只读审计 |
| `npm --prefix frontend run test:unit` | Vue 实际模块的 Node 环境纯逻辑测试，无浏览器/游戏 |
| `npm --prefix frontend run test:smoke` | 合成桥接的浏览器交互、错误/缓存/身份隔离和布局；先构建 frontend 并设置 `BROWSER_EXECUTABLE` |
| `node scripts/workflow.mjs check` | 构建/CI/依赖配置等工程策略 |

CI 位于 `.github/workflows/ci.yml`：Linux portable 成功后执行 Windows 统一构建；建议性分析单独产生结果。具体结果核对提交号和 [Actions](https://github.com/ZJing1125/LeaguePingo/actions)，不能把另一个提交的成功套用到当前改动。静态分析现有漏洞和工具链更新见 [DEPENDENCIES.md](DEPENDENCIES.md)。

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

## 人工验收集中后置

按用户安排，当前优先完成非人工任务；[ACCEPTANCE.md](ACCEPTANCE.md)、[HOLD_INPUT.md](HOLD_INPUT.md)、[INSTALLATION.md](INSTALLATION.md)与[性能记录](PERFORMANCE.md)中的整局、焦点/DPI/反作弊、干净 Win10、发布证书及 FPS/延迟检查保留待验收。CI、单账号只读成功或离线资源采样均不能替代这些结论。
