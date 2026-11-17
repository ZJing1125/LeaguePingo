# LeaguePingo

面向《英雄联盟》腾讯国服的本地桌面战绩助手。当前产品目标为 **Windows amd64**，使用 Go、Wails 3 beta.29、Vue 3 和 TypeScript；仍处于开发与国服实机联调阶段。

- **玩家概览与战绩**：单双排/灵活排位段位、近期表现、常用英雄、熟练度、模式筛选、分页和双方详情。
- **对局辅助**：客户端外侧栏、加载/局内快览、当前资料与上一局复盘。按住 `~` 临时查看；默认 `Ctrl+Shift+P` 或主窗/托盘可手动切换。
- **英雄攻略**：海克斯大乱斗强化/出装，以及其他已支持模式的公共攻略；[来源与范围](docs/HERO_GUIDES.md)。组队标记只使用来源实际返回的字段，[国服准确性待验收](docs/PREMADE_GROUPS.md)。
- **可选自动化**：自动接受、跳过点赞、返回房间及排队均默认关闭，在“游戏辅助”中独立设置或一键停用；[行为边界](docs/POST_GAME.md)。

启动腾讯国服客户端并完成登录，保持助手运行即可等待连接。首次未连接时首页提供三步指引。关闭主窗后驻留托盘，退出使用托盘菜单。设置中可切换深色、浅色或跟随系统，并查看实际版本与提交号。

## 下载迭代测试版

每次 dev 更新通过完整 CI 后，自动在 [Releases](https://github.com/ZJing1125/LeaguePingo/releases) 发布 Windows 测试版。展开最新版本的 **Assets**：下载 `leaguepingo.exe` 可直接运行，或选择 `leaguepingo-setup.exe` 安装；无需在游戏电脑构建。发布附件不受 Actions 产物 14 天保留期影响。[完整下载说明](docs/CLOUD_BUILDS.md)。

## 开发与构建

先读 [AGENTS.md](AGENTS.md) 和[当前主机约定](docs/ENVIRONMENTS.md)。依赖需事先配置，使用 Bash；Windows 完整构建需要 Go 1.26.11+、Node 22+、锁定的 Wails CLI 和前端依赖。入口详见[工程结构](docs/PROJECT_STRUCTURE.md)。

```bash
# Windows amd64：完整验证、构建及本机产物审计
bash scripts/build.sh
# Linux/macOS 开发：显式通用子集，不生成 Windows 产品
bash scripts/verify.sh portable
```

交付程序只走统一构建入口；保留当前候选与最近已验收回退版，不覆盖运行文件。安装器、WebView2 与签名见[安装说明](docs/INSTALLATION.md)，版本留存见[产物管理](docs/ARTIFACTS.md)。

## 数据与验证

LCU 读取本机客户端；战绩/段位等使用已核验的腾讯当前大区 SGP，公共攻略来自 OP.GG。第三方攻略请求不包含玩家或客户端身份。不注入游戏、不读游戏内存、不模拟点击，也不自动遥测或上传诊断。

战绩页摘要可按[缓存协议](docs/ARCHITECTURE.md#缓存与数据边界)保存在本机：HMAC 索引、不保存原始身份与姓名，重启后须重新核验身份；授权、资料、独立统计和时间线仅在内存。素材政策见[第三方声明](THIRD_PARTY_NOTICES.md)。

[CI](https://github.com/ZJing1125/LeaguePingo/actions)验证自动检查和 Windows 构建；真实客户端、窗口焦点、反作弊兼容、完整赛后链条、FPS/延迟及干净机器安装仍以[人工验收清单](docs/ACCEPTANCE.md)为准。自动测试通过不等于整局体验已验收。

[架构](docs/ARCHITECTURE.md) · [测试入口](docs/TESTING.md) · [优化进度](docs/OPTIMIZATION_BACKLOG.md) · [待解决问题](docs/OPEN_ISSUES.md) · [变更索引](CHANGELOG.md) · [完整修复记录](BUGFIX_LOG.md)
