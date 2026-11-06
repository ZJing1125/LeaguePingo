# 自动跳过点赞（BUG-0082）

“设置与诊断 → 游戏辅助 → 赛后点赞与下一局”提供独立的“自动跳过点赞”开关，默认关闭，旧配置升级不自动开启。它可以单独使用，只结束点赞页；是否返回房间、是否排队仍由原有两个开关决定。三个赛后开关和排队延迟在同一次保存成功后一起应用，失败恢复原值，恢复默认只是待保存草稿。

## 执行

沿用既有赛后任务：必须在本连接、账号观察过InProgress并冻结同局复盘；普通房间、退选、观战、结算阶段才启动助手不触发。每局一轮，关闭/取消后同局不重启；下一场可重新执行。没有增加常驻任务、局内资料读取或SGP请求。

使用本机固定GET `/lol-honor-v2/v1/ballot`、GET `/lol-honor-v2/v1/vote-completion`与无请求体的POST `/lol-honor/v1/ballot`。只解析比赛编号及是否已存在用户点赞，完成gameId必须匹配本局；fullTeamVote=false不表示本人尚未完成。候选玩家和身份在瞬时解析后丢弃，不进入前端、缓存、设置或诊断。缺失/非法字段、接口不支持和其他比赛的ballot不能当作可提交的空票。

找到本局未选择对象的ballot后，主窗/展开侧栏同步显示3秒倒计时，可取消本次。提交前再次核验账号、阶段和最新ballot；若用户已手动选择，助手不提交，在原有预算内等待手动完成。不调用honor/honor-player，不发送任何接收者或点赞类型，不模拟点击或注入，不选择/领取奖励。

POST每局最多一次。无论响应成功或超时，以本局vote-completion、提交后的PreEndOfGame→EndOfGame、当前已进入missions-celebration进度展示、旧的清票+EndOfGame或实际Lobby证明荣誉步骤结束，不再强求旧ballot清空。5秒未取得明确证据仍停止，不重发；不能只凭200/204判定页面结束。关闭功能不能撤销客户端已处理的请求。

自动返回开启时，还会处理用户反馈的“继续”步骤：读取固定`/lol-pre-end-of-game/v1/currentSequenceEvent`，只对明确的`missions-celebration`在原返回倒计时后复核账号/阶段/事件，再固定POST `/lol-pre-end-of-game/v1/complete/missions-celebration`一次。核验事件推进或阶段完成后才调用play-again，随后按原设置开排；超时只核验、不重发。不调用通用skip-pre-end-of-game或注册接口，不选择/领取奖励，也不完成未知选择页。单开跳过不调用这个继续步骤。兼容原EndOfGame返回路径，旧客户端不支持序列读取时不伪造读取结果。

WaitingForStats不读取或提交荣誉页，仍先等待复盘结算；PreEndOfGame尚无ballot时最多沿用原15秒等待，明确无活动页后保持原返回行为。全轮仍90秒、每请求1秒，ballot与vote-completion合计最多24次，序列读取最多12次，等候使用原约1秒探测间隔。每种写动作各最多一次、每步核验5秒。已手动选择时也受上述总期限/读取上限约束，不能无限等待。错误、账号/连接/比赛/阶段变化、退出或取消拒绝迟到结果。房间队长/就绪/同模式门控保持不变，不按排位白名单过滤；海克斯2400与其他正常队列共用此流程，详见[赛后流程](POST_GAME.md)。

## 验证

Go合成与本机HTTPS覆盖：默认无请求、独立仅跳过、跳过后返回/排队、成功响应丢失只提交一次、阶段未切换不误判成功、接口不支持、比赛/账号不符、手动选择及提交前竞态、取消/禁用/换号迟到、下一局重新执行、无点赞页、WaitingForStats与读取上限。设置及服务出口覆盖独立持久化、旧配置默认关闭、失败不生效。浏览器桥接覆盖三个独立开关、保存/草稿/失败、跨窗倒计时与取消、未知结果、深浅主题和280/880/1280布局。

接口参考为[LeagueAkari HonorHttpApi](https://github.com/LeagueAkari/LeagueAkari/blob/56cdfde00e56148f4641d1a8cf86acb66e903124/src/shared/http-api-axios-helper/league-client/honor.ts)、其同提交pre-end-of-game.ts/lobby-flow-controller.ts的固定进度完成调用，以及[客户端公开接口结构](https://github.com/KebsCS/lcu-and-riotclient-api/blob/main/lcu/swagger.json)。独立Go实现，无插件运行依赖或上游源码复制。用户已反馈跳过可用，但当时完整链条失败；2026-11-06用户已确认P6/P7通过，实际测试版本等元数据待补，见[实机反馈](acceptance/R1-feedback.md)。Windows原生自动验证以本提交CI为准，不将420/440/450/2400合成链条通过写作国服实测通过。
