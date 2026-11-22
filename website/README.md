# LeaguePingo 下载官网

Astro 静态站，独立于桌面助手。公开介绍、页面文案和演示资源不读取玩家或LCU数据。

```bash
npm --prefix website ci
npm --prefix website run sync-release
npm --prefix website test
ASTRO_TELEMETRY_DISABLED=1 npm --prefix website run build
BROWSER_EXECUTABLE=/path/to/chromium npm --prefix website run test:browser
npm --prefix website run preview
```

`sync-release` 使用本机已配置的 `gh` 或 Actions 的 `GH_TOKEN`。只下载三个小型说明文件，并与GitHub资产摘要及构建说明中的EXE哈希交叉核验；失败保留原清单，CI停止新页面部署。正常构建不访问GitHub，默认使用最近已核验的受管清单。

默认网址为 `https://zjing1125.github.io/LeaguePingo/`。自定义域名时设置 `WEBSITE_SITE=https://你的域名`、`WEBSITE_BASE=/`。没有配置域名时不生成CNAME。`downloads.json`与HTML在同批构建生成，无浏览器GitHub API请求和客户端令牌。

官网变动触发独立website工作流，不重建桌面程序；桌面CI成功发布Release后显式调用同一website工作流。网页PR只构建验证；部署只接受dev。仅deploy任务持pages:write/id-token:write，不能写软件Release。仓库Settings → Pages → Source需为GitHub Actions；github-pages环境需允许dev部署，私有/付费仓库需满足GitHub Pages可用条件。设置访问被拒绝时不修改默认分支或绕过权限。

官网在发布时验证上海腾讯云 COS 的公开 HTTPS 对象，五个文件全部通过后生成国内下载链接。仓库 Secrets、对象路径与旧 VPS 清理命令见[下载分发](deploy/README.md)。

发布后调用网站工作流时读取最新dev页面源码；部署前核对源码仍为dev当前提交，避免旧软件CI排队完成后把较新的官网内容覆盖。无脚本时不显示无法使用的阶段切换按钮，默认演示与下载始终可用。
