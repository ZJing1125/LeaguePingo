# 对局统计取数管线：诊断与改造方案（BUG-0084）

日期：2026-11-01（Asia/Shanghai）。状态：P0、P1-S1~S5 与 P2 已实施并通过单元/前端回归，待实机复测。

范围：选人侧栏、局内快览、首页“用户自己”三处共用同一份独立统计（ScoutReport）与同一条比赛历史取数链。本文解释为什么在用户网络“能正常打游戏”的情况下，这三处会同时显示“统计尚未读全”，并给出分批改造。

本文不改变 BUG-0050/0066/0073 已定的数据口径与隐私边界：独立统计只认 SGP SUMMARY，不拿客户端 20 场前缀冒充 30 天/本赛季完整统计，不落盘玩家统计。

## 1. 用户报告与已核实证据

现象：产品无法取得完整玩家数据，覆盖用户自己、赛前选人侧栏、对局中；侧栏显示“统计尚未读全，可在主程序刷新数据”。用户反馈客户端与对局不卡顿，但未实际测量网络。

已核实（用户诊断导出截图，构建 `a6410f7`）：

| 观测 | 数值 | 含义 |
| --- | --- | --- |
| `mastery` | 33 / 1 | 统计任务确实运行过约 3 轮、每轮取到熟练度；该接口只被独立统计调用 |
| `sgp-history` | **不存在** | 整场会话没有任何一次 SGP 战绩页请求被记录 |
| `history`（LCU） | 52 / 0，平均 954 ms，最慢 2244 ms | 客户端取历史本身接近 1 秒、峰值超过 2 秒 |
| `rank` / `profile` / `season` | 46/0、294/0、11/0 | 本地接口与身份解析正常 |

补充证据（用户后续截图）：首页“最近战绩”同时是失败态（`0 / 20 场`、`战绩暂不可用，可刷新重试`），而段位、等级、熟练度均正常。说明失败不止于 SGP，而是**整条比赛历史取数链**：列表与独立统计一起取不到数。

本机（亚洲东部，非用户机器）用生产代码执行只读审计：关闭 VPN 后 SGP 每页 500–610 ms、`parsed 20 / excluded 0 / verified`；开启 VPN 时同样代码 4 秒超时 3/3。证明本次修复不能用“用户网络不好”一句话结案，也不能把本机结果当用户实机结论。

## 2. 现有路径

### 2.1 20 场列表（`TencentLCU.History`）

`internal/source/tencent.go`：先请求 SGP SUMMARY 第一页；失败后回退 LCU `/lol-match-history/`。**两段共用调用方的同一个上下文**：准备阶段给 `History()` 的总预算是 6 秒（`internal/engine/engine.go`），而 SGP 的 HTTP 客户端超时是 5 秒。SGP 卡满 5 秒后，回退只剩 1 秒，而客户端取历史实测峰值 2.2 秒——回退来不及完成就被整体取消。

### 2.2 独立统计（`prepareScouting`）

`internal/engine/scouting.go`：每个阶段作用域（连接/阶段/对局）启动一轮，最多 10 人、500 页、180 秒；每页 4 秒、页间 150 ms；**所有人共用一把 `sgpMu` 串行取页**；任一页出错即 `c.done = true`，该玩家本阶段不再尝试。

### 2.3 服务冷却（`serviceCooldown`）

`internal/source/recovery.go`：超时、连接失败、401/429/5xx 都被归为 transient，按 5/15/45/60 秒递增开门。门关上后 `sgpSummaryPage` 与 `getAuthorized` 直接返回、不发起请求，也不写入任何请求计数。

## 3. 根因分层

### L1 展示层：没有“范围”这个概念

`ScoutReport` 只有 `read`、`complete12`、`complete30` 三个布尔/计数，没有覆盖范围、失败原因、重试时间。UI 只能二选一：显示破折号，或显示一句通用提示（`SidebarPlayer.vue`）。用户看到的是“—”，读成“没有数据”，而真实含义是“没读到”。

### L2 策略层：一次失败等于全盘放弃

1. **预算耦合**：SGP 与 LCU 回退共享 6 秒，慢 SGP 会挤掉本可成功的本地回退。
2. **失败即终止**：单页失败 → 该玩家本阶段冻结；`attempted` 标记阻止重试。
3. **冷却放大**：一次超时 → 5 秒门 → 其余九人的首页瞬间失败 → 全员 `read = 0`；连续失败还会升到 15/45/60 秒。
4. **部分结果不复用**：跨阶段只复用 `Complete30` 的报告，选人阶段读到的 5 页进游戏后从第 1 页重读。
5. **抢占即取消**：手动请求会取消在途自动请求，用户点得越多，读页越难完成。

