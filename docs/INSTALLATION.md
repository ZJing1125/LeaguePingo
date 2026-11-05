# Windows 安装、依赖、版本与签名

U3 已建立 Windows amd64 的受管 NSIS 安装链、启动前 WebView2 引导和只读版本检查。自动化验证不代表干净 Windows 10 或发布者信任已经验收；真实结果填写到 `acceptance/U3-results.csv`。

## 用户侧行为

- 安装器按当前用户安装到 `%LOCALAPPDATA%\Programs\LeaguePingo`，不要求管理员权限，创建开始菜单和桌面快捷方式，并登记卸载信息。卸载只删除程序与快捷方式，保留 `%APPDATA%\LeaguePingo` 中的设置、缓存和用户主动导出的诊断。
- 安装器与程序启动前都会检查 Microsoft Edge WebView2 Runtime。缺失时显示中文说明，并只在用户确认后打开微软固定下载地址；不静默下载、不修改系统策略。程序在创建 Wails/WebView 窗口前完成检查，因此不会因缺少运行库只留下闪退。
- 启动后异步读取固定的 `https://raw.githubusercontent.com/ZJing1125/LeaguePingo/main/update.json`。响应最多 32 KiB，只接受 schema 1、SemVer 和本仓库 GitHub Releases 的 HTTPS 地址。失败保持安静，不影响使用、不重试循环、不保存网络错误。
- 只有静态清单版本高于当前 `VERSION` 时，托盘才显示“发现新版本”；点击后打开 Releases 页面。助手不下载、替换或启动更新程序。

`update.json` 需要随正式发布提交到 `main`。发布时先修改根目录 `VERSION`，同步 Windows 资源并把 `update.json` 指向已存在的 Releases 版本；`node scripts/version.mjs check` 会拒绝仓库内版本漂移。开发版本可以带 SemVer 预发布后缀，Windows 文件版本使用对应的四段数字。

## 受管构建与 NSIS

在 Windows amd64、Git Bash 中准备 Go 1.26.10+、Node 22+、锁定的 Wails CLI、前端依赖和 NSIS 3.13.0。`node scripts/environment.mjs doctor` 只报告工具是否可发现；不会安装依赖。NSIS 不在 PATH 时可在可信本机配置中设置 `LEAGUEPINGO_MAKENSIS`。

```bash
# 构建新的受管 EXE，再生成安装器；不覆盖已有输出
bash scripts/package-windows.sh

# 或只打包当前已审计的受管 EXE
bash scripts/package-windows.sh \
  --binary bin/leaguepingo-example.exe \
  --output artifacts/installers/leaguepingo-example-installer.exe

# 同机隔离验证静默安装与卸载
bash scripts/verify-installer.sh artifacts/installers/leaguepingo-example-installer.exe
```

`wails3 task package` 和 `wails3 task windows:create:nsis:installer` 进入同一脚本。脚本只接受当前 `artifacts.json` 已审计的 EXE，输出仅位于 `artifacts/`，并拒绝覆盖。CI 固定安装 NSIS 3.13.0，执行受管构建、生成安装器，再在临时目录静默安装/卸载；不上传安装器，也不创建 Release。

MSIX、跨平台打包和 Docker 构建仍被策略拒绝。NSIS 与签名入口已同步纳入 `scripts/workflow.mjs` 和回归测试，不能用模板或直接编译绕过。

CI静默卸载检查使用本次测试独有的外部卸载副本，并按[NSIS官方参数](https://nsis.sourceforge.io/Docs/Chapter3.html#3.2.2)将未加引号的`_?=<隔离安装目录>`置于最后，直接等待实际卸载进程。只清理自建副本目录，安装目录的程序及卸载器必须由NSIS移除后才通过；不通过测试脚本删除原文件来伪造成功。正常交互卸载、自复制启动和干净Win10仍按I1/I6人工验收。

## 代码签名

签名证书及发布者身份必须由维护者提供。仓库不接收 PFX、密码、私钥、证书导出文件或密钥路径。证书应先安全安装到 Windows 证书存储，再设置其 40 位 SHA-1 thumbprint：

```bash
export LEAGUEPINGO_SIGN_CERT_SHA1=0123456789ABCDEF0123456789ABCDEF01234567
# signtool 不在 PATH 时设置可信本机路径
export LEAGUEPINGO_SIGNTOOL='/c/Program Files (x86)/Windows Kits/10/bin/.../x64/signtool.exe'
bash scripts/package-windows.sh
```

`build.sh` 在 EXE 登记哈希前签名，安装器生成后再签名；两者使用 SHA-256 和固定 HTTPS 时间戳，并立即执行 `signtool verify /pa /all`。签名脚本拒绝修改已经登记的 EXE，防止清单哈希失效。未配置证书时仍可生成明确的未签名测试安装器，不能称为正式可信发布。

## 干净 Windows 10 验收

使用未安装开发工具的 Windows 10 amd64 虚拟机或实体机，记录系统版本、WebView2 状态、程序提交、安装器 SHA-256、签名状态与实际结果：

| 编号 | 场景 | 通过标准 |
| --- | --- | --- |
| I1 | 已有 WebView2，交互安装、快捷方式启动、托盘退出、卸载 | 无开发工具也可安装运行；卸载移除程序和快捷方式 |
| I2 | 无 WebView2，分别从安装器和 EXE 启动 | 有中文说明；仅确认后打开微软页面；安装运行库并重启后可运行，无闪退 |
| I3 | 静态清单等于/低于当前版本，及无法联网 | 无更新噪音；程序其他功能可用，不循环请求 |
| I4 | 发布清单版本高于当前版本 | 托盘出现一次新版本入口，点击只打开本仓库 Releases 页面 |
| I5 | 使用实际发布证书构建 EXE 与安装器 | Windows 属性和 `signtool verify /pa /all` 显示预期发布者且时间戳有效 |
| I6 | 已有设置后覆盖安装与卸载 | 更新安装可启动；卸载保留用户配置，重新安装可继续使用 |

每项填写 PASS、FAIL、BLOCKED 或 NOT_RUN。CI 的 Windows Server runner、合成清单测试和无证书安装包不能替代 I1–I6。
