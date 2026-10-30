// 处理任意文本消息, 用于通用地从外部源下载文件

package handlers

import (
	"errors"
	"path"
	"regexp"
	"strings"

	"github.com/celestix/gotgproto/dispatcher"
	"github.com/celestix/gotgproto/ext"
	"github.com/charmbracelet/log"
	"github.com/gotd/td/tg"
	"github.com/krau/SaveAny-Bot/client/bot/handlers/utils/dirutil"
	"github.com/krau/SaveAny-Bot/client/bot/handlers/utils/msgelem"
	"github.com/krau/SaveAny-Bot/client/bot/handlers/utils/shortcut"
	"github.com/krau/SaveAny-Bot/common/i18n"
	"github.com/krau/SaveAny-Bot/common/i18n/i18nk"
	"github.com/krau/SaveAny-Bot/common/utils/fsutil"
	"github.com/krau/SaveAny-Bot/common/utils/tgutil"
	"github.com/krau/SaveAny-Bot/config"
	"github.com/krau/SaveAny-Bot/parsers"
	"github.com/krau/SaveAny-Bot/pkg/enums/tasktype"
	"github.com/krau/SaveAny-Bot/pkg/tcbdata"
	"github.com/krau/SaveAny-Bot/storage"
)

// httpURLRegex matches plain http(s) URLs in a text message, used by the
// yt-dlp fallback when no parser handles the message.
var httpURLRegex = regexp.MustCompile(`https?://\S+`)

// trailingPunct is trimmed from URL matches so punctuation after a link
// (e.g. CJK and ASCII sentence punctuation) is not part of the URL.
const trailingPunct = ".,;:!?)\u3001\u3002\uff0c\uff1b\uff1a\uff01\uff1f\uff09>\"'\u2026"

// extractURLs returns deduplicated http(s) URLs found in text.
func extractURLs(text string) []string {
	matches := httpURLRegex.FindAllString(text, -1)
	seen := make(map[string]struct{}, len(matches))
	urls := make([]string, 0, len(matches))
	for _, m := range matches {
		m = strings.TrimRight(m, trailingPunct)
		if m == "" {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		urls = append(urls, m)
	}
	return urls
}

// fallbackToYtdlpKeyboard is the non-silent fallback: let the user pick a
// storage, then the selection creates yt-dlp tasks for the URLs (same flow as
// the /ytdlp command).
func fallbackToYtdlpKeyboard(ctx *ext.Context, u *ext.Update, source string) error {
	urls := extractURLs(source)
	if len(urls) == 0 {
		return dispatcher.EndGroups
	}
	markup, err := msgelem.BuildAddSelectStorageKeyboard(storage.GetUserStorages(ctx, u.GetUserChat().GetID()), tcbdata.Add{
		TaskType:   tasktype.TaskTypeYtdlp,
		YtdlpURLs:  urls,
		YtdlpFlags: nil,
	})
	if err != nil {
		log.FromContext(ctx).Errorf("Failed to build storage selection keyboard for yt-dlp fallback: %s", err)
		return dispatcher.EndGroups
	}
	ctx.Reply(u, ext.ReplyTextString(i18n.T(i18nk.BotMsgYtdlpInfoUrlsSelectStorage, map[string]any{
		"Count": len(urls),
	})), &ext.ReplyOpts{
		Markup: markup,
	})
	return dispatcher.EndGroups
}

// fallbackToYtdlpTask is the silent-mode fallback: directly create yt-dlp
// tasks for the URLs found in the message, saving to the default storage.
func fallbackToYtdlpTask(ctx *ext.Context, u *ext.Update, text string) error {
	urls := extractURLs(text)
	if len(urls) == 0 {
		return dispatcher.EndGroups
	}
	stor := storage.FromContext(ctx)
	if stor == nil {
		return dispatcher.EndGroups
	}
	msg, err := ctx.Reply(u, ext.ReplyTextString("解析器无法处理该链接, 改用 yt-dlp 下载..."), nil)
	if err != nil {
		return err
	}
	dirPath := dirutil.PathFromContext(ctx)
	return shortcut.CreateAndAddYtdlpTaskWithEdit(ctx, stor, dirPath, urls, nil, msg.ID, u.GetUserChat().GetID())
}

func handleTextMessage(ctx *ext.Context, u *ext.Update) error {
	logger := log.FromContext(ctx)
	text := u.EffectiveMessage.Text
	entityUrls := tgutil.ExtractMessageEntityUrls(u.EffectiveMessage.Message)
	if len(entityUrls) > 0 {
		text += "\n" + strings.Join(entityUrls, "\n")
	}
	lines := strings.Split(text, "\n")
	seen := make(map[string]struct{})
	var processedLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		processedLines = append(processedLines, line)
	}
	source := strings.TrimSpace(strings.Join(processedLines, "\n"))
	ok, _ := parsers.CanHandle(source)
	if !ok {
		if config.C().Ytdlp.Fallback {
			return fallbackToYtdlpKeyboard(ctx, u, source)
		}
		return dispatcher.EndGroups
	}
	msg, err := ctx.Reply(u, ext.ReplyTextString(i18n.T(i18nk.BotMsgParseInfoParsing, nil)), nil)
	if err != nil {
		return err
	}

	// Use ParseWithContext instead of pser.Parse so that, when the first
	// matching parser fails, the remaining matching parsers act as fallbacks.
	item, err := parsers.ParseWithContext(ctx, source)
	if errors.Is(err, parsers.ErrNoParserFound) {
		return dispatcher.EndGroups
	}
	if err != nil {
		logger.Error("Failed to parse text", "error", err)
		ctx.Reply(u, ext.ReplyTextString(i18n.T(i18nk.BotMsgParseErrorParseTextFailed, map[string]any{
			"Error": err.Error(),
		})), nil)
		if config.C().Ytdlp.Fallback {
			return fallbackToYtdlpKeyboard(ctx, u, source)
		}
		return dispatcher.EndGroups
	}
	logger.Debug("Parsed item from text message", "title", item.Title, "url", item.URL)
	userID := u.GetUserChat().GetID()
	markup, err := msgelem.BuildAddSelectStorageKeyboard(storage.GetUserStorages(ctx, userID), tcbdata.Add{
		TaskType:   tasktype.TaskTypeParseditem,
		ParsedItem: item,
	})
	if err != nil {
		logger.Errorf("Failed to build storage selection keyboard: %s", err)
		ctx.Reply(u, ext.ReplyTextString(i18n.T(i18nk.BotMsgParseErrorBuildStorageSelectKeyboardFailed, map[string]any{
			"Error": err.Error(),
		})), nil)
		return dispatcher.EndGroups
	}
	text, entities, err := msgelem.BuildParsedTextEntity(*item)
	if err != nil {
		logger.Errorf("Failed to build parsed text entity: %s", err)
		ctx.Reply(u, ext.ReplyTextString(i18n.T(i18nk.BotMsgParseErrorBuildParsedTextEntityFailed, map[string]any{
			"Error": err.Error(),
		})), nil)
		return dispatcher.EndGroups
	}
	ctx.EditMessage(userID, &tg.MessagesEditMessageRequest{
		Message:     text,
		ReplyMarkup: markup,
		Entities:    entities,
		ID:          msg.ID,
	})

	return dispatcher.EndGroups
}

