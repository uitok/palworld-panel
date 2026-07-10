# PalPanel 前端

幻兽帕鲁开服面板前端，面向同仓库 `backend` 目录中的 Go REST API。前端使用 React + Vite + React Router + Tailwind + lucide + Recharts，不引入额外 UI 框架。

## 开发目录

- 前端源码：`frontend`
- 后端源码：`backend`
- 运行数据：`data`

## 环境变量

复制 `.env.development.example` 为 `.env.development`：

```env
VITE_APP_BRAND=PalPanel
VITE_STORAGE_PREFIX=palpanel
VITE_DEFAULT_BACKEND_URL=
VITE_DEFAULT_BACKEND_PORT=64217
VITE_DEV_API_PROXY_TARGET=http://127.0.0.1:64217
VITE_DEV_PORT=63107
```

前端开发端口来自 `VITE_DEV_PORT`，默认 `63107`。开发模式默认后端地址由 `VITE_DEFAULT_BACKEND_URL` 或 `VITE_DEFAULT_BACKEND_PORT` 生成；生产构建默认使用同源 `/api`，适合由后端直接托管 `frontend/dist`。

品牌名来自 `VITE_APP_BRAND`，默认 `PalPanel`。本地存储 key 使用 `VITE_STORAGE_PREFIX` 生成，例如默认 token key 是 `localStorage.palpanel_token`；旧版 key 会在读取时迁移一次。生产和开发构建均不接受构建期面板 Token，Token 只能由用户在运行时输入并保存在浏览器本地存储中。

## 常用命令

```powershell
npm install
npm run dev
npm run typecheck
npm run lint
npm run test
npm run build
npm run check
```

## 后端联调

1. 在 `backend` 目录启动后端：`PALPANEL_LISTEN_ADDR=0.0.0.0:64217 PALPANEL_CORS_ORIGINS=http://127.0.0.1:63107,http://localhost:63107 go run ./cmd/palpanel`
2. 在本目录启动前端：`npm run dev -- --host 0.0.0.0`
3. 浏览器访问 `http://127.0.0.1:63107/dashboard`
4. 如需手动设置 token，在控制台执行：

```js
localStorage.setItem('palpanel_token', '<your-panel-token>')
```

## 路由

- `/setup` 开服向导
- `/dashboard` 系统总览
- `/monitor` 实时监控
- `/players` 玩家管理
- `/banlist` 封禁列表
- `/pals` 帕鲁管理
- `/bases` 基地列表
- `/mods` Mod 管理
- `/security` PalDefender 安全
- `/backups` 备份管理
- `/tasks` 任务队列
- `/audit` 操作审计
- `/settings` 服务器设置

`/setup` 会显示本地/最新 Steam Build ID，并提供“检查更新”和“检查后更新”。`/tasks` 可创建“检查更新”计划任务；该计划只产生提醒，不会自动安装更新。

## 移动端验证

建议检查 `390x844`、`768x1024`、`1024x768`、`1440x900` 四个视口：导航抽屉、底部快捷操作、表格卡片、设置保存按钮、弹窗滚动都应正常显示，且页面不应出现横向溢出。
