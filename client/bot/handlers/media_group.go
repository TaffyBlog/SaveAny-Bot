package handlers

import (
	"strings"
	"sync"
	"time"

	"github.com/celestix/gotgproto/dispatcher"
	"github.com/celestix/gotgproto/ext"
	"github.com/charmbracelet/log"
	"github.com/gotd/td/tg"
	"github.com/krau/SaveAny-Bot/client/bot/handlers/utils/mediautil"
	"github.com/krau/SaveAny-Bot/client/bot/handlers/utils/msgelem"
	"github.com/krau/SaveAny-Bot/client/bot/handlers/utils/shortcut"
	"github.com/krau/SaveAny-Bot/common/i18n"
	"github.com/krau/SaveAny-Bot/common/i18n/i18nk"
	"github.com/krau/SaveAny-Bot/config"
	"github.com/krau/SaveAny-Bot/database"
	"github.com/krau/SaveAny-Bot/pkg/tcbdata"
	"github.com/krau/SaveAny-Bot/pkg/tfile"
	"github.com/krau/SaveAny-Bot/storage"
)

// mediaGroupKey uniquely identifies a media group by chat, sender, and group
// ID so files from different users in the same chat can never be mixed.
type mediaGroupKey struct {
	chatID  int64
	userID  int64
	groupID int64
}

type MediaGroupHandler struct {
	groups    map[mediaGroupKey][]tfile.TGFileMessage
	timers    map[mediaGroupKey]*time.Timer
	mu        sync.Mutex
	timeout   time.Duration
	setupOnce sync.Once
}

func (m *MediaGroupHandler) SetupTimeout(timeoutSec int) {
	m.setupOnce.Do(func() {
		if timeoutSec < 1 {
			timeoutSec = 1
		}
		m.timeout = time.Duration(timeoutSec) * time.Second
	})
}

var (
	mediaGroupHandler = &MediaGroupHandler{
		groups: make(map[mediaGroupKey][]tfile.TGFileMessage),
		timers: make(map[mediaGroupKey]*time.Timer),
		mu:     sync.Mutex{},
	}
)

func handleGroupMediaMessage(ctx *ext.Context, update *ext.Update, message *tg.Message, groupID int64) error {
	mediaGroupHandler.SetupTimeout(max(config.C().Telegram.MediaGroupTimeout, 1))
	logger := log.FromContext(ctx)
	media := message.Media
	supported := mediautil.IsSupported(media)
	if !supported {
		logger.Warnf("[album] skip unsupported media chat=%d msg=%d type=%s",
			update.EffectiveChat().GetID(), message.GetID(), media.TypeName())
		return dispatcher.EndGroups
	}
	userId := update.GetUserChat().GetID()
	userDB, err := database.GetUserByChatID(ctx, userId)
	if err != nil {
		return err
	}
	tfOpts := mediautil.TfileOptions(ctx, userDB, message)
	file, err := tfile.FromMediaMessage(media, ctx.Raw, message, tfOpts...)
	if err != nil {
		logger.Errorf("Failed to get file from media: %s", err)
		return dispatcher.EndGroups
	}
	key := mediaGroupKey{
		chatID:  update.EffectiveChat().GetID(),
		userID:  userId,
		groupID: groupID,
	}
	// Trace every item that enters an album batch: when the saved files do not
	// match what the user thinks they sent, this line shows exactly which
	// message (msg id) and which document (size/name) the server delivered.
	logger.Infof("[album] recv chat=%d user=%d msg=%d grouped=%d type=%s size=%d name=%q %s",
		key.chatID, userId, message.GetID(), groupID, media.TypeName(), file.Size(), file.Name(),
		mediautil.DescribeMedia(media))
	mediaGroupHandler.mu.Lock()
	defer mediaGroupHandler.mu.Unlock()
	if mediaGroupHandler.groups[key] == nil {
		mediaGroupHandler.groups[key] = make([]tfile.TGFileMessage, 0)
	}
	mediaGroupHandler.groups[key] = append(mediaGroupHandler.groups[key], file)

	if timer, exists := mediaGroupHandler.timers[key]; exists {
		timer.Stop()
	}
	mediaGroupHandler.timers[key] = time.AfterFunc(mediaGroupHandler.timeout, func() {
		processMediaGroup(ctx, update, key)
	})
	return dispatcher.EndGroups
}

// Album modes. Telegram delivers every item of an album as its own message
// sharing one grouped_id, so forwarding "one video" out of a post that is
// actually an album brings the whole album along. These modes let the user
// decide what a forwarded album should save.
const (
	albumModeBatch   = "batch"   // save every item (upstream behaviour)
	albumModeFirst   = "first"   // save only the first item
	albumModeVideo   = "video"   // save only videos, skip photos
	albumModeLargest = "largest" // save only the biggest item
)

