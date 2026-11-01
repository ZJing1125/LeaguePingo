# 开发主机、交付构建与游戏验收

产品当前目标为 Windows amd64；代码可以在另一台 Windows、Linux 或 macOS 主机编辑。这不意味着已提供 Linux/macOS 桌面产品，也不意味着本批已经实测这些主机。

## 三层配置

1. `AGENTS.md`：跟踪到 Git 的通用产品/开发规则，不包含用户名、盘符、个人代理或固定工具路径。
2. `AGENTS.local.md`：可选本机说明，忽略 Git。根规则要求开发助手显式阅读，不能假定客户端会自动加载该文件。
3. `.leaguepingo/environment.sh`：可选可信本机 Bash 配置，忽略 Git。`scripts/env.sh` 由 build/verify/run/dev 入口显式 source；其他命令如直接调用 Node/gh 不自动读取。配置是可执行代码，首次使用前必须检查，不能从远程仓库或其他主机盲目复制。不要放入凭据、token 或系统网络修改命令。

没有本机配置时，使用现有 PATH、标准工具环境变量和工具发现。项目不自动安装依赖、不启动代理、不修改全局环境。已有用户专门指定的 shell/GitHub 入口优先遵守，本批未修改这些工具或路由。

可用配置变量：

| 变量 | 用途 |
| --- | --- |
| `PATH` | 提供 Go、Node/npm、Wails CLI、NSIS `makensis` 与 Windows SDK `signtool` 等工具位置，不假定固定盘符 |
| `GOPATH` / `GOCACHE` | 可选的本机 Go 工作/缓存位置，按主机管理，不共享验收状态 |
| `LEAGUEPINGO_BASH` | 测试子进程使用的 Bash 可执行文件；未配置时由正在执行的 Bash 发现 |
| `BROWSER_EXECUTABLE` | 当前主机已安装的 Chromium/Edge 文件；未配置时完整验证通过标准安装目录/PATH 查找，找不到明确失败，不下载 |
| `LEAGUEPINGO_GH` | 可选的本机 gh 路由入口说明；开发者需按本机约定显式调用，不由构建脚本发起 GitHub 操作 |
| `LEAGUEPINGO_MAKENSIS` | 可选的受信任 NSIS `makensis.exe` 绝对路径；未配置时从 PATH 和 Windows 标准安装目录发现 |
| `LEAGUEPINGO_SIGNTOOL` | 可选的受信任 Windows SDK `signtool.exe` 绝对路径；仅签名入口使用 |
| `LEAGUEPINGO_SIGN_CERT_SHA1` | 已安装到 Windows 证书存储的发布证书 40 位 SHA-1 thumbprint；不接受 PFX、密码或私钥路径 |

在项目目录中可显式加载后检查工具可用性：

```bash
task_root="$PWD"
source scripts/env.sh
node scripts/environment.mjs doctor
```

doctor 仅报告当前主机与必要工具存在性，不证明依赖已安装正确、测试通过或远端可访问；不输出任意环境变量或证书 thumbprint。Go 1.25+、Node 22+、锁定的 Wails CLI beta.29 及前端依赖仍需事先配置。NSIS 只在 Windows 安装器任务中必需，签名还需维护者提供证书；项目不会自动安装工具或导入证书。审批/安装权限沿用用户要求。

## 明确的验证范围

| 命令 | 执行范围 | 不代表 |
| --- | --- | --- |
| `bash scripts/verify.sh portable` | 跨主机策略与纯逻辑测试、已有绑定上的前端类型/生产构建、internal Go 测试/vet、diff 检查 | 不生成绑定、不执行 Wails 路由/Windows 进程集成、不跑完整桌面包或浏览器交互，不完成 Windows 交付 |
| `bash scripts/verify.sh` 或 `windows` | Windows amd64 完整自动验证，含绑定、Go 全包、前端及浏览器回归、Windows 流程集成 | 不代替真实账号、整局、FPS/延迟和热重载桌面验收 |
| `bash scripts/build.sh` | 在合格 Windows 主机上完整验证、编译、登记并清理其本地受管产物 | 不部署到游戏电脑，不替用户验收，不清理其他主机 |

portable 是用户/开发者显式选择的子集，不因缺少 Windows 依赖自动降级。缺少子集必需的工具仍失败。默认完整验证在不支持的主机上失败，并保留“Windows 未验证”的状态；完整构建不能通过继承 portable 环境变量绕过要求。

在 Windows 执行 portable 时，Go 仍按当前宿主编译内部包，可能包含内部包的 Windows 专用单测；这不把该子集提升为完整验证。Linux/macOS 的实际运行结果必须分别记录，本批仅在当前 Windows 测试分支选择、可配置路径与完整验证，不冒称已在远程 Linux/macOS 跑通。

## 多主机产物边界

远程开发机、Windows 构建机、游戏电脑各自独立管理本地路径、进程、清单和证据。`bin/artifacts.json` 不提交 Git，不跨机器复制来冒充验收；运行中判断只观察执行脚本的主机。不得在网络共享/挂载的其他机器产物目录运行清理，以免本机进程查询无法保护远端运行文件。

当前没有远程部署/下载/安装流程。将来需要分发时应另立批次，实现文件传输、SHA-256 核验、本机登记/切换及本机回退保护。远程构建成功只能证明远程构建结果，不能据此删除本机已验收版。构建机没有用户验收记录时只能称前一候选，不应宣称它已通过游戏验收。

`.github/workflows/ci.yml`为远程Actions入口：dev推送、指向dev/main的PR及手动触发先执行Linux portable，再执行Windows `bash scripts/build.sh bin/leaguepingo-ci.exe`。build.sh内已含完整验证，不重复调用verify；锁定CLI版本取自go.mod，使用runner自带浏览器。Windows任务还固定安装NSIS 3.13.0，生成未签名测试安装器，并只在runner临时目录验证静默安装/卸载。`docs/ci/verify.yml.example`是保留的历史草案，不作为启用入口。本机专用环境文件不能上传到CI；CI用其自身工具环境和依赖配置。

截图保留7天。EXE及其本机产物清单仅在手动触发并明确勾选upload_exe时上传，用于测试；测试安装器不上传。流程不发布release、不自动部署到游戏电脑或登记为用户已验收。远程成功只证明该提交在runner通过自动化验证、构建和隔离安装/卸载，国服客户端、游戏性能、干净Win10安装、缺WebView2交互和发布者信任仍未验证。静态分析建议任务按优化清单Q4另批接入。

2026-10-29，dd33476的[CI 40857941063](https://github.com/ZJing1125/LeaguePingo/actions/runs/40857941063)已通过Linux及Windows两级任务。此前307fa29的首轮发现Windows侧栏720px高度溢出，修复后保留原断言并通过。该记录是hosted runner自动化结果，不是游戏电脑的实机验收。PR事件尚未专门新建PR触发验证；手动入口受GitHub默认分支规则限制，当前默认main未含此工作流，需后续正常合入默认分支后才可使用，不为启用按钮擅自修改main。
