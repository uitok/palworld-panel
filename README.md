# PalPanel

<p align="center">
  <strong>面向 Palworld Dedicated Server 的跨平台自托管管理面板</strong>
</p>

<p align="center">
  将开服、更新、配置、监控、备份、存档解析、Mod 管理和 GM 操作集中到一个中文 Web 界面中。
</p>

<p align="center">
  <a href="https://github.com/uitok/palworld-panel/releases/latest">最新版本</a>
  ·
  <a href="#快速开始">快速开始</a>
  ·
  <a href="#已完成功能">功能列表</a>
  ·
  <a href="#待开发功能">开发计划</a>
  ·
  <a href="#交流">QQ 交流群</a>
</p>

> 当前稳定版本为 `v1.1.0`，提供 Linux amd64 和 Windows amd64 安装包。Windows 程序暂未进行 Authenticode 签名，首次运行时可能触发 SmartScreen 提示。

## 项目简介

部署一台 Palworld 服务器并不困难，真正麻烦的是后续维护：更新服务端、修改配置、检查运行状态、管理备份、处理 Mod、查询存档，以及在出现问题时确认故障发生在哪一层。

PalPanel 将这些操作整合到浏览器中，并提供清晰的数据目录、异步任务记录、权限控制和审计日志，适合个人服、朋友服以及需要长期维护的 Palworld Dedicated Server。

## 界面预览

<p align="center">
  <img src="docs/images/setup-guide.png" width="49%" alt="PalPanel 开服向导">
  <img src="docs/images/mod-management.png" width="49%" alt="PalPanel Mod 管理">
</p>

<p align="center">
  <img src="docs/images/server-settings.png" width="49%" alt="PalPanel 服务器设置">
  <img src="docs/images/base-list.png" width="49%" alt="PalPanel 基地与存档索引">
</p>

## 已完成功能

### 服务端安装与生命周期

- 支持 Linux amd64 和 Windows amd64
- 支持 Linux Docker/Wine 运行方式与 Windows 原生运行方式
- 通过开服向导检查运行环境并安装 Palworld Dedicated Server
- 启动、停止、强制停止、重启和安全重启服务器
- 检查服务端版本，并按需执行更新
- 保存世界、发送服务器公告和计划关服
- 查看服务端端口、版本、运行时间和当前运行状态
- 提供便携运行、版本化升级、故障恢复和卸载工具

### 运行监控与任务管理

- 查看在线人数、CPU、内存、FPS、端口和世界运行时间
- 查看实时监控快照与历史监控数据
- 实时查看 PalServer 日志
- 对文件日志和 Docker 日志进行大小限制与轮转
- 查看异步后台任务的执行状态和结果
- 查看系统告警并进行确认处理
- 创建、编辑、删除和手动执行计划任务

### 配置管理

- 编辑服务器启动参数
- 编辑 `PalWorldSettings.ini`
- 根据配置 Schema 展示和校验字段
- 保存配置前进行格式和字段验证
- 初始化服务器配置并管理运行目录
- 配置 Docker 镜像源和运行模式

### 备份与世界管理

- 手动创建世界备份
- 查看、下载、校验、恢复和删除备份
- 世界重置前自动创建保护性备份
- 管理当前世界和世界存档目录
- 在面板中执行世界重置操作
- 升级和普通卸载时保留配置、数据库、存档和备份数据

### 存档解析与数据查询

- 解析 PlM1/Oodle 世界存档
- 解析玩家独立存档
- 查看玩家、公会、基地、帕鲁和容器数据
- 对存档数据建立索引
- 在面板中查询和浏览世界信息
- 将存档解析器作为独立 `sav-cli` 进程运行，避免阻塞主服务

### Mod 管理

- 搜索和查看 Steam Workshop Mod
- 下载、安装、更新、启用、禁用和删除 Workshop Mod
- 管理 Pak、LogicMods 和 UE4SS Mod
- 扫描服务器本地 Mod 文件
- 检测缺失、重复、禁用和手动安装的 Mod
- 对扫描结果执行修复、忽略和状态调整操作
- 支持从 Steam Workshop、GitHub Release、公网 HTTPS ZIP 和本地 ZIP 导入 Mod
- 在安装前检查压缩包内容并选择正确的 Mod 候选项
- 支持私有 Workshop 下载和 SteamCMD 登录状态检查

### AI 翻译

