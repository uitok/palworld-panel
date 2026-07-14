# PalPanel

PalPanel 是一个面向 Palworld Dedicated Server 的自托管管理面板。

开一台服并不难，麻烦的是后面的日常维护：更新、备份、看日志、改配置、处理 Mod，以及在出问题时弄清楚到底是哪一层没有正常工作。PalPanel 把这些操作放进一个中文 Web 界面里，同时保留可审计的任务记录和清晰的数据目录。

当前 Linux 热修复版是 `v1.0.3-hotfix.1`；Windows 请自行编译

![PalPanel 系统总览](docs/images/system-overview.png)

## 能做什么

- 安装、启动、停止、重启和更新 Palworld Dedicated Server。
- 管理启动参数与 `PalWorldSettings.ini`，保存前会做字段校验。
- 查看在线人数、CPU、内存、FPS、端口和世界运行时间。
- 实时查看 PalServer 日志，文件和 Docker 日志都会做大小限制与轮转。
- 创建、校验、下载和恢复备份；世界重置前会自动生成保护性备份。
- 解析存档中的玩家、公会、基地、帕鲁和容器信息，包括 PlM1/Oodle 与玩家独立存档。
- 从 Workshop、GitHub Release、公网 HTTPS ZIP 或本地 ZIP 检查、安装和更新 Mod。
- 配置 OpenAI-compatible 翻译服务，在面板内翻译 Workshop 描述。
- 使用内嵌 PalDefender 1.8.1 和 `/gm` 页面查看玩家与背包、批量发放物品、发送消息以及踢出或封禁玩家。
- 使用账号密码、HttpOnly 会话和可撤销开发密钥鉴权，并记录写操作审计日志。

## 界面

<p align="center">
  <img src="docs/images/setup-guide.png" width="49%" alt="开服向导">
  <img src="docs/images/mod-management.png" width="49%" alt="Mod 管理">
</p>

<p align="center">
  <img src="docs/images/server-settings.png" width="49%" alt="服务器设置">
  <img src="docs/images/base-list.png" width="49%" alt="基地与存档索引">
</p>

## 快速开始

### Linux 一键安装

准备一台 Linux amd64 主机，执行：

```bash
curl -fsSL https://raw.githubusercontent.com/uitok/palworld-panel/main/install.sh | sudo bash
```

脚本会从 GitHub 获取最新正式版，校验 SHA-256，然后安装内嵌 Web UI 的 PalPanel 主程序和 `sav-cli`。默认监听 `127.0.0.1:8080`；安装完成会显示面板地址。首次打开网页时创建管理员账号，之后使用账号密码登录，不需要填写后端地址。

如果访问 GitHub 需要 SOCKS5 代理，可以把代理地址传给脚本（例如本机 `127.0.0.1:10808`）：

```bash
curl --proxy socks5h://127.0.0.1:10808 --noproxy '' -fsSL https://raw.githubusercontent.com/uitok/palworld-panel/main/install.sh | sudo bash -s -- --proxy socks5h://127.0.0.1:10808
```

需要从局域网或公网访问时，安装时显式指定监听地址：

```bash
curl -fsSL https://raw.githubusercontent.com/uitok/palworld-panel/main/install.sh | sudo bash -s -- --listen 0.0.0.0:8080
```

如果 Docker 已安装、Docker socket 可用且存在 `docker` 组，脚本会自动给独立的 `palpanel` 服务账号启用 Docker 访问。也可以用 `--docker` 强制启用，或用 `--no-docker` 禁用自动检测。Docker socket 基本等同于宿主机 root 权限，只应在使用 Wine Docker 模式时启用。

公网环境建议通过 HTTPS 反向代理访问，并在防火墙中只开放实际需要的面板和游戏端口。

### 手动安装

