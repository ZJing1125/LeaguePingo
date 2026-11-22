# 下载分发

官网继续由 GitHub Pages 托管。安装包、免安装版和三份校验或说明文件通过上海 COS 默认 HTTPS 域名分发。GitHub Release 保留同版本备用下载。

发布目录为 `https://pingo-1538719098.cos.ap-shanghai.myqcloud.com/dev/<Release 标签>/`，文件名与 Release 一致。测试目录 `dev/v0.0.0-test/` 独立保留。

1. 在 COS 为发布前缀配置匿名读取对象的权限。桶的写权限只授予发布身份。
2. 在 GitHub 仓库 Settings → Secrets and variables → Actions → Secrets 添加 `COS_SECRET_ID`、`COS_SECRET_KEY`，来自仅对该桶 `dev/*` 有 `cos:PutObject` 权限的 CAM 子账号。凭据只传给 dev 官网工作流的上传步骤。也可以手工上传原始五个附件后重跑工作流，仅使用匿名验证。
3. 官网工作流先按原流程核验 Release，再核对每个源文件的大小、SHA-256。缺失对象使用单次 PUT 和 Content-MD5 上传。对默认 HTTPS 地址使用匿名 Range 读取，将返回的大小、单段 ETag 与源文件比对；无法用 ETag 证明一致时，读取完整对象核对 SHA-256。失败可在修复权限或补齐文件后重跑。上一版官网继续提供下载。

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
