# 前端迭代与组件预览

U8：组件中文文案、提示、无障碍标签和动态格式统一从uiText读取，按共享/功能语义分组，不新增i18n依赖。本次迁移保留原文案；新增文案先加入目录，check-copy会扫描全部53个产品组件（含根应用）的脚本、模板和属性。Vue文本格式使用toDisplayString保留空值/对象显示，原JS模板保留原生转换；匹配片段/位置键/选择器需与使用方同步。资源名、已有TS表现层的数据映射和后端错误目录保持原归属，本批不重写取数或状态逻辑。

Q6：全部53个产品Vue组件使用scoped/module并沿用设计变量。存量私有全局样式已分五批迁完，38组已迁组件在scripts/component-style-owners.json登记；七份全局CSS仅保留共享tokens、控件、语义工具类与公共宿主契约。check-styles使用锁定的开发依赖css-tree 3.2.1解析选择器，并从模板、绑定、class属性和过渡名称识别实际归属，新组件无需另补正则即可受到保护；私有和无人使用的正向class选择器禁止回流全局。负向/关系引用不代表被选元素归属，分组中的私有分支仍单独核验。:where中的作用域避免增加权重，:is保留旧复杂选择器权重，跨子组件路径使用:deep；声明、媒体条件及相对级联保持。PlayerView保留原公共select契约的局部副本以维持同权重的加载顺序；新增样式不继续复制该历史兼容规则。该治理不改取数、模板、脚本或交互。

BUG-0113：AutomationView以独立loading及aria-busy标识Info读取，载入和保存期间冻结配置控件；失败可显式重新读取本机设置，没有自动重试或客户端写动作。post-game使用响应屏障测试重载持久值、失败重试与保存期间禁止被迟到响应覆盖的草稿；先等读取完成再断言真实开关，不能只等工作区挂载。组件文案现已由U8统一迁入uiText，原设置守卫不变。

U7：player-analysis末尾复用keyboard-access的真实按键路径，不改变test:smoke入口；覆盖两主题的首页/资料展开、详情、搜索、战绩筛选、分析标签、设置和工作区导航。只有Tab/Shift+Tab/回车/空格/方向键及Esc驱动操作，并核对焦点返回、可见边框、原有减少动态效果与核心语义色对比度。该检查不替代屏幕阅读器或Windows原生窗口验收。

U5：ConnectionGuide在首页未连接时显示启动国服客户端、完成登录、保持助手运行三步；使用原主题、局部样式及uiText文案，无弹窗、焦点或网络副作用。home-theme覆盖首次离线启动、断线旧资料、重连收起、亮暗主题及880/1280宽度；只读指引不改变后台查询策略。uiText已由U8覆盖全部产品组件文案。

Q3：`npm --prefix frontend run test:unit`先以tsconfig.unit.json检查测试类型，再通过固定Vitest 5.1.0在Node环境运行tests/unit。当前19项测试覆盖historyBrowser的实际Vue生命周期/缓存/扫描/迟到隔离和presentation的缺失值/统计/输入/错误展示；只模拟Wails桥接，不连接游戏，不需要浏览器。scripts/verify.sh在portable及Windows路径都强制执行，workflow策略拒绝移除。生产test:smoke仍负责浏览器交互和布局，两者不代替国服验收。

S1：游戏辅助设置补充“只监听已配置的按键，不记录其他输入”与主窗/托盘手动入口说明，不增桥接或设置字段；overlay-smoke核对说明与禁用按住后手动按钮仍调用ToggleOverlay，原设置保存失败/焦点回归保留。见HOLD_INPUT.md。

BUG-0083：PostGameNotice新增等待进度/正在继续/未确认等固定状态，继续核验时可取消后续动作。SettingsPanel说明自动返回包含已识别进度页；前端不调用序列接口、不抢焦点。结果以客户端“继续→再来一局→房间→开排”的真实完成为准，不要求用户读取诊断状态。

BUG-0082：SettingsPanel的autoSkipHonor为独立默认关闭草稿，与返回/排队/延迟用一次ConfigurePostGame四参数调用保存；Info与postgame.State分别返回autoSkipHonor/skipHonor。PostGameNotice新增荣誉等候/3秒倒计时/提交确认与手动处理状态，主窗/侧栏同revision同步，纯前端不读取ballot。恢复默认只改草稿，失败不改变当前生效值；post-game.cjs覆盖独立使用与保存失败。见SKIP_HONOR。

