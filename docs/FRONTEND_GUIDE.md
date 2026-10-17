# 前端迭代与组件预览

本指南对应现有 Windows 桌面助手的增量改进。视觉基准仍是 [UI_DESIGN.md](UI_DESIGN.md)，数据与请求边界仍以 [ARCHITECTURE.md](ARCHITECTURE.md) 为准。

## 模块边界

- `App.vue` 组合工作区、桌面事件与组件，不承载查询队列和资料合并算法。
- `composables/useLeagueSnapshot.ts` 管理阶段展示和快照顺序；只有接受的快照才交给工作区同步。
- `composables/usePlayerWorkspace.ts` 管理最多八个玩家标签、当前选择、各玩家浏览状态和断线旧资料。
- `composables/usePlayerActions.ts` 管理显式查询、更新、资料补取队列及忙碌状态；继续使用原有 Go 调度和预算。
- `composables/useOverlayControls.ts` 管理快览状态版本及显隐、置顶操作。
- `playerState.ts` 保留连接/历史/资料分别择新的合并规则及稳定标签 ID。
- `PlayerProfileHeader.vue` 和 `MatchHistoryRow.vue` 接收数据和发送交互事件。战绩行的双方详情通过插槽提供；组件自身不导入 Wails、不访问客户端或 CDN。
- `historyBrowser.ts` 保持已有分页、样本、停止/继续和来源受限规则。展示拆分不能增加后台查询或改变统计分母。

## 组件预览

组件工作台使用 Histoire，独立放在 `frontend/preview`。当前实际发布的 `histoire` / `@histoire/plugin-vue` 为 `1.0.0-beta.1`，要求 Vite 7；产品继续使用原锁定的 Vite 8。两个目录分别维护锁文件，预览工具不进入产品运行时或 Windows 交付入口。

在项目根目录执行：

```bash
npm --prefix frontend ci
npm --prefix frontend/preview ci
npm --prefix frontend run preview:components
```

工作台只绑定 `127.0.0.1`，地址以终端输出为准。样例包含战绩行五种状态、资料头部五种状态、快览四种状态及玩家标签一种交互，共 4 组 / 15 个变体。数据为固定时间的合成样例，不包含真实身份；复用本地图集。预览构建和开发配置移除 Histoire 默认的 Google Fonts 导入，沿用系统字体；通过 Iconify 支持的预加载方式使用本地 Carbon 图标集，工作台导航也不依赖图标 CDN。

构建及浏览器检查：

```bash
npm --prefix frontend run build:components
BROWSER_EXECUTABLE=/path/to/existing/chromium npm --prefix frontend run test:components
```

Windows Git Bash 可将 `BROWSER_EXECUTABLE` 指向现有 Edge。检查使用产品已有 Playwright，不自动下载浏览器。预览产物在 `artifacts/component-preview`；可选 `UI_SCREENSHOT_PREFIX` 输出合成截图到本机 `artifacts` 子目录，不提交截图或生成产物。

预览验证范围是组件渲染、缺失字段、窄布局、标签键盘/焦点、无页面异常和无外部请求。产品回归仍执行 `test:smoke`，Windows 交付仍走 `scripts/build.sh`。组件预览不等于 Wails 原生窗口、多屏/DPI、游戏悬浮或性能验收。

产品全局样式只注入组件 iframe，避免深色 tokens 和通用按钮样式覆盖工作台的浅色/深色导航。工作台本身和组件变体分别进行浏览器检查。

Histoire 上游主分支文档另有更高 beta 版本的 MCP 能力；本项目只使用已锁定发布版的工作台，**没有配置或验证该 MCP 服务**。AI 可使用现有 Playwright 对组件截图和 DOM 做核对。

## 给 AI 的改动约束

1. 先指出要修改的组件、对应交互状态和现有规则；已有视觉方向优先于通用设计技能的偏好。
2. 颜色优先复用 `style.css` 的语义 tokens，数字保持等宽数字；缺失值、真实零、过期资料、来源受限分别展示。
3. 展示层改动先用合成变体核对，再执行已有分页/切人/失败保留回归；不得以增加请求来填补视觉占位。
4. 玩家标签支持左右键循环、Home/End 和 Delete；仅当前标签处于顺序焦点中，关联对应面板。关闭最后一个标签后焦点回到搜索框。
5. 设置打开时背景不可交互，Tab 留在设置内；关闭或 Esc 后焦点返回触发入口。后台数据事件不主动抢焦点。
6. 最后核对 1280×800 五条完整战绩、880×620 主窗口、540×660 十人快览和 400×360 最小快览；浏览器验证和 Windows 实机结果分开记录。

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
