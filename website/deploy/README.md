# Vultr / Ubuntu 下载镜像

适用于现有 Ubuntu 24.04 小内存服务器。官网在 GitHub Actions 构建；服务器只需 Python 3.11+、Nginx、curl 和 systemd，不在服务器安装 Node/Go 或编译助手。

## 首次部署

在 Vultr 网页控制台登录服务器，先安装运行依赖，再执行本批已发布提交中的安装脚本。将 `完整40位提交` 替换为维护者给出的已验证 dev 提交。命令由服务器管理员手动执行，不需要向助手提供服务器密码或私钥。

```bash
sudo apt-get update
sudo apt-get install -y --no-install-recommends nginx python3 curl
mirror_ref=完整40位提交
curl --fail --location --proto '=https' --proto-redir '=https' \
  "https://raw.githubusercontent.com/ZJing1125/LeaguePingo/$mirror_ref/website/deploy/install.sh" \
  -o /tmp/leaguepingo-install.sh
sudo bash /tmp/leaguepingo-install.sh "$mirror_ref" 8080
```

安装前确保网站工作流已部署 `mirror.json` 和 `mirror.tar.gz`。初次同步会下载当前稳定版/测试版的安装器、免安装 EXE、构建说明及校验文件，正常需要约 130MB 或更多磁盘空间。网络或校验失败会停止安装，不把半套文件发布为可下载版本。

默认测试网址为 `http://YOUR_SERVER_IP:8080/`。若配置了 Vultr Cloud Firewall，请在该实例关联的防火墙中放行 TCP 8080。脚本仅在 UFW 已启用时添加该端口，不启用或重置防火墙。已有服务使用 8080 时可把命令末尾端口改为其他未占用的 1024–65535 端口。

安装器只维护 `/srv/leaguepingo/`、`/usr/local/lib/leaguepingo/mirror.py`、专属 Nginx 配置和专属 systemd 服务/定时器。拒绝接管非空未登记目录或同名未登记配置，不更改 Nginx 默认站点、现有其他服务或全局网络参数。依赖包安装仍由管理员先执行，Nginx 安装/start 的系统影响应在当前服务器核对。

这是 **IP＋HTTP 的试用入口**，不是 HTTPS 正式入口。测试通过后再注册域名、配置 DNS 和 HTTPS；不要把 GitHub Pages 的 HTTPS 下载按钮改为 HTTP 文件链接。

## 更新与回退

网站每次成功构建都会生成根路径镜像包和对应描述文件，原 Pages 项目路径仍正常使用 GitHub 下载。服务器每约五分钟检查一次描述文件，未变化时只联网读取这一个小文件，不重复下载安装包。

同步器严格核对来源仓库、版本标签、提交、平台、五个固定附件、文件大小和 SHA-256；构建说明及校验清单交叉核验。官网包拒绝路径穿越、符号/硬链接、特殊文件和超量展开，并核对页面 `downloads.json` 与本地下载链接。全部就绪后原子切换 `current`。保留当前及前一版页面引用的程序文件，仅清理带本工具登记的旧版本，不删除未知文件。

```bash
sudo systemctl status leaguepingo-mirror.timer --no-pager
sudo journalctl -u leaguepingo-mirror.service -n 30 --no-pager
sudo systemctl start leaguepingo-mirror.service
```

首次同步日志在控制台，定时更新日志在 journal。网络失败、校验失败或磁盘不足时保持当前页面及下载文件；目录中的未知文件不参与自动清理。单条 SHA-256 只证明文件与发布清单一致，不代表程序已有发布签名或国服实机验收。

回退时先停本工具的定时器和更新任务，防止下一次同步立即切回新版：

```bash
sudo systemctl stop leaguepingo-mirror.timer leaguepingo-mirror.service
sudo runuser -u leaguepingo-mirror -- python3 /usr/local/lib/leaguepingo/mirror.py rollback
```

回退会核对上一版文件并切换入口，不重新联网取包。问题处理完成后执行 `sudo systemctl start leaguepingo-mirror.timer` 恢复检查。

## 核验范围

开发机可验证合成发布的同步、失败保留、重复执行、路径安全、磁盘不足、两版保留与回退，以及 Pages/镜像两种路径的真实浏览器行为。真实安装包可在临时目录流式下载并核对 SHA-256。

Vultr 的 Nginx/systemd 首次安装、公网端口、真实 HTTP 文件下载及大陆三网晚高峰速度，必须在该服务器和玩家网络核对。部署脚本或 CI 通过不能当作公网部署或大陆下载测速通过。
