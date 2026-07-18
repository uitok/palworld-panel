## 这次更新

v1.2.1 修复了 Issue #2 中的界面问题，补齐 Linux 导入后启动链路，并扩展了跨平台存档归档格式。

### 新增

- Windows 和 Linux 存档导入支持 `.tar`、`.tar.gz`、`.tgz`，继续兼容 `.zip`
- ZIP/TAR 统一使用跨平台路径规范化，兼容 Windows 反斜杠目录项
- TAR 导入拒绝路径穿越、绝对路径、符号链接、硬链接和超限展开内容

### 修正

- 品牌 SVG 由嵌入式静态资源直接返回，不再被 SPA fallback 错误替换
- 修复基础 CSS 覆盖 Tailwind 内边距和响应式隐藏规则导致的开服向导、按钮和高级设置排版问题
- 修复移动端导航抽屉被移出视口的问题
- Linux Wine runner 强制规范化入口脚本换行，避免 Windows 工作区的 CRLF 导致 `/usr/bin/env: bash\r` 启动失败
- Linux systemd unit 和 Shell/Docker 构建文件统一使用 LF
- Windows 打包测试遇到临时目录文件锁时能够自动重试
- 源码归档不再误收 `dist` 构建产物
