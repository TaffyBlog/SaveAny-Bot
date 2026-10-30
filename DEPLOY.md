# DEPLOY.md — SaveAny-Bot (TaffyBlog fork) NAS 部署指南

本项目是 [krau/SaveAny-Bot](https://github.com/krau/SaveAny-Bot) 的私有二开 fork, 目标: **发推文链接 → 自动以最高画质下载视频到 NAS 本地目录**, Telegram 仅回复任务状态, 不回传文件。

## 工作原理 (下载链路)

```
发链接 (私有白名单用户)
  │
  ├─ 1. fxtwitter 解析 (内置, 返回的已是最高码率直链) ──成功──→ 直接下载落盘
  │        失败 (NSFW/限流等)
  ├─ 2. getxbot 插件 (plugins/getxbot_parser.js) ──成功──→ 直接下载落盘
  │        失败
  └─ 3. yt-dlp 兜底 (带 X 登录 cookies, fallback = true 时自动触发)
```

- B 站 / YouTube 等其他站点链接同样走上述流程: 无内置解析器时直接由 yt-dlp 兜底下载。
- 静默模式 (`/silent`) 下全程无需点按钮; 非静默模式会弹存储选择按钮。

## 部署步骤

1. **准备材料** (自行准备, 本文档不代管):
   - BotFather 创建的 Bot token
   - 你自己的 Telegram 数字 id (向 `@userinfobot` 发消息可查)
   - (推荐) X 账号导出的 cookies.txt (Netscape 格式, 可用浏览器插件 "Get cookies.txt LOCALLY" 导出)
2. **上传文件到 NAS**: 在 NAS 上建一个目录, 放入:
   - `docker-compose.yml` (来自 `deploy/docker-compose.yml`)
   - `config.toml` (来自 `deploy/config.toml`, 填好所有 "!!必改!!" 项)
   - `cookies.txt` (可选)
   - 整个 fork 仓库源码 (compose 采用本地构建, 含本 fork 改动)
   目录结构参见 `deploy/docker-compose.yml` 顶部注释。

   > **免构建方式**: 代码推送到 GitHub 后, Actions 会自动构建镜像并发布到
   > `ghcr.io/taffyblog/saveany-bot:latest` (workflow: `.github/workflows/docker.yml`)。
   > NAS 上把 compose 里的 `build: .` 换成 `image: ghcr.io/taffyblog/saveany-bot:latest`
   > 即可直接拉取, 无需上传源码。首次使用需 `docker login ghcr.io`, 且该镜像包默认
   > 为 private, 可在 GitHub 仓库 → Packages 里改为 public。
3. **修改 config.toml**:
   - `[telegram] token`
   - `[[users]] id` (仅你自己的 id, 非白名单用户会被拒绝)
   - 需要代理时: `[telegram.proxy]` (连 Telegram) 与 `[parser.twitter] proxy` (连 fxtwitter API) 是两条独立配置; compose 中的 `HTTP_PROXY/HTTPS_PROXY` 环境变量影响容器内其余出网 (yt-dlp 等)。
4. **构建并启动**:
   ```bash
   docker compose up -d --build
   docker compose logs -f
   ```
5. **初始化 Bot (一次性, 在 Telegram 里做)**:
   - 向你的 Bot 发送 `/start`
   - `/storage` → 选择 `nas-local` 设为默认存储
   - `/silent` → 开启静默模式 (此后发链接即直接下载, 不再询问)
   - `/help` 可查看全部命令

## 验证清单

| # | 场景 | 预期 |
|---|------|------|
| 1 | 发普通推文链接 | 自动解析落盘, TG 收到任务开始/完成通知, `downloads/` 出现最高画质 mp4 |
| 2 | 发 NSFW 敏感内容推文链接 | fxtwitter 失败后自动降级 getxbot 或 yt-dlp+cookies, 下载成功 |
| 3 | 发 B 站 / YouTube 链接 | 直接由 yt-dlp 兜底下载成功 |
| 4 | 非白名单用户发链接 | Bot 回复无权限 (`You don't have permission`), 不下载 |
| 5 | `docker compose restart` 后再发链接 | 会话正常 (data/ 持久化), 配置/cookies 正常 (volume 挂载), 无需重新 /start |

排查: `docker compose logs -f` 看日志; 解析降级时会看到 `Parser twitter failed to parse ... trying next parser` 字样。

## 常见问题

- **换 Bot token 后报错**: 删除 `data/session.db` 后重启容器。
- **画质不对**: 确认 `[ytdlp] max_height = 0`; fxtwitter 链路返回的就是源视频最高码率 variant, 无需额外配置。
- **NSFW 仍失败**: 确认 cookies.txt 有效 (在浏览器中登录 X 后重新导出) 且 `cookies_file` 路径正确; cookies 过期需重新导出。
- **容器内时间/编码**: 配置文件必须 UTF-8 编码。

## 本 fork 相对上游的改动

| 文件 | 改动 |
|------|------|
| `config/ytdlp.go` | 新增 `cookies_file`、`fallback`、`path` 配置项 |
| `config/viper.go` | 上述各项的默认值 |
| `core/tasks/ytdlp/execute.go` | 将 cookies 传给 yt-dlp (`--cookies`); 应用 `path` 指定的可执行文件 |
| `api/media.go` | 元数据提取同样应用 `path` 指定的 yt-dlp |
| `common/utils/fsutil/fs.go` | 新增 `OpenResumeFile` (不截断打开并定位到文件尾) |
| `core/tasks/parsed/execute.go` | 下载失败断点续传 (Range) + 完整性校验 (字节数与预期不符自动重试) |
| `parsers/parser.go` | 解析失败时继续尝试下一个匹配的解析器 (兜底链) |
| `parsers/js/api.go` | JS 插件 ghttp 新增 `postJSON` |
| `client/bot/handlers/parse.go` | 文本消息解析失败/无解析器时, 可选回退 yt-dlp (`[ytdlp] fallback`) |
| `parsers/native/twitter/parser.go` | 文件名改为 昵称(@handle)_推文内容.ext (原为 twimg 随机对象名) |
| `plugins/getxbot_parser.js` | 新增 getxbot 兜底解析插件 |
| `.github/workflows/docker.yml` | push 时自动构建镜像并发布到 GHCR |
| `config.example.toml` | 文档同步 |
| `deploy/` | 生产配置模板 + compose + 本文档 |
