# 从 GitHub 下载迭代测试版

每次向 `dev` 推送，GitHub Actions 自动执行 Linux 检查、质量检查、Windows 完整验证和构建。全部成功后由独立发布任务在 Releases 发布 Windows amd64 预发布测试版；游戏电脑无需安装 Go、Node、Wails 或重新编译。

## 下载与运行

1. 打开 [Releases](https://github.com/ZJing1125/LeaguePingo/releases)，选择最新的 **Pre-release** 测试版并展开 **Assets**。
2. 下载 `leaguepingo.exe` 可直接运行；如需开始菜单或桌面快捷方式，下载 `leaguepingo-setup.exe` 安装。两种方式任选一种，不需要下载 Source code。
3. 从托盘退出旧版助手后启动新版。现有本机偏好沿用，保留上一版可用程序。
4. 反馈问题时附 `BUILD-INFO.json` 中的提交号。Windows 需要 WebView2 Runtime，缺失时按程序提示安装。

Actions 仍保留整包 ZIP 作为备用：[dev 构建列表](https://github.com/ZJing1125/LeaguePingo/actions/workflows/ci.yml?query=branch%3Adev) → 成功运行 → Artifacts → `leaguepingo-dev-windows-amd64-…`，登录 GitHub 后下载并解压。

| 发布附件 / 包内文件 | 用途 |
| --- | --- |
| `leaguepingo.exe` | 可直接运行的助手 |
| `leaguepingo-setup.exe` | 可选 NSIS 安装包，已通过 runner 隔离安装/卸载 |
| `BUILD-INFO.json` | 版本、完整提交号、构建链接、文件大小与 SHA-256 |
| `SHA256SUMS.txt` | 两个程序及说明文件的校验值 |
| `README.txt` | 离线使用说明 |

需要核对程序时，在解压目录的 PowerShell 执行 `Get-FileHash .\leaguepingo.exe -Algorithm SHA256`，与 `SHA256SUMS.txt` 对比。

## 保留与验证范围

- Releases 按提交生成唯一标签 `v程序版本.g提交号前8位`，只发布预发布版本，不自动设为正式 Latest；不自动删除或覆盖已发布附件。附件不受 Actions 保留期影响。
- Actions 备用下载包保留 **14 天**，到期后可由有权限的维护者在对应运行中重新运行任务。连续推送会取消尚未完成的旧构建，以最新提交为准；失败、取消的构建不提供本次可用包。
- PR 继续执行验证，不上传程序或发布 Release。手动入口如仓库可用，选择 `dev` 并勾选 `upload_exe` 才上传；日常推送无需手动触发或勾选。
- 下载包是开发测试版，尚未配置发布签名。CI 成功代表自动化检查通过，真实国服数据、窗口行为和游戏体验仍需在游戏电脑验收。
- 只有独立发布任务拥有内容写权限，复用同次 Windows 任务已验证的产物，核对提交与所有 SHA-256 后先上传草稿、再发布。上传失败保留草稿供重跑；已发布版本只读核对，不覆盖文件。
- 不自动安装到游戏电脑、不清理旧版、不登记用户已验收。包内不包含构建机的 `bin/artifacts.json`、测试证据或本机配置；`BUILD-INFO.json` 仅是可携带的构建说明，不能用于替代本机验收记录。

构建仍走唯一受管入口，详见[环境说明](ENVIRONMENTS.md)、[产物管理](ARTIFACTS.md)及[安装说明](INSTALLATION.md)。
