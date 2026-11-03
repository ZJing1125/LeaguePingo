# 依赖更新与验证

S10配置已准备在dev。GitHub只从默认分支加载Dependabot配置；当前默认main尚未包含它，需后续正常合入后才激活。没有因配置就绪而创建或合并main，也没有把尚未出现的依赖PR记为已验证。

| 范围 | 配置和节奏 |
| --- | --- |
| Go `/` 与产品 npm `/frontend` | desktop跨生态组，共同PR到dev；上海时间每周一10:00 |
| npm `/frontend/preview` | 独立preview组，同一每周计划，不混入产品Vite/runtime |
| GitHub Actions `/` | 独立actions组，同一每周计划，保持完整提交SHA |

跨生态组让Wails后端与前端运行时进入同一更新审阅，不保证第三方每次同时发布兼容版本；统一verify仍校验版本一致，完整CI失败不得合入。保持受管Windows构建、绑定一致性和浏览器回归，PR不自动合并、不发布或部署。

当前Go 1.25与Wails beta.29等实际版本未在本批升级，已知漏洞仍按advisory报告处理；后续工具链/模块升级需一起更新CI、分析器兼容性和版本检查。新增更新配置不会自动清除Q4的存量漏洞或开启强制分析门禁。

本批使用官方仓库API核验的Actions固定点（主版本保持）：

| Action | 版本 | 提交 |
| --- | --- | --- |
| actions/checkout | v4 | `11d5960a326750d5838078e36cf38b85af677262` |
| actions/setup-go | v5 | `40f1582b2485089dde7abd97c1529aa768e1baff` |
| actions/setup-node | v4 | `49933ea5288caeca8642d1e84afbd3f7d6820020` |
| actions/cache | v4 | `0057852bfaa89a56745cba8c7296529d2fc39830` |
| actions/upload-artifact | v4 | `ea165f8d65b6e75b540449e92b4886f43607fa02` |

验证来源：[GitHub跨生态配置](https://docs.github.com/en/code-security/how-tos/secure-your-supply-chain/secure-your-dependencies/configuring-multi-ecosystem-updates)、[target-branch规则](https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference#target-branch)、[SchemaStore配置Schema](https://github.com/SchemaStore/schemastore/blob/master/src/schemas/json/dependabot-2.0.json)。本地已解析YAML并用该Schema校验；`node --test scripts/workflow.test.mjs`覆盖固定SHA、dev目标和产品组被破坏时失败。实际激活与生成PR仍待默认分支配置生效后核对。
