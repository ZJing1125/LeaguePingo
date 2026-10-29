# 用户错误与本地诊断

U1将请求失败、输入校验和桌面操作统一为固定错误码。中文文案来自`internal/apperrors/messages.json`：Go嵌入同一文件，前端按码读取，避免两套文案漂移。提示说明当前问题与可执行的下一步。没有匹配类型时显示统一重试/导出诊断提示，不按错误字符串猜测原因，不把未知错误当成未登录。

| 条件 | 错误码 | 用户处理 |
| --- | --- | --- |
| 未发现客户端 | ClientNotRunning | 启动并登录客户端 |
| 已发现但连接信息不可读/未就绪 | ClientPermissionDenied / ClientConfigurationInvalid | 检查运行权限或等待客户端就绪 |
| 登录授权尚未就绪 | NotLoggedIn | 完成登录，必要时重新登录 |
| 会话无访问权限 | AccessDenied | 确认登录状态及大区 |
| 请求限流 / 服务繁忙 | RateLimited / ServiceBusy | 等待后重试 |
| 取消 / 超时 | RequestCanceled / RequestTimeout | 按需要重新操作；保留已有数据 |
| 数据格式不可用 / 连接已变化 | InvalidResponse / StaleData | 重试或重新查询 |
| 设置保存 / 快捷键注册失败 | SettingsWriteFailed / ShortcutUnavailable | 检查目录权限或更换快捷键，核对生效项 |

HTTP分类不改变端点、请求次数、超时、重试、回退及迟到结果隔离。LCU与SGP保留401/403/429/5xx等类型，取消与超时仍可用`errors.Is`检查。SGP授权及大区初始化不再把底层超时统一误报为没有授权。公共攻略服务的401/403不提示用户登录游戏，使用GuideUnavailable；429与5xx仍分类。模式攻略部分成功仍沿用已有部分结果提示。

Wails beta.29总会把返回错误的`Error()`拼到顶层`RuntimeError.message`；`MarshalError`只控制`cause`。因此所有AppService的error返回都有`defer apperrors.Present(&serviceErr)`，先移除未知原文及包装文本；注册的序列化器只输出固定`{code,message}`。前端只识别该封套中已登记的码，并从本地目录映射文案，绝不回显异常的message、HTML、URL或String(reason)。前端自身分页校验也通过固定码报错。异步连接/攻略/辅助窗口告警在发布前使用相同固定映射。

显式“导出脱敏诊断”新增`errors`数组，只有固定服务层、错误码、HTTP状态与累计次数。内存最多128桶，超量新组合合并到desktop/Unexpected；没有原始错误文本、响应正文、完整URL、玩家/分组标识、进程参数或凭据，不上传、不自动落盘。记录是**各层的失败观察次数**：同一请求在传输和界面边界均可观察到，不能相加当成唯一失败请求数；请求数仍以原requests分类计数为准。无原始细节可恢复，不能靠该导出复原响应内容。

验证覆盖共享目录映射、真实Wails错误封套、服务出口保护、超时/取消原因、敏感包装文本排除、诊断并发与容量上限、原请求次数、错误玩家隔离和失败保留。当前Linux执行internal逻辑/竞态、前端及portable；固定CLI按Windows目标生成绑定，Windows原生执行及完整受管构建以本提交Actions结果为准。真实国服故障场景仍需人工验收。自动退避/恢复属于下一项U2，本批不增加自动重试。
