## 这次更新

v1.2.0 主要整理了客户端界面，并把存档、地图、配种和日常运维功能接到同一套工作流里。

### 新增

- 新的暖砂雾蓝客户端主题与合并后的侧边栏
- 多存档数据源、ZIP 导入和存档索引状态
- PalCalc v1.17.6 配种实验室与 QQ 受限页面
- AstrBot 插件：角色绑定、签到、积分、一次性登录链接和快捷配种
- Palpagos 游戏地图与实时/存档坐标叠加
- WebDAV 连接测试、自动归档和单个备份上传
- 定时安全重启、定时备份和任务队列入口

### 修正

- 在线备份遇到 PalDefender 日志持续写入时，manifest 记录实际写入大小，避免误报校验失败
- 调整开服向导、卡片和状态颜色，移除大面积深蓝卡片
- 精简导航层级，保留旧地址兼容跳转

### 验证

- Windows Palworld Dedicated Server Build `24181105` 实机启停、保存、备份、索引和安全重启通过
- 导入已有服务器后能够扫描现有 Workshop Mod
- 测试 Mod 的安装、启用、禁用和卸载闭环通过
- 临时开发 Token 创建、鉴权、撤销通过
- 后端、Windows CGO `sav-cli`、PalCalc bridge、AstrBot 插件、前端单元测试和浏览器 E2E 通过

当前仍不支持 Xbox WGS。AstrBot 的真实 QQ 群与在线玩家验证码流程需要部署者在自己的 NapCat、AstrBot 和 PalDefender 环境中完成最后联调。
