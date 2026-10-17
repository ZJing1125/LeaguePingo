# 第三方资源

游戏中文名称、英雄/装备/召唤师技能图标及符文名称来自 Riot Games Data Dragon `16.20.1`，以元数据和雪碧图随应用打包，仅作游戏资料识别。来源：<https://ddragon.leagueoflegends.com/>；图标更新脚本为 `scripts/sync-game-assets.mjs`，符文名称为 `scripts/sync-rune-metadata.mjs`（输出补丁，不直接改文件）。图标完整性摘要位于静态资源目录 `catalog.json`。

LeaguePingo 是独立原型，不由 Riot Games 或腾讯赞助或认可。游戏名称、图标及相关素材权利属于其权利人。正式发布前需复核 Riot Games Legal Jibber Jabber、适用开发者政策及腾讯国服相关条款；本文件不等同于获得商业分发许可。

开源依赖版本见 `go.mod` 和 `frontend/package-lock.json`，适用各依赖自身许可证。

组件预览使用独立的 Histoire / Vue / Vite 开发工具链，版本与完整依赖树见 `frontend/preview/package-lock.json`，适用各依赖自身许可证。预览工具及合成样例不进入产品运行时。本批外部仓库仅作工程与设计参考，未复制同类助手源码；参考与采用范围见 `docs/FRONTEND_GUIDE.md`。
