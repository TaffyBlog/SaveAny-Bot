package twitter

import "testing"

func TestTweetFilename(t *testing.T) {
	cases := []struct {
		name      string
		mediaURL  string
		nickname  string
		screen    string
		tweetID   string
		text      string
		index     int
		total     int
		want      string
	}{
		{
			name:     "video with nickname and text",
			mediaURL: "https://video.twimg.com/amplify_video/123/vid/avc1/2160x3840/KxdSrCnYpGCHam3l.mp4?tag=29",
			nickname: "小明",
			screen:   "elonmusk",
			tweetID:  "2099867020091347357",
			text:     "这是一条测试推文\n第二行",
			index:    0, total: 1,
			want: "小明(@elonmusk)_这是一条测试推文.mp4",
		},
		{
			name:     "nickname identical to handle",
			mediaURL: "https://video.twimg.com/amplify_video/123/vid/x.mp4?tag=12",
			nickname: "elonmusk",
			screen:   "elonmusk",
			tweetID:  "41",
			text:     "hello",
			index:    0, total: 1,
			want: "@elonmusk_hello.mp4",
		},
		{
			name:     "empty nickname falls back to handle",
			mediaURL: "https://video.twimg.com/amplify_video/123/vid/x.mp4?tag=12",
			nickname: "",
			screen:   "someone",
			tweetID:  "47",
			text:     "hi",
			index:    0, total: 1,
			want: "@someone_hi.mp4",
		},
		{
			name:     "long nickname truncated to 20 runes",
			mediaURL: "https://video.twimg.com/amplify_video/123/vid/x.mp4?tag=12",
			nickname: "这是一个特别特别特别特别特别特别特别特别长的昵称啊",
			screen:   "someone",
			tweetID:  "48",
			text:     "",
			index:    0, total: 1,
			want: string([]rune("这是一个特别特别特别特别特别特别特别特别长的昵称啊")[:20]) + "(@someone).mp4",
		},
		{
			name:     "long text truncated to 40 runes",
			mediaURL: "https://video.twimg.com/amplify_video/123/vid/x.mp4?tag=12",
			nickname: "小明",
			screen:   "someone",
			tweetID:  "42",
			text:     "这是一段非常长的推文内容需要被截断处理因为超过了四十个字符的限制所以后面会被去掉这是一段非常长的推文内容需要被截断处理",
			index:    0, total: 1,
			want: "小明(@someone)_" + string([]rune("这是一段非常长的推文内容需要被截断处理因为超过了四十个字符的限制所以后面会被去掉这是一段非常长的推文内容需要被截断处理")[:40]) + ".mp4",
		},
		{
			name:     "photo extension preserved",
			mediaURL: "https://pbs.twimg.com/media/AbCdEfG.jpg?format=jpg&name=large",
			nickname: "小明",
			screen:   "someone",
			tweetID:  "43",
			text:     "photo tweet",
			index:    0, total: 1,
			want: "小明(@someone)_photo tweet.jpg",
		},
		{
			name:     "multiple media get index suffix",
			mediaURL: "https://video.twimg.com/amplify_video/124/vid/a.mp4",
			nickname: "小明",
			screen:   "someone",
			tweetID:  "44",
			text:     "multi",
			index:    0, total: 3,
			want: "小明(@someone)_multi_1.mp4",
		},
		{
			name:     "multiple media index 3",
			mediaURL: "https://video.twimg.com/amplify_video/124/vid/c.mp4",
			nickname: "小明",
			screen:   "someone",
			tweetID:  "44",
			text:     "multi",
			index:    2, total: 3,
			want: "小明(@someone)_multi_3.mp4",
		},
		{
			name:     "illegal chars cleaned",
			mediaURL: "https://video.twimg.com/amplify_video/126/vid/w.mp4",
			nickname: "小明",
			screen:   "someone",
			tweetID:  "46",
			text:     `标题: "引用" <测试>?`,
			index:    0, total: 1,
			want: "小明(@someone)_标题_ _引用_ _测试.mp4",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := tweetFilename(c.mediaURL, c.nickname, c.screen, c.tweetID, c.text, c.index, c.total)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
