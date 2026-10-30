<div align="center">

# <img src="docs/static/logo.png" width="45" align="center"> Save Any Bot

> **把 Telegram 上的文件转存到多种存储端**

> [!NOTE]
> 本仓库是 [krau/SaveAny-Bot](https://github.com/krau/SaveAny-Bot) 的个人二开 fork, 增强: **发推文链接 → 自动以最高画质下载视频到 NAS 本地目录**
> - 多级解析兜底链 (fxtwitter → getxbot 插件 → yt-dlp+cookies)
> - 断点续传 + 下载完整性校验 (Range 请求, 字节数不符自动重试)
> - 按"昵称(@handle)_推文内容"生成可读文件名
> - yt-dlp 新增 `cookies_file` / `fallback` / `path` 配置
>
> Docker 镜像: `ghcr.io/taffyblog/saveany-bot:latest` (每次 push 到 main 由 GitHub Actions 自动构建). 部署见 [DEPLOY.md](./DEPLOY.md).

[![Build Status](https://img.shields.io/github/actions/workflow/status/TaffyBlog/SaveAny-Bot/docker.yml?label=build)](https://github.com/TaffyBlog/SaveAny-Bot/actions/workflows/docker.yml)
[![Stars](https://img.shields.io/github/stars/TaffyBlog/SaveAny-Bot?style=flat)](https://github.com/TaffyBlog/SaveAny-Bot/stargazers)
[![Issues](https://img.shields.io/github/issues/TaffyBlog/SaveAny-Bot)](https://github.com/TaffyBlog/SaveAny-Bot/issues)
[![License](https://img.shields.io/github/license/TaffyBlog/SaveAny-Bot)](./LICENSE)

</div>

## 🎯 特性

- 支持文档/视频/图片/贴纸…甚至还有 [Telegraph](https://telegra.ph/)
- 破解禁止保存的文件
- 批量下载
- 流式传输
- 多用户使用
- 基于存储规则的自动整理
- 监听并自动转存指定聊天的消息, 支持过滤
- 在不同存储端之间转存文件
- 集成 yt-dlp, 从所支持的网站下载并转存媒体文件
- 集成 Aria2, 支持直链/磁力下载和转存
- 使用 js 编写解析器插件以转存任意网站的文件
- 存储端支持:
  - Alist
  - S3
  - WebDAV
  - 本地磁盘
  - Rclone
  - Telegram (重传回指定聊天)

## 快速开始

创建文件 `config.toml` 并填入以下内容:

```toml
[telegram]
token = "" # 你的 Bot Token, 在 @BotFather 获取
[telegram.proxy]
# 启用代理连接 telegram
enable = false
url = "socks5://127.0.0.1:7890"

[[storages]]
name = "本地磁盘"
type = "local"
enable = true
base_path = "./downloads"

[[users]]
id = 114514 # 你的 Telegram 账号 id
storages = []
blacklist = true
```

使用 Docker 运行 Save Any Bot:

```bash
docker run -d --name saveany-bot \
    -v ./config.toml:/app/config.toml \
    -v ./downloads:/app/downloads \
    ghcr.io/taffyblog/saveany-bot:latest
```

请 [**查看文档**](https://sabot.unv.app/) 以获取更多配置选项和使用方法 (上游文档; 本 fork 的 NAS 部署指南见 [DEPLOY.md](./DEPLOY.md)).

## 赞助

本项目受到 [YxVM](https://yxvm.com/) 与 [NodeSupport](https://github.com/NodeSeekDev/NodeSupport) 的支持.

如果这个项目对你有帮助, 你可以考虑通过以下方式赞助我:

- [爱发电](https://afdian.com/a/unvapp)

## 鸣谢

- [gotd](https://github.com/gotd/td)
- [TG-FileStreamBot](https://github.com/EverythingSuckz/TG-FileStreamBot)
- [gotgproto](https://github.com/celestix/gotgproto)
- [tdl](https://github.com/iyear/tdl)
- All the dependencies, contributors, sponsors and users.

## 社区和关于作者

- [![通知群组](https://img.shields.io/badge/ProjectSaveAny-Group-blue)](https://t.me/ProjectSaveAny)
- [![讨论区](https://img.shields.io/badge/Github-Discussion-white)](https://github.com/krau/saveany-bot/discussions)
- [![个人频道](https://img.shields.io/badge/Krau-PersonalChannel-cyan)](https://t.me/acherkrau)