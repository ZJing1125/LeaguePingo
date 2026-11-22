# 下载分发

官网继续由 GitHub Pages 托管。安装包、免安装版和三份校验或说明文件通过上海 COS 默认 HTTPS 域名分发。GitHub Release 保留同版本备用下载。

发布目录为 `https://pingo-1538719098.cos.ap-shanghai.myqcloud.com/dev/<Release 标签>/`，文件名与 Release 一致。测试目录 `dev/v0.0.0-test/` 独立保留。

1. 在 COS 为发布前缀配置匿名读取对象的权限。桶的写权限只授予发布身份。
2. 默认手动上传。将 GitHub Release 的五个原始附件放入 `dev/<Release 标签>/`，保留文件名，再重跑官网工作流。当前版本 `v0.2.0-dev.g31315fae` 的三份小文件已核验，只需补齐 `leaguepingo-setup.exe` 和 `leaguepingo.exe`。新版按对应 Release 标签创建目录，上传五个附件。
3. 可选自动上传。在 GitHub 仓库 Settings → Secrets and variables → Actions → Secrets 添加 `COS_SECRET_ID`、`COS_SECRET_KEY`，来自仅对该桶 `dev/*` 有 `cos:PutObject` 权限的 CAM 子账号，并在 Variables 添加 `COS_AUTO_UPLOAD=true`。密钥在该选项开启后传给可信 dev 上传步骤。
4. 官网工作流先按原流程核验 Release，再核对每个源文件的大小、SHA-256。自动上传模式下，缺失对象使用单次 PUT 和 Content-MD5 上传。对默认 HTTPS 地址使用匿名 Range 读取，将返回的大小、单段 ETag 与源文件比对；无法用 ETag 证明一致时，读取完整对象核对 SHA-256。失败可在修复权限或补齐文件后重跑。上一版官网继续提供下载。

上传签名由锁定的腾讯云 SDK 生成，经标准输入交给 curl。传输设置连接、总时长和低速超时，签名保留在进程内存中。日志记录文件名和阶段状态，GitHub Secrets 仅传给上传步骤。

2026-11-23 用户选择默认域名的免费上传路径，并提出可手动发布。本批默认采用手动上传与自动校验。GitHub runner 已观测到约 16 KB/s 的跨境上传速度，可选自动上传先用三份小文件验证权限，单次大文件请求允许 90 分钟，整轮最多 4 小时。已有文件重新校验后复用，失败重跑保留已完成文件，旧官网持续提供上一版下载。该观测不能代表大陆玩家的下载速度。

在 [下载清单](https://zjing1125.github.io/LeaguePingo/downloads.json) 查看实际部署版本与地址。云开发机的下载验证不能代替大陆玩家线路测速。

## 退役东京 Vultr 镜像

在该 VPS 的 Ubuntu 网页控制台执行以下命令。将 `REF` 设为包含本脚本的已核对完整提交号。

```bash
curl --fail --location --proto '=https' --proto-redir '=https' \
  "https://raw.githubusercontent.com/ZJing1125/LeaguePingo/${REF}/website/deploy/retire-vultr.sh" \
  -o /tmp/retire-leaguepingo.sh
sudo bash /tmp/retire-leaguepingo.sh
sudo bash /tmp/retire-leaguepingo.sh --apply
```

脚本根据安装标记停用更新服务和定时器、卸载专用 Nginx 配置、清除已识别的站点、Release 附件及旧脚本。遇到未知文件列出保留目录。服务器及共用 Nginx、软件包、防火墙规则保留。若 TCP 8080 仅供旧镜像使用，可在 Vultr 防火墙及主机防火墙移除对应专用放行规则。

用户已提供清理完成的控制台输出。2026-11-23 开发机复查旧 8080 镜像入口返回 HTTP 503，旧镜像已停止对外提供下载。
