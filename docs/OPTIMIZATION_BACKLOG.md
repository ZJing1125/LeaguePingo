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

执行记录（实施起始基线4d57e0b，附件审阅基线cc229e7）：Q1已实施并通过dev推送实跑，见下方记录；Q4建议性staticcheck、govulncheck与npm audit已实施，强制门禁等待Go标准库漏洞随依赖治理清零；F1本地度量已实施，口径与验证见PERFORMANCE.md；F2已核实后端逐人发布链路并补齐前端骨架和逐卡替换回归；S3共享动作执行器已实施并接入自动接受/赛后流程，两个编排循环均小于60行；S4引擎Stage/Phase强类型与集中行为表已实施，对外快照字符串未变且远程Windows验证通过；S5攻略会话复用、功能开关、超时/缓存/原文降级已实施且远程Windows验证通过；S6战绩缓存字节上限、锁外编码、HMAC键、同轮重建、Rename重试与失败诊断已实施且远程Windows验证通过；S7页缓存TTL/LRU、锁外授权、独立并发上限及换号/首页变化迟到隔离已实施且远程Windows验证通过；Q3前端19项单测与类型检查已接入统一verify门禁且远程Windows验证通过；F3已核实首页/磁盘策略并补齐ID/选中玩家过期缓存先显及后台更新；F4统一退避抖动、并行成功隔离及长Retry-After保护已实施；S8入口与桌面依赖组装拆分已实施，本机目标staticcheck/portable通过，根包及桌面由Windows CI核验；Q5正则复用、缓存失败诊断与忽略错误/报告留存审阅已实施；S10依赖组更新配置与Actions SHA固定已准备，实际激活待正常进入默认分支；Q2生产超长函数全部拆分并接入AST门禁，Windows全包竞态及完整构建已通过，已结项；S2已完成十三批职责迁移，Engine由59降为26字段（18组件/依赖与8核心守卫，建议20未达），完整引擎竞态通过，超长函数仍归Q2；U6已核实现有监听说明/冲突回滚和Windows自动回归，按已有证据结项；U5首页三步连接指引与首次离线/断线/重连/主题窄窗回归已实施；Q7当前架构/精简README/CHANGELOG与TESTING分工已完成并核对策略；U7两主题键盘主流程、共享焦点及浅色对比度修正已实施；S9已完成官方根候选研究与测试专用严格探针，国服链/名称核验后置，生产传输未改；U1错误码与中文提示、U2降级与有界恢复已实施，协议与验证分别见ERRORS.md、DATA_RECOVERY.md及本提交CI，R1实验性标注与分轮记录已准备；用户已反馈A1/A2/P1/P2没有问题，后续确认跳过可用但继续/返回/开排链条失败，修复版与版本信息及其他项目待补，整体R1未完成；S1回调瘦身/备用入口核验/设置说明已实施，国服反作弊专项待实机；U3启动前依赖检查、静态版本检查、NSIS与证书存储签名代码链已实施，发布证书与干净Win10的I1–I6仍待人工；U4实验性说明、行为边界和一键停用已实施并由前端合成回归覆盖；其余任务尚未实施。该清单是任务输入，原审阅统计与推断不能当作当前事实。其他任务仍按依赖逐项核实；未执行的Windows/国服检查保留未验证状态。

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
status: implemented
confidence: verified-static
needs_human: false
problem: >-
  Engine原59字段共用RWMutex；经十三批职责迁移后降为26字段，异步任务/缓存/预算/发布均归组件。
  保留18个显式组件/依赖及8个同锁核心字段；建议不超过20的数量目标未达到，超长函数仍归Q2。
evidence:
  - internal/engine/engine.go（Engine 结构体、poll）
  - internal/engine/event_subscription.go、grouping_state.go、review_state.go
  - internal/engine/player_cache.go、preparation_state.go、preparation.go
  - internal/engine/initial_match_state.go、initial_match.go、query_session.go、player_queries.go
  - internal/engine/scouting_state.go、scouting.go、roster_discovery.go、client_discovery.go、snapshot_publisher.go
  - internal/engine/history_store.go、history_recovery_state.go
