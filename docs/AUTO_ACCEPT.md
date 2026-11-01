# 自动接受对局（BUG-0069）

这是用户主动启用的纯本地辅助功能。设置位于游戏辅助，默认关闭，默认延迟1秒，支持立即/1/3/5秒；配置保存在现有settings.json。旧配置升级保持关闭，无效延迟会关闭自动接受。恢复默认只改草稿，显式保存成功后生效，保存失败保留原生效设置。

## 执行与边界

Engine将核验后的连接及阶段通知internal/readycheck.Controller，现有阶段事件也可以提前唤醒/取消；Controller由Engine.Run启动并随父context结束，与玩家资料串行调度分离。它只在功能开启且处于Matchmaking/ReadyCheck时工作。阶段事件丢失时独立每约1秒读取阶段，确认阶段按500ms/倒计时最多250ms探测；ready-check单主题事件是唤醒信号，不能代替HTTP状态校验。事件合并且最短250ms，单次请求800ms，订阅失败不在同阶段循环重连，保留轮询。正常资料/选人/局内阶段不增加这些请求。

接口仅为本机LCU：GET /lol-gameflow/v1/gameflow-phase、GET /lol-matchmaking/v1/ready-check、POST /lol-matchmaking/v1/ready-check/accept。接受写方法固定端点，不向前端暴露任意POST；现有鉴权仅在内存，不记录原始响应、玩家身份或token。诊断仅保留ready-check、ready-check-accept、ready-check-events三类聚合计数。接受端点允许200或204，不能因此放宽其他GET或身份查询的状态检查。

先确认ReadyCheck及state=InProgress/playerResponse=None，再开始延迟。每次提交前重新读取阶段和本人响应。单任务串行提交，提交后再GET核对，只有Accepted才显示已接受。提交即使超时也先核验；仍明确pending才有界重试，总计不超过3次，未知或非pending停止。首次待确认起15秒后结束行动窗口，不循环重试。网络耗时使这些间隔成为调度目标，不是实时延迟保证。

“跳过本次”只取消助手本次动作，不向客户端发送拒绝，之后仍可手动接受。同阶段重复通知不会清除跳过状态；新的确认周期恢复自动接受。即使中间阶段通知同时丢失，已核验Accepted/Declined后再次读到pending也可识别新邀请；接受后仅保留低频状态观察，离开确认阶段即结束该周期。手动接受/拒绝不再POST。离开阶段、断线、账号换代、功能关闭和退出取消当前context，旧epoch响应不能提交或写回。阶段读取按请求发出时间与事件时间择新，较早发出的HTTP阶段响应即使迟到也不能清除跳过或重启本次任务。客户端请求已经发出后，关闭功能不能撤销客户端已处理的接受；界面只在倒计时期间提供跳过。

## 界面与兼容

独立State带递增revision，通过leaguepingo:auto-accept事件同步主界面和展开的客户端侧栏；初始化读取AutoAcceptStatus，较旧结果丢弃。显示待连接、已开启、倒计时/跳过、确认中、已接受、手动响应、结果未确认和客户端不支持状态。它不改变玩家Snapshot，不触发玩家查询，不创建窗口或抢焦点。主窗口隐藏、侧栏关闭时后端仍可执行。游戏辅助页统一标注实验性边界并提供“停用全部自动化”；该操作同时关闭自动接受、跳过点赞、返回房间和自动排队，已经由客户端处理的写请求仍不能撤销。

本自动接受模块不引入服务端、截图识别、模拟点击、CEF注入、自动重排、自动拒绝或自动选英雄。用户另行批准的赛后返回/排队独立见[POST_GAME.md](POST_GAME.md)。现有开源参考是LeagueAkari的阶段/取消流程、LeagueKit的事件与轮询兜底及提交前核验、ezlol的外部本地实现；本批为独立Go实现，没有复制其源码或新增运行依赖。LCU非正式支持接口，腾讯版本兼容和产品审核结论仍需单独核验，见上一轮研究及Riot官方League Client API说明：https://developer.riotgames.com/docs/lol 。

## 验证

internal/readycheck使用合成Source覆盖轮询发现、连续匹配、重复通知、超时实际成功、未知结果不重试、三次上限、跳过、手动响应、阶段退出、断线、关闭、旧连接迟到结果及关闭时零请求。Engine集成测试用阻塞History证明自动接受不等待资料。LCU HTTPS合成测试核验固定方法/端点与204，settings测试核验默认/持久化/异常配置。frontend/tests/auto-accept.cjs使用合成桥接核验保存失败、默认草稿、两组设置同次保存、一键停用、跨窗状态/跳过、旧revision拒绝和深浅/窄布局；接入test:smoke。

Linux通用检查及Windows目标编译不等于Windows原生执行或国服接受成功。实机清单见ACCEPTANCE.md，本批不生成或替换交付程序。
