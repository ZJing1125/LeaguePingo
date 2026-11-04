# S9 证书核验准备

2026-11-04完成公开来源核查和测试专用探针；**生产LCU传输仍未切换为严格证书校验**。按用户安排，国服实机步骤集中后置。本地解析/指纹检查和CI不能证明国服当前证书链可用，也不能据此把S9标完成。

## 公开来源与尚缺证据

[Riot官方文档](https://developer.riotgames.com/docs/lol#game-client-api_root-certificatessl-errors)提供[根证书](https://static.developer.riotgames.com/docs/lol/riotgames.pem)，明确说明可验证Game Client API（2999端口）。[League Client API段落](https://developer.riotgames.com/docs/lol#league-client-api)没有对第三方保证该服务兼容；不能由此推断腾讯国服随机LCU端口一定由相同根签发，也不知道其叶证书是否含127.0.0.1的有效名称。

| 公开候选证书字段 | 核查结果 |
| --- | --- |
| Subject / Issuer | LoL Game Engineering Certificate Authority，Riot Games，自签名 |
| 有效期 | 2013-12-04 至 2043-11-27（UTC） |
| DER SHA-256 | `ca8c9d325b4cdc464c6c94a585c85e91ec23d40ba5bf3ae2822b951a4a504ea3` |
| 自签名算法 | sha1WithRSAEncryption；老格式候选，不据此关闭叶证书算法/名称检查 |

公开PEM原字节只放在`internal/lcu/testdata/riot-game-client-root.pem`并由`tls_audit_test.go`嵌入测试程序，产品不嵌入、不下载、不安装到系统信任库。普通测试验证只有一个证书、指纹与有效期；这仅证明候选文件完整。

## 游戏电脑后置步骤

客户端登录稳定时，在配置好Go的游戏电脑工作副本执行一次：

```bash
LEAGUEPINGO_LIVE_TLS_TEST=1 go test ./internal/lcu -run '^TestLiveLCUTLSCandidate$' -count=1 -v
```

探针沿用真实发现和Client读取/定向WatchPhase代码，只把测试专用HTTP传输替换为候选专用RootCAs。Go标准证书链、有效期、算法与127.0.0.1名称验证全部保留；不使用系统其他根、InsecureSkipVerify、自定义放过回调或失败回退。

本机发现沿用既有实现；其后的两个网络步骤共用8秒父预算：一个阶段HTTPS GET，加一条定向阶段WSS握手/订阅后立即取消。不查询身份/战绩、不发写动作，不读游戏内存，不导出授权、证书响应正文或玩家数据。WSS的101只证明本次TLS/升级握手成功，不冒称持续事件交付已经验收。取消清理另有1秒上限。

未设置开关、未发现客户端、SKIP和NOT RUN都不是通过；HTTPS/WSS任一失败即NOT VERIFIED，不能为通过而关闭名称/证书算法校验。只填写[结果CSV](acceptance/S9-results.csv)中的版本、模式和两个固定结果；不提交客户端鉴权、完整接口响应或身份。

## 后续接入条件

待真实客户端证明候选链可用，再决定生产RootCAs接入并复验HTTP/WSS、重连和不同国服版本。如果候选失败，先在用户电脑核对实际链与可信发布来源，不能把未验证的网络叶证书当信任锚。生产切换仍需独立小批、现有LCU竞态/来源回归及Windows验证；S9当前为准备就绪，尚未满足“不再使用InsecureSkipVerify且客户端可用”的最终验收。