actions:
  - 先完成 S4（类型化状态），再按职责抽出组件：阶段监听、选人监听、预取器、查询服务、复盘、分组、历史
  - 第一步已实施：phase/champ事件订阅抽出独立锁与生命周期，Engine字段59→51；引擎竞态及相关统计三轮复测通过
  - 第二步已实施：groupingState独立拥有匿名字段/标签与取消生命周期，51→45字段；原预算与迟到守卫保留，完整引擎竞态通过
  - 第三步已实施：reviewState独立拥有候选/冻结复盘/账号/取消令牌，45→41字段；原结算预算与跨局守卫保留，完整引擎竞态通过
  - 第四步已实施：playerCache独立持有玩家/别名/资料冷却/详情/时间线五表，41→37字段；原容量/时效/连接守卫及持久化边界保留，完整引擎竞态通过
  - 第五步已实施：preparationState独立持有房间/选人准备取消/令牌/窗口/次数/优先级，37→30字段；原预算、手动抢占退还次数与逐记录资料资格保留，完整引擎竞态通过
  - 第六步已实施：initialMatchPreparation独立管理开局单轮资格/取消/版本/比赛/令牌；同局重连不重试、空目标消费资格、旧任务不取消新局及父context寿命均有回归，字段仍30
  - 第七步已实施：querySession拥有12秒手动独占槽与首页子任务取消，30→29字段；保留并行拒绝/取消后worker释放/阶段与账号迟到隔离，不扩大取消至其他手动查询
  - 第八步已实施：scoutPreparation独立拥有统计范围/取消/预算/已尝试身份/报告/游标/推送节流，仍29字段；修复旧游标清空后迟到复活，原180秒/500页/两worker/十身份及逐人推送保留
  - 第九批已实施：playerQueries负责手动历史/可选资料编排，显式依赖queryState与scheduler；Engine保留原子守卫/写回，字段仍29，原预算/失败保留/独立时钟回归保留
  - 第十批已实施：historyRecovery独立拥有有界重试/后台合并任务/本机更新队列；修复旧任务清理误释放新任务，保留原额度/时限/抢占退款及换号隔离，字段仍29
  - 第十一批已实施：rosterDiscovery独立拥有本地名单三次就绪窗口与完成状态，29→28字段；加载→游戏仅未完整时续窗，同局手动刷新不续窗，原读取/新局/迟到回归保留
  - 第十二批已实施：clientDiscovery独立拥有十秒发现节流，28→27字段；发现回调在Engine及组件锁外执行，原失败/恢复/账号隔离和精确冷却并发回归通过
  - 第十三批已实施：snapshotPublisher独立拥有32条快照队列与单回调worker，27→26字段；原丢最旧/保持最新、顺序、深复制及回调可重入回归通过
  - 已完成：异步子任务/缓存/预算/发布的状态和锁归组件；Engine保留API编排、原子阶段/身份守卫与提交
  - 保留现有的 generation / clientVersion / phaseEpoch 迟到结果丢弃机制，不要削弱
  - 每次只迁移一个组件，迁移后跑竞态测试
acceptance:
  - 已完成显著减少：59→26；建议目标不超过20未达到，26个字段为18个显式组件/依赖及8个需原子核验的核心字段
  - 十三批职责迁移完成，名单/发现/发布等生命周期无剩余迁移；超长函数由独立Q2任务继续处理
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
status: implemented
confidence: verified-static
needs_human: false
problem: >-
  原schema 2整文件写入只限制身份/页数，序列化持锁、无字节上限；写入失败静默，
  PUUID使用无密钥SHA-256。现已升级schema 3并丢弃旧索引。
evidence:
  - internal/engine/history_store.go、history_recovery_state.go:215 json.Marshal
  - internal/engine/history_store.go、history_recovery_state.go:232 os.Rename
  - internal/engine/history_store.go、history_recovery_state.go:91,99 sha256
