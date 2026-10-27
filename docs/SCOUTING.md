# 对局统计与本批处理清单（BUG-0050）

用户于 2026-10-21（Asia/Shanghai）批准：英雄经验覆盖全部模式，英雄排位表现按单双排/灵活分别展示；顶部统一切换并默认跟随当前对局；近期位置使用最近30天、所选排位；找不到可靠跨赛季场次来源时采用当前赛季。本文替代 BUG-0049 的近20场子样本口径。

## 数据源核验

本批未找到可验证、适用当前国服且完整覆盖 S3 至今的英雄累计场次来源。这是本次调查结论，不宣称任何第三方数据库都不存在相关数据；未接入未经核验的私有服务、开发者密钥或其他账号登录。

- [LeagueAkari SGP history helper](https://github.com/LeagueAkari/LeagueAkari/blob/56cdfde00e56148f4641d1a8cf86acb66e903124/src/shared/http-api-axios-helper/sgp/match-history-query.ts) 支持 SUMMARY 分页。其 [stats helper](https://github.com/LeagueAkari/LeagueAkari/blob/56cdfde00e56148f4641d1a8cf86acb66e903124/src/shared/http-api-axios-helper/sgp/stats.ts) 是单场结算，不是生涯英雄聚合接口。
- [LCU 文档](https://github.com/KebsCS/lcu-and-riotclient-api) 的 `lcu/swagger.json` 提供 `/lol-seasons/v1/season/product/LOL` 及 seasonStart/seasonEnd/metadata；只采用实际返回的当前有效时间范围，支持秒/毫秒与对象/数组，不从系统年份猜赛季起点。
- `/lol-champion-mastery/v1/{puuid}/champion-mastery` 提供英雄等级/熟练度分数，[现有开源封装](https://github.com/LeagueAkari/LeagueAkari/blob/56cdfde00e56148f4641d1a8cf86acb66e903124/src/shared/http-api-axios-helper/league-client/champion-mastery.ts) 与 LCU 文档一致。本批只用 GET，分数不换算为场次，返回身份冲突拒绝。
- 按批准回退：本英雄场次与排位 K/D/A 使用独立读取的当前赛季记录。真实熟练度作为全模式经验补充，字段含义单独展示。没有可信累计值时隐藏该指标，不显示“累计数据暂不可用”。

以上是接口结构与本批实现依据；当前主机没有实际客户端，未完成新端点的国服实机覆盖验证。

## 三份独立数据

1. 玩家查询：保留原有模式、10/20/30/40/50场分析、每页20场和显式后续分页规则。
2. 首页：复用查询浏览逻辑，但具有自己的模式/场数/页码/追加记录；账号切换清理，隐藏时停止后续扫描。近期表现/常用英雄跟随分析范围，翻页不改变分析样本。
3. 对局室和快览：只读 Participant.scout（ScoutReport），不拿以上两份列表的近20场作为回退。每份报告同时包含420/440两种排位，前端切换仅改变展示；SetScoutQueue 不查询资料。

Snapshot.queueId 来自已经读取的房间/gameflow内容；直接在选人阶段启动且没有队列缓存时，由有界准备补读一次gameflow。Snapshot.scoutQueue 为统一选择：排位默认跟随本局；手动选择本局保持，新阶段/新局重新跟随，非排位使用本次运行中上次选择，首次为单双排。快览直接消费同一快照，按住模式不增加交互控件。

英雄经验为本英雄当前赛季全部模式场次；英雄胜负、胜率和平均K/D/A按选定排位独立计算。12小时与30天分别按同一个统计截止时间取对应窗口；窗口跨赛季时保留前一赛季仍在30天内的记录，英雄赛季场次只用当前赛季。

近期位置仅统计最近30天、所选排位的明确position，展示主要位置、有效记录数和分布条，悬停列出全部位置。至少3场有效位置记录、唯一最多且占比至少一半才显示主要位置；未知不按lane/英雄猜测，也不据此重排两队。

## 完整性与调度

后台按阶段/对局准备，每范围最多10个有效身份，共用180秒截止、最多500页（每页20条，合计最多10000条来源记录），串行、手动优先，每次分页4秒、页间150ms。轮询、切卡、换英雄、模式/主题切换、快览显隐不重开预算；显式“刷新对局数据”可重开独立统计准备。旧的首批历史/资料45秒准备及手动12秒操作仍保留，不改变其重试规则。

按玩家轮流分页，避免第一名玩家占尽预算。先取得可选熟练度，再逐步准备赛季及时间范围；到达 min(赛季开始、30天窗口开始) 后停止。没有可信赛季边界时仅准备30天，不把它叫当前赛季累计。时间缺失、未来时间、顺序异常、重复页或来源限制均阻止完整标记。

仅“来源没有下一页”不能证明整赛季/30天完整：必须有实际记录到达相应窗口起点。范围未到达时标为已读范围；未知胜负不计失败，真实0%与未知分别呈现。未读到本英雄的当前排位记录不能暗示玩家从未使用过英雄。

离开阶段、换局、换号、断线和退出取消；结果核对连接/范围/token与当前成员，拒绝迟到覆盖。有效30天的终态报告五分钟内可复用，原统计截止保留；赛季已结束时不复用为本季，未完整的赛季仍标已读。普通中间进度最多每秒推送一次，首份数据与终态及时推送，减少双窗口反复渲染。

显式刷新收到空结果或较短范围时，保留同一有效赛季已有的较完整统计及原截止时间，界面标注“上次统计”；新结果覆盖足够范围后替换。新赛季不沿用旧赛季英雄场次，不把刷新失败当成玩家数据归零。

SGP延续可信当前大区、内存授权、正常TLS、不跟随重定向与脱敏诊断。列表与统计共用最多512页的15秒内存页缓存；首页/查询取得新历史头部时使错位缓存失效，SGP首屏始终重新读取，换号清空。无玩家资料落盘，无新后台定时刷新。

## 界面与素材核对

- [x] 窗口按钮、刷新/分页/模式按钮、输入与筛选控件统一样式及焦点/禁用反馈；保持原生拖动、最大化/还原与托盘逻辑。
- [x] 全局细圆角滚动条，隐藏系统上下箭头；WebView使用自定义滚动条，不被原有scrollbar-width:thin覆盖。
- [x] 首页三栏上下边缘对齐，窄窗段位移到上方；列表内部滚动与独立分页，不用内容行数决定各栏高度。
- [x] 首页补齐模式、分析场数、分页/跳页、继续/停止；打开完整战绩不共用其筛选。
- [x] 173张Riot高清横版原画替代首页放大的308×560载入图；10个官方段位图标用于首页、查询页与对局室。
- [x] 顶部统一单双排/灵活切换；段位标注模式，时段和位置分布随之切换。
- [x] 移除旧“本英雄样本不足”的近20场推断，英雄经验与排名表现各有明确范围。
- [x] 保留十人布局、详情浮层、快捷键、查询隔离及深浅主题。
- [x] 刷新失败/短范围保留已有统计与原截止，明确标注旧值；新赛季隔离。
- [ ] Windows/WebView2、真实玩家的新端点覆盖、DPI、多屏、按住快览与整局性能实机验收，见 ACCEPTANCE。

新增素材保留原字节：173张横版图与10个1000×1000段位PNG共34,380,468字节（约32.79MiB），SHA-256/来源清单分别在splashes和ranks目录。显式维护命令 `python3 scripts/sync-ui-assets.py`；只用于开发维护，不进入普通构建或应用运行。完整性/分辨率/50MiB预算检查接入verify，不新增产品依赖。来源与权利说明见 THIRD_PARTY_NOTICES.md。

## BUG-0057：段位与排位聚合胜负

BUG-0063根据新的十人赛前截图补充并替代本节“不处理非本人字面0”的旧结论：本人胜负正常，而多名其他玩家同时出现数百胜/0负，证明该值在产品上下文中是访问范围信号，不能作为全胜事实。最新LeagueAkari的[排位数据模块](https://github.com/LeagueAkari/LeagueAkari/blob/56cdfde00e56148f4641d1a8cf86acb66e903124/src/renderer/src-main-window/views/player-tabs/components/player-tab/data/ranked-stats.ts)也仅用LCU读取排位，并在[界面](https://github.com/LeagueAkari/LeagueAkari/blob/56cdfde00e56148f4641d1a8cf86acb66e903124/src/renderer/src-main-window/views/player-tabs/components/player-tab/widgets/RankedPane.vue#L289-L292)以`losses !== 0`为胜率展示条件。产品现在保留非本人的段位、LP与胜场，但将`wins>0 && losses==0`的负场视为未知，再使用同队列独立ScoutReport的当前赛季胜负；完整、部分和缺失分别显示“本季”“本季已读”“胜负待确认”。已核验本人显式0及逐场实际全胜仍可显示100%。原始解析/审计保留字面值，不跨来源拼字段，也不新增请求。

BUG-0060补充统一显式核验入口：`bash scripts/audit-player-data.sh rank` 同时报告LCU两容器、合并结果、SGP及差异；history/match核对真实页与字段/时间线，见 [PLAYER_DATA_AUDIT.md](PLAYER_DATA_AUDIT.md)。最高/上季字段不会再被同响应仅补计数的第二容器清空。下文保留BUG-0057当时的调查记录，当前产品结论以BUG-0063为准。

用户确认e9a9a1f战绩查询页单双排、灵活排位卡均0负场。该版本段位只读LCU `/lol-ranked/v1/ranked-stats/{puuid}`，缺失/null保持为空、前端显示—，不能归因为UI补零。也不能仅凭截图判断服务对他人数据脱敏。BUG-0056的本人/他人零负场猜测已移除。

统一资料流程现在优先接入同大区SGP `/leagues-ledge/v2/rankedStats/puuid/{puuid}`，使用专属league-session JSON字符串授权，独立于SUMMARY的entitlements。数据契约参考[LeagueAkari端点](https://github.com/LeagueAkari/LeagueAkari/blob/56cdfde00e56148f4641d1a8cf86acb66e903124/src/shared/http-api-axios-helper/sgp/leagues-ledge.ts)、[字段](https://github.com/LeagueAkari/LeagueAkari/blob/56cdfde00e56148f4641d1a8cf86acb66e903124/src/shared/types/sgp/ranked.ts)与[会话授权](https://github.com/LeagueAkari/LeagueAkari/blob/56cdfde00e56148f4641d1a8cf86acb66e903124/src/shared/http-api-axios-helper/league-client/league-session.ts)。SGP小段用rank，LCU用division；队列、可空胜负与真实currentSeasonId分别保留，不根据年份猜赛季。

SGP尝试最多1.2秒且不超过资料剩余预算的一半，保留LCU回退时间；仍属于原串行资料任务，不新增准备轮次/定时刷新/逐场扫描。两队列齐全直接返回，缺队列或失败回退LCU整条队列，不跨来源拼字段。两种授权各自两分钟内存缓存，401/403只清对应token、不自动重试，换号清全部。仅可信当前大区、正常TLS、拒绝重定向，凭据不出Go、不落盘。

排位卡优先使用可信且完整的SGP胜负，明确标排位范围；已有的逐场报告不能用已读全胜前缀覆盖非零聚合负场。聚合缺失或被BUG-0063判定为非本人受限零负时，独立ScoutQueue.season有有效胜负可提供本季/本季已读：完整且胜负齐全才为本季，未知不当败场。再无逐场结果时才使用可信的LCU完整胜负。本人真实零负保留，空总场次胜率—。首页/资料头/侧栏展开/快览统一；近20场/筛选列表不参与。12小时、30天、英雄排位和全部模式经验仍按独立报告及原口径展示。

WeGame官方公开战绩页使用GetBattleReport/GetPlayerRecentStat等独立聚合服务；存在各赛季单双排/灵活的分离计数，不等同本地LCU字段。当前没有WeGame授权，未读其Cookie或查询该玩家，不直接填入截图数字。SGP与WeGame必须对齐身份、大区、队列、赛季/赛段后才能比较，两个服务的总数不保证相同。

状态：新截图确认产品曾把非本人字面零负误当完整聚合，BUG-0063已改为用同队列真实逐场赛季记录提供可核验胜负，而非只隐藏数字。代码与合成回归仍不证明Windows候选已恢复；须在游戏电脑复验最终卡片，并用审计区分原始零、容器冲突、来源失败与范围不同。步骤见ACCEPTANCE BUG-0063/0057。
