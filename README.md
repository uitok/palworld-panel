# PalPanel

<p align="center">
  面向服主与玩家的《幻兽帕鲁》服务器管理、存档解析与配种计算平台
</p>

<p align="center">
  <a href="https://github.com/uitok/palworld-panel/releases"><img alt="GitHub Release" src="https://img.shields.io/github/v/release/uitok/palworld-panel?display_name=tag&sort=semver"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-GPL--3.0--or--later-315b3d"></a>
  <img alt="Windows amd64" src="https://img.shields.io/badge/Windows-amd64-4b7f91">
  <img alt="Linux amd64" src="https://img.shields.io/badge/Linux-amd64-8aa17d">
</p>

PalPanel 是一个 Go + React 构建的 Palworld Dedicated Server 管理面板。它将开服、监控、玩家管理、Mod、备份、多存档解析和 PalCalc 配种计算放在同一个客户端中，并提供 AstrBot 插件，让 QQ 群成员可以绑定游戏角色、签到获取积分并发起配种计算。

当前主分支已经包含新版暖灰、墨绿与鼠尾草绿客户端，以及完整的存档中心、配种实验室和 QQ 受限入口。Windows 与 Linux 发布包会同时管理后端、`sav-cli` 和 `palcalc-bridge`；配种侧车不可用时，服务器管理和存档浏览仍可继续使用。

## 界面预览

### 服务器总览

![PalPanel 新版服务器总览](docs/images/dashboard-new.png)

<p align="center">
  <img src="docs/images/save-sources-new.png" width="49%" alt="PalPanel 存档中心">
  <img src="docs/images/breeding-lab-new.png" width="49%" alt="PalPanel 配种实验室">
</p>

<p align="center">
  <img src="docs/images/breeding-mobile-new.png" width="360" alt="PalPanel QQ 用户移动端配种实验室">
</p>

截图由 [Playwright 用例](frontend/e2e/readme-screenshots.spec.ts)从当前 React 客户端生成，不是独立设计稿或静态演示页面。

## 已开发功能

### 服务器与运维

- 安装新 Palworld Dedicated Server，或原地接管已有 Steam / SteamCMD 服务端目录。
- 启动、停止、强制停止、安全重启、保存世界、更新服务端和广播公告。
- 编辑启动参数与 `PalWorldSettings.ini`，保留待重启状态和危险操作确认。
- 展示在线人数、CPU、内存、Server FPS、端口、运行时间、历史趋势和实时日志。
- 备份、恢复、计划任务、任务队列、告警、封禁列表、操作审计和 Debug 日志。
- 搜索、安装、更新、扫描、忽略、修复和删除 Workshop、Pak、UE4SS Mod。
- Windows Launcher 负责子进程监督、健康检查和关闭回收；Linux 使用 systemd 与 `palpanelctl` 管理服务。

### 玩家中心与 PalDefender

- 从 PalDefender 官方 GitHub Release 安装或更新 PalDefender，并自动检查 UE4SS 依赖。
- 查看在线玩家、实时背包和已解析的存档物品。
- 发放物品、科技点和科技，管理帕鲁模板与玩家帕鲁。
- 管理处罚、白名单、临时管理员和持久访问设置。
- 在实时地图中合并存档坐标和在线玩家位置；PalDefender 不可用时保留最近一次存档位置。
- 不提供任意 RCON 命令输入框，高权限操作使用固定且经过约束的命令。

### 多存档中心

- 内置“当前服务器存档”，自动跟踪 PalPanel 已接管的世界。
- 导入包含 `Level.sav` 的标准 Steam / 服务端 ZIP 存档。
- 对数据源执行重命名、激活、重建索引和删除；内置服务器源不可删除。
- ZIP 导入限制文件数量、单项路径和解压总大小，并拒绝软链接与路径穿越。
- 索引保存数据源 ID、文件指纹、解析器版本、更新时间和警告。
- 切换存档后按 PlayerUID 重新验证 QQ 绑定；角色缺失时冻结绑定，不按同名昵称迁移。

### 世界档案与帕鲁仓库

