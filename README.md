<div align="center">

# <img src="docs/static/logo.png" width="45" align="center"> Save Any Bot

**English** | [简体中文](./README_zh.md)

> **Save Any Telegram File to Anywhere 📂. Support restrict saving content and beyond telegram.**

> [!NOTE]
> This is a personal fork of [krau/SaveAny-Bot](https://github.com/krau/SaveAny-Bot) with enhancements for auto-downloading X/Twitter videos at the best quality to a local NAS directory:
> - Multi-level parser fallback chain (fxtwitter → getxbot plugin → yt-dlp with cookies)
> - Resumable downloads with integrity verification (Range requests)
> - Human-readable filenames derived from tweet author and text
> - yt-dlp `cookies_file` / `fallback` / `path` options
>
> Docker image: `ghcr.io/taffyblog/saveany-bot:latest` (built automatically by GitHub Actions on every push to `main`).

[![Build Status](https://img.shields.io/github/actions/workflow/status/TaffyBlog/SaveAny-Bot/docker.yml?label=build)](https://github.com/TaffyBlog/SaveAny-Bot/actions/workflows/docker.yml)
[![Stars](https://img.shields.io/github/stars/TaffyBlog/SaveAny-Bot?style=flat)](https://github.com/TaffyBlog/SaveAny-Bot/stargazers)
[![Issues](https://img.shields.io/github/issues/TaffyBlog/SaveAny-Bot)](https://github.com/TaffyBlog/SaveAny-Bot/issues)
[![License](https://img.shields.io/github/license/TaffyBlog/SaveAny-Bot)](./LICENSE)

</div>

## 🎯 Features

- Support documents / videos / photos / stickers… and even [Telegraph](https://telegra.ph/)
- Bypass "restrict saving content" media
- Batch download
- Streaming transfer
- Multi-user support
- Auto organize files based on storage rules
- Watch specified chats and auto-save messages, with filters
- Transfer files between different storage backends
- Integrate with yt-dlp to download and save media from 1000+ websites
- Aria2 integration to download files from URLs/magnets and save to storages
- Write JS parser plugins to save files from almost any website
- Storage backends:
  - Alist
  - S3
  - WebDAV
  - Local filesystem
  - Rclone (via command line)
  - Telegram (re-upload to specified chats)

## 📦 Quick Start

Create a `config.toml` file with the following content:

```toml
lang = "en" # Language setting, "en" for English
[telegram]
token = "" # Your bot token, obtained from @BotFather
[telegram.proxy]
# Enable proxy for Telegram
enable = false
url = "socks5://127.0.0.1:7890"

[[storages]]
name = "Local Disk"
type = "local"
enable = true
base_path = "./downloads"

[[users]]
id = 114514 # Your Telegram account id
storages = []
blacklist = true
```

Run Save Any Bot with Docker:

```bash
docker run -d --name saveany-bot \
    -v ./config.toml:/app/config.toml \
    -v ./downloads:/app/downloads \
    ghcr.io/taffyblog/saveany-bot:latest
```

Please [**read the docs**](https://sabot.unv.app/en/) for more configuration options and usage (upstream documentation; see [DEPLOY.md](./DEPLOY.md) for this fork's NAS deployment guide).

## Sponsors

This project is supported by [YxVM](https://yxvm.com/) and [NodeSupport](https://github.com/NodeSeekDev/NodeSupport).

If this project is helpful to you, consider sponsoring me via:

- [Afdian](https://afdian.com/a/unvapp)

## Thanks To

- [gotd](https://github.com/gotd/td)
- [TG-FileStreamBot](https://github.com/EverythingSuckz/TG-FileStreamBot)
- [gotgproto](https://github.com/celestix/gotgproto)
- [tdl](https://github.com/iyear/tdl)
- All the dependencies, contributors, sponsors and users.

## Contact

- [![Group](https://img.shields.io/badge/ProjectSaveAny-Group-blue)](https://t.me/ProjectSaveAny)
- [![Discussion](https://img.shields.io/badge/Github-Discussion-white)](https://github.com/krau/saveany-bot/discussions)
- [![PersonalChannel](https://img.shields.io/badge/Krau-PersonalChannel-cyan)](https://t.me/acherkrau)