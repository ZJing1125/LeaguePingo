---
title: LeaguePingo 优化待办（agent 友好版）
schema_version: 1
generated: 2026-10-29
review_type: 静态审阅；未运行 go test / go build / 真机（审阅环境无 Go 工具链）
snapshot: >-
  dev 分支 ZIP，最新提交说明为 "fix: retain partial group metadata and compare
  anonymous party baselines"（截图可见哈希 cc229e7；ZIP 内无 .git，未核对哈希）
suggested_repo_path: docs/OPTIMIZATION_BACKLOG.md
---

# 1. 使用规则（agent 先读）

执行记录（实施起始基线4d57e0b，附件审阅基线cc229e7）：Q1已实施并通过dev推送实跑，见下方记录；Q4建议性staticcheck、govulncheck与npm audit已实施，强制门禁等待Go标准库漏洞随依赖治理清零；F1本地度量已实施，口径与验证见PERFORMANCE.md；F2已核实后端逐人发布链路并补齐前端骨架和逐卡替换回归；S3共享动作执行器已实施并接入自动接受/赛后流程，两个编排循环均小于60行；S4引擎Stage/Phase强类型与集中行为表已实施，对外快照字符串未变且远程Windows验证通过；S5攻略会话复用、功能开关、超时/缓存/原文降级已实施并由单测覆盖；U1错误码与中文提示、U2降级与有界恢复已实施，协议与验证分别见ERRORS.md、DATA_RECOVERY.md及本提交CI，R1实验性标注与分轮记录已准备；用户已反馈A1/A2/P1/P2没有问题，后续确认跳过可用但继续/返回/开排链条失败，修复版与版本信息及其他项目待补，整体R1未完成；S1回调瘦身/备用入口核验/设置说明已实施，国服反作弊专项待实机；U3启动前依赖检查、静态版本检查、NSIS与证书存储签名代码链已实施，发布证书与干净Win10的I1–I6仍待人工；U4实验性说明、行为边界和一键停用已实施并由前端合成回归覆盖；其余任务尚未实施。该清单是任务输入，原审阅统计与推断不能当作当前事实。其他任务仍按依赖逐项核实；未执行的Windows/国服检查保留未验证状态。

