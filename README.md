# PalPanel / Palworld 开服管理面板

这是一个面向 Palworld Windows Dedicated Server 的本地/私有化运维面板，包含 Go 后端和 React 前端。

## 推荐运行方式：离线包

生成 Linux amd64 与 Windows amd64 离线包：

```bash
scripts/package.sh --targets linux-amd64,windows-amd64
```

产物位于 `dist/packages/`：

- `palpanel_<version>_linux_amd64.tar.gz`
- `palpanel_<version>_windows_amd64.zip`

解压后运行包内脚本：

```bash
./scripts/start.sh
```

```powershell
.\scripts\start.ps1
```

首次启动会从 `config/palpanel.env.example` 生成 `config/palpanel.env`，创建随机 `PANEL_TOKEN`，并打印访问地址。默认访问 `http://127.0.0.1:8080/dashboard`。

离线包内包含：

- `bin/palpanel[.exe]`
- `bin/sav-cli[.exe]`
- `frontend/dist/`
- `backend/deployments/wine-runner/`
- `config/palpanel.env.example`
- `scripts/start.*`
- `README.md`
- `checksums.txt`

## 本地开发

```powershell
cd frontend
npm ci
npm run build

cd ..\backend
$env:PANEL_TOKEN="replace-with-a-random-32-byte-token"
$env:PALPANEL_FRONTEND_DIST="..\frontend\dist"
go run ./cmd/palpanel
```

## 关键环境变量

- `PANEL_TOKEN`: admin token，默认必填，不能使用 `change-me`。
- `PANEL_OPERATOR_TOKEN`: operator token，可执行服务器、配置、Mod、玩家操作。
- `PANEL_VIEWER_TOKEN`: viewer token，只读。
- `PALPANEL_REQUIRE_AUTH`: 默认 `true`。只在隔离开发环境设为 `false`。
- `PALPANEL_CORS_ORIGINS`: 允许的前端来源，逗号分隔。
- `PALPANEL_FRONTEND_DIST`: 前端构建产物目录。
- `PALPANEL_MAX_UPLOAD_MB`: Mod zip 上传大小限制，默认 `256`。
- `PALPANEL_DATA_DIR`: 运行数据根目录。离线包默认是包内 `data/`。
- `PALPANEL_BACKEND_DIR`: 后端资源目录。离线包默认是包内 `backend/`。
- `PALPANEL_PALDEFENDER_REST_BASE_URL`: PalDefender REST 地址，默认 `http://127.0.0.1:17993`。
- `PALPANEL_PALDEFENDER_REST_PORT`: 写入 PalDefender RESTConfig 的端口，默认 `17993`。
- `PALPANEL_GAME_DATA_TIMEOUT_MS`: 官方 `/game-data` 代理超时，默认 `3000` 毫秒。
- `PALPANEL_GAME_DATA_MAX_MB`: 官方 `/game-data` 响应上限，默认 `16` MiB。

## 配置优先级

后端只读取进程环境变量。离线包的 `scripts/start.*` 会先加载 `config/palpanel.env`，再为未设置的运行路径补齐包内默认值：

- `PALPANEL_FRONTEND_DIST=<package>/frontend/dist`
- `PALPANEL_BACKEND_DIR=<package>/backend`
- `PALPANEL_DATA_DIR=<package>/data`
- `PALPANEL_RUNNER_DIR=<package>/backend/deployments/wine-runner`

前端生产包默认使用同源 `/api`，开发模式才默认连接 `127.0.0.1:<VITE_DEFAULT_BACKEND_PORT>`。

## 部署建议

- 推荐使用 HTTPS 反向代理暴露公网，只转发 `/api/*` 到后端，或让后端通过 `PALPANEL_FRONTEND_DIST` 托管前端。
- 不要把 `PALPANEL_REQUIRE_AUTH=false` 用于公网。
- 离线包运行数据默认都在包内 `data/`；`data/server`、`data/backups`、`data/logs`、`data/palpanel.db` 都是需要备份的运行数据。
- 项目当前只提供 Wine runner Dockerfile；完整后端 Docker/Compose 仍建议按部署环境单独编写，确保 Palworld 存档目录使用稳定宿主机卷。

## 运维功能

- 安全重启会创建后端任务：保存/通知、等待倒计时、停止、启动。
- 更新判断使用 Steam Build ID：读取本地 `appmanifest_2394010.acf`，再通过 SteamCMD 查询 public 分支最新 Build。服务器运行时另从官方 `/info` 展示语义版本，并按 `1.0.0` 兼容目标提示 Mod、PalDefender 和存档解析风险；配置规范版本不会被当作游戏版本。
- “检查后更新”会先比对 Build ID，只有发现新版本才执行备份、停服、更新和必要的重启。
- 恢复备份前会先停止服务器并创建 `pre-restore` 备份。
- 管理员可在总览中重置当前世界；任务会校验 `RESET WORLD` 与世界 ID、创建并验证 `pre-world-reset` 备份，只移走当前世界目录。新世界启动失败时不会覆盖旧数据，备份和暂存路径会写入任务错误。
- 设置页支持 OpenAI-compatible AI 翻译配置。API Key 仅保存到 `data/secrets/ai-translation.key`（`0600`），Workshop 详情按需翻译 Steam 权威描述并按原文哈希与模型缓存。
- PalServer 新进程统一启用 stdout 日志。Wine 模式同时写入 Docker 输出和 `data/logs/palserver.log`；文件与 Docker 日志均按 20 MiB、5 份轮转，总览运行时每 3 秒刷新，停服后仍保留最后日志。
- 写操作会记录审计日志，可在 `/audit` 查看。
- 玩家踢出等依赖运行时命令能力的功能，在未接入 PalDefender/RCON 后端时会返回明确 `unsupported`。