从 [v1.0.3-hotfix.1 Release](https://github.com/uitok/palworld-panel/releases/tag/v1.0.3-hotfix.1) 下载 `palpanel_v1.0.3-hotfix.1_linux_amd64.tar.gz`，然后执行：

```bash
tar -xzf palpanel_v1.0.3-hotfix.1_linux_amd64.tar.gz
cd palpanel_v1.0.3-hotfix.1_linux_amd64
sudo ./palpanelctl install --listen 127.0.0.1:8080
```

安装完成后打开输出的面板地址并注册首个管理员。注册是数据库中的原子操作，同一实例只允许一个请求完成首次注册。

面板默认不会直接暴露到局域网或公网。也可以在安装后编辑 `/etc/palpanel/palpanel.env`：

```ini
PALPANEL_LISTEN_ADDR=0.0.0.0:8080
```

然后重启面板：

```bash
sudo systemctl restart palpanel.service
```

### 便携模式

不安装 systemd 也可以直接在解压目录运行：

```bash
./palpanelctl init
./palpanelctl start
./palpanelctl status
```

便携模式把配置、数据、PID 和有界日志放在包内，适合试用或单用户环境。正式长期运行更推荐 systemd 安装。

### Windows

从 [v1.0.2 Release](https://github.com/uitok/palworld-panel/releases/tag/v1.0.2) 下载 `palpanel_v1.0.2_windows_amd64.zip`，校验 `SHA256SUMS` 后完整解压，双击 `PalPanel.exe`。Launcher 会初始化配置、启动后端与 `sav-cli`、等待健康检查并打开浏览器；首次启动直接在浏览器注册管理员。

三个 EXE 暂未使用 Authenticode 证书签名，Windows SmartScreen 可能显示“未知发布者”。不要直接在 ZIP 内运行，也不要从非 Release 页面下载二次打包文件。

## 文件放在哪里

systemd 模式使用三个互相独立的位置：

| 内容 | 路径 |
| --- | --- |
| 版本化程序 | `/opt/palpanel/<version>` |
| 当前版本链接 | `/opt/palpanel/current` |
| 面板配置 | `/etc/palpanel/palpanel.env` |
| 游戏、存档、备份和数据库 | `/var/lib/palpanel` |
| systemd 服务 | `palpanel.service`、`palpanel-sav-cli.service` |

升级只切换程序版本，不会覆盖 `/etc/palpanel` 和 `/var/lib/palpanel`。普通卸载也会保留配置与数据，只有 `uninstall --purge` 会一并删除。

## 常用命令

```bash
# 服务状态
sudo /opt/palpanel/current/palpanelctl status

# 查看或持续跟踪日志
sudo /opt/palpanel/current/palpanelctl logs
sudo /opt/palpanel/current/palpanelctl logs -f

# 重启面板与 sav-cli
sudo /opt/palpanel/current/palpanelctl restart

# 卸载程序但保留配置和数据
sudo /opt/palpanel/current/palpanelctl uninstall
```

安装面板不会自动启动 PalServer。游戏安装、首次启动和世界初始化由开服向导或面板中的服务器控制完成。

脚本或自动化调用应在“设置”页创建开发密钥。完整的 `ppk_` 密钥只显示一次，可随时撤销：

```bash
curl -H 'Authorization: Bearer ppk_...' http://127.0.0.1:8080/api/server/status
```

## Workshop 与 Steam

Workshop 搜索默认使用发布二进制中按字节 XOR 混淆的内置 Steam Web API Key；`STEAM_WEB_API_KEY` 可在运行时优先覆盖。按 ID 读取详情使用 Steam 的公开无 Key 接口，API 状态只返回 `bundled` 或 `environment` 来源，不返回 Key。混淆不是加密，公开二进制中的内置值仍可被逆向恢复。

需要注意，能搜索到 Mod 不代表 Steam 允许匿名下载其内容。是否支持 `steamcmd +login anonymous` 由具体游戏的 Workshop 分发策略决定；某些 Palworld Mod 需要拥有游戏的 Steam 账号，或者只能从作者提供的 GitHub/Nexus 发布页手动获取。

## AI 翻译

管理员可以在“设置”页填写 OpenAI-compatible 服务的 Base URL、模型名和 API Key，并先执行连通性测试。Key 单独保存为权限 `0600` 的秘密文件，不会由读取接口返回，也不会进入正常日志或审计详情；不要把真实 Key 写进仓库、截图或问题报告。

## PalDefender 与 GM

“安全防护”页可安装 PalDefender、生成面板专用 REST Token 并应用配置；随后在 `/gm` 使用 GM 工具。页面通过类型化后端代理读取玩家和六类背包容器，支持按 ItemID 或中文名搜索 2,455 项物品并显示图标，一次批量发放最多 100 项物品，还可发送玩家消息、全服广播或警报，以及踢出、封禁和解封玩家。

PalPanel 固定内嵌 `PalDefender.dll` 1.8.1，SHA-256 为 `18b9f63eea2dd407f29b77a262f9d33b1dcd4b744328892c13d5822701418d03`。安装任务从 PalDefender 官方 Release 获取 `d3d9.dll` loader，但始终优先安装这份本地内嵌、固定哈希的 DLL，不会用 Release 中的 DLL 或不明来源文件替换它。

PalDefender REST 默认使用 `127.0.0.1:17993`。只应让它监听本机回环地址或受控的可信管理网络，不要直接暴露到公网，也不要在聊天、日志、截图、反向代理头或前端代码中泄露其 Bearer Token。GM 数据读取需要 `read`；发物品、消息和处罚操作需要 `players:write`；安装、配置和创建 REST Token 需要管理员的 `security:write`。

## 安全默认值

- 首次打开网页时原子注册管理员，密码使用 bcrypt 摘要保存。
- 配置文件权限固定为 `0600`，进程环境变量优先于配置文件。
- `palpanel.env` 按数据解析，不会执行 shell、变量替换或命令替换。
- 默认启用鉴权并只监听本机回环地址。
- 会话 Cookie 使用 HttpOnly、SameSite=Lax；会话写请求还会执行同源检查。
- 发布二进制内嵌 Web UI，并固定使用同源 `/api`。
- Steam Key、Authorization 和上游完整请求不会写入正常日志。

配置示例见 [scripts/palpanel.env.example](scripts/palpanel.env.example)。常用变量包括：

| 变量 | 用途 |
| --- | --- |
| `PALPANEL_REQUIRE_AUTH` | 是否要求登录，生产环境保持 `true` |
| `PALPANEL_LISTEN_ADDR` | 面板监听地址，默认 `127.0.0.1:8080` |
| `PALPANEL_DATA_DIR` | 数据根目录 |
| `STEAM_WEB_API_KEY` | 可选的 Workshop 搜索 Key 覆盖值 |
| `PALPANEL_STEAM_API_TIMEOUT_SECONDS` | Steam API 超时，默认 15 秒 |
| `PALPANEL_MONITOR_RETENTION_DAYS` | 监控历史保留天数，默认 7；`0` 禁用历史写入 |
| `PALPANEL_AI_TRANSLATION_TIMEOUT_SECONDS` | AI 翻译超时，默认 90 秒 |
| `PALPANEL_LOG_LEVEL` | `debug`、`info`、`warn` 或 `error` |

从静态 Token 版本升级时，旧 `PANEL_TOKEN`、`PANEL_OPERATOR_TOKEN` 和 `PANEL_VIEWER_TOKEN` 会被忽略。升级保留配置、SQLite、Palworld 配置、服务器数据、存档和 Mod；升级后首次打开网页注册管理员即可。

## 项目结构

```text
frontend/   React + TypeScript 管理界面
backend/    Go API、SQLite、服务器与任务管理
sav-cli/    存档解析 sidecar
scripts/    打包、安装、冒烟和发布检查
```

浏览器只连接内嵌 Web UI 的 Go 后端。后端负责鉴权、SQLite、Docker/Wine、Palworld REST 和 Steam API；sav-cli 作为独立 sidecar 解析存档，避免把原生 Oodle 解析器耦合进主进程。

正式包会让 `sav-cli` 随面板自动启动：systemd 安装同时启用两个 unit，Linux 便携控制器和 Windows Launcher 都会先启动 sidecar 并检查健康状态。索引器读取 `Players/*.sav`，把 `InventoryInfo` 容器归属到对应玩家；单个玩家存档缺失或损坏只产生 warning，不会让整个世界索引失败。

## 从源码运行

需要 Go `1.25.12`、Node.js 22 和 npm。Linux 下构建 sav-cli 正式包还需要可用的 C/C++ 工具链。

```bash
# 后端
cd backend
go test -p=1 ./...

# 存档解析器
cd ../sav-cli
CGO_ENABLED=1 go test -p=1 ./...

# 前端
cd ../frontend
npm ci
npm run check

# Linux 正式包
cd ..
scripts/package.sh --version v1.0.3-hotfix.1 --targets linux-amd64 --clean
```

产物会写入 `dist/packages/`。正式 Release 包括 Linux tar.gz、Windows ZIP、完整项目源码、sav-cli vendored 源码、第三方许可清单、SBOM 和 SHA-256 校验文件。

## Windows 状态

仓库包含可双击运行的 `PalPanel.exe` Launcher，它负责初始化配置、启动后端与 sav-cli、等待健康检查并打开浏览器。Windows CI 使用原生 runner 和 MinGW CGO 做构建与进程清理测试。

从 `v1.0.2` 开始，GitHub Release 会发布经过 Windows 原生 runner、MinGW CGO 和 Launcher 进程清理测试的 ZIP。当前 ZIP 未签名，获取 Authenticode 证书后会在后续版本补充签名与发布者身份验证。

## 致谢与相关项目

- 感谢 [zaigie/palworld-server-tool](https://github.com/zaigie/palworld-server-tool) 为 Palworld 存档工具链调研和物品图标素材整理提供参考。
- 如果需要管理《饥荒联机版》服务器，推荐独立项目 [miracleEverywhere/dst-management-platform-api](https://github.com/miracleEverywhere/dst-management-platform-api)。

## 群交流
<p align="center">
  <img src="docs/images/2.jpg" width="320" style="max-width: 100%; height: auto;" alt="PalPanel QQ交流群二维码">
</p>

## 许可证

除 [THIRD_PARTY_LICENSES.txt](THIRD_PARTY_LICENSES.txt) 中单独标识的第三方材料外，PalPanel 的后端、前端、Windows Launcher、安装脚本和 `sav-cli` 均按 [GPL-3.0-or-later](LICENSE) 分发。完整项目源码包和包含 vendored gooz 的 `sav-cli` 源码包随 Release 提供。