U2：HistoryServiceStatus显示原fetchedAt的相对时间和独立服务状态，正常时间嵌入原工具栏，不减少五场可见密度；只在可见空闲首页/查询首批调度受Go配额限制的RetryHistory，失败等候不占手动忙状态，离开/隐藏/阶段变化取消。HistoryPage无结果时的同页重试仍受原预算限制；player-updated仅合并当前连接已打开玩家，不自动开标签或换页。见DATA_RECOVERY.md。

## 错误提示

操作异常统一调用`presentation.errorMessage`，只按Wails `cause.code`映射共享固定中文；未知异常使用统一提示，禁止渲染reason.message或String(reason)。本地校验可用`errors.codedError`，不能把接口正文包装成文案。服务出口及脱敏诊断协议见[ERRORS.md](ERRORS.md)。

BUG-0075：MayhemAugments复用到竞技场时切换来源推荐与胜率/样本展示，不能把默认tier=0渲染成OP级；GuidePanel仅ranked/flex显示分路，轮换版本说明常驻，搭档数据采用竞技场标题。模式装备在本地图集中缺少图标时，同时展示来源装备名，不按变体ID猜成普通装备，不额外下载图片。

BUG-0073：MayhemAugments只消费Guide.base.augments；搜索品质在本地，候选通过GuideCandidates共享并由revision事件同步。GuidePanel与OverlayView额外核对当前英雄，拒绝同局旧英雄攻略。品质1/4/8与梯度分开，未知不补零，快览攻略面板明确使用深色tokens。

BUG-0072：useGuide以单窗口共享订阅消费leaguepingo:guide，GuideStatus只读内存，SelectGuide只选择合法分路和当前敌方英雄。GuidePanel正文使用文本插值，禁止v-html或执行压缩响应。BUG-0071的PremadeBadge消费匿名分组与人数，不从历史推算。


## BUG-0070 赛后自动操作

SettingsPanel新增两个默认关闭的独立开关与10秒默认排队倒计时，统一显式保存、恢复默认仅草稿及失败保留。PostGameNotice与自动接受状态分离，通过post-game事件/revision同步主窗和侧栏，等待/倒计时可显式取消，不请求玩家资料或改变窗口焦点。post-game.cjs接入test:smoke；严格模拟桥接只补齐PostGameStatus内存读取。

## BUG-0069 自动接受设置与状态

SettingsPanel新增默认关闭的开关与0/1/3/5秒延迟，仍使用区域统一显式保存及部分保存语义。AutoAcceptNotice在主界面与展开侧栏复用，订阅独立事件并按revision择新；倒计时只更新本地时间，不调用游戏接口，跳过为显式动作。关闭设置、换主题和普通玩家快照不发接受请求。auto-accept.cjs接入test:smoke，原有严格模拟桥接补齐只读状态方法。

## BUG-0068：首页常用英雄位置

HomeView只实例化一份ChampionAnalysis，放入home-ranks并位于MasteryPanel之前；home-analysis-main只保留近期分析、战绩和样本说明。home-match-list随20条记录自然增高，不创建固定高度嵌套滚动；useHistoryBrowser通过可选reveal回调让首页筛选/翻页定位战绩区，查询页仍复位自身滚动容器。home-theme回归核对组件归属、DOM顺序和无内部滚动。常用英雄的本地范围/搜索不得改变全部模式熟练度，也不得新增玩家请求。

## BUG-0067：对局室状态选择

App传入完整Snapshot给MatchRoom。当前页面直接消费participants并复用MatchRoster/RosterProgress，房间/选人alliesOnly，加载/局内按返回名单展示；ReviewRoom仅消费review，归档与当前数据不混用。当前和归档各自保留玩家筛选/弹层状态，连接或当前局变化时重置当前选择，归档新gameId时重置归档。两种浮层都connected=false，只允许缓存浏览；prepared/captured分别解释当前准备和历史归档，无自动历史/资料/详情读取。当前模式经SetScoutQueue同步辅助窗，归档模式仍局部切换。

