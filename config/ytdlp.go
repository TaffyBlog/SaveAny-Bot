package config

// DefaultYtdlpFilenameTemplate is used when ytdlp.filename_template is not set.
const DefaultYtdlpFilenameTemplate = "%(title)s.%(ext)s"

type YtdlpConfig struct {
	// Path is the path to the yt-dlp executable. Use an absolute path or a
	// name resolvable via $PATH (default "yt-dlp"). The official Docker image
	// ships yt-dlp in $PATH, so the default works there.
	Path string `toml:"path" mapstructure:"path" json:"path"`
	// MaxHeight limits the video resolution by height in pixels (e.g. 1080, 720).
	// 0 means no limit (best available). Ignored when Format is set.
	MaxHeight int `toml:"max_height" mapstructure:"max_height" json:"max_height"`
	// Format is a raw yt-dlp format selector (-f). When set, it takes precedence
	// over MaxHeight and gives the user full control.
	Format string `toml:"format" mapstructure:"format" json:"format"`
	// Recode is the target video container yt-dlp recodes into (e.g. mp4).
	// Empty disables recoding.
	Recode string `toml:"recode" mapstructure:"recode" json:"recode"`
	// FilenameTemplate is the yt-dlp output template for downloaded files, e.g.
	// "%(uploader)s - %(title)s.%(ext)s", relative to the bot's temp directory.
	FilenameTemplate string `toml:"filename_template" mapstructure:"filename_template" json:"filename_template"`
	// RestrictFilenames keeps file names ASCII-only (yt-dlp --restrict-filenames),
	// dropping non-latin characters from titles. Disabled by default.
	RestrictFilenames bool `toml:"restrict_filenames" mapstructure:"restrict_filenames" json:"restrict_filenames"`
	// CookiesFile is the path to a cookies file in Netscape format (e.g.
	// cookies.txt exported from a logged-in browser), passed to yt-dlp via
	// --cookies. Used for age-gated / login-required content such as NSFW
	// posts on X. Empty disables it.
	CookiesFile string `toml:"cookies_file" mapstructure:"cookies_file" json:"cookies_file"`
	// Fallback: when no parser can handle the URLs in a text message, or all
	// matching parsers fail, fall back to yt-dlp for those URLs instead of
	// ignoring the message.
	Fallback bool `toml:"fallback" mapstructure:"fallback" json:"fallback"`
}
