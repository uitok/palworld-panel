# Workshop Mod 专用面板候选报告（2026-07-18）

## 方法与边界

本报告在 2026-07-18 查询 Palworld Workshop（AppID `1623730`）的“总订阅数”前 30。查询通过本机 `socks5h://127.0.0.1:10808` 完成；榜单会变化，排名不代表安全、服务端兼容或 PalPanel 推荐安装。

专用适配器必须同时满足：服务端可用、配置文件结构稳定、字段含义可以从作者说明或上游源码验证、保存和重启/重载语义明确。未满足这些条件的 Mod 只进入受限原始文件编辑器；客户端 Mod、无配置 Mod、弃用 Mod 不创建专用表单。

## 本轮结论

- 已进入首批专用适配器：UE4SS Experimental、PalSchema、Extended Base Range。
- PalDefender 不在 Steam Workshop 榜单中，但其配置契约稳定且已有服务端集成，因此同样提供专用面板。
- `QualityOfLife` 和 `PalSync` 保留为下一轮重点候选：前者公开了 JSON 配置，但 2026-07 仍在进行结构迁移；后者需要继续验证当前服务端配置契约和升级兼容性。
- 其余条目本轮不建立专用适配器。发现到允许的文本配置时仍可使用受限原始编辑器。

## 热门前 30 快照

| 排名 | Workshop 项目 | ID | 本轮处理 |
| ---: | --- | --- | --- |
| 1 | [UE4SS Experimental (Palworld)](https://steamcommunity.com/sharedfiles/filedetails/?id=3625223587) | `3625223587` | 首批专用适配器 |
| 2 | [PalSchema](https://steamcommunity.com/sharedfiles/filedetails/?id=3625280368) | `3625280368` | 首批专用适配器 |
| 3 | [Pal Surgery Table Unlocker](https://steamcommunity.com/sharedfiles/filedetails/?id=3625364851) | `3625364851` | 无稳定服务端配置契约，原始编辑 |
| 4 | [Extended Base Range](https://steamcommunity.com/sharedfiles/filedetails/?id=3625907101) | `3625907101` | 首批专用适配器 |
| 5 | [Creative Menu](https://steamcommunity.com/sharedfiles/filedetails/?id=3625287786) | `3625287786` | 管理/客户端交互为主，不做服务端表单 |
| 6 | [Paldar](https://steamcommunity.com/sharedfiles/filedetails/?id=3626993007) | `3626993007` | 客户端 UI 为主 |
| 7 | [Map Unlocker](https://steamcommunity.com/sharedfiles/filedetails/?id=3625982992) | `3625982992` | 客户端功能，无稳定配置契约 |
| 8 | [MapCollectablesMod (Chinese Version)](https://steamcommunity.com/sharedfiles/filedetails/?id=3626444067) | `3626444067` | 客户端地图 UI |
| 9 | [Pals Drop Dog Coins](https://steamcommunity.com/sharedfiles/filedetails/?id=3626214974) | `3626214974` | 未验证可配置字段 |
| 10 | [QualityOfLife](https://steamcommunity.com/sharedfiles/filedetails/?id=3761921027) | `3761921027` | 下一轮候选；等待 JSON 结构稳定 |
| 11 | [Smaller Plantations](https://steamcommunity.com/sharedfiles/filedetails/?id=3645372157) | `3645372157` | 作者未承诺服务端支持 |
| 12 | [Remote Fast Travel](https://steamcommunity.com/sharedfiles/filedetails/?id=3762839811) | `3762839811` | 客户端功能 |
| 13 | [Hermans Slot RNG buff](https://steamcommunity.com/sharedfiles/filedetails/?id=3625834903) | `3625834903` | 数据资产修改，无稳定配置契约 |
| 14 | [100x Palsphere and Ammo Crafting](https://steamcommunity.com/sharedfiles/filedetails/?id=3633828073) | `3633828073` | Pak 数据修改，服务端说明未验证 |
| 15 | [Pal Surgery Table Unlocker (PalSchema)](https://steamcommunity.com/sharedfiles/filedetails/?id=3761679027) | `3761679027` | 使用 PalSchema JSON，先走原始编辑 |
| 16 | [*DEPRECATED* Pal Analyzer 0.85](https://steamcommunity.com/sharedfiles/filedetails/?id=3638574108) | `3638574108` | 已弃用，拒绝专用适配器 |
| 17 | [Auto Hatch](https://steamcommunity.com/sharedfiles/filedetails/?id=3633175440) | `3633175440` | 未验证稳定服务端配置 |
| 18 | [Map Collectables Fixed 1.0](https://steamcommunity.com/sharedfiles/filedetails/?id=3764013308) | `3764013308` | 客户端地图 UI |
| 19 | [FIX-Paldar Map 小地图](https://steamcommunity.com/sharedfiles/filedetails/?id=3760099911) | `3760099911` | 客户端地图 UI |
| 20 | [Less Building Restrictions](https://steamcommunity.com/sharedfiles/filedetails/?id=3625996709) | `3625996709` | 未验证稳定配置字段 |
| 21 | [Currencies, Keys and More are Key Items](https://steamcommunity.com/sharedfiles/filedetails/?id=3654444678) | `3654444678` | 数据资产修改，无配置面板 |
| 22 | [PalSync](https://steamcommunity.com/sharedfiles/filedetails/?id=3609943545) | `3609943545` | 下一轮候选；继续验证服务端与升级契约 |
| 23 | [Pal Analyzer](https://steamcommunity.com/sharedfiles/filedetails/?id=3677771546) | `3677771546` | 客户端分析功能为主 |
| 24 | [INSTANT BREEDING RANCH](https://steamcommunity.com/sharedfiles/filedetails/?id=3625557007) | `3625557007` | 未验证稳定配置字段 |
| 25 | [Expanded Storage - 2x Vanilla](https://steamcommunity.com/sharedfiles/filedetails/?id=3668496427) | `3668496427` | 固定倍率变体，无配置面板 |
| 26 | [资源互通](https://steamcommunity.com/sharedfiles/filedetails/?id=3763145359) | `3763145359` | 新项目，等待服务端与配置验证 |
| 27 | [Accessory Condenser Workbench](https://steamcommunity.com/sharedfiles/filedetails/?id=3628915309) | `3628915309` | 未验证稳定配置字段 |
| 28 | [Fast Travel From Anywhere & Uncharted Teleport](https://steamcommunity.com/sharedfiles/filedetails/?id=3704801428) | `3704801428` | 客户端功能为主 |
| 29 | [Expanded Storage - 4x Vanilla](https://steamcommunity.com/sharedfiles/filedetails/?id=3750791224) | `3750791224` | 固定倍率变体，无配置面板 |
| 30 | [Build Oil Extractor Anywhere](https://steamcommunity.com/sharedfiles/filedetails/?id=3629047276) | `3629047276` | 未验证稳定配置字段 |

## 下轮复审要求

开发版本更新时重新抓取同一榜单并记录日期、排序方式和榜单变化。候选进入专用适配器前，必须保存作者配置说明或源码证据，并补充字段边界、服务端兼容、升级迁移、备份恢复和重启/在线重载测试。