actions:
  - 已完成：8MiB总上限，按更新时间保留最新身份，继续限制64身份/每人50页
  - 已完成：锁内深拷贝和版本戳，锁外编码，版本未变化才回写淘汰结果
  - 已完成：固定脱敏诊断计数；Rename最多3次
  - 已完成：独立0600本机随机32字节密钥的HMAC-SHA256索引
  - 已完成：schema 1/2、密钥丢失、坏格式或超限文件同轮清空重建，程序继续启动
acceptance:
  - 已完成：8MiB上限和按新旧淘汰有单元测试
  - 已完成：写入失败在AppInfo/诊断导出的`history-cache-write`聚合可见
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
status: implemented
confidence: verified-static
needs_human: false
problem: >-
  原SGP页缓存满512页或首页游标满64人时整体清空。远端HTTP此前已移到锁外，
  本机授权刷新仍持共享锁。现已改为TTL/LRU与锁外授权/HTTP、分通道并发限制。
evidence:
  - internal/source/sgp.go:138 容量 512
  - internal/source/sgp.go:106 sgpMu.Lock
  - "同样加锁的位置：internal/source/ranked.go:113、tencent.go:40、match_analysis.go:15"
actions:
  - 已完成：512页/15秒TTL的LRU；64个首页游标逐人淘汰，不清空其他有效页面
  - 已完成：本机授权/远端HTTP在锁外，同类授权刷新合并，entitlements与league-session各自等待
  - 已完成：四个功能通道各最多4个远端请求，等待可取消且计入原预算，取槽后重查冷却
  - 已完成：账号epoch/冷却版本拒绝迟到状态，首页变化后拒绝旧偏移页回填
acceptance:
  - 已完成：TTL/LRU/首页游标容量与局部失效回归通过
  - 已完成：冷授权跨通道隔离、合并刷新、并发限制/取消、换号与首页变化迟到拒绝有竞态回归
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
status: implemented
confidence: verified-static
needs_human: false
problem: 原main同时组装桌面全部依赖；现由desktopApplication显式持有并按职责分开。
evidence:
  - main.go:28
actions:
  - 已完成：desktop_app/desktop_controls/desktop_workers按职责持有依赖、注册回调与管理生命周期
  - 已完成：保留单实例早于服务、窗口/托盘/侧栏政策及快捷键配置回滚，增加配置隔离与关闭回归
acceptance:
  - 已完成：main不超过60行，目标staticcheck/portable通过；原生根包与构建由Windows CI核验
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
status: prepared-awaiting-live-validation
confidence: verified-static
needs_human: true
problem: 官方Game Client根候选及严格只读探针已准备；国服LCU可信链/名称尚未实测，生产兼容传输未改，S9未完成。
evidence:
  - internal/lcu/client.go:132
  - internal/lcu/tls_audit_test.go、docs/LCU_TLS.md
actions:
  - 已完成：官方文档/PEM/指纹核查；根明确用于Game Client，不能推断国服LCU兼容
  - 已完成：测试专用严格RootCAs的HTTPS/WSS有界只读探针、完整性/竞态及空白验收表
  - 待后置实机：验证链/名称后再决定生产可信传输接入，不绕过Go标准校验
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
status: configured-awaiting-activation
confidence: verified-static
needs_human: false
problem: 自动更新配置与Actions提交固定已就绪；默认main未含配置，实际激活/首个更新PR待正常合入后核对。
evidence:
  - go.mod（wails/v3 v3.0.0-beta.29）
  - frontend/package.json
actions:
  - 已完成：Dependabot v2分别配置Go、产品npm、preview及Actions，周一上海10:00，PR指向dev
  - 已完成：产品Go/npm跨生态组共同审阅Wails前后端更新，既有版本一致性门禁保留
  - 已完成：五类官方Actions由API核验后固定40位SHA，策略测试拒绝浮动标签
