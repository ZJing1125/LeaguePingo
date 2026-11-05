# 静态分析

CI 的 `Quality checks` 在 portable 任务通过后运行三项强制检查；全部成功后才运行 Windows 验证和构建：

- `staticcheck v0.8.2`：按 Windows amd64 目标分析全部 Go 包。
- `govulncheck v1.8.0`：按 Windows amd64 目标检查可达漏洞。
- `npm audit --audit-level=high`：检查前端锁文件中的高危及严重依赖漏洞。

产品Go最低补丁版本由go.mod声明为1.26.10，三个CI任务统一读取go-version-file，分析器最低Go要求为1.26.0。GOTOOLCHAIN=local禁止静默下载另一套Go，策略检查拒绝版本回退、独立CI版本及任务内auto覆盖。三项均使用默认失败语义，不设置continue-on-error。Go检查在前端/工具准备成功且任务未取消时各自运行，npm只依赖前端准备；前一个检查失败不会阻止其他独立结果产出。always摘要保留实际outcome，任一检查失败都会让quality失败，Windows依赖该job而停止；准备失败显示跳过，不能记为扫描通过。

2026-11-05 / BUG-0128：同一份623a197业务代码的升级前扫描确认11项标准库可达漏洞；改用Go 1.26.10及上述兼容分析器后，本机Windows目标根包/internal/cmd扫描为No vulnerabilities found，staticcheck通过。Wails及其他第三方产品模块版本未升级，go.sum未变，tidy仅将Windows测试直接引用的go-ole改为直接依赖。c711426的42024419785已通过Windows完整构建、全包竞态、绑定/原生/浏览器回归及隔离安装卸载，强制门禁按PR后续提交确认，国服人工验收仍后置。

BUG-0127：每个job都有独立checkout，advisory必须自己执行锁文件安装和前端生产构建，再运行分析器。main.go嵌入frontend/dist，portable中成功构建不能替代本job的资源；缺失会让staticcheck提前编译失败。workflow回归保护前置步骤的内容、job归属与顺序，不使用空目录/占位文件规避检查。已在本机实际复现缺资源失败及构建后目标staticcheck通过，远程按修复提交核验。

2026-11-01 本地基线：`staticcheck` 的一处测试写法已修复，`npm audit` 为 0；`govulncheck` 在 Go 1.25.14 标准库中报告 11 个可达漏洞，修复版本要求 Go 1.26.9 或 1.26.10。Go 与 Wails 的兼容升级属于依赖治理任务，不能只为清空报告在本任务内改变产品工具链。该历史报告已由BUG-0128安全升级及BUG-0129强制门禁跟进；不将旧报告覆盖为新结果。

BUG-0129策略回归覆盖移除Windows依赖、job跳过/强行执行、允许失败、shell掩盖、独立检查被success条件跳过、错误目标、失去准备ID、工具安装后移和虚假摘要。PR的当前完整CI确认实际扫描及Windows构建；未专门对远程CI注入失败，不冒称该种实验已经执行。

本地复现需要 Go 1.26.10+ 工具位于 `PATH`：

```bash
npm --prefix frontend ci
npm --prefix frontend run build
GOTOOLCHAIN=local go install honnef.co/go/tools/cmd/staticcheck@v0.8.2
GOTOOLCHAIN=local go install golang.org/x/vuln/cmd/govulncheck@v1.8.0
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 GOTOOLCHAIN=local staticcheck ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 GOTOOLCHAIN=local govulncheck ./...
npm --prefix frontend audit --audit-level=high
```

全包命令适用于干净checkout。本机若在忽略目录保留旧Go证据，它们也可能被`./...`扫描；可在本机显式检查根包、`./internal/...`及`./cmd/...`并说明范围，不删除历史证据，也不缩小干净CI的全包检查。
