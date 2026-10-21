# 本地设计技能：Taste Skill

用户于 2026-10-21 要求将 [Leonxlnx/taste-skill](https://github.com/Leonxlnx/taste-skill) 保存到项目，供后续设计改进复用。

## 本地副本与版本

- 技能入口：[design-taste-frontend / SKILL.md](../.agents/skills/design-taste-frontend/SKILL.md)。项目级目录便于支持 `.agents/skills` 的工具发现；当前会话未列出时，直接读取入口，或在更新代码后开启新会话。
- 固定上游提交：[6010f91a9935f1c167dfc500fb1a475681892e6b](https://github.com/Leonxlnx/taste-skill/commit/6010f91a9935f1c167dfc500fb1a475681892e6b)。本地包含该提交 `skills/taste-skill/` 的全部文件，以及上游根目录 MIT [LICENSE](../.agents/skills/design-taste-frontend/LICENSE)。原始文件不作本地修改。
- [UPSTREAM.json](../.agents/skills/design-taste-frontend/UPSTREAM.json) 记录来源路径、提交、逐文件 Git blob、字节数与 SHA-256。`.agents/.gitattributes` 保持换行原字节，方便 Windows/Linux 核对。
- 本项目的适配说明集中在本文与根 AGENTS.md，和上游源文件分开维护。无需全局安装、npm 安装器或上游桌面程序；该目录随 dev 拉取。

## 在 LeaguePingo 中怎样使用

先读根 AGENTS.md、UI_DESIGN.md、FRONTEND_GUIDE.md，再读本地 SKILL.md。用户要求、已确认的产品设计和本项目架构边界优先；技能提供设计方法，不扩大写入、联网、发消息或发布权限。

| 当前任务 | 使用方式 |
| --- | --- |
| 首页、对局室、战绩、设置、快览的界面改进 | 读取完整 `references/app-screens.md`，采用其应用界面流程：明确玩家要解决的问题、真实字段及数据范围、信息优先级、可操作状态、实际渲染核对。 |
| 应用图标 | 借鉴产品特征、轮廓独特性和视觉自审；本项目另补 16/24/32/48px 的真实像素检查，以及浅/深任务栏背景。生成图中的缩小示意不能代替这些检查。技能不是 ICO 生成器。 |
| 未来的官网或宣传页 | 使用主 SKILL 的页面流程，另核对图像/字体来源与适用许可；不把宣传页面的动画和图片数量要求套到助手界面。 |
| 现有设计评审 | 先记录原界面，再按 `references/critique.md` 检查实际截图，每轮列出三个具体问题与修改方法，至多三轮。 |

具体约束：

- 保留玩家已选择的深色/浅色、已认可的图标探索方向及真实数据语义；上游对紫色、卡片等风格偏好不自动推翻产品要求。需要保留既有主题时使用 `--keep palette,type`，评审中注明适用例外。
- 对局室继续支持十人比较、独立统计、统一排位模式和有界后台准备；设计改动不以渲染、切主题、悬停或快览显示增加玩家请求。
- 保持 Wails/Vue/TypeScript、现有组件与语义 tokens；不自动复制上游 CSS、图表、字体或网络图片进入产品。应用运行与普通构建保持现有资源和网络边界。
- 浏览器审阅只使用合成资料与隔离预览。技能检查是额外的视觉参考，不替代现有产品回归、Windows 交付检查和实机验收，也不能把作者的研究评分当成独立测评。

## 手动命令

在项目根目录的 Bash / Windows Git Bash 执行，需要已有 Node 22+。浏览器检查需要本机已安装 Chrome/Edge/Chromium，不由本说明安装。

先列出可用的应用类型或风格（只读）：

```bash
taste_skill=.agents/skills/design-taste-frontend
node "$taste_skill/scripts/draw.mjs" --app --list kinds
node "$taste_skill/scripts/draw.mjs" --app --list styles
```

按助手概览/数据阅读任务探索候选（`--options` 只打印候选，保持已有配色和字体）：

```bash
TASTE_SKILL_HISTORY="$PWD/artifacts/design/taste-skill/history.json" \
node "$taste_skill/scripts/draw.mjs" --app --kind analytics \
  --brief "LeaguePingo：英雄联盟玩家的对局比较与战绩查阅" \
  --screens overview,list,detail,settings --theme dark --keep palette,type \
  --options 3 --out artifacts/design/taste-skill
```

需要保存某个候选方向时，用上一步返回的真实风格 ID 加 `--style <id>`，并去掉 `--options 3`。保留 `TASTE_SKILL_HISTORY` 和 `--out`，把历史及产物限制在本工作副本的 artifacts。原脚本默认的 `.taste-skill/` 也已加入忽略，不能提交含玩家信息的截图、浏览历史或诊断。

对已经运行的**合成预览页面**做应用界面审阅：

```bash
CHROME_PATH="$BROWSER_EXECUTABLE" \
node "$taste_skill/scripts/inspect.mjs" "$LEAGUEPINGO_DESIGN_PREVIEW_URL" \
  --app --widths 1280,1100,880 --out artifacts/design/taste-skill/inspect
```

执行前把 `BROWSER_EXECUTABLE` 设为本机浏览器路径、`LEAGUEPINGO_DESIGN_PREVIEW_URL` 设为当前实际的本地合成预览地址；它们不是已预置的全局配置。上游脚本原生读取的是 `CHROME_PATH`，这里显式映射已有变量。设计文件不作为产品发布入口，仍按 build.sh 和既有验收流程交付。

## 维护和更新

更新为显式维护操作：在忽略的工具目录取得上游新提交，先检查 diff、许可与脚本，再替换本地技能目录、保留该提交 LICENSE、重新生成 UPSTREAM.json。不得对本地副本直接 `git pull`、自动跟随 main 或执行远程安装脚本。

更新后核对文件集合与逐文件 SHA-256、frontmatter、JSON 和 JavaScript 语法，运行候选列举/打印检查并记录结果；若实际上修改助手界面，再按该次改动运行产品回归。更改已认可的配色/图标等产品决策，需要按实际任务和用户要求处理，不能据技能默认值擅自替换。