companion-browser覆盖无review时局内十人、房间/选人只我方、载入/局内双方、空名单迟到、实时更新、断线保留、首页入口、新房间旧复盘隔离、进入加载默认当前、重复快照不抢选择、两套筛选与缓存浮层无额外请求及1280/1100/880布局。原生窗口与真实国服取数仍单独验收。

## BUG-0059：分析模块与验证

当前概览、查询与详情层级见 PLAYER_ANALYSIS/UI_DESIGN，替代下文纵向档案海报与平铺详情。新增共享 RankPanel、OverviewPerformance、ChampionAnalysis、PlaystylePanel、MasteryPanel；analysisData.ts集中定义有效分母、明确位置、已读7天、团队指标与经济差。查询摘要默认折叠，完整分析可随时展开；左右滚动互不挤压，面板不设等高。

MatchHistoryRow只消费传入记录和阵容，不读接口；MatchDetails用键盘Tabs组织概览/团队/时间线/出装符文，MatchTimeline与MatchBuild复用同一结果。analysisCache仅在内存按连接/玩家/比赛隔离，详情32局、时间线8局、5分钟，旧连接不会跨范围命中；后端独立核验同样边界。冻结provided详情保持只读，时间线未捕获不偷偷补取。标签、主题和筛选不发新的自动请求。

player-analysis.cjs加入test:smoke：合成桥接核对完整概览、丰富战绩、按需双方/时间线、六项比较、地图/滑块/符文、标签复用及1280/1100/880和两主题；scout-stats增加生产analysisData纯逻辑验证。现有四组产品交互与组件工作台继续覆盖，测试限定查询工作区，避免把隐藏首页的共享战绩行重复计数。原生窗口和真实国服覆盖单独验收。官方符文/地图由显式scripts/sync-analysis-assets.py维护，常规构建/运行不下载，资源预算与哈希纳入ui-assets.test.mjs。


## BUG-0055：单场详情与证据

MatchSummary负责本玩家的个人指标/装备/技能/符文，MatchHistoryRow统一复用；MatchDetails只处理显式加载或已提供的双方结果。HomeView保留selectedGame本地选择，用PlayerDrawer的label/fit呈现单场内容，modal事件交由App禁用背景业务区；不创建玩家查询标签或改首页分页。PlayerDrawer原有焦点圈定、Esc与原按钮恢复继续复用。

浏览器回归增加未锁定英雄的实际缓存统计、首页单场原位/焦点/1280与1100与880几何、双方七槽/双技能/记分板。ui-artifacts.cjs只在显式UI_SCREENSHOT_PREFIX下保存合成PNG；指定详情还能冻结无脚本HTML供本地Taste检查，不用于产品运行或真实资料导出，不提交证据文件。

## BUG-0054 游戏助手前端说明

新增AuxiliaryApp独立懒加载入口（view=companion；view=overlay保留兼容）：只订阅快照/窗口状态，不初始化首页、查询标签与历史浏览。ClientSidebar只取我方，SidebarPlayer默认紧凑摘要、details展开完整独立报告；OverlayView复用快览卡，preview与hold只读。应用首屏nextTick后通知AuxiliaryReady，原生等待就绪；自动展示生命周期由Go控制，不依赖前端倒计时动画。

ReviewRoom只消费Snapshot.review：本局结果与赛前资料分别显示，模式与浏览状态局部保存，缓存详情connected=false，不自动补历史/资料。显式查询最新战绩才进入原查询流程。MatchDetails的provided模式展示已核验结算，不再发读取请求。旧主程序实时名单交互测试已由companion-browser的阶段侧栏、缓存报告和复盘隔离场景取代；overlay-smoke保留查询/设置/临时提示与快览控件，history-browser保留连续稀少模式扫描/边界/分页/离线/查询隔离，home-theme保留首页/主题/排队查询。scout-stats与5组23变体组件预览继续验证核心卡片状态。新桥接由固定Wails beta.29 CLI按Windows目标生成，不手改绑定；见COMPANION.md。


## BUG-0053 当前界面行为

scoutStatus.ts 为纯展示聚合，RosterProgress 在主窗口和快览复用。它从实际 participants 计算身份/近期战绩/当前模式独立统计覆盖；时效只使用报告 fetchedAt，覆盖标记必须包含对应队列。ScoutRing 保留图形和数值，移除重复短标签；卡片和固定/按住快览不会因可读性调整发请求。PlayerView 的“本局统计说明”读取独立报告，与该浮层的战绩筛选分离。

