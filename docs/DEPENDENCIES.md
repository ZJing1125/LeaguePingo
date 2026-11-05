# 依赖更新与验证

S10配置已准备在dev。GitHub只从默认分支加载Dependabot配置；当前默认main尚未包含它。[激活PR #3](https://github.com/ZJing1125/LeaguePingo/pull/3)已准备，只增加与dev逐字节相同的配置，不带入两分支之间其余产品改动。PR尚未合并，不能把提交就绪记作已激活或把尚未出现的依赖PR记为已验证；不直接修改main、不改默认分支、不自动合并。

| 范围 | 配置和节奏 |
| --- | --- |
| Go `/` 与产品 npm `/frontend` | desktop跨生态组，共同PR到dev；上海时间每周一10:00 |
| npm `/frontend/preview` | 独立preview组，同一每周计划，不混入产品Vite/runtime |
| GitHub Actions `/` | 独立actions组，同一每周计划，保持完整提交SHA |

跨生态组让Wails后端与前端运行时进入同一更新审阅，不保证第三方每次同时发布兼容版本；统一verify仍校验版本一致，完整CI失败不得合入。保持受管Windows构建、绑定一致性和浏览器回归，PR不自动合并、不发布或部署。

BUG-0128安全升级通过PR进行：Go最低补丁版为1.26.10，CI三个任务都从go.mod读取，分析器同步为staticcheck v0.8.2和govulncheck v1.8.0；GOTOOLCHAIN=local不允许任务自行切换工具链。官方Go下载元数据的SHA-256已核验，本机升级前11项可达报告、升级后0项，c711426的42024419785已通过Windows完整验证/构建及隔离安装卸载。Wails前后端继续beta.29，第三方产品模块版本及go.sum不变；不是一次性升级全部依赖。

参考来源：[Go官方支持版本与校验和](https://go.dev/dl/?mode=json)、[staticcheck v0.8.2模块要求](https://proxy.golang.org/honnef.co/go/tools/@v/v0.8.2.mod)、[govulncheck v1.8.0模块要求](https://proxy.golang.org/golang.org/x/vuln/@v/v1.8.0.mod)。BUG-0129将三项检查接为强制quality任务，Windows构建依赖其成功，失败后其余独立检查仍可产出真实摘要。工具链升级与S10配置激活分别登记；Dependabot仍需配置进入默认分支才激活，未修改main或自动合并。

本批使用官方仓库API核验的Actions固定点（主版本保持）：

| Action | 版本 | 提交 |
| --- | --- | --- |
| actions/checkout | v4 | `11d5960a326750d5838078e36cf38b85af677262` |
| actions/setup-go | v5 | `40f1582b2485089dde7abd97c1529aa768e1baff` |
| actions/setup-node | v4 | `49933ea5288caeca8642d1e84afbd3f7d6820020` |
| actions/cache | v4 | `0057852bfaa89a56745cba8c7296529d2fc39830` |
| actions/upload-artifact | v4 | `ea165f8d65b6e75b540449e92b4886f43607fa02` |

验证来源：[GitHub跨生态配置](https://docs.github.com/en/code-security/how-tos/secure-your-supply-chain/secure-your-dependencies/configuring-multi-ecosystem-updates)、[target-branch规则](https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference#target-branch)、[SchemaStore配置Schema](https://github.com/SchemaStore/schemastore/blob/master/src/schemas/json/dependabot-2.0.json)。本地已解析YAML并用该Schema校验；`node --test scripts/workflow.test.mjs`覆盖固定SHA、dev目标和产品组被破坏时失败。实际激活与生成PR仍待默认分支配置生效后核对。

## 激活后的核对

PR #3只增加50行`.github/dependabot.yml`，文件SHA-256为`002d0f39cb34192bea24beae6ea8c72b9b943f5e940b196b0c50e9888826ccfa`，与已验证的dev配置相同。本批workflow策略回归再次通过；默认main尚无产品CI，此配置PR不编译产品，不能把没有检查记为CI通过。

合并后依次核对：

1. 确认默认分支仍为main，默认分支配置与dev一致。
2. 在仓库的[Dependabot更新页面](https://github.com/ZJing1125/LeaguePingo/network/updates)确认Go、产品npm、preview npm及Actions四个目录/生态条目的任务实际运行；配置进入默认分支是前置条件，不保证立即生成更新PR。
3. 核对首个更新PR目标为dev；产品Go/npm共同审阅，preview与Actions独立；Actions仍固定完整提交SHA，Wails前后端版本一致。
4. 更新PR必须通过dev的portable、quality及Windows完整CI后再人工审阅；不自动合并或发版。没有待更新版本时保留“无更新”，以任务成功记录证明已执行，不凭空补出PR。

只有默认分支配置生效且实际更新任务成功后才能登记S10激活；若有更新PR，再分别记录其完整CI结果。当前状态为激活PR待审阅，后续实际任务未核验。
