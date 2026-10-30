package twitter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"regexp"
	"strings"

	"github.com/krau/SaveAny-Bot/common/utils/fsutil"
	"github.com/krau/SaveAny-Bot/common/utils/netutil"
	"github.com/krau/SaveAny-Bot/pkg/parser"
)

type TwitterParser struct {
	client    http.Client
	apiDomain string
}

const (
	fxTwitterApi = "api.fxtwitter.com"
)

var _ parser.ConfigurableParser = (*TwitterParser)(nil)

var (
	twitterSourceURLRegexp *regexp.Regexp = regexp.MustCompile(`(?:twitter|x)\.com/([^/]+)/status/(\d+)`)
)

func getTweetID(sourceURL string) string {
	matches := twitterSourceURLRegexp.FindStringSubmatch(sourceURL)
	if len(matches) < 3 {
		return ""
	}
	return matches[2]
}

// tweetFilename derives a human-readable file name from the tweet:
// {nickname}(@{screen_name})_[{text}_]_{n}.ext — the author part leads with
// the display name (nickname) when available, the handle is kept for
// traceability; multiple media get an index suffix to avoid collisions.
func tweetFilename(mediaURL, nickname, screenName, tweetID, text string, index, total int) string {
	base := path.Base(strings.Split(mediaURL, "?")[0])
	ext := strings.ToLower(path.Ext(base))
	if ext == "" {
		ext = ".mp4"
	}
	author := authorLabel(nickname, screenName)
	title := strings.TrimSpace(strings.SplitN(text, "\n", 2)[0])
	const maxTitleRunes = 40
	if runes := []rune(title); len(runes) > maxTitleRunes {
		title = string(runes[:maxTitleRunes])
	}
	var b strings.Builder
	b.WriteString(author)
	if title != "" {
		b.WriteString("_")
		b.WriteString(title)
	}
	if total > 1 {
		fmt.Fprintf(&b, "_%d", index+1)
	}
	return fsutil.NormalizePathname(b.String()) + ext
}

// authorLabel renders the author as nickname(@handle); the nickname is
// truncated to 20 runes and skipped when empty or identical to the handle.
func authorLabel(nickname, screenName string) string {
	handle := strings.TrimPrefix(strings.TrimSpace(screenName), "@")
	if nickname == "" {
		return "@" + handle
	}
	nickname = strings.TrimSpace(nickname)
	if runes := []rune(nickname); len(runes) > 20 {
		nickname = string(runes[:20])
	}
	if strings.EqualFold(nickname, handle) {
		return "@" + handle
	}
	return nickname + "(@" + handle + ")"
}

func (p *TwitterParser) Parse(ctx context.Context, u string) (*parser.Item, error) {
	id := getTweetID(u)
	if id == "" {
		return nil, errors.New("invalid Twitter URL")
	}
	apiUrl := fmt.Sprintf("https://%s/_/status/%s", p.apiDomain, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request to Twitter API: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Twitter API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch Twitter API, status code: %d", resp.StatusCode)
	}
	var fxResp FxTwitterApiResp
	if err := json.NewDecoder(resp.Body).Decode(&fxResp); err != nil {
		return nil, fmt.Errorf("failed to decode Twitter API response: %w", err)
	}
	if fxResp.Code != 200 {
		return nil, fmt.Errorf("request twitter API error: %s", fxResp.Message)
	}
	if len(fxResp.Tweet.Media.All) == 0 {
		return nil, errors.New("no media found in the tweet")
	}
	resources := make([]parser.Resource, 0, len(fxResp.Tweet.Media.All))
	total := len(fxResp.Tweet.Media.All)
	for i, media := range fxResp.Tweet.Media.All {
		var size int64
		resp, err := p.client.Head(media.URL)
		if err == nil {
			size = resp.ContentLength
			resp.Body.Close()
		}
		resources = append(resources, parser.Resource{
			URL:      media.URL,
			Filename: tweetFilename(media.URL, fxResp.Tweet.Author.Name, fxResp.Tweet.Author.ScreenName, id, fxResp.Tweet.Text, i, total),
			Size:     size,
		})
	}
	item := &parser.Item{
		Site:        "Twitter",
		Title:       fmt.Sprintf("Tweet/%s", id),
		URL:         fxResp.Tweet.URL,
		Description: fxResp.Tweet.Text,
		Author:      fxResp.Tweet.Author.Name,
		Tags:        make([]string, 0),
		Extra:       make(map[string]any),
		Resources:   resources,
	}
	return item, nil
}

func (p *TwitterParser) CanHandle(u string) bool {
	return twitterSourceURLRegexp.MatchString(u)
}

func (p *TwitterParser) Name() string {
	return "twitter"
}

func (p *TwitterParser) Configure(config map[string]any) error {
	if config == nil {
		p.apiDomain = fxTwitterApi
		p.client = *netutil.DefaultParserHTTPClient()
		return nil
	}
	if domain, ok := config["api_domain"].(string); ok && domain != "" {
		p.apiDomain = domain
	} else {
		p.apiDomain = fxTwitterApi
	}
	if proxyUrl, ok := config["proxy"].(string); ok && proxyUrl != "" {
		proxyClient, err := netutil.NewProxyHTTPClient(proxyUrl)
		if err != nil {
			return fmt.Errorf("failed to create proxy client: %w", err)
		}
		p.client = *proxyClient
	}
	return nil
}