App 的等待查询只保留一次用户明确提交，等 homeLoading/manualBusy 均结束后走原 search；输入变化、换连接、断线取消，不并发争用后端查询通道。保持原12秒预算，无新增桥接或后台请求。设置导航不请求玩家；仅保存有变化的快捷键项，失败时以返回的实际 AppInfo 为生效依据。

布局检查包括300px十人卡、236px快览、880宽资料摘要和七装备可见性；少量常用英雄采用整组布局，保持首页筛选/分页独立和键盘跳页。PlayerProfileHeader/MatchHistoryRow 属子组件，父级局部样式须用 deep 选择器，避免只移动外层但仍继承旧纵向布局。不能以容器不溢出代替内容不被裁切的检查。controls/theme共享规则同时覆盖被Teleport到主应用外的drawer-floating，避免详情继承原生控件或浅色悬停背景。


本指南对应现有 Windows 桌面助手的增量改进。视觉基准仍是 [UI_DESIGN.md](UI_DESIGN.md)，数据与请求边界仍以 [ARCHITECTURE.md](ARCHITECTURE.md) 为准。

## BUG-0056 当前展示规则

rankRecord.ts统一提供有范围的排位胜负，rankPresentation返回record，首页、资料头与PlayerCardData共用。完整独立本季报告才显示本季，部分胜负/覆盖显示本季已读；段位接口回退显示排位，未知负场不计算胜率。模式使用420/440既有缓存，不改列表样本，也不新增服务请求。加载阶段的首页/赛后主窗手动入口可调快览，辅助前端显示载入快览文案；系统资格、一次/6秒与收起仍由Go控制。段位图标与模式同排，胜负分行；1100快览保持236px卡高，680px以下两列窄布局改为至少250px并随内容增长，英雄/LP/范围换行不会覆盖圆环，不减小原读数字号。

## 模块边界

- `BrandIcon.vue` 为标题栏/导航/快览共用的 C 款应用图标，使用本地 `public/brand` 的 1×/2×/3× PNG；外观不随主题改色，不能再套字母 P 的 clip-path。Histoire 与产品共享 public 资源。源稿、ICO、派生图和校验命令见 APP_ICON.md；新增图像不产生玩家请求。

- `useTheme.ts` 负责浅色/深色/跟随系统、系统外观变化及本机保存，首帧选择在 index.html 解析；`theme.css` 为语义颜色，不能只替换首页背景。快览保持深色，不随主窗口使用浅色。
- `HomeView.vue` 只展示本人首批历史/资料；`useHomePlayer.ts` 按 Snapshot.selfKey 合并首页结果与名单资料，每连接/账号在空闲首页最多自动调用 CurrentPlayer 一次。标签、列表顺序、最近查询者不能用于推断本人；返回首页/换主题/普通快照不重复查询，失败须显式刷新。CurrentPlayer 复用原调度和 12 秒预算，离开空闲取消；对局前后原有准备仍由 Go 管理，首页不启动另一批。
- 产品 `test:smoke` 包含原四组交互及 `home-theme.cjs`：首页本人识别、一次准备/失败保留/换号、两种主题与系统切换、重启保存、跨窗口存储同步、存储失败和窄布局均使用合成桥接验证。组件工作台仍是 5 组/23 变体，不将其计作新增首页的真实账号或 Windows 验收。