### L3 结构层：串行 + 全局锁 + 生命周期绑死

- 一次只有一页在途，耗时随延迟线性增长；10 人 × 十几页在 2 秒延迟下已超过 180 秒整轮预算。
- `sgpMu` 与共享冷却把历史、段位、单场、组队绑在一起，一个慢不影响另一个的隔离性不存在。
- 任务生命周期 = 阶段作用域，阶段一变就销毁；而 30 天覆盖本质上是“跨阶段也要继续”的长任务。
- 预算自相矛盾：500 页上限在 180 秒、1 秒/页下最多只能跑 180 页；4 秒/页时只有 45 页。

结论：不是某一个函数的 bug，而是“逐页串行扫描所有人全部历史”与“阶段内、可抢占、有整轮上限的任务槽”这两件事被设计在了一起。只要延迟略高或场次略多，结果必然为空。

## 4. 目标与非目标

目标：

1. 慢网络下仍能给出**可用的部分结果**（已读 N 场），并标明范围。
2. 失败原因对用户可见、对诊断可查，不再把“超时”说成“尚未读全”。
3. 单次失败不再冻结整批，同一阶段内可自动恢复。
4. 同一会话内已读进度不因阶段切换丢光。

非目标：

1. 不承诺在任意网络下取得 30 天全量；延迟是物理边界。
2. 不把 LCU 20 场前缀标成 30 天/本赛季完整统计。
3. 不把玩家统计落盘，不扩展 SGP 信任域与隐私边界。
4. 不引入新服务端、第三方登录或依赖。

## 5. 方案

### P0 快速修复（本批，改动小、可单独回滚）

1. **预算解耦**：`History()` / `HistoryPage()` 里给 SGP 尝试单独上限（4 秒，且不超过剩余预算的一半），保证 LCU 回退至少还有调用方预算剩余。
2. **冷却只针对服务拒绝**：仅 401/403/429/5xx 开门；超时与连接错误只让当前请求失败，不冻结整批。整轮仍受 180 秒与页数上限约束，不会变成无限重试。
3. **页级一次重试**：单页失败允许重试一次（计入页数与整轮预算），仍失败才冻结该玩家。
4. **失败不覆盖可用旧报告**：本批以现有 `saveScout` 保留逻辑为准（旧报告已读场次更多且新结果非终态时保留并标注“上次统计”），不新增字段；后续 P1 再纳入增量报告。
5. **文案区分**：侧栏在 `read = 0` 且报告带失败原因时展示原因（如“请求超时，已有资料保留”），不再让用户去点无效刷新。

### P1 取数管线（下一批）

1. **可恢复作业**：按连接/账号维护统计作业，每名玩家一个游标；阶段变化只降优先级、不销毁进度。
2. **SGP 分通道**：history / rank / detail / group 各自独立锁与冷却，历史慢不拖累段位与单场；并发 2 路。
3. **增量报告**：每读完一页发布一次，报告携带 `pagesRead`、`coverageEndsAt`、`retryAt`。
4. **预算自适应**：页超时取 `max(4s, 2×最近 P95)`，整轮上限按“每人页数”而非全局页数。

### P2 展示层

1. 破折号改为“待读取/服务超时/尚未到达该范围”三种明确状态。
2. 环形图按已读样本换算并标注分母。
3. 自动重试倒计时，替代“可在主程序刷新数据”。

## 6. 预算模型对比

| 场景（10 人，每人约 15 页） | 现状 | P0 | P1（并发 2 + 可恢复） |
| --- | --- | --- | --- |
| 1 秒/页 | 150 秒，临界 | 150 秒 + 重试余量 | 约 75 秒 |
| 2 秒/页 | 300 秒，必然溢出 | 部分完成（已读范围） | 约 150 秒，跨阶段继续 |
| 4 秒/页 | 600 秒，全空 | 首页失败但 LCU 列表可用 | 后台继续，界面显示已读 |

P0 不改变上表的线性关系，只保证“失败不再传染、部分结果能留下”；P1 才改变斜率。

## 7. 数据模型变化

P0 不新增字段（避免绑定重新生成），复用 `ScoutReport.message` 承载失败原因。

P1 计划新增：`ScoutReport.reason`（固定码）、`retryAt`、`pagesRead`、`coverageEndsAt`；需要重新生成 Wails 绑定，并同步前端绑定类型。

## 8. 边界与隐私

