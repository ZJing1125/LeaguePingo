# 应用图标与 Windows 资源

当前正式设计为用户选定并通过视觉复核的 C 款黑绿珐琅侦察员：保留不对称折耳、黑色眼罩与象牙白眼睛。应用图标独立于界面主题配色，切换深浅主题不改色、不套旧 P 字切角。

## 唯一原稿与派生资源

- `build/appicon.png`：1254 × 1254 的定稿原始 PNG，保留原字节及透明外沿。SHA-256 为 `cb5224d8f1869795eb7966e0af798e2cd2ce7eb7979551b4b2a2ffc8ba061f3a`。
- `build/windows/icon.ico`：由固定 Wails beta.29 生成，包含 16、20、24、32、40、48、64、96、128、256 像素十帧，涵盖常见任务栏/托盘尺寸和 DPI 比例。框架按原生系统指标选择 ICO 帧，原生实际效果仍须实机核验。
- `frontend/public/brand/icon-*.png`：直接提取 ICO 中的无损 PNG 帧，与原生资源保持同一套像素；主窗口统一顶部栏与快览共用 `BrandIcon.vue`，每个窗口仅一份，提供 1×/2×/3×源，favicon 使用 32 像素版本。
- `build/brand.json`：记录原稿及全部派生资源的字节数与 SHA-256。检查不改文件；缺失、不同步或损坏直接报错。

上述文件是正式产品资源。设计工具、副本、草稿、预览截图和过程记录继续留在忽略的本地目录，不进入产品或仓库。

## 维护与常规构建

准备好固定 Wails CLI 后，从项目根目录显式更新派生资源：

```bash
node scripts/app-icons.mjs generate
# 同一维护入口也可通过 wails3 task common:generate:icons 调用
node scripts/app-icons.mjs check
```

生成过程只制作 Windows ICO 和前端 PNG，不安装依赖、不访问图片服务或生成 macOS 包。只有换图标时才执行 generate；常规构建使用已提交资源并校验完整性。

产品继续使用 `bash scripts/build.sh`。Windows 完整验证与隔离热重载都会由固定 Wails CLI 将当前 ICO、`build/windows/wails.exe.manifest` 和 `build/windows/info.json` 写入根目录 `wails_windows_amd64.syso`，再由 Go 链接进程序。该对象文件已忽略，不交付、不提交。每次重新生成，避免复用旧图标或模板图标。

`icon.go` 同时嵌入当前 ICO，供系统托盘与框架窗口回退使用；没有运行时画图或图片下载。托盘菜单、点击唤起、关闭到托盘及快览按键行为保持原有实现。

## 验证边界

`scripts/app-icons.test.mjs` 已接入 verify：核验完整尺寸、ICO 各帧、前端字节一致、清单哈希和失败场景。产品浏览器回归核验深浅主题中标题栏/导航/快览图片能加载、旧裁切与主题底色未污染图标。

Linux 可以做资源检查、浏览器预览和 Windows 目标编译；不能据此宣称桌面、任务栏、托盘和多屏 DPI 已通过。Windows 实机检查见 ACCEPTANCE 的 BUG-0051 专项。普通构建无须手工拷图或另跑图标生成命令。