- `App.vue` 组合工作区、桌面事件与组件，不承载查询队列和资料合并算法。
- `DesktopTitleBar.vue` 是统一顶部栏外壳：App 通过 slot 提供唯一品牌、导航/查询等业务入口，组件保留 Wails Window 状态/事件与最右侧窗口按钮。slot 在弹窗时 inert，窗口控制仍可用；双击处理排除全部交互区域，查询框可选字。保持最大化查询与原生事件顺序、卸载清理和关闭到托盘，不调用玩家服务，不加入快览。
- `composables/useLeagueSnapshot.ts` 管理阶段展示和快照顺序；只有接受的快照才交给工作区同步。
- `composables/usePlayerWorkspace.ts` 管理最多八个玩家标签、当前选择和断线旧资料。战绩页与本局抽屉各有独立的浏览状态表；连接/对局 ID 改变只重置抽屉状态，关闭玩家标签不清除抽屉筛选。
- `composables/usePlayerActions.ts` 管理显式查询、更新、资料补取队列及忙碌状态；继续使用原有 Go 调度和预算。
- `composables/useOverlayControls.ts` 管理快览状态版本及显隐、置顶操作。
- `playerState.ts` 保留连接/历史/资料分别择新的合并规则及稳定标签 ID。
- 姓名不由历史时间决定：当前同身份的有效姓名可以更新保留的历史；占位不能盖掉已知姓名。`presentation.playerName` 只生成明确标注的展示占位，不写回身份或缓存；资料结果的姓名要同步回当前名单和详情。每局开局补齐由 Go 驱动，前端只接受快照，不能再并行发一批玩家查询。
- `PlayerScoutCard.vue` 共享对局室/快览的账号头部与 PlayerCardData；`scout.css` 管理分区布局及两种主题。`scoutStats.ts` 是仅依赖已有首批历史的纯统计，提供有界时段子样本、可核验历史位置及事实标签；`matchStats.ts` 保留原 sampleStats 的分母/KDA算法，presentation 继续兼容导出。RankEmblem/RoleIcon 使用本地原创矢量，不发外部请求。`ChampionPortrait.vue` 继续服务首页/档案背景，不再覆盖对局卡。
- `PlayerProfileHeader.vue` 和 `MatchHistoryRow.vue` 接收数据和发送交互事件。战绩行的双方详情通过插槽提供；组件自身不导入 Wails、不访问客户端或 CDN。
- `historyBrowser.ts` 将样本补齐与远页跳转分开计预算：样本按模式持续至目标数量/来源边界/90 秒，远页仍五请求/15 秒。停止、切人、隐藏工作区或卸载会停止后续请求，保留已完成页；资料/历史仍按来源戳择新。展示拆分不能增加后台查询或改变统计分母。

## 组件预览

组件工作台使用 Histoire，独立放在 `frontend/preview`。当前实际发布的 `histoire` / `@histoire/plugin-vue` 为 `1.0.0-beta.1`，要求 Vite 7；产品继续使用原锁定的 Vite 8。两个目录分别维护锁文件，预览工具不进入产品运行时或 Windows 交付入口。

在项目根目录执行：

```bash
npm --prefix frontend ci
npm --prefix frontend/preview ci
npm --prefix frontend run preview:components
```

工作台只绑定 `127.0.0.1`，地址以终端输出为准。样例包含战绩行五种状态、资料头部七种状态、快览五种状态（含按住只读）、玩家标签一种交互及双方名单五种状态，共 5 组 / 23 个变体。数据为固定时间的合成样例，不包含真实身份；复用本地图集及原画。预览构建和开发配置移除 Histoire 默认的 Google Fonts 导入，沿用系统字体；通过 Iconify 支持的预加载方式使用本地 Carbon 图标集，工作台导航也不依赖图标 CDN。

构建及浏览器检查：

```bash
npm --prefix frontend run build:components
BROWSER_EXECUTABLE=/path/to/existing/chromium npm --prefix frontend run test:components
```

Windows Git Bash 可将 `BROWSER_EXECUTABLE` 指向现有 Edge。检查使用产品已有 Playwright，不自动下载浏览器。预览产物在 `artifacts/component-preview`；可选 `UI_SCREENSHOT_PREFIX` 输出合成截图到本机 `artifacts` 子目录，不提交截图或生成产物。

预览验证范围是组件渲染、缺失字段、窄布局、标签键盘/焦点、无页面异常和无外部请求。产品回归仍执行 `test:smoke`，Windows 交付仍走 `scripts/build.sh`。组件预览不等于 Wails 原生窗口、多屏/DPI、游戏悬浮或性能验收。

产品全局样式只注入组件 iframe，避免产品 tokens 和通用按钮样式覆盖工作台的浅色/深色导航。工作台本身和组件变体分别进行浏览器检查。

Histoire 上游主分支文档另有更高 beta 版本的 MCP 能力；本项目只使用已锁定发布版的工作台，**没有配置或验证该 MCP 服务**。AI 可使用现有 Playwright 对组件截图和 DOM 做核对。

