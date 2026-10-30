// getxbot 兜底解析器: 当内置 Twitter 解析器 (fxtwitter) 失败时
// (例如 NSFW 敏感内容被 fxtwitter 拒绝), 调用 getxbot.com 的 /api/parse
// 接口获取视频直链, 选择码率最高的一项。
//
// 接口说明 (实测):
//   POST https://www.getxbot.com/api/parse
//   body: {"videoId": "<完整推文链接>", "p": 1}
//   成功响应:
//   {
//     "code": 0, "message": "success",
//     "data": {
//       "【视频1】": [
//         {"bitrate": 2176000, "content_type": "video/mp4",
//          "url": "https://video.twimg.com/...mp4?tag=29", "size": "1080x1920"}
//       ]
//     }
//   }
//
// 注意: getxbot 的 /api/download/sign 接口有 WAF (403),
// 本插件只使用 /api/parse 拿直链, 下载由框架完成。

const metadata = {
    name: "GetXBot Twitter Fallback",
    version: "1.0.0",
    description: "当 fxtwitter 解析失败时, 通过 getxbot.com 兜底解析推文视频",
    author: "TaffyBlog",
}

const canHandle = function (url) {
    return /(?:twitter|x)\.com\/[^/\s]+\/status\/\d+/.test(url);
}

const parse = function (url) {
    var resp = ghttp.postJSON("https://www.getxbot.com/api/parse", {
        videoId: url,
        p: 1,
    });
    if (resp.error) {
        console.log("getxbot request failed:", resp.error);
        throw new Error("getxbot request failed: " + resp.error);
    }
    var body = resp.data;
    if (!body || body.code !== 0 || !body.data) {
        var msg = body && body.message ? body.message : "unknown error";
        console.log("getxbot api error:", msg);
        throw new Error("getxbot api error: " + msg);
    }

    // data 是 { 分组名: [ {bitrate, content_type, url, size}, ... ] } 的结构,
    // 在所有分组中选出 content_type 为 video/mp4 且码率最高的一项
    var best = null;
    var title = "";
    for (var group in body.data) {
        var variants = body.data[group];
        if (!Array.isArray(variants)) {
            continue;
        }
        if (!title && group) {
            title = group;
        }
        for (var i = 0; i < variants.length; i++) {
            var v = variants[i];
            if (!v || !v.url) {
                continue;
            }
            if (v.content_type && v.content_type.indexOf("video/mp4") !== 0) {
                continue;
            }
            if (best === null || (v.bitrate || 0) > (best.bitrate || 0)) {
                best = v;
            }
        }
    }
    if (!best) {
        throw new Error("getxbot returned no downloadable video variant");
    }

    // 从直链路径取文件名, 失败则用推文 ID 命名
    var filename = "tweet_video.mp4";
    var idMatch = url.match(/status\/(\d+)/);
    var pathPart = best.url.split("?")[0].split("#")[0];
    var base = pathPart.substring(pathPart.lastIndexOf("/") + 1);
    if (base) {
        filename = base;
    } else if (idMatch) {
        filename = "tweet_" + idMatch[1] + ".mp4";
    }

    var item = {
        site: "Twitter (getxbot)",
        url: url,
        title: idMatch ? "Tweet/" + idMatch[1] : (title || "Tweet"),
        author: "",
        description: "",
        tags: [],
        resources: [
            {
                url: best.url,
                filename: filename,
                mime_type: best.content_type || "video/mp4",
                extension: "mp4",
                size: 0, // getxbot 只返回 "1080x1920" 这类分辨率字符串, 拿不到字节数
                hash: {},
                headers: {},
                extra: { bitrate: best.bitrate || 0, size: best.size || "" },
            },
        ],
        extra: {},
    };
    return item;
}

registerParser({
    metadata,
    canHandle,
    parse
});
