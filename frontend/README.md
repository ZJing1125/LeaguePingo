# LeaguePingo 前端

桌面界面使用 Vue 3、TypeScript 和 Vite，通过 Wails 绑定调用 Go 服务。当前与 Windows 助手联调；macOS 适配暂缓。完整安装、构建与版本说明见 [项目 README](../README.md)。

## 目录职责

- `src/App.vue`：工作区布局与桌面事件组合；快照、玩家标签、手动操作和快览控制分别在 `src/composables/` 管理。
- `src/components/`：玩家战绩、对局详情、快览、设置与通用组件。
- `src/historyBrowser.ts`、`src/presentation.ts`：分页/样本状态和展示计算。
- `src/style.css`：桌面界面样式，使用系统字体。
- `src/assets/game/`：随程序打包的游戏元数据和图集，来源见 [第三方说明](../THIRD_PARTY_NOTICES.md)。
- `bindings/`：Wails 从 Go 生成的接口与模型，纳入版本控制，不手工修改。
- `tests/`：使用合成玩家与模拟 Wails 绑定的浏览器回归，不查询真实账号。
- `preview/`：独立 Histoire 工具链及 4 组 / 15 个合成状态变体，按其锁文件单独安装；不进入产品运行时。
- `dist/`、`node_modules/`：本机构建产物和依赖，不提交 Git。

欢迎页模板的 `public/` 背景、标志、独立样式和 Inter 字体已经移除；实际页面使用 `src/style.css`，游戏图集保留在 `src/assets/game/`。

组件拆分、键盘操作、AI 迭代规范、预览命令和外部参考见 [前端迭代指南](../docs/FRONTEND_GUIDE.md)。

## 常用命令

在项目根目录使用 Git Bash，并按项目 README 配置工具路径：

```bash
# 首次安装或锁文件更新后，按锁文件安装依赖
npm --prefix frontend ci

# Go 服务或模型变更后重新生成绑定
wails3 generate bindings -clean=true -ts -i

# TypeScript 检查与生产资源构建
npm --prefix frontend run build

# 已构建前端的离线浏览器回归，使用本机 Edge/Chromium
npm --prefix frontend run test:smoke
```

浏览器回归需要 `BROWSER_EXECUTABLE` 指向本机浏览器；它不会自动下载浏览器。`bash scripts/verify.sh` 会按顺序生成绑定、构建、执行 Go 检查和前端回归，并在本机已安装 Edge 时设置该路径。

完整桌面开发入口是根目录的 `wails3 dev`。单独运行 Vite 只有前端开发服务器，不会提供真实 Go 服务。Wails 前端运行时与 Go 依赖固定在相同 beta.29 版本，升级时需一起核对。
