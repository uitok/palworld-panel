# PalPanel

PalPanel 是一个 Palworld Dedicated Server 管理面板。常用的开服、更新、备份、日志、Mod 和存档操作都可以在浏览器里完成。

当前版本是 `v1.1.0`，提供 Linux amd64 和 Windows amd64 安装包。Windows 程序暂未签名，首次运行可能会触发 SmartScreen 提示。

## 已完成功能

### 服务端与运行环境

- 支持 Linux amd64 和 Windows amd64，提供 Linux Docker/Wine 与 Windows 原生运行方式
- 通过开服向导安装和初始化 Palworld Dedicated Server
- 安装、启动、停止、重启和更新 Palworld 服务端
- 查看在线人数、CPU、内存、FPS、端口和世界运行时间
- 提供便携运行、版本化升级、故障恢复和卸载工具

### 配置与日常运维

- 编辑启动参数和 `PalWorldSettings.ini`，保存前进行字段校验
- 实时查看 PalServer 日志，并对文件日志和 Docker 日志进行大小限制与轮转
- 创建、校验、下载和恢复备份，世界重置前自动生成保护性备份
- 管理世界存档、计划任务和可审计的后台任务记录

### 存档与世界数据

- 解析 PlM1/Oodle 世界存档和玩家独立存档
- 查看玩家、公会、基地、帕鲁和容器数据
- 对存档数据建立索引，支持在面板中查询和管理世界信息

### Mod 与扩展管理

- 搜索、安装、更新、启停、扫描和修复 Steam Workshop Mod
- 管理 Pak、LogicMods 和 UE4SS Mod
- 检测缺失、重复、禁用和手动安装的 Mod 文件
- 支持从 Workshop、GitHub Release、公网 HTTPS ZIP 和本地 ZIP 导入 Mod
- 配置 OpenAI-compatible 翻译服务，在面板中翻译 Workshop 描述

### PalDefender 与 GM 管理

- 安装和更新 PalDefender，并自动处理所需的 UE4SS 运行环境
- 通过 `/gm` 页面查看玩家和背包数据
- 支持批量发放物品、发送消息、踢出和封禁玩家
- 提供中文物品名称与图标、权限控制、审计记录和幂等保护

### 账号与安全

- 使用账号密码登录，并通过 HttpOnly 会话保持登录状态
- 创建、撤销和管理单独的开发密钥
- 默认仅监听本机地址，Linux Docker 模式下 REST、RCON 和 PalDefender REST 仅映射到回环地址
- 对写操作保留审计记录，并避免在命令参数中暴露密码和第三方 API Key

## 待开发功能

- [ ] **配种解析**：解析存档中的配种相关数据，并在面板中提供查询与展示
- [ ] **QQ 机器人**：对接 PalPanel API，提供服务器状态查询、玩家通知和常用管理操作
- [ ] **界面优化**：优化整体视觉风格、信息层级、交互反馈和移动端适配

## 截图

<p align="center">
  <img src="docs/images/setup-guide.png" width="49%" alt="开服向导">
  <img src="docs/images/mod-management.png" width="49%" alt="Mod 管理">
</p>

<p align="center">
  <img src="docs/images/server-settings.png" width="49%" alt="服务器设置">
  <img src="docs/images/base-list.png" width="49%" alt="基地与存档索引">
</p>

## Linux 安装

```bash
curl -fsSL https://raw.githubusercontent.com/uitok/palworld-panel/main/install.sh | sudo bash
```

默认监听 `127.0.0.1:8080`。安装完成后打开脚本输出的地址，注册第一个管理员账号。

局域网访问：

```bash
curl -fsSL https://raw.githubusercontent.com/uitok/palworld-panel/main/install.sh | sudo bash -s -- --listen 0.0.0.0:8080
```

通过 SOCKS5 代理下载：

```bash
curl --proxy socks5h://127.0.0.1:10808 --noproxy '' -fsSL https://raw.githubusercontent.com/uitok/palworld-panel/main/install.sh | sudo bash -s -- --proxy socks5h://127.0.0.1:10808
```

需要 Wine Docker 模式时加 `--docker`。Docker 组权限接近 root，不需要 Docker 时不要开启。

Linux 的私有 Workshop 下载使用 `/etc/palpanel/palpanel.env` 中的
`STEAM_USERNAME` 和 `STEAM_PASSWORD`。配置文件保持 `0600` 权限，修改后重启
PalPanel；密码不会写入 Docker 命令参数。REST、RCON 和 PalDefender REST 在
Docker 模式下只映射到宿主机回环地址。

## Windows 安装

从 [v1.1.0 Release](https://github.com/uitok/palworld-panel/releases/tag/v1.1.0) 下载 `palpanel_v1.1.0_windows_amd64.zip`，校验 `SHA256SUMS` 后完整解压，再运行 `PalPanel.exe`。

Launcher 会启动后端和 `sav-cli`，健康检查通过后自动打开浏览器。不要直接在 ZIP 压缩包里运行程序。

## 便携运行

Linux 解压包可以不安装 systemd，直接在目录中运行：

```bash
./palpanelctl init
./palpanelctl start
./palpanelctl status
```

## 文件位置

| 内容 | 路径 |
| --- | --- |
| 当前程序 | `/opt/palpanel/current` |
| 版本目录 | `/opt/palpanel/<version>` |
| 面板配置 | `/etc/palpanel/palpanel.env` |
| 游戏、数据库、存档和备份 | `/var/lib/palpanel` |

升级不会覆盖 `/etc/palpanel` 和 `/var/lib/palpanel`。普通卸载也会保留这两个目录。

## 常用命令

```bash
# 状态
sudo /opt/palpanel/current/palpanelctl status

# 日志
sudo /opt/palpanel/current/palpanelctl logs -f

# 重启面板和 sav-cli
sudo /opt/palpanel/current/palpanelctl restart

# 卸载程序，保留配置和数据
sudo /opt/palpanel/current/palpanelctl uninstall

# 连配置和数据一起删除
sudo /opt/palpanel/current/palpanelctl uninstall --purge
```

安装 PalPanel 不会自动启动 Palworld。游戏服务端的安装和首次启动在开服向导中完成。

## 从源码运行

需要 Go `1.25.12`、Node.js 22 和 npm。Linux 正式包还需要 C/C++ 工具链。

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
```

构建 Linux 包：

```bash
scripts/package.sh --version v1.1.0 --targets linux-amd64 --clean
```

产物位于 `dist/packages/`。

## 说明

- 默认只监听本机；需要公网访问时请配置 HTTPS 反向代理和防火墙
- 配置文件按普通 `KEY=VALUE` 数据读取，不会作为 shell 脚本执行
- 密码、开发密钥和第三方 API Key 不要写入源码或问题截图
- Windows ZIP 未做 Authenticode 签名

## 推荐

- Palworld 服务器管理：[PST](https://github.com/zaigie/palworld-server-tool)
- 《饥荒联机版》服务器管理：[DST 管理平台](https://github.com/miracleEverywhere/dst-management-platform-api)

## 交流

<p align="center">
  <img src="docs/images/2.jpg" width="320" style="max-width: 100%; height: auto;" alt="PalPanel QQ 交流群二维码">
</p>

## 许可证

PalPanel 使用 GPL-3.0-or-later。第三方组件及素材的许可证见 [THIRD_PARTY_LICENSES.txt](THIRD_PARTY_LICENSES.txt)。