- 配置 OpenAI-compatible 翻译服务
- 测试模型接口、代理和连接状态
- 在面板中翻译 Workshop 标题与描述
- 保存配置时隐藏第三方 API Key，不向前端返回密钥原文

### PalDefender 与 GM 管理

- 查看 PalDefender 可用版本和当前运行状态
- 安装、更新和回滚 PalDefender
- 自动检查并安装所需的 UE4SS 运行环境
- 管理 PalDefender 配置、预设和 REST Token
- 在 `/gm` 页面查看在线玩家和玩家背包
- 搜索带有中文名称和图标的物品数据
- 向玩家批量发放物品
- 发送消息、踢出和封禁玩家
- 为 GM 写操作提供权限校验、审计记录和幂等保护

### 账号、API 与安全

- 首次打开面板时注册管理员账号
- 使用账号密码登录
- 使用 HttpOnly Cookie 保持登录会话
- 创建、查看和撤销开发密钥
- 提供稳定的 OpenAPI HTTP 接口
- 对服务器控制、配置、备份、Mod、审计和安全操作进行权限校验
- 记录重要写操作和 GM 操作的审计日志
- 默认仅监听 `127.0.0.1`
- Linux Docker 模式下，Palworld REST、RCON 和 PalDefender REST 默认仅映射到宿主机回环地址
- 私有 Workshop 密码不会写入 Docker 命令参数

## 快速开始

### Linux 一键安装

```bash
curl -fsSL https://raw.githubusercontent.com/uitok/palworld-panel/main/install.sh | sudo bash
```

安装脚本会下载并校验最新正式版本，安装 PalPanel、Web UI 和 `sav-cli`，默认监听：

```text
127.0.0.1:8080
```

安装完成后，打开脚本输出的面板地址并注册第一个管理员账号。

> 安装 PalPanel 不会自动启动 Palworld。服务端安装和首次启动需要在面板的开服向导中完成。

### 局域网访问

需要从局域网访问时，可以在安装时显式修改监听地址：

```bash
curl -fsSL https://raw.githubusercontent.com/uitok/palworld-panel/main/install.sh | sudo bash -s -- --listen 0.0.0.0:8080
```

不要直接将未配置 HTTPS 和访问控制的面板暴露到公网。

### 通过 SOCKS5 代理安装

```bash
curl --proxy socks5h://127.0.0.1:10808 --noproxy '' -fsSL https://raw.githubusercontent.com/uitok/palworld-panel/main/install.sh | sudo bash -s -- --proxy socks5h://127.0.0.1:10808
```

### Linux Docker/Wine 模式

需要通过 Docker/Wine 运行 Palworld 服务端时，在安装命令后加入 `--docker`：

```bash
curl -fsSL https://raw.githubusercontent.com/uitok/palworld-panel/main/install.sh | sudo bash -s -- --docker
```

Docker 组权限接近 root。不需要 Docker/Wine 模式时，请不要开启该选项。

私有 Workshop 下载需要在 `/etc/palpanel/palpanel.env` 中配置：

```env
STEAM_USERNAME=your_steam_username
STEAM_PASSWORD=your_steam_password
```

配置文件应保持 `0600` 权限。修改后重启 PalPanel，密码不会写入 Docker 命令参数。

### Windows 安装