acceptance:
  - 已完成：YAML/JSON Schema/策略回归验证配置，统一CI保护保留
  - 待激活：配置正常进入默认分支后核对首个依赖PR，未修改main或冒称已生成
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
status: implemented
confidence: verified-static
needs_human: false
problem: >-
  097fbe9基线实际有10个生产函数及其中1个闭包超过100行，现全部按职责拆分至100行内。
  AST门禁、internal竞态与Windows目标staticcheck已通过；092eab6的CI41995367430完成Windows全包竞态/原生回归/构建，任务结项。
evidence:
  - internal/engine/engine.go、scouting.go（poll/replaceLocked/runScoutCursor首批已拆分）
  - internal/source/ranked.go、internal/core/timeline.go、scouting.go
  - internal/guides/service.go
  - overlay.go、cmd/perfprobe/main_windows.go、main_windows_test.go
  - cmd/functioncheck、internal/functioncheck、scripts/verify.sh
actions:
  - 已完成：runSession/runTask随S3、main随S8；首批poll/名单写回及游标页归并/报告按职责分开，完整引擎竞态通过
  - 已完成代码：段位合并/时间线schema/英雄汇总、攻略选择/加载归并、同UI dispatch侧栏/预览核验及性能采样循环，保留原边界
  - 已完成：AST长度门禁/边界与闭包单测、workflow变异防移除、Windows全包竞态入口；性能工具新增只读自身进程CSV/复用PID拒绝回归
  - 已完成：092eab6的Windows全包竞态、原生窗口回归、完整构建及隔离安装卸载（CI41995367430）
acceptance:
  - 已完成AST审计：没有超过100物理行的生产声明或闭包，原回归保留；Windows原生测试/新增全包竞态及完整构建通过
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
status: implemented
confidence: verified-static
needs_human: false
problem: 原前端只有浏览器脚本；现已增加Vitest独立单测，类型检查后在Node环境运行。
evidence:
  - frontend/package.json
  - frontend/src/historyBrowser.ts、presentation.ts（纯逻辑，适合单测）
actions:
  - 已完成：Vitest固定5.1.0，实际historyBrowser.ts与presentation.ts共19项测试
  - 已完成：独立类型检查，无浏览器/游戏依赖，只模拟桌面桥接
  - 已完成：通过verify.sh接入Linux与Windows门禁，策略与变异回归防止移除
acceptance:
  - 已完成：test:unit运行实际模块，既有Playwright入口不变；CI经统一verify执行
depends_on: [Q1]
verify:
  - npm --prefix frontend run test:unit
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
status: implemented
confidence: verified-static
needs_human: false
problem: 已核实并修复快捷键重复编译和攻略缓存失败静默，完成忽略错误及显式诊断报告留存审阅。
evidence:
  - "internal/settings/store.go:141 regexp.MustCompile 每次调用都编译（NormalizeShortcut）"
  - 被忽略的错误（先用 grep 逐个核对，不要一律改）
  - 诊断导出文件是否有清理逻辑（未核实）
actions:
  - 已完成：快捷键正则包级复用，原允许范围不变
  - 已完成：逐类核对忽略错误；攻略缓存维护失败可诊断并清理自身临时文件、保留内存结果
  - 已完成：诊断仅显式生成，无自动轮转；作为用户证据保留并说明可按需手动清理，不误删历史报告
acceptance:
  - 已完成：错误处理意图已记录，实际缓存静默失败已修复；没有新增自动重发或改变配置/报告留存行为
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
status: implemented
confidence: verified-static
needs_human: false
problem: >-
  README及ARCHITECTURE已按当前状态重写，历史索引/完整日志与测试入口分开；
  四份整理文档合计由原两份64,341字节缩至21,234字节，AGENTS操作规则和固定短语保留。
evidence:
  - docs/ARCHITECTURE.md
  - BUGFIX_LOG.md
