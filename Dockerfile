FROM golang:alpine AS builder

ARG VERSION="dev"
ARG GitCommit="Unknown"
ARG BuildTime="Unknown"

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 \
    go build -trimpath \
    -ldflags=" \
    -s -w \
    -X 'github.com/krau/SaveAny-Bot/config.Version=${VERSION}' \
    -X 'github.com/krau/SaveAny-Bot/config.GitCommit=${GitCommit}' \
    -X 'github.com/krau/SaveAny-Bot/config.BuildTime=${BuildTime}' \
    -X 'github.com/krau/SaveAny-Bot/config.Docker=true' \
    " \
    -o saveany-bot .

FROM alpine:latest

RUN apk add --no-cache curl ffmpeg yt-dlp

WORKDIR /app

COPY --from=builder /app/saveany-bot .
COPY entrypoint.sh .
# 内置 getxbot 兜底插件: 只有 [parser] plugin_enable = true 时才会被加载.
# 挂载 ./plugins:/app/plugins 时会覆盖这里, 不挂也能直接用 plugin_dirs = ["./plugins"].
# 示例插件 (example_*.js) 不打进镜像, 避免无关解析器进入兜底链.
COPY --from=builder /app/plugins/getxbot_parser.js ./plugins/getxbot_parser.js

RUN chmod +x /app/saveany-bot && \
    chmod +x /app/entrypoint.sh

ENTRYPOINT ["/app/entrypoint.sh"]