- 仍只向可信当前大区的 SGP 发送请求，仍不跟随重定向、不落盘授权。
- 诊断只新增固定分类与计数，不记录 URL、平台值、令牌、玩家标识。
- LCU 回退仍标注“客户端最近至多 50 场”，不冒充完整统计。

## 9. 验收

P0：

1. 单测：SGP 超过自身上限时 LCU 回退仍执行；超时不再触发共享冷却；失败页重试一次后仍失败才冻结；旧报告不被 0 场覆盖。
2. 竞态：`go test -race ./internal/source ./internal/engine`。
3. 前端：文案变更后的 smoke 与生产构建。
4. 实机：需在用户机器执行 `bash scripts/audit-player-data.sh history` 并导出一份脱敏诊断；合成测试不等于国服通过。

## 10. 风险与回滚

- 取消共享冷却可能让慢服务收到更多重试：用整轮 180 秒、页数上限和页级一次重试三重约束兜底。
- 预算解耦会缩短 SGP 单次可用时间（4 秒）：极端慢网络下 SGP 可能更早失败，但换来列表可用，属于有意取舍。
- 全部改动集中在 `internal/source` 与 `internal/engine`，可按批次单独回滚，不影响既有绑定。

## 11. 迁移顺序

1. P0（本批）：预算解耦 → 冷却分类 → 页重试 → 文案。
2. 用户机器复测与诊断导出，确认 P0 是否把“全空”变成“部分可用”。
3. 依据复测结果决定 P1 的并发度与是否保留 500 页上限。

## 12. P0 实施记录（2026-11-01）

| 改动 | 位置 |
| --- | --- |
| SGP 尝试独立窗口（≤剩余预算一半、≤4 秒），LCU 回退保留剩余时间 | `internal/source/tencent.go` 的 `History` / `HistoryPage` / `sgpAttemptWindow` |
| 共享冷却只由 401/403、429、5xx 触发 | `internal/source/recovery.go` 的 `transientServiceError` |
| 每页一次有界重试，仍是新请求并计入页数 | `internal/engine/scouting.go` 的 `scoutCursor.pageFails` |
| `read = 0` 时展示报告内的失败原因 | `frontend/src/components/SidebarPlayer.vue` |

回归：`TestHistoryKeepsLocalFallbackWhenSummaryIsSlow`（SGP 阻塞时仍返回 LCU 页且不耗尽 6 秒预算）、`TestRequestTimeoutDoesNotOpenTheSharedServiceGate`、`TestServiceRejectionStillOpensTheSharedServiceGate`、`TestScoutingRetriesOneFailedPage`、`TestScoutingStopsAfterRepeatedPageFailures`。

已执行：`go build ./internal/...`、`go vet ./internal/...`、`go test ./internal/source/... ./internal/engine/...`、`npm --prefix frontend run build`、`npm --prefix frontend run test:smoke`（11 组全部通过）。未执行：`-race`（本机无 cgo 工具链）、Windows 完整验证、真实国服与用户机器复测。

改动目录外的行为：无。未新增请求、未落盘玩家数据、未改变 SGP 信任域。

### P1-S1 可恢复游标（同日实施）

选人阶段读到一半的进度不再随阶段切换清零：`scoutPreparation.progress` 以 PUUID 保存已读比赛、offset、时间边界与完整标记（内存、每人最多 1000 条、整表最多 16 人、5 分钟过期），新阶段启动时从上次 offset 续读而不是第 1 页重来。换连接/换账号清空；进入结算阶段（`WaitingForStats`/`PreEndOfGame`/`EndOfGame`）视为范围已变化而清空；过期游标按新范围重读。恢复时仍补取英雄熟练度，但不覆盖已有报告为 0 场。

回归：`TestScoutingResumesProgressAcrossStageChange`（首个请求应为 offset 20 且最终 120 场完整）、`TestStaleScoutProgressIsIgnored`、`TestFinishedMatchDropsScoutProgress`。

仍未实施：SGP 分通道与并发、增量报告的显式字段、自适应页超时（需重新生成绑定，见第 7 节）。

### P1-S2 SGP 分通道（同日实施）

历史、排位、单场、组队各自持有独立的 `serviceGate`（自己的冷却 + 只串行本通道），SGP 的共享状态（可信大区、令牌、页缓存）只在短临界区内加锁；HTTP 请求本身不再持有全局锁。于是慢分页不会再挡住段位或单场读取，某个通道的 429 也不会让其他通道跟着等待。