## 给 AI 的改动约束

1. 先指出要修改的组件、对应交互状态和现有规则；已有视觉方向优先于通用设计技能的偏好。
2. 颜色优先复用 `style.css` 的语义 tokens，数字保持等宽数字；缺失值、真实零、过期资料、来源受限分别展示。
3. 名单展示集中在 MatchRoster，浮层/内联焦点生命周期集中在 PlayerDrawer；展示层改动先用合成变体核对，再执行已有分页/切人/失败保留回归；不得以增加请求来填补视觉占位。
4. 玩家标签支持左右键循环、Home/End 和 Delete；仅当前标签处于顺序焦点中，关联对应面板。关闭最后一个标签后焦点回到搜索框。
5. 设置打开时背景不可交互，Tab 留在设置内；关闭或 Esc 后焦点返回触发入口。后台数据事件不主动抢焦点。
6. 最后核对 1280×800 五条完整战绩、十人阵容上下两排与底部详情、880×620 主窗口/详情浮层及键盘焦点、1100×640 十人快览和 400×360 最小快览；浏览器验证和 Windows 实机结果分开记录。
7. 窗口标题栏使用 `--wails-draggable: drag`，交互控件显式 `no-drag`；不要给整个页面设置拖动。原生窗口选项集中在 `desktop_window.go`。标题栏调用固定版本运行时，不为这些操作新增业务绑定或手改生成文件。

## 本批使用的外部参考

以下覆盖对话中的六项产品参考与四项工程参考；Vue Skills 在两组重复，因此共九个不同仓库。采用适用的设计/工程方法，并非全部引入运行时依赖；未复制同类产品源码。