1. 打开 [PalPanel v1.1.0 Release](https://github.com/uitok/palworld-panel/releases/tag/v1.1.0)。
2. 下载 `palpanel_v1.1.0_windows_amd64.zip` 和 `SHA256SUMS`。
3. 校验文件后，将 ZIP 完整解压到独立目录。
4. 运行 `PalPanel.exe`。
5. Launcher 会启动后端和 `sav-cli`，健康检查通过后自动打开浏览器。
6. 在浏览器中注册第一个管理员账号。

不要直接在 ZIP 压缩包中运行程序。由于程序暂未签名，SmartScreen 可能显示“未知发布者”。

## 便携运行

Linux 解压包可以不安装 systemd，直接在当前目录初始化并运行：

```bash
./palpanelctl init
./palpanelctl start
./palpanelctl status
```

## 文件位置

使用 Linux 一键安装或 systemd 安装时，主要目录如下：

| 内容 | 路径 |
| --- | --- |
| 当前程序 | `/opt/palpanel/current` |
| 版本目录 | `/opt/palpanel/<version>` |
| 面板配置 | `/etc/palpanel/palpanel.env` |
| 游戏、数据库、存档和备份 | `/var/lib/palpanel` |

升级不会覆盖 `/etc/palpanel` 和 `/var/lib/palpanel`。普通卸载也会保留这两个目录，只有使用 `--purge` 才会删除配置和数据。

## 常用命令

```bash
# 查看运行状态
sudo /opt/palpanel/current/palpanelctl status

# 实时查看 PalPanel 日志
sudo /opt/palpanel/current/palpanelctl logs -f

# 重启 PalPanel 和 sav-cli
sudo /opt/palpanel/current/palpanelctl restart

# 卸载程序，保留配置和数据
sudo /opt/palpanel/current/palpanelctl uninstall

# 卸载程序并删除配置和数据
sudo /opt/palpanel/current/palpanelctl uninstall --purge
```

## 技术栈

| 模块 | 技术 |
| --- | --- |
| Web 前端 | React、TypeScript、Vite、Tailwind CSS、TanStack Query、Recharts |
| 后端服务 | Go、Gin、SQLite |
| 存档解析 | Go、CGO、Oodle/PlM1 解析 |
| API 契约 | OpenAPI 3.1 |
| 测试 | Go Test、Vitest、Playwright |
| Linux 运行 | systemd 或便携模式，可选 Docker/Wine |
| Windows 运行 | 原生 Launcher、PalPanel 后端和独立 `sav-cli` |

## 从源码运行

### 环境要求

- Go `1.25.12`
- Node.js 22
- npm
- Linux 正式包需要 C/C++ 工具链

### 后端测试

```bash
cd backend
go test -p=1 ./...
```

### 存档解析器测试

```bash
cd sav-cli
CGO_ENABLED=1 go test -p=1 ./...
```

### 前端开发与检查

```bash
cd frontend
npm ci
npm run dev
```

完整检查：

```bash
npm run check
npm run test:e2e
```

### 构建 Linux 安装包

```bash
scripts/package.sh --version v1.1.0 --targets linux-amd64 --clean
```

构建产物位于：

```text
dist/packages/
```

## 安全说明

- 面板默认仅监听本机地址，需要远程访问时请配置 HTTPS 反向代理和防火墙
- 不要直接将 Palworld REST、RCON 或 PalDefender REST 端口暴露到公网
- `/etc/palpanel/palpanel.env` 按普通 `KEY=VALUE` 数据读取，不会作为 Shell 脚本执行
- 密码、开发密钥、Steam 凭据和第三方 API Key 不要提交到 Git 仓库或放入问题截图
- 下载 Release 后建议使用 `SHA256SUMS` 校验安装包
- Windows ZIP 暂未进行 Authenticode 签名
- Docker 组具有接近 root 的权限，请只在确实需要 Docker/Wine 时启用

## 待开发功能

- [ ] **配种解析**
  - 解析存档中的配种牧场和配种相关数据
  - 展示配种中的帕鲁、父母信息和孵化进度
  - 提供配种查询与结果展示

- [ ] **QQ 机器人**
  - 对接 PalPanel 开发密钥和 HTTP API
  - 查询服务器状态、在线人数和运行指标
  - 推送开关服、异常、玩家加入和离开通知
  - 提供经过权限控制的常用管理命令

- [ ] **界面优化**
  - 统一配色、间距、字体和组件视觉规范
  - 优化首页信息密度和页面层级
  - 改进加载、错误、空状态和操作反馈
  - 优化移动端和小屏设备适配

## 推荐项目

- Palworld 服务器管理：[PST](https://github.com/zaigie/palworld-server-tool)
- 《饥荒联机版》服务器管理：[DST 管理平台](https://github.com/miracleEverywhere/dst-management-platform-api)

## 交流

<p align="center">
  <img src="docs/images/2.jpg" width="320" style="max-width: 100%; height: auto;" alt="PalPanel QQ 交流群二维码">
</p>

提交问题前，请尽量提供：

- PalPanel 版本和操作系统
- 使用的运行模式，例如 Windows 原生或 Linux Docker/Wine
- 相关操作步骤
- 已隐藏密码、Token、API Key 和公网地址的日志

## 许可证

PalPanel 使用 [GPL-3.0-or-later](LICENSE) 许可证。

第三方组件、素材和对应许可证见 [THIRD_PARTY_LICENSES.txt](THIRD_PARTY_LICENSES.txt)。
