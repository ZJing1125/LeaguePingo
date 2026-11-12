# 依赖更新与验证

S10配置已进入默认分支。[PR #3](https://github.com/ZJing1125/LeaguePingo/pull/3)于2026-11-05 15:34:10 UTC由仓库流程合入main，合并提交8e7d6af；只增加与dev逐字节相同的配置，没有带入其余产品改动。当前状态为configured-awaiting-service-verification：尚未观测到实际更新任务或依赖PR，不把配置合入记为任务成功。不直接修改main、不改默认分支、不自动合并。

2026-11-12复核：默认分支仍为main，配置blob仍为`af4cf235a33b6413ee5f456db00464e75c1c0688`，与dev一致。全部PR列表中仍无Dependabot PR，按`dependabot[bot]`筛选的Actions运行列表为0；工作流列表仅返回产品CI，未返回Dependabot动态任务。这些只读结果不能证明更新服务未执行或执行失败，也不能把“未见PR”登记为“无更新”。S10继续等待更新页面的实际任务结果；原定11月9日计划时间已过，按配置下一次计划为11月16日上海10:00（UTC 02:00），不是产品上线或交付时间。仓库维护者可在[更新页面](https://github.com/ZJing1125/LeaguePingo/network/updates)核对任务成功/失败及错误摘要；没有服务记录前不改动频率、依赖版本或仓库安全设置。

| 范围 | 配置和节奏 |
| --- | --- |
| Go `/` 与产品 npm `/frontend` | desktop跨生态组，共同PR到dev；上海时间每周一10:00 |
| npm `/frontend/preview` | 独立preview组，同一每周计划，不混入产品Vite/runtime |
| GitHub Actions `/` | 独立actions组，同一每周计划，保持完整提交SHA |

跨生态组让Wails后端与前端运行时进入同一更新审阅，不保证第三方每次同时发布兼容版本；统一verify仍校验版本一致，完整CI失败不得合入。保持受管Windows构建、绑定一致性和浏览器回归，PR不自动合并、不发布或部署。

BUG-0128安全升级通过PR进行：Go最低补丁版为1.26.10，CI三个任务都从go.mod读取，分析器同步为staticcheck v0.8.2和govulncheck v1.8.0；GOTOOLCHAIN=local不允许任务自行切换工具链。官方Go下载元数据的SHA-256已核验，本机升级前11项可达报告、升级后0项，c711426的42024419785已通过Windows完整验证/构建及隔离安装卸载。Wails前后端继续beta.29，第三方产品模块版本及go.sum不变；不是一次性升级全部依赖。

参考来源：[Go官方支持版本与校验和](https://go.dev/dl/?mode=json)、[staticcheck v0.8.2模块要求](https://proxy.golang.org/honnef.co/go/tools/@v/v0.8.2.mod)、[govulncheck v1.8.0模块要求](https://proxy.golang.org/golang.org/x/vuln/@v/v1.8.0.mod)。BUG-0129将三项检查接为强制quality任务，Windows构建依赖其成功，失败后其余独立检查仍可产出真实摘要。工具链升级与S10服务执行分别登记；配置已合入默认分支，实际更新任务仍待核验。

本批使用官方仓库API核验的Actions固定点（主版本保持）：

| Action | 版本 | 提交 |
| --- | --- | --- |
| actions/checkout | v4 | `11d5960a326750d5838078e36cf38b85af677262` |
| actions/setup-go | v5 | `40f1582b2485089dde7abd97c1529aa768e1baff` |
| actions/setup-node | v4 | `49933ea5288caeca8642d1e84afbd3f7d6820020` |
| actions/cache | v4 | `0057852bfaa89a56745cba8c7296529d2fc39830` |
| actions/upload-artifact | v4 | `ea165f8d65b6e75b540449e92b4886f43607fa02` |

验证来源：[GitHub跨生态配置](https://docs.github.com/en/code-security/how-tos/secure-your-supply-chain/secure-your-dependencies/configuring-multi-ecosystem-updates)、[target-branch规则](https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference#target-branch)、[SchemaStore配置Schema](https://github.com/SchemaStore/schemastore/blob/master/src/schemas/json/dependabot-2.0.json)。本地已解析YAML并用该Schema校验；`node --test scripts/workflow.test.mjs`覆盖固定SHA、dev目标和产品组被破坏时失败。默认分支配置已核对，实际执行及首个依赖PR仍待服务侧证据。

## 激活后的核对

PR #3只增加50行`.github/dependabot.yml`，文件SHA-256为`002d0f39cb34192bea24beae6ea8c72b9b943f5e940b196b0c50e9888826ccfa`，与已验证的dev配置相同。本批workflow策略回归再次通过；默认main尚无产品CI，此配置PR不编译产品，不能把没有检查记为CI通过。

配置合入后的核对：

1. 确认默认分支仍为main，默认分支配置与dev一致。
2. 在仓库的[Dependabot更新页面](https://github.com/ZJing1125/LeaguePingo/network/updates)确认Go、产品npm、preview npm及Actions四个目录/生态条目的任务实际运行；配置进入默认分支是前置条件，不保证立即生成更新PR。
3. 核对首个更新PR目标为dev；产品Go/npm共同审阅，preview与Actions独立；Actions仍固定完整提交SHA，Wails前后端版本一致。
4. 更新PR必须通过dev的portable、quality及Windows完整CI后再人工审阅；不自动合并或发版。没有待更新版本时保留“无更新”，以任务成功记录证明已执行，不凭空补出PR。

只有配置生效且实际更新任务成功后才能登记S10服务验收；若有更新PR，再分别记录其完整CI结果。2026-11-05核验：main配置blob为af4cf235a33b6413ee5f456db00464e75c1c0688，与dev一致；API尚无Dependabot更新PR/dynamic任务，依赖图返回0个manifest。Dependabot安全更新状态API对当前集成返回403，GraphQL报告告警未启用；二者不能证明版本更新服务被禁用。官方[版本更新说明](https://docs.github.com/en/code-security/how-tos/secure-your-supply-chain/secure-your-dependencies/configure-version-updates)以配置文件启用版本更新，[依赖图说明](https://docs.github.com/en/code-security/how-tos/secure-your-supply-chain/secure-your-dependencies/enable-dependency-graph)提供仓库设置入口。当时无可调用的已连接浏览器工具，公开API也未提供本任务可用的版本更新立即执行入口；保留服务核验，不为消除待办擅开安全更新、改配置频率或伪造运行记录。当时下一次计划检查为2026-11-09上海10:00，也可在更新页面查看/执行Check for updates；这是外部执行核验，不是剩余产品代码。