actions:
  - 已完成：CHANGELOG历史索引、TESTING验证入口、33行README及当前架构说明，修正过时状态
  - 已完成：核对requireText无README/ARCHITECTURE依赖，AGENTS固定短语原样保留
acceptance:
  - 已完成：ARCHITECTURE开头为当前架构，四份文档本地链接和workflow策略/变异回归通过
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
status: implemented
confidence: verified-static
needs_human: false
problem: 已核实原有等待提示并补齐首页三步连接引导；断线保留旧资料，重连收起，无弹窗或额外取数。
evidence:
  - frontend/src/components/ConnectionGuide.vue
  - frontend/tests/home-theme.cjs
actions:
  - 已完成：未连接首页展示国服启动、登录、保持助手运行三步，沿用主题与局部样式
acceptance:
  - 已完成：首次离线/断线旧资料/重连/导航和亮暗窄窗由home-theme覆盖，核对无额外请求；真实连接不冒称实测
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
status: implemented
confidence: verified-static
needs_human: false
problem: 已核实S1/U1/S8既有监听说明、替代入口、冲突提示和保存回滚，相关Windows自动回归已通过。
evidence:
  - internal/settings/store.go（NormalizeShortcut）
  - internal/hold/
actions:
  - 已完成：SettingsPanel包含原文监听说明、按键冲突帮助及主窗/托盘替代入口
  - 已完成：注册冲突使用固定安全提示并保留原键，overlay-smoke验证冲突/改键/保存回滚
acceptance:
  - 已完成：当前代码和5ee4809的Windows CI可核对；国服实机仍按S1/R1后置
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
status: implemented
confidence: verified-static
needs_human: false
problem: 已补齐共享焦点与滚动区域、修正浅色小字对比度，并通过两主题真实键盘主流程回归；未冒称完整WCAG验收。
evidence:
  - frontend/src/controls.css、style.css
  - frontend/tests/keyboard-access.cjs（由player-analysis复用）
actions:
  - 已完成：真实按键覆盖首页/搜索/战绩/详情/设置/工作区和焦点返回；补链接/summary/滚动区域焦点
  - 已完成：浅色核心语义小字至少4.5:1，焦点至少3:1；保留减少动态效果
acceptance:
  - 已完成：两主题Playwright键盘路径、实际计算样式与对比度断言，纳入原test:smoke入口；边界见ACCESSIBILITY.md
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
status: implemented
confidence: verified-static
needs_human: false
problem: 已核实首页/磁盘页具备该策略，但完整ID与选中玩家的过期内存缓存仍同步等待，现已补齐。
evidence:
  - internal/engine/history_store.go、history_recovery_state.go
actions:
  - 已完成：两种查询入口返回本连接已核验的过期快照及原时间，复用既有合并后台刷新与本地更新事件
  - 已完成：显式强制刷新语义保留，取槽前后及写回均核验连接/阶段，失败保留原数据与资料
acceptance:
  - 已完成：可控慢来源未放行时，ID/选中玩家/首页均已返回缓存；成功更新与失败保留有竞态回归
depends_on: [S6, F1]
verify:
  - go test -race ./internal/engine/...
do_not:
  - 缓存内容超出 C8 的限制
effort: M
```

```yaml
id: F4
title: 统一的重试退避与并发限制
priority: P1
area: performance
status: implemented
confidence: verified-static
needs_human: false
problem: 已核实SGP共用冷却策略与独立并发上限；本批补齐抖动、并行成功误清门与重叠拒绝缩短等待问题。
evidence:
  - internal/source/
  - internal/engine/scheduler.go（已有单槽串行与手动优先）
actions:
  - 已完成：原指数退避基线增加0–20%非负抖动并封顶，服务端更长Retry-After优先且不被后续拒绝缩短
  - 已完成：拒绝序号隔离限流前成功，保留S7通道并发上限及U2降级，不新增内部重试
acceptance:
  - 已完成：并行429与迟到成功后64次调用均被门拦截，网络计数仍为初始两次；抖动/封顶/长等待有竞态回归
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