func handleSilentSaveText(ctx *ext.Context, u *ext.Update) error {
	logger := log.FromContext(ctx)
	stor := storage.FromContext(ctx)
	text := u.EffectiveMessage.Text
	if text == "" {
		return dispatcher.EndGroups
	}
	item, err := parsers.ParseWithContext(ctx, text)
	if errors.Is(err, parsers.ErrNoParserFound) {
		if config.C().Ytdlp.Fallback {
			return fallbackToYtdlpTask(ctx, u, text)
		}
		return dispatcher.EndGroups
	}
	if err != nil {
		logger.Error("Failed to parse text", "error", err)
		ctx.Reply(u, ext.ReplyTextString(i18n.T(i18nk.BotMsgParseErrorParseTextFailed, map[string]any{
			"Error": err.Error(),
		})), nil)
		if config.C().Ytdlp.Fallback {
			return fallbackToYtdlpTask(ctx, u, text)
		}
		return dispatcher.EndGroups
	}
	logger.Debug("Parsed item from text message", "title", item.Title, "url", item.URL)
	userID := u.GetUserChat().GetID()
	text, entities, err := msgelem.BuildParsedTextEntity(*item)
	if err != nil {
		logger.Errorf("Failed to build parsed text entity: %s", err)
		ctx.Reply(u, ext.ReplyTextString(i18n.T(i18nk.BotMsgParseErrorBuildParsedTextEntityFailed, map[string]any{
			"Error": err.Error(),
		})), nil)
		return dispatcher.EndGroups
	}
	msg, err := ctx.SendMessage(userID, &tg.MessagesSendMessageRequest{
		Message:  text,
		Entities: entities,
		ReplyTo: &tg.InputReplyToMessage{
			ReplyToMsgID:  u.EffectiveMessage.ID,
			ReplyToPeerID: u.GetUserChat().AsInputPeer(),
		},
	})
	if err != nil {
		logger.Errorf("Failed to send message: %s", err)
		return dispatcher.EndGroups
	}
	dirPath := ""
	if len(item.Resources) > 1 {
		dirPath = fsutil.NormalizePathname(item.Title)
	}
	if p := dirutil.PathFromContext(ctx); p != "" {
		dirPath = path.Join(p, dirPath)
	}
	return shortcut.CreateAndAddParsedTaskWithEdit(ctx, stor, dirPath, item, msg.ID, userID)
}