Q1首次运行：[40856437383](https://github.com/ZJing1125/LeaguePingo/actions/runs/40856437383)，Linux成功，Windows原生Go检查通过，侧栏720px五卡高度回归失败。已根据日志修复摘要内边距，保留原验收断言。修复提交dd33476的[40857941063](https://github.com/ZJing1125/LeaguePingo/actions/runs/40857941063)两个job全部成功：Windows全包test/vet、绑定生成/一致性、九项浏览器回归、EXE编译及产物审计均通过。EXE只存在runner，未上传、部署或登记为用户已验收；截图保留7天。PR事件配置已做语法检查，未专门新建PR测试。默认分支仍为main，手动触发入口需工作流进入默认分支后才能使用；本批未修改main或默认分支。

已核实差异：海克斯实际使用固定公共专页；当前guides.Service已为每次内容任务设置8秒总超时及30秒失败冷却，基础攻略与对位分别有界加载。S5原文“最坏15秒”不适用于该产品调用路径。F2/F3需核对既有逐人发布、缓存先显后更新，不能直接重写。Q4建议性检查单独实施，不把尚未运行的检查列为通过。

1. 先读仓库的 `AGENTS.md`，其规则优先于本文件。
2. 一次只做一个任务（一个任务一个提交或 PR），先看 `depends_on`，并满足 `do_not`。
3. 改动前后各跑一次 `verify` 里列出的命令，把结果写进提交说明。不得声称通过了未运行的检查。
4. `confidence` 说明证据强度：
   - `verified-static`：我读过对应代码，位置准确，但没有运行。
   - `inferred`：根据结构或文档推断，动手前先自行核实。
   - `unverified`：未核实是否存在，先调查再决定是否做。
5. `needs_human: true` 表示需要真实 Windows 机器或国服客户端，agent 不得虚构实机结果（仓库文档明确禁止）。
6. 行号来自当前 ZIP，改动后会漂移，请以符号名为准。
7. `effort` 是粗略估计：S 约半天内，M 约 1 到 2 天，L 多天。

# 2. 项目事实

```yaml
product: 本地优先的英雄联盟（腾讯国服）战绩助手，Windows 桌面程序
stack: Go 1.25 + Wails v3 (3.0.0-beta.29) + Vue 3 + TypeScript + Vite
data_sources:
  - 本机客户端 LCU 接口（127.0.0.1，凭据来自 lockfile 或进程）
  - 腾讯 SGP 战绩服务（固定 *-sgp.lol.qq.com:21019）
  - OP.GG MCP 接口（https://mcp-api.op.gg/mcp，海克斯攻略）
layout:
  main.go: 程序入口与组装（main() 约 344 行）
  appservice.go: Wails 桥接，约 20 个转发方法
  internal/core: 纯模型与解析
  internal/source: 数据源接口与腾讯适配（sgp.go 等）
  internal/engine: 阶段机、调度、缓存、分组、历史（约 3738 行非测试代码，16 个文件）
  internal/lcu: 本机客户端接口、事件
  internal/hold: Windows 低级键盘钩子（monitor_windows.go）
  internal/readycheck: 自动接受控制器
  internal/postgame: 赛后动作控制器
  internal/guides: OP.GG 海克斯攻略
  internal/settings: 设置持久化
  frontend/src: Vue 界面；frontend/tests/*.cjs 为 Playwright 脚本
  scripts/: build.sh verify.sh workflow.mjs artifacts.mjs environment.mjs 等
  docs/: ARCHITECTURE.md ACCEPTANCE.md AUTO_ACCEPT.md PREMADE_GROUPS.md ENVIRONMENTS.md
commands:
  portable_verify: bash scripts/verify.sh portable   # Linux 可跑的子集
  windows_verify: bash scripts/verify.sh windows     # 需要 Windows amd64、wails3、Edge/Chrome
  windows_build: bash scripts/build.sh               # 唯一受管的交付构建入口（内含 verify）
  frontend_build: npm --prefix frontend run build
  frontend_smoke: npm --prefix frontend run test:smoke   # 需要 BROWSER_EXECUTABLE
  policy_check: node scripts/workflow.mjs check
metrics_snapshot:
  go_src_lines: 12077
  go_test_lines: 9747
  go_test_funcs: 292
  go_funcs_over_100_lines: 19
  markdown_bytes: 468000
```

# 3. 不可违反的约束

```yaml
constraints:
  - id: C1
    rule: Windows 编译只能走 scripts/build.sh，不得在 Taskfile 或脚本里直接写 go build
    source: scripts/workflow.mjs（"Unmanaged compiler"、"Delivery step order changed"）
  - id: C2
    rule: docs/ci/verify.yml.example 必须保留字符串 "bash scripts/build.sh bin/leaguepingo-ci.exe"，不要修改或删除该文件
    source: scripts/workflow.mjs、scripts/workflow.test.mjs
  - id: C3
    rule: 前端 @wailsio/runtime 与后端 wails/v3 版本必须一致（当前 3.0.0-beta.29）
    source: scripts/verify.sh
  - id: C4
    rule: 构建环境变量 GOFLAGS、GARBLE_ARGS、EXTRA_TAGS 等必须为空，GOOS/GOARCH 只能是 windows/amd64
    source: scripts/workflow.mjs checkEnvironment
  - id: C5
    rule: NSIS打包和证书存储签名仅走受管脚本并受workflow策略/测试保护；MSIX、跨平台打包仍为unsupported
    source: scripts/workflow.mjs（expected 映射）
  - id: C6
    rule: 键盘钩子不得吞键，必须始终调用 CallNextHookEx，且只处理已配置的按键
    source: internal/hold/monitor_windows.go 与 README
  - id: C7
    rule: 不得向游戏注入、读内存或模拟点击；自动化只能通过 LCU 接口
    source: README / docs/AUTO_ACCEPT.md
  - id: C8
    rule: 组队分组码只存内存，不写入磁盘、设置或诊断；缓存写盘前不得包含昵称；令牌只存内存
    source: docs/PREMADE_GROUPS.md、internal/engine/history_store.go
  - id: C9
    rule: 不得虚构真机或国服验收结果；未验收的功能必须标注
    source: docs/ACCEPTANCE.md、BUGFIX_LOG.md
  - id: C10
    rule: 规则、任务入口、脚本、文档和测试必须同步维护；AGENTS.md 中的固定短语（如 "bash scripts/build.sh"、"node scripts/artifacts.mjs audit"）不得删除
    source: AGENTS.md、scripts/workflow.mjs
```

# 4. 建议执行顺序

```yaml
order:
  - Q1    # 先上 CI，后面的重构才有保护
  - F1    # 先加度量，再谈优化
  - U1    # 错误码化，后续降级逻辑依赖它
  - U2
  - R1    # 真机验收，需要人
  - S1    # 键盘钩子实测，需要人
  - U3
  - S4    # 类型化状态，先于拆 engine
  - S3
  - S2
  - Q2
  - S5
  - S6
  - S7
  - Q3
  - Q4
  - U4
  - R2
  - F2
  - F3
  - F4
  - Q7
  - 其余 P2
```

# 5. 任务

## 系统设计

```yaml
id: S1
title: 键盘钩子的反作弊兼容性验证与回调瘦身
priority: P0
area: design
status: awaiting-live-validation
confidence: verified-static
needs_human: true
problem: >-
  已将进程/按键读取、状态锁、通知及定时器移到独立工作任务，回调仅原子更新与单槽通知；
  保留无全局热键的主窗/托盘入口并补充设置说明。真实腾讯反作弊兼容与体验仍待K1–K6；
  实现和验证边界见HOLD_INPUT.md，不能因CI通过标作实机完成。
evidence:
  - internal/hold/monitor_windows.go
  - internal/hold/worker.go
  - docs/HOLD_INPUT.md
actions:
  - 回调里只更新状态或投递事件，进程和前台窗口判断移到回调之外
  - 增加不依赖全局热键的备用入口（托盘菜单或窗口按钮）
  - 在设置页写明"只监听已配置的按键，不记录其他输入"
  - 在真实对局中实测并记录结果（需要人）
acceptance:
  - 钩子回调内没有进程查询或其他阻塞调用
  - 有真实 Windows 加国服客户端的实测记录（版本、场景、结果）
depends_on: []
verify:
  - bash scripts/verify.sh windows
do_not:
  - 违反 C6、C7、C9
effort: M
```

```yaml
id: S2
title: 拆分 engine 上帝对象
priority: P1
area: design
status: todo
confidence: verified-static
needs_human: false
problem: >-
  Engine 结构体约 57 个字段共用一把 RWMutex，同时承担阶段轮询、选人监听、预取、
  手动查询、缓存、复盘、分组、历史；engine 包 3738 行。
evidence:
  - internal/engine/engine.go（Engine 结构体、poll）
  - internal/engine/scouting.go:225 prepareScouting 191 行
  - internal/engine/grouping.go:15 syncGroupingLocked 167 行
  - internal/engine/history_store.go
actions:
  - 先完成 S4（类型化状态），再按职责抽出组件：阶段监听、选人监听、预取器、查询服务、复盘、分组、历史
  - 每个组件持有自己的状态和锁；Engine 只做编排和事件分发
  - 保留现有的 generation / clientVersion / phaseEpoch 迟到结果丢弃机制，不要削弱
  - 每次只迁移一个组件，迁移后跑竞态测试
acceptance:
  - Engine 字段显著减少（建议目标不超过 20）
  - 新功能不再向 Engine 增加字段
  - go test -race ./internal/engine/... 通过
depends_on: [S4]
verify:
  - go test -race ./internal/engine/...
  - bash scripts/verify.sh portable
do_not:
  - 一次性大重写；改变对前端暴露的快照字段含义
effort: L
```

```yaml
id: S3
title: 抽出共享的"单次动作执行器"
priority: P1
area: design
status: implemented
confidence: verified-static
needs_human: false
problem: readycheck与postgame现已复用会话换代、取消、跳过、超时和动作许可，业务状态机仍保持独立。
evidence:
  - internal/actionlane/executor.go
  - internal/actionlane/executor_test.go
  - internal/readycheck/controller.go
  - internal/postgame/controller.go
actions:
  - 已完成：共享Executor统一换代、取消、跳过/重置、总超时及动作窗口/次数许可
  - 已完成：控制器状态读取与阶段步骤收敛锁边界，写入前仍由业务层复核客户端状态
  - 已保持：默认关闭、自动接受最多3次/15秒、跳过不发拒绝、赛后未知结果不重发
acceptance:
  - runSession为21行，runTask为34行
  - 现有readycheck、postgame测试及共享组件单元测试通过竞态检测
depends_on: []
verify:
  - go test -race ./internal/readycheck/... ./internal/postgame/...
do_not:
  - 改变自动接受的默认值或安全约束（C7）
effort: M
```

```yaml
id: S4
title: Stage、Phase 改为带类型的枚举
priority: P1
area: design
status: implemented
confidence: verified-static
needs_human: false
problem: 引擎内部Stage与Phase已改为强类型常量并集中映射；对外快照继续使用兼容字符串。
evidence:
  - internal/engine/stage.go
  - internal/engine/stage_test.go
  - internal/engine/engine.go
actions:
  - 已完成：定义Stage/Phase常量、Phase到Stage映射及Stage行为表
  - 已完成：引擎轮询、事件、选人、恢复、复盘和统计路径不再比较阶段裸字符串
  - 已保持：core.Snapshot与前端绑定仍使用原字符串取值
acceptance:
  - 引擎产品代码除集中常量定义外不再出现阶段裸字符串
  - frontend/bindings无需改动，远程Windows绑定一致性已通过
depends_on: []
verify:
  - bash scripts/verify.sh windows
do_not:
  - 修改对外 JSON 取值
effort: M
```

```yaml
id: S5
title: OP.GG 攻略：复用会话、功能开关、失败降级
priority: P1
area: design
status: implemented
confidence: verified-static
needs_human: false
problem: >-
  标准模式原先每次获取先后做initialize、notifications/initialized、tools/call；
  每内容任务已有8秒总预算，但重复握手增加延迟和限流风险。第三方接口仍可能变更或限流。
evidence:
  - internal/guides/opgg.go:16 接口地址常量
actions:
  - 已完成：进程内复用MCP协议/会话15分钟，400/404/410失效后只重建一次
  - 已保持：每个基础/对位内容任务8秒总预算、HTTP 5秒及30秒失败冷却
  - 已完成：默认开启的独立功能开关；关闭取消并隐藏，失败保留有效缓存或显示原文链接
  - 已保持：固定HTTPS地址、不跟随重定向、响应上限2MiB、不发送玩家身份
acceptance:
  - 已完成：接口不可用时8秒内结束，有缓存先保留，无缓存显示固定提示和原文链接
  - 已完成：单元测试覆盖会话复用、服务端过期、超时、开关与旧设置迁移
depends_on: []
verify:
  - go test ./internal/guides/...
do_not:
  - 向第三方发送玩家标识
effort: M
```

```yaml
id: S6
title: history_store 的容量、容错与脱敏强化
priority: P1
area: design
status: todo
confidence: verified-static
needs_human: false
problem: >-
  每次变更把整个缓存序列化后整文件重写（500 ms 防抖），序列化时持锁；
  容量按玩家数（64）和每人页数限制，不是按字节；写入和 Rename 失败被静默忽略；
  PUUID 用未加盐 SHA-256 做键，属于假名化而非匿名。
evidence:
  - internal/engine/history_store.go:215 json.Marshal
  - internal/engine/history_store.go:232 os.Rename
  - internal/engine/history_store.go:91,99 sha256
actions:
  - 增加按字节的总容量上限和淘汰
  - 序列化放到锁外（先拷贝快照）
  - 写入失败记入诊断计数；Windows 上 Rename 失败时重试几次
  - 键改用带本机随机密钥的 HMAC
  - 调用方在缓存文件损坏时禁用缓存并重建，不影响启动（先核实现状）
acceptance:
  - 缓存文件有明确大小上限，并有测试
  - 写入失败在诊断导出里可见
depends_on: []
verify:
  - go test ./internal/engine/...
do_not:
  - 写入昵称或其他可识别信息（C8）
effort: M
```

```yaml
id: S7
title: SGP 缓存与并发策略
priority: P1
area: design
status: todo
confidence: verified-static
needs_human: false
problem: SGP 页面缓存满 512 条时整体清空；sgpMu 在网络请求期间持锁，串行化所有 SGP 调用。
evidence:
  - internal/source/sgp.go:138 容量 512
  - internal/source/sgp.go:106 sgpMu.Lock
  - "同样加锁的位置：internal/source/ranked.go:113、tencent.go:40、match_analysis.go:15"
actions:
  - 改为带 TTL 的 LRU
  - 锁只保护状态，网络请求在锁外；按通道限流
acceptance:
  - 缓存不再整体清空
  - 慢请求不阻塞其他通道（有并发测试）
depends_on: []
verify:
  - go test -race ./internal/source/...
do_not:
  - 放宽 SGP 的地址校验与禁止重定向
effort: M
```

```yaml
id: S8
title: 拆分 main()，显式传入依赖
priority: P2
area: design
status: todo
confidence: verified-static
needs_human: false
problem: main() 约 344 行，闭包互相捕获 mainWindow、monitor 等变量，初始化顺序靠约定。
evidence:
  - main.go:28
actions:
  - 抽出 wire() 或 app.go，把控制器依赖显式传入，main() 只负责启动
acceptance:
  - main() 不超过 60 行；行为不变
depends_on: []
verify:
  - bash scripts/verify.sh windows
do_not:
  - 改变窗口、托盘和单实例行为
effort: M
```

```yaml
id: S9
title: LCU 证书校验
priority: P2
area: design
status: todo
confidence: verified-static
needs_human: false
problem: LCU 使用 InsecureSkipVerify（自签名证书的常见做法，且固定 127.0.0.1，风险较低）。
evidence:
  - internal/lcu/client.go:132
actions:
  - 改为自定义 VerifyPeerCertificate，校验 Riot 根证书；证书获取方式先调研并核实
acceptance:
  - 不再使用 InsecureSkipVerify，且客户端连接仍然可用（需要真机确认）
depends_on: []
verify:
  - go test ./internal/lcu/...
do_not:
  - 在未验证真机连接前合并
effort: M
```

```yaml
id: S10
title: 依赖治理
priority: P2
area: design
status: todo
confidence: verified-static
needs_human: false
problem: Wails v3 仍是 beta；vue 版本范围偏宽（^3.2.45）；没有自动化的依赖更新。
evidence:
  - go.mod（wails/v3 v3.0.0-beta.29）
  - frontend/package.json
actions:
  - 启用 Dependabot 或 Renovate，Go、npm、GitHub Actions 分别配置
  - 前后端 Wails 版本要一起升级（C3）
  - 为 Actions 使用提交哈希固定
acceptance:
  - 依赖更新以 PR 形式出现，并由 CI 把关
depends_on: [Q1]
verify:
  - bash scripts/verify.sh windows
do_not:
  - 单独升级前端或后端的 Wails 版本
effort: S
```

## 代码质量

```yaml
id: Q1
title: 启用 GitHub Actions CI
priority: P0
area: quality
status: implemented
confidence: verified-static
needs_human: false
problem: 仓库只有 docs/ci/verify.yml.example 草案，没有启用远程 CI。
evidence:
  - docs/ci/verify.yml.example
  - AGENTS.md:93 与 docs/ENVIRONMENTS.md:51 写着"未启用远程 CI"
actions:
  - 将已起草的 ci.yml 放到 .github/workflows/ci.yml（Windows 任务只调用一次 bash scripts/build.sh bin/leaguepingo-ci.exe，因为它已内含 verify.sh）
  - 首次运行后根据日志修正（尚未实际运行过）
  - 同步修改 AGENTS.md:93 与 docs/ENVIRONMENTS.md:51 的说法
acceptance:
  - push 到 dev 与指向 dev、main 的 PR 自动运行 portable 与 windows 两个任务并通过
depends_on: []
verify:
  - node scripts/workflow.mjs check
do_not:
  - 违反 C1、C2、C10
effort: S
```

```yaml
id: Q2
title: 拆分超长函数
priority: P1
area: quality
status: todo
confidence: verified-static
needs_human: false
problem: 19 个函数超过 100 行。
evidence:
  - main.go:28 main 344 行
  - internal/engine/scouting.go:225 prepareScouting 191 行
  - internal/readycheck/controller.go:183 runSession 186 行
  - internal/postgame/controller.go:280 runTask 179 行
  - internal/engine/engine.go:204 poll 174 行
  - internal/engine/grouping.go:15 syncGroupingLocked 167 行
  - overlay.go:248 reconcileCompanion 135 行
actions:
  - 按职责拆分；runSession、runTask 与 S3 合并处理，main 与 S8 合并处理
acceptance:
  - 逐步降到没有超过 100 行的函数，且每次拆分都有测试保护
depends_on: [Q1, S3, S8]
verify:
  - go test -race ./...
do_not:
  - 在没有测试覆盖的路径上做大范围重构
effort: L
```

```yaml
id: Q3
title: 前端单元测试
priority: P1
area: quality
status: todo
confidence: verified-static
needs_human: false
problem: 前端只有 Playwright 浏览器脚本（frontend/tests/*.cjs），package.json 里没有 vitest 或 jest。
evidence:
  - frontend/package.json
  - frontend/src/historyBrowser.ts、presentation.ts（纯逻辑，适合单测）
actions:
  - 引入 vitest，先覆盖 historyBrowser.ts 与 presentation.ts
  - 把 vitest 加入 CI
acceptance:
  - 上述模块有快速的单元测试并在 CI 中运行
depends_on: [Q1]
verify:
  - npm --prefix frontend run build
do_not:
  - 改变现有 Playwright 脚本的入口（workflow 策略检查依赖 package.json 的部分脚本）
effort: M
```

```yaml
id: Q4
title: 静态分析（staticcheck、govulncheck、npm audit）
priority: P1
area: quality
status: implemented
confidence: verified-static
needs_human: false
problem: 已在CI增加建议性staticcheck、govulncheck与npm audit；当前Go标准库漏洞未清零，因此尚未改为阻断。
evidence:
  - .github/workflows/ci.yml 的 advisory 任务（建议性，失败不拦截）
  - docs/STATIC_ANALYSIS.md
actions:
  - 已完成：建议性运行，三项分别显示结果且失败不拦截Windows构建
  - 已完成：固定与Go 1.25兼容的staticcheck v0.7.0和govulncheck v1.7.0，禁止自动切换工具链
  - 待依赖治理：Go升级到修复版并清零govulncheck后改成必须通过
acceptance:
  - CI 中有结果；存量告警清零后改为强制
depends_on: [Q1]
verify:
  - 查看 CI advisory 任务的输出
do_not:
  - 为了让检查通过而批量加忽略注释
effort: M
```

```yaml
id: Q5
title: 零散代码问题
priority: P2
area: quality
status: todo
confidence: inferred
needs_human: false
problem: 若干小问题。
evidence:
  - "internal/settings/store.go:141 regexp.MustCompile 每次调用都编译（NormalizeShortcut）"
  - 被忽略的错误（先用 grep 逐个核对，不要一律改）
  - 诊断导出文件是否有清理逻辑（未核实）
actions:
  - 正则提到包级变量；逐个核对被忽略的错误；确认诊断导出的轮转或清理
acceptance:
  - 无遗留，且不改变行为
depends_on: []
verify:
  - go test ./internal/settings/...
do_not:
  - 顺手重构无关代码
effort: S
```

```yaml
id: Q6
title: 全局 CSS 治理
priority: P2
area: quality
status: todo
confidence: inferred
needs_human: false
problem: frontend/src 下的 style.css 与 foundation.css 体量很大（旧版统计约 2000 与 1700 行），为全局样式。
evidence:
  - frontend/src/style.css
  - frontend/src/foundation.css
actions:
  - 新组件使用 scoped 样式或设计变量；存量逐步迁移
acceptance:
  - 新组件不再向全局文件追加样式；亮暗主题截图无回归
depends_on: [Q1]
verify:
  - npm --prefix frontend run test:smoke
do_not:
  - 一次性迁移所有样式
effort: L
```

```yaml
id: Q7
title: 文档瘦身与结构整理
priority: P2
area: quality
status: todo
confidence: verified-static
needs_human: false
problem: >-
  Markdown 约 468 KB，比 Go 源码还大；docs/ARCHITECTURE.md 中有 13 个带 BUG 编号的标题，
  真正的架构说明被压在后面；README 与 AGENTS.md 嵌有大量 BUG 规则。
evidence:
  - docs/ARCHITECTURE.md
  - BUGFIX_LOG.md
actions:
  - 把变更记录拆到 CHANGELOG，ARCHITECTURE.md 只描述当前状态
  - README 控制在一页内；去掉互相重复的内容
  - 改动前先确认 workflow.mjs 的 requireText 没有依赖被移动的文本（C10）
acceptance:
  - ARCHITECTURE.md 开头就是架构说明
  - node scripts/workflow.mjs check 通过
depends_on: []
verify:
  - node scripts/workflow.mjs check
do_not:
  - 删除 AGENTS.md 中被策略检查引用的固定短语
effort: M
```

```yaml
id: Q8
title: 许可证与素材授权
priority: P2
area: quality
status: todo
confidence: verified-static
needs_human: true
problem: 根目录没有 LICENSE；仓库含约 450 个英雄、符文图片（Riot 素材），仅有 THIRD_PARTY_NOTICES.md（未逐条审阅）。
evidence:
  - THIRD_PARTY_NOTICES.md
actions:
  - 选定许可证并补 LICENSE
  - 公开仓库前核对素材授权，必要时改为运行时按需获取
acceptance:
  - 许可证与授权清单齐全
depends_on: []
verify:
  - 人工审阅
do_not:
  - 在未确认授权前公开仓库
effort: S
```

## 用户体验

```yaml
id: U1
title: 错误码化，界面不再出现原始技术错误
priority: P0
area: ux
status: implemented
confidence: verified-static
needs_human: false
problem: >-
  底层HTTP与Go错误曾直接显示；现已使用固定错误码/共享中文目录，AppService出口
  同时保护Wails顶层message与cause，前端只按已知码显示固定文案，异步警告同样归一。
  本地诊断仅保留固定层/码/状态/计数，不保留原文；验证口径见ERRORS.md及本提交CI。
evidence:
  - internal/lcu/client.go:177  # fmt.Errorf("LCU HTTP %d")
  - internal/source/sgp.go:196  # fmt.Errorf("数据服务 HTTP %d")
  - internal/engine/engine.go:219  # message = err.Error()
  - internal/engine/engine.go:239  # "客户端本地接口连接失败：" + err.Error()
actions:
  - 底层返回带类型的错误码，例如 ClientNotRunning、NotLoggedIn、ServiceBusy、RateLimited
  - 界面统一映射成"发生了什么＋你可以怎么做"的中文提示
  - 原始细节只写入诊断导出，且仍需脱敏，不得包含玩家身份
acceptance:
  - 界面上不再出现 HTTP 状态码或 Go 错误原文
  - 错误码到文案的映射有单元测试
depends_on: []
verify:
  - bash scripts/verify.sh windows
do_not:
  - 在界面或诊断中输出令牌、完整 URL 或玩家标识
effort: M
```

```yaml
id: U2
title: 数据服务不可用时的降级与恢复
priority: P0
area: ux
status: implemented
confidence: inferred
needs_human: false
problem: >-
  已实现共享退避/Retry-After门、保留LCU与旧资料、独立服务状态及获取时间；
  可见空闲首页/查询首批与无回退的显式分页有界恢复，手动优先，局内及只读卡不新增读取。
  范围、次数、存储与验证见DATA_RECOVERY.md及本提交CI；真实国服故障场景待人工。
evidence:
  - internal/source/sgp.go（401 时会清令牌、拒绝重定向）
actions:
  - 保留本机客户端数据，只把战绩部分标为"暂不可用，稍后自动重试"
  - 显示"更新于 N 秒前"
  - 401、429、5xx 做带退避的自动重试，并提供手动重试按钮
acceptance:
  - 断开数据服务后界面仍可用，提示清楚
  - 重试有上限和退避，并有测试
depends_on: [U1]
verify:
  - bash scripts/verify.sh windows
do_not:
  - 无限重试或绕过限流
effort: M
```

```yaml
id: U3
title: 安装、依赖检测、签名与版本检查
priority: P0
area: ux
status: awaiting-live-validation
confidence: verified-static
needs_human: true
problem: >-
  WebView2启动引导、固定静态版本检查、受管NSIS和证书存储签名入口已实施；
  实际发布证书、干净Win10安装/缺运行库/发布者信任及覆盖安装仍待I1-I6人工验收。
evidence:
  - docs/INSTALLATION.md
  - scripts/package-windows.sh
  - scripts/sign-windows.sh
actions:
  - 已完成：启动前WebView2检测与下载引导
  - 已完成：受限静态JSON版本检查与新版托盘入口
  - 已完成：NSIS受管构建及CI隔离安装/卸载验证入口
  - 待人工：提供发布证书并完成签名、干净Win10与覆盖安装验收
acceptance:
  - 干净的 Win10 机器上可安装并运行
depends_on: [Q1]
verify:
  - node scripts/workflow.mjs check
  - bash scripts/verify.sh windows
do_not:
  - 绕过策略直接启用 unsupported 任务
effort: L
```

```yaml
id: U4
title: 自动化功能的预期管理
priority: P1
area: ux
status: implemented
confidence: verified-static
needs_human: false
problem: 自动接受、赛后自动返回并排队等功能保持默认关闭；游戏辅助页现已明确标注实验性边界并提供一键停用。
evidence:
  - docs/AUTO_ACCEPT.md
  - docs/POST_GAME.md
  - frontend/src/components/AutomationView.vue
  - frontend/tests/auto-accept.cjs
actions:
  - 已完成：游戏辅助页标注"实验性"，说明不发送拒绝、不模拟鼠标键盘及不替用户作选择
  - 已完成：提供一键停用全部自动化；保持默认关闭
  - 已完成：修复同次保存两组设置时后一组草稿被首个响应覆盖的问题
acceptance:
  - 游戏辅助页有明确说明；frontend/tests/auto-accept.cjs 覆盖双组保存及一键停用
depends_on: []
verify:
  - npm --prefix frontend run test:smoke
do_not:
  - 把默认值改为开启
effort: S
```

```yaml
id: U5
title: 首次使用引导与空状态
priority: P2
area: ux
status: needs-investigation
confidence: unverified
needs_human: false
problem: 对前端源码做过关键词搜索，没有找到首次使用提示，但搜索不严谨。
evidence:
  - frontend/src
actions:
  - 先确认是否已有引导；没有则在"未连接客户端"的空状态写清三步：启动国服客户端、登录、保持程序运行
acceptance:
  - 新用户不看文档能完成首次连接
depends_on: []
verify:
  - npm --prefix frontend run test:smoke
do_not:
  - 弹出阻塞式教程
effort: S
```

```yaml
id: U6
title: 热键的透明说明与冲突处理
priority: P2
area: ux
status: todo
confidence: inferred
needs_human: false
problem: 设置页需要说明键盘钩子的监听范围，并处理冲突。
evidence:
  - internal/settings/store.go（NormalizeShortcut）
  - internal/hold/
actions:
  - 写明"只监听已配置的按键，不记录其他输入"；冲突时给出提示
acceptance:
  - 有说明和冲突提示，并有测试
depends_on: [S1]
verify:
  - bash scripts/verify.sh windows
do_not:
  - 扩大监听范围
effort: S
```

```yaml
id: U7
title: 无障碍与键盘可用性
priority: P2
area: ux
status: todo
confidence: verified-static
needs_human: false
problem: aria/role 约 172 处，基础不错，但 :focus-visible 只出现在 5 个文件，键盘事件处理只有 7 处。
evidence:
  - frontend/src（统计来自旧版 ZIP，请重新 grep）
actions:
  - 纯键盘走一遍主流程；补焦点样式；检查亮暗主题对比度
acceptance:
  - 主流程可仅用键盘完成；有对应 Playwright 用例
depends_on: [Q1]
verify:
  - npm --prefix frontend run test:smoke
do_not:
  - 去掉现有的 prefers-reduced-motion 处理
effort: M
```

```yaml
id: U8
title: 文案集中管理
priority: P2
area: ux
status: todo
confidence: verified-static
needs_human: false
problem: 没有 i18n 库，中文文案写死在组件里。
evidence:
  - frontend/package.json（无 i18n 依赖）
actions:
  - 先把文案抽到统一文件，不引入新依赖
acceptance:
  - 组件内不再硬编码中文文案
depends_on: []
verify:
  - npm --prefix frontend run build
do_not:
  - 在此任务中改动文案内容
effort: M
```

## 产品流畅度

```yaml
id: F1
title: 加入度量（先测量再优化）
priority: P0
area: performance
status: implemented
confidence: inferred
needs_human: false
problem: 原有请求计数与离线探针不足以提供本次运行的首批结果耗时、延迟分布及局内资源汇总。
evidence:
  - cmd/perfprobe（已有性能探针，先评估能否复用）
  - 设置页已显示 Go 堆信息（旧版审阅所见）
actions:
  - 记录选人开始到首次出结果的耗时、SGP 请求 p95 延迟、钩子回调耗时、游戏中 CPU 与内存
  - 写入诊断导出，只含计数与耗时，不含玩家身份
acceptance:
  - 诊断导出里能看到上述指标
depends_on: []
verify:
  - go test ./internal/...
do_not:
  - 上传任何遥测（项目当前没有遥测）
effort: M
```

```yaml
id: F2
title: 逐个玩家渐进渲染
priority: P1
area: performance
status: implemented
confidence: verified-static
needs_human: false
problem: 已确认赛前战绩与独立统计按玩家发布；前端现为尚无统计的玩家展示骨架，单个结果到达即只替换对应卡片。
evidence:
  - internal/engine/scouting.go
  - internal/engine/initial_match_test.go
  - frontend/src/components/PlayerCardData.vue
  - frontend/tests/companion-browser.cjs
actions:
  - 已确认：赛前战绩逐人完成后立即publish，独立统计双worker并在首批数据到达时publish
  - 已完成：无可用统计的玩家显示稳定骨架；已有部分统计继续直接展示
  - 已完成：两位待加载玩家先完成一位时，只替换对应卡片
acceptance:
  - 单个玩家数据先返回时立即显示，其余显示骨架屏
depends_on: [F1]
verify:
  - npm --prefix frontend run test:smoke
do_not:
  - 增加额外的轮询或请求
effort: M
```

```yaml
id: F3
title: 缓存优先展示（stale-while-revalidate）
priority: P1
area: performance
status: needs-investigation
confidence: inferred
needs_human: false
problem: 新版已有磁盘缓存（history_store），但展示策略是否"先显示缓存再后台刷新"未核实。
evidence:
  - internal/engine/history_store.go
actions:
  - 先显示缓存并标注时间，再后台刷新；刷新失败保留缓存
acceptance:
  - 再次打开同一玩家时几乎立即出现数据
depends_on: [S6, F1]
verify:
  - go test ./internal/engine/...
do_not:
  - 缓存内容超出 C8 的限制
effort: M
```

```yaml
id: F4
title: 统一的重试退避与并发限制
priority: P1
area: performance
status: todo
confidence: inferred
needs_human: false
problem: 401、429、5xx 的处理与并发限制缺少统一策略。
evidence:
  - internal/source/
  - internal/engine/scheduler.go（已有单槽串行与手动优先）
actions:
  - 指数退避加抖动；限制并发；遇到限流时降级而不是重试风暴
acceptance:
  - 模拟限流的测试中不出现请求雪崩
depends_on: [S7, U2]
verify:
  - go test -race ./internal/source/... ./internal/engine/...
do_not:
  - 绕过服务端限流
effort: M
```

```yaml
id: F5
title: 局内资源开销实测
priority: P2
area: performance
status: todo
confidence: unverified
needs_human: true
problem: 程序对游戏帧率和内存的影响未测。
evidence:
  - "scripts/verify.sh 末尾声明：NOT RUN: real client/game acceptance and FPS/network measurements"
actions:
  - 在真实对局中测量帧率、CPU、内存；必要时降低轮询频率
acceptance:
  - 有实测数据，并给出可接受的阈值
depends_on: [F1]
verify:
  - 人工记录
do_not:
  - 虚构数据（C9）
effort: M
```

## 验收与合规

```yaml
id: R1
title: 真机验收
priority: P0
area: release
status: awaiting-live-validation
confidence: verified-static
needs_human: true
problem: >-
  Linux和Windows CI已通过自动化检查与构建；2026-10-30用户报告A1/A2/P1/P2通过，
  后续确认跳过可用但继续/返回/开排仍需手动；BUG-0083修复全链待复验，程序/客户端版本及其他项目未补齐。实机反馈见acceptance/R1-feedback.md。
  实验性标注、分轮清单与空白CSV已准备；完整赛后流程与R1整体尚未验收完成。
evidence:
  - docs/ACCEPTANCE.md
  - BUGFIX_LOG.md（BUG-0073、BUG-0074 自述国服准确性验收未完成）
actions:
  - 按 docs/ACCEPTANCE.md 逐项在 Windows 加国服客户端上验收：自动接受、赛后排队、组队分组、海克斯攻略、历史缓存
  - 未验收的功能在界面和文档中标注为实验性
acceptance:
  - 每个功能都有真机记录（日期、客户端版本、结果）
depends_on: []
verify:
  - 人工
do_not:
  - 虚构结果（C9）
effort: L
```

```yaml
id: R2
title: 平台规则确认
priority: P1
area: release
status: todo
confidence: verified-static
needs_human: true
problem: 自动接受、自动排队、敌方组队推断可能触及平台规则；文档自述"腾讯版本兼容和产品审核结论仍需单独核验"。
evidence:
  - docs/AUTO_ACCEPT.md
  - docs/PREMADE_GROUPS.md
actions:
  - 发布前对照腾讯与 Riot 的相关规则，形成书面结论
  - 在结论出来前保持相关功能默认关闭
acceptance:
  - 有书面结论并写入文档
depends_on: []
verify:
  - 人工
do_not:
  - 在未确认前默认开启这些功能
effort: S
```

```yaml
id: R3
title: 发版前的固定回归清单
priority: P2
area: release
status: todo
confidence: inferred
needs_human: true
problem: 缺少固定的手动回归清单。
evidence:
  - docs/ACCEPTANCE.md
actions:
  - 固定一份清单：首次启动、断线重连、客户端重启、热键冲突、WebView2 缺失、托盘退出
acceptance:
  - 每次发版逐项勾选并留存
depends_on: [R1]
verify:
  - 人工
do_not:
  - 用自动化测试结果代替真机清单
effort: S
```
