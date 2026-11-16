# 从 GitHub 下载迭代测试版

每次向 `dev` 推送，GitHub Actions 自动执行 Linux 检查、质量检查、Windows 完整验证和构建。成功后提供 Windows amd64 下载包；游戏电脑无需安装 Go、Node、Wails 或重新编译。

## 下载与运行

1. 登录 GitHub，打开 [dev 构建列表](https://github.com/ZJing1125/LeaguePingo/actions/workflows/ci.yml?query=branch%3Adev)。选择目标提交且标记为绿色成功的运行。
2. 在运行摘要点击“下载本次迭代 ZIP”，或在底部 **Artifacts** 下载 `leaguepingo-dev-windows-amd64-提交号-运行编号-次数`。`ui-screenshots` 仅是测试截图。
3. 将 ZIP 解压到新的版本目录。从托盘退出旧版助手，再双击 `leaguepingo.exe`。如需开始菜单或桌面快捷方式，可选择运行 `leaguepingo-setup.exe` 安装。
4. 现有本机偏好沿用。保留上一版可用程序；反馈问题时附 `BUILD-INFO.json` 中的提交号。Windows 需要 WebView2 Runtime，缺失时按程序提示安装。

| 包内文件 | 用途 |
| --- | --- |
| `leaguepingo.exe` | 可直接运行的助手 |
| `leaguepingo-setup.exe` | 可选 NSIS 安装包，已通过 runner 隔离安装/卸载 |
| `BUILD-INFO.json` | 版本、完整提交号、构建链接、文件大小与 SHA-256 |
| `SHA256SUMS.txt` | 两个程序及说明文件的校验值 |
| `README.txt` | 离线使用说明 |

需要核对程序时，在解压目录的 PowerShell 执行 `Get-FileHash .\leaguepingo.exe -Algorithm SHA256`，与 `SHA256SUMS.txt` 对比。

## 保留与验证范围

- 下载包保留 **14 天**，到期后可由有权限的维护者在对应运行中重新运行任务。连续推送会取消尚未完成的旧构建，以最新提交为准；失败、取消的构建不提供本次可用包。
- PR 继续执行验证，但不提供迭代下载包。手动入口如仓库可用，选择 `dev` 并勾选 `upload_exe` 才上传；日常推送无需手动触发或勾选。
- 下载包是开发测试版，尚未配置发布签名。CI 成功代表自动化检查通过，真实国服数据、窗口行为和游戏体验仍需在游戏电脑验收。
- 此流程不创建 Release、不自动安装到游戏电脑、不清理旧版、不登记用户已验收。包内不包含构建机的 `bin/artifacts.json`、测试证据或本机配置；`BUILD-INFO.json` 仅是可携带的构建说明，不能用于替代本机验收记录。

构建仍走唯一受管入口，详见[环境说明](ENVIRONMENTS.md)、[产物管理](ARTIFACTS.md)及[安装说明](INSTALLATION.md)。
