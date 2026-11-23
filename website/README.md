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

默认网址为 `https://zjing1125.github.io/LeaguePingo/`，用户已购买 `lolpingo.top` 用于官网。工作流读取 GitHub Pages 当前地址，自动设置构建的 `WEBSITE_SITE` 与 `WEBSITE_BASE`；自定义域名使用根路径，未绑定时使用原项目路径。部署前再次检查源码和 Pages 地址，设置变更时重跑工作流。手动构建可使用这两个环境变量覆盖默认值。`downloads.json`与HTML在同批构建生成，无浏览器GitHub API请求和客户端令牌。

## 域名接入

GitHub 仓库 Settings → Pages → Custom domain 填写 `lolpingo.top` 并保存。通过 Actions 发布的站点以此设置绑定域名，构建文件中的 CNAME 不承担绑定作用。

在 DNSPod 为 `lolpingo.top` 添加以下记录，线路选择默认，TTL 使用 600 秒或控制台默认值。免费版的相同主机记录、记录类型和线路最多允许两条记录，因此使用两个 GitHub Pages 官方 IPv4 地址接入。[DNSPod 套餐说明](https://cloud.tencent.com/document/product/302/104713)

| 类型 | 主机记录 | 记录值 |
| --- | --- | --- |
| A | @ | 185.199.108.153 |
| A | @ | 185.199.109.153 |
| CNAME | www | zjing1125.github.io |

重跑官网工作流后核验根路径页面、资源、下载链接以及旧地址跳转。待 GitHub Pages 签发证书后开启 Enforce HTTPS，并分别测试 `lolpingo.top` 和 `www.lolpingo.top`。DNS 与证书状态以实际查询和访问结果为准。[GitHub 配置说明](https://docs.github.com/en/pages/configuring-a-custom-domain-for-your-github-pages-site/managing-a-custom-domain-for-your-github-pages-site)

官网变动触发独立website工作流，不重建桌面程序；桌面CI成功发布Release后显式调用同一website工作流。网页PR只构建验证；部署只接受dev。仅deploy任务持pages:write/id-token:write，不能写软件Release。仓库Settings → Pages → Source需为GitHub Actions；github-pages环境需允许dev部署，私有/付费仓库需满足GitHub Pages可用条件。设置访问被拒绝时不修改默认分支或绕过权限。

官网在发布时验证上海腾讯云 COS 的公开 HTTPS 对象，五个文件全部通过后生成国内下载链接。仓库 Secrets、对象路径与旧 VPS 清理命令见[下载分发](deploy/README.md)。

发布后调用网站工作流时读取最新dev页面源码；部署前核对源码仍为dev当前提交，避免旧软件CI排队完成后把较新的官网内容覆盖。无脚本时不显示无法使用的阶段切换按钮，默认演示与下载始终可用。

## 搜索收录

`robots.txt` 和 `sitemap.xml` 在构建时使用与首页 canonical 相同的站点地址与路径生成，随官网一起部署。前者允许公开页面抓取并指向站点地图；后者目前只列出首页，不包含下载清单、安装包或页面锚点。

正式地址为 `https://lolpingo.top/robots.txt` 和 `https://lolpingo.top/sitemap.xml`。发布后检查两者返回 200，站点地图中的唯一页面为 `https://lolpingo.top/`。更换域名后重跑官网工作流，勿单独维护另一份地址。

站长平台需由网站所有者登录自己的账号完成验证；账号密码、登录凭据和推送 API token 不写入仓库。DNS 验证所需的公开 TXT/CNAME 记录可直接添加在 DNSPod，保留现有 A 与 www CNAME 记录，不替换它们。

1. [Google Search Console](https://search.google.com/search-console)：添加“网域”资源 `lolpingo.top`，按平台给出的 DNS TXT 名称和值添加记录，TTL 600，返回平台验证。通过后在“站点地图”提交 `https://lolpingo.top/sitemap.xml`。保留验证记录，以便后续持续核验。[所有权验证说明](https://support.google.com/webmasters/answer/9008080?hl=zh-Hans)
2. [必应站长工具](https://www.bing.com/webmasters/)：可从已验证的 Google Search Console 导入站点与站点地图；或手动添加 `https://lolpingo.top/`，按平台给出的 DNS 验证记录完成验证，再提交同一份站点地图。[添加与验证说明](https://www.bing.com/webmasters/help/add-and-verify-site-12184f8b)
3. [百度搜索资源平台](https://ziyuan.baidu.com/site/index)：添加 HTTPS 站点 `lolpingo.top`，按页面提供的 DNS 验证方式完成验证。验证后在平台实际开放的普通收录/站点地图入口提交首页或站点地图；若账号未开放相应入口，如实保留待处理状态，不把 DNSPod 推送开启当作已提交或已收录。[平台工具说明](https://ziyuan.baidu.com/college/articleinfo?id=3159)

验证 TXT/CNAME 的主机记录与记录值一律照抄当前账号页面，不使用示例代填。添加后可查询公共 DNS，但平台验证通过、站点地图提交成功和搜索实际收录分别记录，不能相互替代。若选择文件或 HTML 标签验证，将平台提供的公开验证文件或标签加入网站后重新部署，再回平台验证。