回归：`TestRankReadIsNotBlockedBySlowHistory`（SUMMARY 挂起时排位仍在 2 秒内返回）、`TestRequestTimeoutDoesNotOpenTheSharedServiceGate`、`TestServiceRejectionStillOpensTheSharedServiceGate`、`TestOutageRetainsLocalHistoryAndGateCoversOtherPlayers`（同通道仍被冷却覆盖）。

实机核验：`bash scripts/audit-player-data.sh all` — `rank_sgp` verified 217 ms、`history_page_1..3` verified 458/453/477 ms、`detail_sgp`/`timeline_sgp` verified；唯一 `different` 是既有的 LCU/SGP `position`、`rune_shards` 字段差异（与本次改动无关，属 BUG-0064/既有解析差异）。

仍未实施：跨玩家并发取页（当前仍是单通道内串行）、增量报告的显式字段、自适应页超时。

### P1-S3 两路并行取页（同日实施）

`prepareScouting` 改为 2 个 worker，各自持有一名玩家的完整游标（含重试与 150 ms 间隔），共享一个原子页计数，预占页额度后再发请求，避免两个 worker 同时越过 500 页上限。SGP 分页不再占用本地串行调度器（它原本用于限制 LCU 的昂贵请求），英雄熟练度改为直接短读、2 秒上限，不再排在其他玩家的分页或熟练度之后。

取舍：手动操作不再“取消在途分页”，分页只在阶段/连接变化或整轮 180 秒截止时停止；这是有意为之——分页是远端请求，不应被本地 UI 操作打断。

回归：`TestScoutingReadsTwoPlayersInParallel`（两名玩家的分页必须同时在途后一起完成）、`TestUnlockedChampionFirstPageArrivesBeforeOtherPlayersMastery`（改为“第一名玩家不被另一名玩家的慢熟练度挡住”，原先的串行先后断言随并行设计更新）。

仍未实施：增量报告字段与自适应页超时（需重新生成绑定）。

### P1-S5 自适应页超时（同日实施）

页预算改为跟随最近延迟：`max(4s, 2×P95(sgp-history))`，上限 8 秒。诊断收集器新增 `Percentile(category, p)`，只复制该分类的 256 样本窗口，不物化全部分类。SGP HTTP 客户端的安全网从 5 秒提到 9 秒，位于页上限之上；各调用方仍传入更短的预算。健康链路保持原来的 4 秒，慢链路不会每页必然超时。

回归：`TestPercentileReadsRecentWindowPerCategory`、`TestScoutPageTimeoutFollowsRecentLatency`（空窗口 4s、3s 延迟→6s、极慢→8s 上限、nil 收集器回落 4s）。

### P2 展示层（同日实施）

- 侧栏三个时段读数在无报告时显示“待读取”，有记录仍用真实百分比；`30天位置` 在无报告时同样显示“待读取”。
- 展开卡的范围标签改为 `30天已读 N 场` / `30天待读取`，圆环副标题保留“有效 N 场”“已读 N 场”，不再用破折号冒充未知。
- 失败原因与自动重试秒数由固定码映射（P1-S4），未知码统一提示。

回归：前端 12 组 smoke（含新增 `scout-coverage.cjs`）与生产构建。

### 已实施总览（BUG-0084 批次）

P0 预算解耦/冷却分类/页级重试/失败文案；P1-S1 跨阶段游标；P1-S2 SGP 分通道；P1-S3 两路并行；P1-S4 覆盖字段与固定码；P1-S5 自适应页超时；P2 展示层。未执行：用户机器复测与 Windows 完整验证。

### P1-S4 增量报告字段（同日实施）

`ScoutReport` 新增 `reason`（固定码，不是错误原文）、`retryAt`、`pagesRead`、`coverageEndsAt`（已读到的最早记录时间）。引擎在每页保存时一并发布：页数、覆盖边界、失败码与冷却到期时间；范围受限与“整轮未读完”分别使用固定码 `range_limited`、`incomplete`。前端 `scoutCoverage.ts` 只按已知码映射静态中文，未知码统一提示，不再渲染后端 `message`。

绑定用固定 CLI 重新生成：`wails3 generate bindings -clean=true -ts -i`（39 方法 / 39 模型），未手改 `frontend/bindings`。

回归：`TestScoutingRetriesOneFailedPage` 增加 `pagesRead`/`coverageEndsAt` 断言，`TestScoutingStopsAfterRepeatedPageFailures` 断言固定失败码；新增 `frontend/tests/scout-coverage.cjs`（码映射、倒计时、未知码与空报告回退），已接入 `test:smoke`。

仍未实施：自适应页超时。
