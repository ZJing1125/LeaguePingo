# 前端迭代与组件预览

本指南对应现有 Windows 桌面助手的增量改进。视觉基准仍是 [UI_DESIGN.md](UI_DESIGN.md)，数据与请求边界仍以 [ARCHITECTURE.md](ARCHITECTURE.md) 为准。

## 模块边界

- `App.vue` 组合工作区、桌面事件与组件，不承载查询队列和资料合并算法。
- `DesktopTitleBar.vue` 只管理主窗口外壳与 Wails Window 运行时，监听当前窗口状态并在卸载时清理。保持最大化状态查询与原生事件的顺序，不调用玩家服务。不要把它加入快览，也不要把窗口关闭改为应用退出。
- `composables/useLeagueSnapshot.ts` 管理阶段展示和快照顺序；只有接受的快照才交给工作区同步。
- `composables/usePlayerWorkspace.ts` 管理最多八个玩家标签、当前选择和断线旧资料。战绩页与本局抽屉各有独立的浏览状态表；连接/对局 ID 改变只重置抽屉状态，关闭玩家标签不清除抽屉筛选。
- `composables/usePlayerActions.ts` 管理显式查询、更新、资料补取队列及忙碌状态；继续使用原有 Go 调度和预算。
- `composables/useOverlayControls.ts` 管理快览状态版本及显隐、置顶操作。
- `playerState.ts` 保留连接/历史/资料分别择新的合并规则及稳定标签 ID。
- 姓名不由历史时间决定：当前同身份的有效姓名可以更新保留的历史；占位不能盖掉已知姓名。`presentation.playerName` 只生成明确标注的展示占位，不写回身份或缓存；资料结果的姓名要同步回当前名单和详情。每局开局补齐由 Go 驱动，前端只接受快照，不能再并行发一批玩家查询。
- `PlayerCardData.vue` 复用阵容/快览的近期总体、本英雄子集、近况与常用统计；`ChampionPortrait.vue` 只取打包的图片并处理回退，不新增请求。
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
