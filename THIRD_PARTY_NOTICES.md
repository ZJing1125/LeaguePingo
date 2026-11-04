# 第三方资源

S9测试夹具保存Riot官方公开Game Client根证书PEM内容（Git归一化行尾，DER内容固定），来源`https://static.developer.riotgames.com/docs/lol/riotgames.pem`，获取日期2026-11-04；DER SHA-256及使用边界见`docs/LCU_TLS.md`。只由测试程序嵌入作候选核验，产品未嵌入/安装它，不表示Riot或腾讯已保证LCU兼容，也不替代Q8整体授权审阅。

BUG-0059 随包增加 Riot Data Dragon 16.20.1 的完整符文树元数据、67张原始符文/符文系PNG与召唤师峡谷map11 PNG，共68张/2,056,567字节。来源为官方 /cdn/{version}/data/zh_CN/runesReforged.json、/cdn/img/{icon}及/cdn/{version}/img/map/map11.png；逐项URL、字节数与SHA-256见 frontend/src/assets/game/analysis/manifest.json。显式维护 scripts/sync-analysis-assets.py；不在运行或构建时下载。这些仍是Riot游戏资源，不作为助手原创图标授权。OP.GG图片仅参考信息组织，没有复制竞品素材、评分或源码。


BUG-0050 新增 173 张 Riot Data Dragon 经典横版原画及 10 个 Riot 官方段位 PNG，共 34,380,468 字节，保留原始字节。横版来源为 `https://ddragon.leagueoflegends.com/cdn/img/champion/splash/{EnglishName}_0.jpg`，段位素材来自 Riot 开发者文档链接的 `https://static.developer.riotgames.com/docs/lol/ranked-emblems-latest.zip`。来源及 SHA-256 分别见 `frontend/src/assets/game/splashes/manifest.json`、`ranks/manifest.json`；可选维护脚本 `scripts/sync-ui-assets.py`，不进入正常构建/运行。它们属于 Riot 资源，不作为 LeaguePingo 原创图标授权。

BUG-0045 随包增加 173 张 Riot Games Data Dragon 经典英雄载入原画（约 7.70 MiB），作为阵容卡和英雄背景，不冒充账号头像。来源模板为 `https://ddragon.leagueoflegends.com/cdn/img/champion/loading/{EnglishName}_0.jpg`。逐图来源、字节数、SHA-256 见 `frontend/src/assets/game/portraits/manifest.json`；显式维护脚本为 `scripts/sync-champion-portraits.mjs`。这些图片未作为本项目原创授权，仍适用下述权利及发布前复核要求。程序运行和常规构建不访问 CDN。


游戏中文名称、英雄/装备/召唤师技能图标及符文名称来自 Riot Games Data Dragon `16.20.1`，以元数据和雪碧图随应用打包，仅作游戏资料识别。来源：<https://ddragon.leagueoflegends.com/>；图标更新脚本为 `scripts/sync-game-assets.mjs`，符文名称为 `scripts/sync-rune-metadata.mjs`（输出补丁，不直接改文件）。图标完整性摘要位于静态资源目录 `catalog.json`。

LeaguePingo 是独立原型，不由 Riot Games 或腾讯赞助或认可。游戏名称、图标及相关素材权利属于其权利人。正式发布前需复核 Riot Games Legal Jibber Jabber、适用开发者政策及腾讯国服相关条款；本文件不等同于获得商业分发许可。

开源依赖版本见 `go.mod` 和 `frontend/package-lock.json`，适用各依赖自身许可证。

组件预览使用独立的 Histoire / Vue / Vite 开发工具链，版本与完整依赖树见 `frontend/preview/package-lock.json`，适用各依赖自身许可证。预览工具及合成样例不进入产品运行时。本批外部仓库仅作工程与设计参考，未复制同类助手源码；参考与采用范围见 `docs/FRONTEND_GUIDE.md`。