- 浏览玩家、公会、基地、容器、地图对象和帕鲁数据。
- 解析帕鲁性别、IV、星级、槽位、主动技能、装备技能、被动词条、旧主人和远征状态。
- 识别队伍、帕鲁终端、基地、观赏笼、次元存储和全局存储等位置类型。
- 玩家中心、世界档案、帕鲁仓库和配种实验室职责分离；旧 `/gm`、`/players`、`/guilds`、`/bases`、`/pals`、`/map` 地址继续兼容。
- 原始 `.sav` 内容不会作为 API 错误、终端输出或浏览器数据直接返回。

### PalCalc 配种实验室

项目固定使用 [PalCalc v1.17.6](https://github.com/tylercamp/palcalc/tree/v1.17.6)，对应提交 `8b7e2f779e47fddae16ddcb973e828ba20c02b80`。`palcalc-bridge` 是独立的 .NET 9 常驻侧车，只引用 `PalCalc.Model` 和 `PalCalc.Solver`，不依赖 WPF、GraphSharp 或 Windows 专用 SaveReader。

- 多目标队列，以及目标帕鲁、性别、必需/可选被动词条和 IV 下限。
- 按玩家、容器、自定义帕鲁来源筛选；QQ 用户只能使用绑定 PlayerUID 名下的帕鲁。
- 最大步骤、迭代数、野生帕鲁、无关词条、线程数、金币上限、手术词条和性别反转等高级参数。
- 配种时间、孵化时间、多牧场、多孵化器、预设和自定义容器。
- 异步任务排队、进度、暂停、恢复、取消、超时和结果缓存。
- 多候选路线、时间估算、概率、蛋数、步骤、野生参与数与完整配种树。
- 区分已有、复合已有、野生、配种后代和手术节点。
- 存档指纹变化后保留旧结果，同时标记过期并提示重新计算。

### AstrBot QQ 插件

仓库内置 [astrbot_plugin_palpanel](astrbot_plugin_palpanel)，面向 AstrBot `>=4.18,<5`、NapCat / OneBot v11（`aiocqhttp`）。插件使用独立 SQLite 保存账号、绑定、签到、积分流水、积分预留、一次性票据、玩家目录和审计记录。

| 命令 | 作用 |
| --- | --- |
| `/bd <游戏昵称>` / `绑定` | 精确匹配在线角色，并通过 PalDefender 私发 5 分钟验证码 |
| `/bdqr <验证码>` / `绑定确认` | 确认 QQ、PlayerUID 与一次性验证码 |
| `/qd` / `签到` | 每日签到，默认奖励 10 积分 |
| `/jf` / `积分` | 查看绑定状态和当前积分 |
| `/pz` / `配种` | 生成一次性、限时的配种实验室链接 |
| `/pz <目标帕鲁> [被动词条...]` | 消耗积分执行快捷配种，并返回最优路线摘要与网页链接 |
| `/paladmin ...` / `帕鲁管理` | 管理员人工绑定、解绑、冻结、积分调整和流水查询 |

计算提交时先预留积分，成功保存结果后结算；失败、取消或超时自动退款。管理员账号免积分，查看已有历史结果不会重复扣费。

## 运行组成

| 组件 | 技术 | 职责 |
| --- | --- | --- |
| `frontend` | React、TypeScript、React Query、Recharts | 管理员客户端和 QQ 受限配种入口 |
| `backend` | Go、Gin、SQLite | 会话、服务端运维、存档数据、任务、审计和集成 API |
| `sav-cli` | Go、CGO | 跨平台读取标准 Palworld Steam / 服务端存档 |
| `palcalc-bridge` | .NET 9、PalCalc Solver | 目录、异步配种任务、进度控制和结果树 |
| `astrbot_plugin_palpanel` | Python、aiohttp、SQLite | QQ 身份、验证码绑定、签到积分和快捷配种 |

管理员浏览器使用普通 PalPanel 会话；QQ 用户使用独立的一次性票据和受限 Cookie。AstrBot 与 PalPanel 之间通过 HMAC-SHA256、时间戳、随机数和正文哈希签名，双方都会拒绝过期请求和 nonce 重放。

## 快速安装

### Linux amd64

```bash
curl -fsSL https://raw.githubusercontent.com/uitok/palworld-panel/main/install.sh | sudo bash
```

默认监听 `127.0.0.1:8080`。允许局域网访问时显式指定监听地址：

```bash
curl -fsSL https://raw.githubusercontent.com/uitok/palworld-panel/main/install.sh | sudo bash -s -- --listen 0.0.0.0:8080
```

需要 Wine Docker 模式时添加 `--docker`。重新运行安装命令会原地升级，并保留 `/etc/palpanel` 与 `/var/lib/palpanel`。安装脚本还支持 `--version`、`--proxy`、`--repo`、`--no-docker` 和显式的旧容器迁移参数，运行 `install.sh --help` 可查看完整说明。

常用命令：

```bash
sudo /opt/palpanel/current/palpanelctl status
sudo /opt/palpanel/current/palpanelctl logs -f
sudo /opt/palpanel/current/palpanelctl restart
sudo /opt/palpanel/current/palpanelctl uninstall
```

Linux 解压包也可以不安装 systemd，直接执行：

```bash
./palpanelctl init
./palpanelctl start
./palpanelctl status
```

### Windows amd64

1. 从 [GitHub Releases](https://github.com/uitok/palworld-panel/releases) 下载最新的 `windows_amd64.zip` 和 `SHA256SUMS`。
2. 校验 SHA-256 后，将 ZIP 完整解压到固定、可写目录，例如 `D:\PalPanel`。
3. 双击 `PalPanel.exe`，确认首次运行的数据目录，然后在浏览器注册第一个管理员。
4. 在“开服向导”中安装新服务端，或选择包含 `PalServer.exe` 的已有服务端目录进行原地接管。

不要直接在 ZIP 内运行。Windows 可执行文件目前未做 Authenticode 签名，SmartScreen 可能显示“未知发布者”；请只使用本项目 Release 并先核对校验和。

Launcher 会依次启动并健康检查 `sav-cli`、`palcalc-bridge` 与后端。默认配置位于 `config\palpanel.env`，数据库、日志、备份和托管数据位于 `data\`。

## 首次使用

1. 打开 `http://127.0.0.1:8080` 并注册管理员。
2. 使用“开服向导”安装或接管 Palworld Dedicated Server。
3. 在“存档中心”确认当前服务器源，或导入包含 `Level.sav` 的 ZIP。
4. 激活数据源并等待索引状态变为 `ready`。
5. 在“世界档案”和“帕鲁仓库”检查解析结果，再进入“配种实验室”创建目标。
6. 如需 QQ 功能，再部署 AstrBot 插件并配置 PalDefender 游戏内验证码通道。

PalPanel 不会在安装面板时自动启动游戏服务端。首次安装、导入和启动均由开服向导显式执行。

## AstrBot 接入

将 `astrbot_plugin_palpanel` 作为本地插件安装到 AstrBot，并在 AstrBot WebUI 中填写配置。完整字段见 [_conf_schema.json](astrbot_plugin_palpanel/_conf_schema.json)。至少需要保持以下配置一致：

| AstrBot 配置 | PalPanel 配置 | 默认值 / 说明 |
| --- | --- | --- |
| `panel_url` | PalPanel 监听地址 | 本机部署通常为 `http://127.0.0.1:8080` |
| `panel_public_url` | 群成员可访问地址 | 用于生成一次性网页链接 |
| `panel_id` | `PALPANEL_ASTRBOT_PANEL_ID` | 默认 `palpanel` |
| `shared_secret` | `PALPANEL_ASTRBOT_SHARED_SECRET` | 必须使用同一个高强度随机密钥 |
| `listen_host:listen_port` | `PALPANEL_ASTRBOT_PLUGIN_URL` | 默认 `127.0.0.1:8092` |
| `allowed_group_id` | — | 首版配置一个 QQ 群 |
| `daily_points` / `solve_cost` | — | 默认签到 `10`，成功计算 `1` |

非回环部署必须使用 HTTPS。不要把插件内部 API、PalDefender REST、HMAC 密钥或面板原始数据路径直接暴露到公网。

## 安全边界

- 管理员 Cookie 与 QQ 配种 Cookie 使用不同的认证与权限中间件。
- QQ 用户只能查看绑定 PlayerUID 的帕鲁和自己的自定义容器，不能访问其他玩家、管理 API 或原始存档路径。
- 游戏内验证码仅保存哈希、单次使用并在 5 分钟后过期；玩家离线或 PalDefender 不可用时不会自动降级绑定。
- 集成请求使用 HMAC-SHA256 签名和防重放校验；日志不记录验证码、共享密钥或原始存档正文。
- 默认只监听回环地址。需要远程访问时，请使用 HTTPS 反向代理、VPN、防火墙和强管理员密码。
- ZIP、发布包升级和维护脚本均包含路径边界检查；执行清理或覆盖操作前仍应保留独立备份。

## 当前支持范围

- 支持 Windows amd64 与 Linux amd64。
- 支持标准 Steam / Palworld Dedicated Server 存档和 ZIP 导入。
- 首版按单 PalPanel、单 QQ 群、NapCat / OneBot v11 设计。
- QQ 用户不能上传个人存档，只能使用服务器已同步且绑定到自己的角色数据。
- 暂不支持 Xbox WGS 存档和多租户 PalPanel。
- NapCat 实际群消息链路和 PalDefender 私信能力需要在目标部署环境完成最终联调。
- Linux 正式发布包需要在原生 Linux amd64 环境完成 CGO 构建。

## 从源码运行

开发环境需要 Go `1.25.12`、Node.js 22、npm 和 .NET 9 SDK。克隆时请初始化 PalCalc 子模块：

```bash
git clone --recurse-submodules https://github.com/uitok/palworld-panel.git
cd palworld-panel
```

分别验证各组件：

```bash
# Go 后端
cd backend
go test ./...

# 存档解析器
cd ../sav-cli
go test ./...

# PalCalc 侧车
cd ../palcalc-bridge
dotnet build -c Release

# React 客户端
cd ../frontend
npm ci
npm run check
```

重新生成 README 截图：

```bash
cd frontend
npm run screenshots:readme
```

截图命令会启动 Vite 和 Chromium，使用固定模拟数据覆盖 `docs/images/*-new.png`；普通 Playwright 测试默认跳过该生成用例。

## 测试与发布质量

仓库包含 Go 单元与集成测试、sav-cli 解析测试、Vitest 前端测试、Playwright 桌面/移动端测试、AstrBot 插件单元测试、PalCalc canary、OpenAPI 合约生成检查，以及 Windows/Linux 打包和维护脚本验证。

发布包包含自包含的 PalCalc 运行时，终端用户无需安装 .NET。第三方许可证、SBOM、校验和与发布内容检查由打包流程统一处理。

## 项目目录

```text
backend/                    Go API、任务、运维和数据服务
frontend/                   React 管理客户端与 QQ 受限入口
sav-cli/                    Palworld 存档解析侧车
palcalc-bridge/             PalCalc .NET 9 求解侧车
third_party/palcalc/        固定版本的 PalCalc Git 子模块
astrbot_plugin_palpanel/    AstrBot QQ 插件
scripts/                    打包、systemd、Windows 维护和验证脚本
docs/                       OpenAPI、发布说明、验证记录与截图
```

更细的接口定义见 [OpenAPI](docs/openapi.yaml)，Windows 能力与剩余实机验证项见 [Windows Capability Matrix](docs/windows-capability-matrix.md)。

## 交流

<p align="center">
  <img src="docs/images/2.jpg" width="320" style="max-width: 100%; height: auto;" alt="PalPanel QQ 交流群二维码">
</p>

## 许可证

PalPanel 使用 [GPL-3.0-or-later](LICENSE)。PalCalc v1.17.6 保留 MIT 许可证；其他第三方组件、数据和素材的许可证见 [THIRD_PARTY_LICENSES.txt](THIRD_PARTY_LICENSES.txt)。
