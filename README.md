# PalPanel

PalPanel 是一个自托管的《幻兽帕鲁》专用服务器管理面板，后端使用 Go，管理界面使用 React。它主要解决服务器安装、日常维护、备份、Mod 管理和存档查询这些事情。

`dev` 分支正在开发存档配种与 AstrBot 接入。这里的代码和开发包会持续更新；如果用于正式服务器，请优先使用 [Releases](https://github.com/uitok/palworld-panel/releases) 中的稳定版本。

<p align="center">
  <a href="https://github.com/uitok/palworld-panel/releases"><img alt="GitHub Release" src="https://img.shields.io/github/v/release/uitok/palworld-panel?display_name=tag&sort=semver"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-GPL--3.0--or--later-315b3d"></a>
  <img alt="Windows amd64" src="https://img.shields.io/badge/Windows-amd64-4b7f91">
  <img alt="Linux amd64" src="https://img.shields.io/badge/Linux-amd64-8aa17d">
</p>

## 界面

![服务器总览](docs/images/dashboard-new.png)

<p align="center">
  <img src="docs/images/save-sources-new.png" width="49%" alt="存档中心">
  <img src="docs/images/breeding-lab-new.png" width="49%" alt="配种实验室">
</p>

<p align="center">
  <img src="docs/images/breeding-mobile-new.png" width="360" alt="QQ 用户移动端配种页面">
</p>

这些截图由 [Playwright 脚本](frontend/e2e/readme-screenshots.spec.ts)直接从当前前端生成。

## 当前状态

| 模块 | 状态 | 说明 |
| --- | --- | --- |
| 服务器管理 | 可用 | 安装、启停、更新、配置、监控、日志、备份和任务管理 |
| Mod 与 PalDefender | 可用 | Workshop、Pak、UE4SS Mod，以及受权限控制的玩家管理操作 |
| 多存档解析 | `dev` 已实现 | 当前服务器存档和 ZIP 数据源；支持标准 Steam / 服务端存档 |
| PalCalc 配种 | `dev` 已实现 | 已接入求解侧车、任务队列、结果树和存档来源筛选 |
| AstrBot 插件 | 开发中 | 插件、签名接口、绑定、签到和积分逻辑已加入仓库，仍需在实际 NapCat 与 PalDefender 环境联调 |
| Xbox WGS | 不支持 | 当前解析器只处理标准 Steam / Palworld Dedicated Server 存档 |

## 主要功能

### 服务器维护

- 安装新的 Palworld Dedicated Server，或接管已有服务端目录
- 启动、停止、安全重启、保存世界、更新服务端和发送公告
- 编辑启动参数与 `PalWorldSettings.ini`
- 查看在线人数、CPU、内存、Server FPS、运行时间和日志
- 创建、恢复和下载备份，配置计划任务并查看执行记录
- 管理 Workshop、Pak、LogicMods 和 UE4SS Mod
- 安装和更新 PalDefender，执行物品发放、处罚、白名单等固定管理操作

### 存档中心

`dev` 分支把原来的单一存档索引改成了数据源。面板会保留一个“当前服务器存档”，管理员也可以导入含有 `Level.sav` 的 ZIP。

数据源可以激活、重命名和重新索引。导入过程会检查路径穿越、软链接、文件数量和解压大小，内置服务器数据源不能删除。

目前可以查询玩家、公会、基地、容器和帕鲁，并解析帕鲁的性别、IV、星级、技能、被动词条、主人与所在容器。切换数据源后，QQ 绑定按 PlayerUID 重新核对，不会按同名昵称自动迁移。

### 配种实验室

项目固定使用 [PalCalc v1.17.6](https://github.com/tylercamp/palcalc/tree/v1.17.6)，对应提交 `8b7e2f779e47fddae16ddcb973e828ba20c02b80`。PalCalc 通过独立的 .NET 9 `palcalc-bridge` 运行，面板后端只负责提交任务、保存进度和读取结果。

当前页面支持：

- 设置目标帕鲁、性别、被动词条和 IV 要求
- 从玩家、容器或自定义帕鲁中选择候选来源
- 调整最大步骤、迭代数、线程数、野生帕鲁和手术词条等参数
- 排队、暂停、恢复和取消计算任务
- 查看多个候选结果、预计时间、蛋数、概率和完整配种树
- 存档发生变化后保留旧结果，并提示结果可能已经过期

普通 QQ 会话只能使用绑定 PlayerUID 名下的帕鲁；管理员可以选择其他玩家或容器。

### AstrBot 插件

插件位于 [`astrbot_plugin_palpanel`](astrbot_plugin_palpanel)，目标环境为 AstrBot `>=4.18,<5` 和 `aiocqhttp`（NapCat / OneBot v11）。首版按一个 PalPanel、一个 QQ 群设计。

| 命令 | 用途 |
| --- | --- |
| `/bd <游戏昵称>` | 请求游戏内绑定验证码 |
| `/bdqr <验证码>` | 确认 QQ 与 PlayerUID 绑定 |
| `/qd` | 每日签到 |
| `/jf` | 查看积分与最近流水 |
| `/pz` | 生成一次性配种页面链接 |
| `/pz <目标帕鲁> [被动词条...]` | 提交快捷配种计算 |
| `/paladmin ...` | 人工绑定、解绑、冻结和积分调整 |

默认签到奖励 10 积分，成功计算消耗 1 积分。提交任务时先预留积分，失败、取消或超时会退回。

游戏内验证码需要 PalDefender 能向在线玩家私发消息。玩家离线或 PalDefender 不可用时不会自动改用昵称绑定，管理员可以使用人工绑定命令处理。

## 安装

### Linux amd64

安装最新正式版：

```bash
curl -fsSL https://raw.githubusercontent.com/uitok/palworld-panel/main/install.sh | sudo bash
```

面板默认监听 `127.0.0.1:8080`。需要从局域网访问时，可以在安装时修改监听地址：

```bash
curl -fsSL https://raw.githubusercontent.com/uitok/palworld-panel/main/install.sh | sudo bash -s -- --listen 0.0.0.0:8080
```

使用 Docker/Wine 运行游戏服务端时加上 `--docker`。不要把未配置 HTTPS 和访问控制的面板直接暴露到公网。

常用命令：

```bash
sudo /opt/palpanel/current/palpanelctl status
sudo /opt/palpanel/current/palpanelctl logs -f
sudo /opt/palpanel/current/palpanelctl restart
sudo /opt/palpanel/current/palpanelctl uninstall
```

Linux 解压包也可以不安装 systemd：

```bash
./palpanelctl init
./palpanelctl start
./palpanelctl status
```

### Windows amd64

1. 从 [GitHub Releases](https://github.com/uitok/palworld-panel/releases) 下载最新的 Windows ZIP 和 `SHA256SUMS`。
2. 校验文件后，将 ZIP 解压到固定的可写目录，例如 `D:\PalPanel`。
3. 运行 `PalPanel.exe`，然后在浏览器中注册第一个管理员。
4. 使用开服向导安装服务端，或选择已有的 `PalServer.exe` 目录。

请不要直接在 ZIP 中运行。Windows 程序目前没有 Authenticode 签名，SmartScreen 可能提示“未知发布者”。

### dev 开发包

推送到 `dev` 后，CI 会同时构建：

- `palpanel-linux-amd64-<commit SHA>`
- `palpanel-windows-amd64-<commit SHA>`

开发包可在 [`dev` 分支的 Actions 运行记录](https://github.com/uitok/palworld-panel/actions/workflows/ci.yml?query=branch%3Adev)中下载。内部版本号为 `v0.0.0-ci.<运行编号>`，Artifact 保留 7 天，不会创建正式 Release。

## 首次使用

1. 打开 `http://127.0.0.1:8080`，注册管理员。
2. 在“开服向导”中安装或接管服务端。
3. 到“存档中心”检查当前服务器数据源，或者导入 ZIP。
4. 等待索引完成后，在“世界档案”和“帕鲁仓库”核对数据。
5. 需要计算配种时，进入“配种实验室”添加目标。
6. 需要 QQ 功能时，再安装 AstrBot 插件并配置 PalDefender。

安装 PalPanel 不会自动启动游戏服务器，首次安装和启动都需要在开服向导中确认。

## AstrBot 配置

将 `astrbot_plugin_palpanel` 作为本地插件安装，在 AstrBot WebUI 中填写配置。字段定义见 [`_conf_schema.json`](astrbot_plugin_palpanel/_conf_schema.json)。

| AstrBot 配置 | PalPanel 配置 | 说明 |
| --- | --- | --- |
| `panel_url` | PalPanel 地址 | 同机部署通常使用 `http://127.0.0.1:8080` |
| `panel_public_url` | — | QQ 用户打开一次性链接时使用的公网或局域网地址 |
| `panel_id` | `PALPANEL_ASTRBOT_PANEL_ID` | 两边必须一致，默认 `palpanel` |
| `shared_secret` | `PALPANEL_ASTRBOT_SHARED_SECRET` | 两边使用同一个随机密钥 |
| `listen_host:listen_port` | `PALPANEL_ASTRBOT_PLUGIN_URL` | 默认 `127.0.0.1:8092` |
| `allowed_group_id` | — | 允许使用命令的 QQ 群 |

PalPanel 与插件之间使用 HMAC-SHA256 签名，并校验时间戳和随机数。跨主机部署时应使用 HTTPS。

## 目录说明

```text
backend/                    Go API、任务和数据服务
frontend/                   React 管理界面与 QQ 受限页面
sav-cli/                    Palworld 存档解析进程
palcalc-bridge/             PalCalc .NET 9 求解进程
third_party/palcalc/        固定版本的 PalCalc 子模块
astrbot_plugin_palpanel/    AstrBot QQ 插件
scripts/                    安装、打包和维护脚本
docs/                       OpenAPI、说明文档和截图
```

## 从源码运行

开发环境需要 Go `1.25.12`、Node.js 22、npm 和 .NET 9 SDK。克隆仓库时要初始化 PalCalc 子模块：

```bash
git clone --recurse-submodules https://github.com/uitok/palworld-panel.git
cd palworld-panel
```

常用检查命令：

```bash
cd backend
go test ./...

cd ../sav-cli
go test ./...

cd ../palcalc-bridge
dotnet build -c Release

cd ../frontend
npm ci
npm run check
npm run test:e2e
```

重新生成 README 截图：

```bash
cd frontend
npm run screenshots:readme
```

接口定义见 [`docs/openapi.yaml`](docs/openapi.yaml)。

## 安全与限制

- 面板默认只监听本机地址；远程访问请配置 HTTPS、反向代理和防火墙。
- 管理员会话与 QQ 配种会话使用不同的 Cookie 和权限检查。
- QQ 用户不能读取其他玩家的数据、管理接口或原始存档路径。
- 验证码只保存哈希，使用一次后失效，最长保留 5 分钟。
- 不要把密码、Token、API Key、HMAC 密钥或未脱敏日志提交到仓库。
- 当前不支持 Xbox WGS、多 PalPanel 租户和 QQ 用户自行上传存档。

## 交流

<p align="center">
  <img src="docs/images/2.jpg" width="320" style="max-width: 100%; height: auto;" alt="PalPanel QQ 交流群二维码">
</p>

提交问题时，请说明 PalPanel 版本、操作系统、运行方式和复现步骤，并先删除日志中的密码、Token、API Key 与公网地址。

## 许可证

PalPanel 使用 [GPL-3.0-or-later](LICENSE)。PalCalc v1.17.6 保留 MIT 许可证，其他第三方组件与素材见 [`THIRD_PARTY_LICENSES.txt`](THIRD_PARTY_LICENSES.txt)。