// filterAlbumItems picks the items of an album to save according to
// telegram.album_mode. Order is preserved; the returned slice may be empty.
func filterAlbumItems(items []tfile.TGFileMessage) ([]tfile.TGFileMessage, string) {
	mode := strings.ToLower(strings.TrimSpace(config.C().Telegram.AlbumMode))
	return filterAlbumItemsByMode(items, mode), mode
}

func filterAlbumItemsByMode(items []tfile.TGFileMessage, mode string) []tfile.TGFileMessage {
	switch mode {
	case albumModeFirst:
		if len(items) <= 1 {
			return items
		}
		return items[:1]
	case albumModeVideo:
		videos := make([]tfile.TGFileMessage, 0, len(items))
		for _, item := range items {
			if isVideoFile(item) {
				videos = append(videos, item)
			}
		}
		return videos
	case albumModeLargest:
		if len(items) <= 1 {
			return items
		}
		largest := items[0]
		for _, item := range items[1:] {
			if item.Size() > largest.Size() {
				largest = item
			}
		}
		return []tfile.TGFileMessage{largest}
	default:
		return items
	}
}

func isVideoFile(file tfile.TGFileMessage) bool {
	msg := file.Message()
	if msg == nil || msg.Media == nil {
		return false
	}
	docMedia, ok := msg.Media.(*tg.MessageMediaDocument)
	if !ok {
		return false
	}
	doc, ok := docMedia.Document.AsNotEmpty()
	if !ok {
		return false
	}
	for _, attr := range doc.Attributes {
		if _, ok := attr.(*tg.DocumentAttributeVideo); ok {
			return true
		}
	}
	return strings.HasPrefix(doc.MimeType, "video/")
}

func processMediaGroup(ctx *ext.Context, update *ext.Update, key mediaGroupKey) {
	logger := log.FromContext(ctx)
	mediaGroupHandler.mu.Lock()
	items := mediaGroupHandler.groups[key]
	delete(mediaGroupHandler.groups, key)
	delete(mediaGroupHandler.timers, key)
	mediaGroupHandler.mu.Unlock()
	if len(items) == 0 {
		logger.Warn("No media items to process for group", "groupID", key.groupID)
		return
	}
	logger.Debugf("Processing media group %d with %d items", key.groupID, len(items))
	msgIDs := make([]int, 0, len(items))
	for _, it := range items {
		if m := it.Message(); m != nil {
			msgIDs = append(msgIDs, m.GetID())
		}
	}
	logger.Infof("[album] flush chat=%d grouped=%d count=%d msg_ids=%v", key.chatID, key.groupID, len(items), msgIDs)

	received := len(items)
	items, mode := filterAlbumItems(items)
	if mode != albumModeBatch {
		logger.Infof("[album] album_mode=%s kept %d of %d items", mode, len(items), received)
	}
	if len(items) == 0 {
		ctx.Reply(update, ext.ReplyTextString(i18n.T(i18nk.BotMsgCommonErrorNoSavableFilesFound, nil)), nil)
		return
	}

	userId := update.GetUserChat().GetID()
	msg, err := ctx.Reply(update, ext.ReplyTextString(i18n.T(i18nk.BotMsgMediaGroupInfoSavingFiles, nil)), nil)
	if err != nil {
		logger.Errorf("Failed to reply: %s", err)
		return
	}
	stor := storage.FromContext(ctx)
	if stor != nil {
		// In silent mode
		if len(items) == 1 {
			shortcut.CreateAndAddTGFileTaskWithEdit(ctx, userId, stor, "", items[0], msg.ID)
			return
		}
		shortcut.CreateAndAddBatchTGFileTaskWithEdit(ctx, userId, stor, "", items, msg.ID)
		return
	}

	stors := storage.GetUserStorages(ctx, userId)
	markup, err := msgelem.BuildAddSelectStorageKeyboard(stors, tcbdata.Add{
		Files:   items,
		AsBatch: len(items) > 1,
	})
	if err != nil {
		logger.Errorf("Failed to build storage selection keyboard: %s", err)
		ctx.EditMessage(userId, &tg.MessagesEditMessageRequest{
			ID: msg.ID,
			Message: i18n.T(i18nk.BotMsgMediaGroupErrorBuildStorageSelectKeyboardFailed, map[string]any{
				"Error": err.Error(),
			}),
		})
		return
	}
	ctx.EditMessage(userId, &tg.MessagesEditMessageRequest{
		ID: msg.ID,
		Message: i18n.T(i18nk.BotMsgMediaGroupInfoGroupFoundFilesSelectStorage, map[string]any{
			"Count": len(items),
		}),
		ReplyMarkup: markup,
	})
}
