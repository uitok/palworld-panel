# PalSphere / Palworld 开服管理面板

这是一个面向 Palworld Windows Dedicated Server 的本地/私有化运维面板，包含 Go 后端和 React 前端。

## 快速启动

```powershell
cd frontend
npm install
npm run build

cd ..\backend
$env:PANEL_TOKEN="replace-with-a-random-32-byte-token"
$env:PALPANEL_FRONTEND_DIST="..\frontend\dist"
go run ./cmd/palpanel
```

访问 `http://127.0.0.1:8080/dashboard`，输入 `PANEL_TOKEN`。

## 关键环境变量

- `PANEL_TOKEN`: admin token，默认必填，不能使用 `change-me`。
- `PANEL_OPERATOR_TOKEN`: operator token，可执行服务器、配置、Mod、玩家操作。
- `PANEL_VIEWER_TOKEN`: viewer token，只读。
- `PALPANEL_REQUIRE_AUTH`: 默认 `true`。只在隔离开发环境设为 `false`。
- `PALPANEL_CORS_ORIGINS`: 允许的前端来源，逗号分隔。
- `PALPANEL_FRONTEND_DIST`: 前端构建产物目录。
- `PALPANEL_MAX_UPLOAD_MB`: Mod zip 上传大小限制，默认 `256`。
- `PALPANEL_DATA_DIR`: 运行数据根目录，默认 `data`。

## 部署建议

- 推荐使用 HTTPS 反向代理暴露公网，只转发 `/api/*` 到后端，或让后端通过 `PALPANEL_FRONTEND_DIST` 托管前端。
- 不要把 `PALPANEL_REQUIRE_AUTH=false` 用于公网。
- `data/server`、`data/backups`、`data/logs`、`data/palpanel.db` 都是需要备份的运行数据。
- 项目当前只提供 Wine runner Dockerfile；完整后端 Docker/Compose 仍建议按部署环境单独编写，确保 Palworld 存档目录使用稳定宿主机卷。

## 运维功能

- 安全重启会创建后端任务：保存/通知、等待倒计时、停止、启动。
- 版本检查使用 Steam Build ID：读取本地 `appmanifest_2394010.acf`，再通过 SteamCMD 查询 public 分支最新 Build；Build ID 不是游戏语义版本号。
- “检查后更新”会先比对 Build ID，只有发现新版本才执行备份、停服、更新和必要的重启。
- 恢复备份前会先停止服务器并创建 `pre-restore` 备份。
- 写操作会记录审计日志，可在 `/audit` 查看。
- 玩家踢出等依赖运行时命令能力的功能，在未接入 PalDefender/RCON 后端时会返回明确 `unsupported`。