| 仓库 | 在 LeaguePingo 中采用的方向 |
| --- | --- |
| [LeagueAkari](https://github.com/LeagueAkari/LeagueAkari) | 对照桌面战绩工具的信息组织与可预览组件。保留本项目 Wails/国服适配，不套用 Electron 通信或假定其腾讯服兼容。 |
| [Naive UI](https://github.com/tusen-ai/naive-ui) | 参考主题语义与紧凑工具界面的做法；现有自定义组件和 tokens 继续使用。 |
| [Vue Skills](https://github.com/vuejs-ai/skills) | 按职责提取 composable，区分展示与操作；保留现有状态一致性和清理。社区实验规则受本项目规范约束。 |
| [UI UX Pro Max](https://github.com/nextlevelbuilder/ui-ux-pro-max-skill) | 用信息层级、响应式、焦点和减少动态效果检查现有设计；不重新生成与现有规范冲突的主题。 |
| [LCU / Riot Client API 文档](https://github.com/KebsCS/lcu-and-riotclient-api) | 校对客户端状态与数据边界；本批前端整理不新增端点、订阅或自动查询。 |
| [CommunityDragon Docs](https://github.com/CommunityDragon/Docs/blob/master/assets.md) | 保持资源 ID 与版本缺失回退，继续本地图集；本批没有新增远端资源抓取。 |
| [Histoire](https://github.com/histoire-dev/histoire) | 独立组件工作台、合成状态变体与可重复的截图/DOM核对。 |
| [Anthropic frontend-design](https://github.com/anthropics/skills/tree/main/skills/frontend-design) | 在既有视觉方向内检查排版、密度与风格一致性，按设计—实现—截图复核迭代。 |
| [Reka UI](https://github.com/unovue/reka-ui) | 参考 Tabs/Dialog 的键盘、面板关联和焦点管理行为，原生 Vue 实现并增加浏览器回归。 |

本批新增的开发依赖只有独立预览工具链；不整体安装上述仓库的技能、组件库或桌面程序。涉及第三方代码复制或资源新增时需另行核对许可证与来源。


## BUG-0045 视觉参考与素材

最新视觉以用户确认的纸白/墨色/橙色信息丰富预览为准，取代上批深色侧栏布局。对照 [OP.GG Desktop](https://op.gg/desktop/en/games/lol) 的近期总体、本英雄子集、最近五场和档案信息层级，没有复制其专有评分、品牌或源码。不能在现有来源没有位置/自身标记时照搬演示数据。

173 张经典载入原画约 7.70 MiB，仅通过显式 `node scripts/sync-champion-portraits.mjs` 维护（Node 环境需可访问 Riot CDN）。应用和常规构建不下载，完整性/体积测试纳入 verify。清单记录原始来源、字节数和 SHA-256；游戏素材归属见 THIRD_PARTY_NOTICES.md。新增运行时依赖为零，Wails 固定 beta.29，新增桥接必须通过 CLI 生成。

## BUG-0046 原画与段位状态

ChampionPortrait 在名单/快览中是卡片直接子元素，使用 contain 保留完整载入原图；渐变为独立伪元素，文字和选中边框位于其上。不恢复窄标题裁切，不在运行时取网络图片。名单仍使用首批历史，底部详情和战绩页浏览状态独立。

rankPresentation 统一卡片和资料头部文案：未请求、失败/超时、来源缺失、该模式未返回、明确未定级与旧值。LoadRanks 只由明确按钮触发；返回值经 acceptProfile 合并到现有工作区，不开新标签，不替换历史。EnsureProfile 的基础增强不查询或改写段位时效，切人、快照和快览显隐不能触发段位请求。组件预览补充原画压缩、段位超时/查询中样例。

## BUG-0047 玩家卡片与统一刷新

最新布局以 UI_DESIGN.md 为准：PlayerCardArt 以完整居中原画加模糊同源底图、纵向渐变构成全卡背景；账号头像与英雄小图分开，去掉 inset 黑边/切角。MatchRoster 的卡片不因详情压缩，固定可读字号，窄窗换列/滚动。PlayerDrawer 始终为模态浮层，提供上一位/下一位，背景 inert、焦点圈定/返回保持。布局不能发批量请求。

PlayerProfileHeader 只呈现头像/等级/两种段位/简短隐私状态，不再含更新资料或段位按钮；PlayerView 的唯一“刷新数据”调用 RefreshData，经各工作区自己的浏览状态保留模式/场数，不创建标签。EnsureProfile 改为完整资料缺失补齐；正常新鲜资料随快照到达，不由卡片渲染逐个发请求。上文 BUG-0046 的段位按钮和底部压缩方案属于历史实现，已由本节替代。

## BUG-0049 图一对照卡

当前卡片视觉以 UI_DESIGN 的 BUG-0049 为准，替代上文 BUG-0047 全卡原画；删除未再引用的 PlayerCardArt，档案原画资源保留。组件预览仍为 5 组/23 变体，名单最后一项改为英雄/段位并列状态。

`node frontend/tests/scout-stats.cjs` 使用既有 TypeScript 编译器运行实际纯模块，核对时段边界/未来或无效时间、未知与零、位置证据/并列、前 20 场上限、KDA 和事实标签；已接入产品 test:smoke。时段基准固定在 historyMeta.fetchedAt，不由计时器或最新一场时间代替。无位置不从 lane 补，空样本不生成评价。卡片选择、模态焦点/筛选隔离及游戏请求预算沿用现有逻辑。


## BUG-0050 当前实现

完整口径与核对清单见 SCOUTING.md。PlayerCardData/scoutStats 只消费 Participant.scout；不使用 history 近20场回退。模式来自 Snapshot.scoutQueue，SetScoutQueue 只切缓存并推送双窗口；QueueSwitch 是无数据请求的展示控件。英雄当前状态优先于较旧历史，报告/历史/资料分别按自己的时间合并。HomeView 复用 useHistoryBrowser，维护独立的账号浏览状态；隐藏和换号停止扫描。

controls.css 统一控件/滚动条并保留键盘行为；首页用 ChampionSplash，档案的纵向 ChampionPortrait 仍保留。RankEmblem 使用原始官方PNG及本地回退，不再自绘段位。资源维护与体积检查见 THIRD_PARTY_NOTICES、ui-assets.test.mjs；不改变运行时依赖和固定Wails版本。

scout-stats.cjs 验证实际纯展示模块只用独立报告，companion-browser.cjs 验证超过20场的英雄经验、排位联动且无额外查询、快览缓存、高清/官方素材、首页筛选/分页与工作区隔离；均接入 test:smoke。原五组产品回归与组件工作台继续验证。Windows交付仍用 build.sh，不能把合成数据当作真实国服结果。
