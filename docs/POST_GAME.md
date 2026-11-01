# 赛后返回与自动排队（BUG-0070）

自动跳过点赞、赛后自动返回房间、返回后自动排队三个独立开关均默认关闭，跳过可单独使用。完整串联需三个选项均开启；自动返回包含已识别进度页的“继续”步骤。没有单双排/灵活白名单，海克斯2400共用流程，具体边界见SKIP_HONOR。排队倒计时默认10秒，可选5/10/15/20秒。只开排队时等待用户手动返回房间；只开返回时任何队员均可尝试返回，不发起搜索。使用现有本机设置持久化，保存成功才应用，失败恢复原配置，恢复默认只修改草稿。自动接受仍由BUG-0069独立控制，不自动替用户打开它。

2026-10-30用户报告P1/P2没有问题，但点赞页仍需手动处理，见[R1反馈](acceptance/R1-feedback.md)。当时基线ff92a24不包含自动跳过点赞；新增独立默认关闭能力见[SKIP_HONOR](SKIP_HONOR.md)，国服P6/P7仍待复验。原流程不能称为完整赛后全自动：PreEndOfGame的15秒是留给用户的等待时间，play-again不等同提交荣誉票。实际点赞页阶段和程序/客户端版本尚未反馈。

新增实现参考已核对的[LeagueAkari的荣誉接口](https://github.com/LeagueAkari/LeagueAkari/blob/56cdfde00e56148f4641d1a8cf86acb66e903124/src/shared/http-api-axios-helper/league-client/honor.ts)及其honor-controller：投票对象由单独honor方法处理，最后用无请求体的POST /lol-honor/v1/ballot提交。该源码只证明存在独立提交步骤，不能证明不投票直接提交在当前腾讯客户端一定能关闭页面。用户后续已明确增加独立选项，现已接入本局空票提交与结果核对，用户后续确认跳过可用，但“继续/再来一局/排队”仍需手动；已按BUG-0083修复这段衔接，修复版全链仍待实机。不引入自动选择点赞对象、插件注入或模拟点击。

## 触发与复盘

Engine在publishLocked中先执行syncReviewLocked，再将标量状态传给postgame.Controller。只有本连接、当前核验账号、已在InProgress观察到非零比赛编号和队列的对局，结束后又存在相同比赛的冻结review时，才允许创建任务。普通建房、助手在结算阶段才启动、退选、观战、掉线或只有旧review均不能触发。每比赛至多创建一轮；取消、禁用、手动排队及失败不重置attempted，重复快照不再启动，新比赛才重新获得资格。

任务使用独立Go协程/取消context，不占玩家资料scheduler。WaitingForStats只等待，不调用返回接口。review仍在等待或部分同步时最多留15秒结算宽限；EndOfGame可进入返回倒计时，PreEndOfGame额外留15秒投票/奖励操作时间。返回前有3秒可取消提示；不自动投票、不选择/领取奖励；自动返回开启时，只对明确missions-celebration完成进度展示，不通用跳过未知序列。房间转换不会清理冻结复盘，结算补取继续沿既有0/5/10秒、最多3次/15秒预算。已经缺失或受限的结果不会被自动返回功能伪造为完整。

## 本地接口与限制

返回/排队固定写入POST /lol-lobby/v2/play-again与POST /lol-lobby/v2/lobby/matchmaking/search；可选跳过另增GET /lol-honor-v2/v1/ballot与无正文POST /lol-honor/v1/ballot。BUG-0083增加GET /lol-honor-v2/v1/vote-completion、GET /lol-pre-end-of-game/v1/currentSequenceEvent与固定无正文POST /lol-pre-end-of-game/v1/complete/missions-celebration。四种固定POST允许200/204；未开放任意写API或自动创建房间。复用GET阶段、GET当前账号、GET /lol-lobby/v2/lobby，单次1秒。房间解析要求明确queueId、isCustom=false、partyId、本人成员/队长状态和canStartActivity；字段缺失不猜测，训练/自定义等不支持场景停止排队。原始房间仅瞬时解析，partyId与排序成员标识在内存哈希用于比较，不进入状态、设置、缓存或诊断。

返回到Lobby后校验同队列，只有本人是队长且canStartActivity=true、没有Pending邀请时才计时。队友未回、房间暂不能开始时最多等30秒；倒计时中继续监测同房间、同模式和同成员组成，变化即停止。不是队长时只显示等待队长开始，不替队长提交。开排前再次读取当前账号、阶段和房间资格；返回前也复核账号及结束阶段。

返回与开排每轮各最多POST一次，提交后依据实际阶段确认。即使HTTP超时但实际进入Lobby或Matchmaking/ReadyCheck/ChampSelect/GameStart也可确认；否则最多观察5秒后提示未确认，不盲目重发。整轮最多90秒，从已确认完成阶段开始，每秒探测一次，不在局内增加自动资料或状态读取。网络耗时使这些间隔不是实时保证。

“取消本次”可在等待结算、等待返回/就绪或倒计时阶段停止后续动作。账号/连接变化、None/离房、新局、手动开始搜索和退出均取消旧context并隔离迟到结果。客户端已处理的请求不能撤销；本功能不会为了取消自动任务去取消玩家已经开始的排队。队列被手动取消后，同局任务不会再次启动。

## UI与验证

PostGameNotice复用于主界面和展开侧栏，通过独立leaguepingo:post-game事件及revision择新，初始化只读内存状态；前端倒计时不查客户端。设置、主题、侧栏显隐及页面切换不启动任务或抢焦点。游戏辅助页明确说明实验性边界，并提供同时关闭自动接受和全部赛后动作的一键停用；关闭只能阻止后续动作，不能撤销客户端已处理的请求。两种主题、880主窗和280侧栏沿用产品tokens与字号。

合成Source覆盖返回/开排超时但真实成功、未知结果不重复、单人/队长/队员、等待队友、只返回/只排队、普通房间/缺复盘无动作、结算等待、取消/禁用/模式/房间/成员变更、换号/断线/迟到结果、就绪超时及跨比赛重新启用。Engine集成验证PlayAgain执行前review已冻结、回房不丢review。LCU HTTPS测试校验固定端点、204及严格房间字段；settings验证默认与持久化。post-game.cjs接入test:smoke，合成桥接验证独立开关/默认/失败保存、手动返回说明、跨窗倒计时/取消、旧revision和深浅窄布局。

参考已检索LeagueAkari与LeagueKit的固定接口及队长/就绪门控；本批独立Go实现，无源码复制、运行依赖、注入或服务端新增。Windows国服荣誉/奖励页面、真实房间字段与组队返回语义仍待ACCEPTANCE实机验证，不能把Linux合成或Windows目标编译当作实机成功。本批不生成或替换交付程序。
