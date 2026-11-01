# 静态分析

CI 的 `Advisory static analysis` 在 portable 任务通过后运行三项检查：

- `staticcheck v0.7.0`：按 Windows amd64 目标分析全部 Go 包。
- `govulncheck v1.7.0`：按 Windows amd64 目标检查可达漏洞。
- `npm audit --audit-level=high`：检查前端锁文件中的高危及严重依赖漏洞。

工具版本固定为支持项目 Go 1.25 的版本，`GOTOOLCHAIN=local` 禁止分析器自行下载另一套 Go。三项各自使用 `continue-on-error`，所以能在同一次运行中给出全部结果；任务摘要会列出每项状态。当前阶段是建议性检查，告警可见但不阻断 Windows 构建。

2026-11-01 本地基线：`staticcheck` 的一处测试写法已修复，`npm audit` 为 0；`govulncheck` 在 Go 1.25.14 标准库中报告 11 个可达漏洞，修复版本要求 Go 1.26.9 或 1.26.10。Go 与 Wails 的兼容升级属于依赖治理任务，不能只为清空报告在本任务内改变产品工具链。升级并复跑清零后，应移除三项的 `continue-on-error`，使其成为强制检查。

本地复现需要 Go 工具位于 `PATH`：

```bash
GOTOOLCHAIN=local go install honnef.co/go/tools/cmd/staticcheck@v0.7.0
GOTOOLCHAIN=local go install golang.org/x/vuln/cmd/govulncheck@v1.7.0
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 GOTOOLCHAIN=local staticcheck ./...
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 GOTOOLCHAIN=local govulncheck ./...
npm --prefix frontend audit --audit-level=high
```
