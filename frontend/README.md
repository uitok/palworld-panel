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

前端开发端口来自 `VITE_DEV_PORT`，默认 `63107`。开发环境通过 Vite 的 `VITE_DEV_API_PROXY_TARGET` 代理 `/api`；生产构建固定使用同源 `/api` 并嵌入 Go 发布二进制。界面不提供后端地址输入。

品牌名来自 `VITE_APP_BRAND`，默认 `PalPanel`。`VITE_STORAGE_PREFIX` 仅用于侧栏等非敏感界面偏好。认证状态来自 `/auth/status`，注册或登录后由浏览器保存 HttpOnly Session Cookie；前端不读取、持久化或注入认证凭据。

## 常用命令

```powershell
npm install
npm run dev
npm run generate:api-types
npm run typecheck
npm run lint
npm run test
npm run build
npm run check
```

`generate:api-types` deterministically regenerates the DTO contracts from `docs/openapi.yaml`. `npm run check` verifies that the generated file is current before type checking and testing.

## 后端联调

1. 在 `backend` 目录启动后端：`PALPANEL_LISTEN_ADDR=0.0.0.0:64217 PALPANEL_CORS_ORIGINS=http://127.0.0.1:63107,http://localhost:63107 go run ./cmd/palpanel`
2. 在本目录启动前端：`npm run dev -- --host 0.0.0.0`
3. 浏览器访问 `http://127.0.0.1:63107/dashboard`
4. 新数据库会进入管理员注册页；已有账号时进入登录页。Vite 代理保持浏览器请求同源，Session Cookie 会自动随 `/api` 请求发送。

## 路由

- `/setup` 开服向导
- `/dashboard` 系统总览
- `/monitor` 实时监控
- `/players` 玩家管理
- `/gm` PalDefender GM 工具
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

Mod 页的统一导入流程支持来源输入或本地 ZIP、GitHub ZIP 候选选择、检查结果、更新提示、异步 Job 进度和待重启状态。Workshop 搜索、详情、翻译和下载会先验证本机 SteamCMD 登录缓存；未登录时，页面只接收 Steam 账户名并打开独立 SteamCMD 窗口，密码与 Steam Guard 验证码不会进入浏览器。GitHub、HTTPS ZIP、本地 ZIP、UE4SS 和 PalDefender 不受 Workshop 登录门禁影响。

`/gm` 通过后端的类型化 PalDefender REST 代理工作，不会把 PalDefender Bearer Token 交给浏览器。页面提供玩家列表与详情、在线状态筛选、六类背包查看、2,455 项 ItemID/中文名/图标搜索、最多 100 行批量发物品、玩家消息、广播、警报、踢出、封禁和解封。页面会分别显示未安装、未通过启动日志确认加载、REST 未启用、Token 未配置和服务未运行状态；对明确离线的玩家禁用发物品、私信和踢出。发物品及处罚操作执行前必须确认，写请求携带幂等键。只读账号可查看；写操作按 `players:write` 禁用，后端仍会独立强制权限。

管理员在 `/settings` 配置 Workshop 翻译所用的 OpenAI-compatible Base URL、模型和 API Key。前端只提交新 Key，不从读取接口取回秘密；测试、截图和示例配置必须使用占位值。

## 许可证

前端与 PalPanel 其余自有代码统一按 GPL-3.0-or-later 分发，完整条款见仓库根目录 `LICENSE`。

## 移动端验证

建议检查 `390x844`、`768x1024`、`1024x768`、`1440x900` 四个视口：导航抽屉、底部快捷操作、表格卡片、设置保存按钮、弹窗滚动都应正常显示，且页面不应出现横向溢出。
