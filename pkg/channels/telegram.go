package channels

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoapi"
	tu "github.com/mymmrac/telego/telegoutil"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/voice"
)

type TelegramChannel struct {
	*BaseChannel
	bot           *telego.Bot
	config        config.TelegramConfig
	chatIDs       sync.Map // string -> int64
	updates       <-chan telego.Update
	cancelPolling context.CancelFunc
	transcriber   *voice.GroqTranscriber
	placeholders  sync.Map // chatID -> messageID
	tempAllows    sync.Map // "chatID:username" -> time.Time (expiry)
	botUsername   string
	botID         int64
}

func NewTelegramChannel(cfg config.TelegramConfig, bus *bus.MessageBus) (*TelegramChannel, error) {
	bot, err := telego.NewBot(cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}

	base := NewBaseChannel("telegram", cfg, bus, cfg.AllowFrom)

	return &TelegramChannel{
		BaseChannel: base,
		bot:         bot,
		config:      cfg,
		transcriber: nil,
	}, nil
}

func (c *TelegramChannel) SetTranscriber(transcriber *voice.GroqTranscriber) {
	c.transcriber = transcriber
}

func (c *TelegramChannel) Start(ctx context.Context) error {
	log.Printf("Starting Telegram bot (polling mode)...")

	c.setRunning(true)

	botInfo, err := c.bot.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("failed to get bot info: %w", err)
	}
	c.botUsername = botInfo.Username
	c.botID = botInfo.ID
	log.Printf("Telegram bot @%s connected", botInfo.Username)

	pollCtx, cancel := context.WithCancel(ctx)
	c.cancelPolling = cancel

	updates, err := c.bot.UpdatesViaLongPolling(pollCtx, &telego.GetUpdatesParams{Timeout: 30})
	if err != nil {
		cancel()
		return fmt.Errorf("failed to start long polling: %w", err)
	}
	c.updates = updates

	go func() {
		for update := range updates {
			if update.Message != nil {
				c.handleMessage(ctx, update)
			}
		}
		log.Printf("Telegram updates channel closed")
	}()

	return nil
}

func (c *TelegramChannel) Stop(ctx context.Context) error {
	log.Println("Stopping Telegram bot...")
	c.setRunning(false)

	if c.cancelPolling != nil {
		c.cancelPolling()
		c.cancelPolling = nil
	}

	return nil
}

// sendWithRetry retries a Telegram API call on rate limit (429) errors.
func (c *TelegramChannel) sendWithRetry(fn func() error) error {
	const maxRetries = 3
	for i := 0; i <= maxRetries; i++ {
		err := fn()
		if err == nil {
			return nil
		}
		var tgErr *telegoapi.Error
		if errors.As(err, &tgErr) && tgErr.Parameters != nil && tgErr.Parameters.RetryAfter > 0 {
			wait := time.Duration(tgErr.Parameters.RetryAfter) * time.Second
			log.Printf("Telegram rate limited, retrying after %d seconds (attempt %d/%d)", tgErr.Parameters.RetryAfter, i+1, maxRetries)
			time.Sleep(wait)
			continue
		}
		return err
	}
	return fmt.Errorf("telegram rate limit: max retries exceeded")
}

func (c *TelegramChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	if !c.IsRunning() {
		return fmt.Errorf("telegram bot not running")
	}

	chatID, err := parseChatID(msg.ChatID)
	if err != nil {
		return fmt.Errorf("invalid chat ID: %w", err)
	}

	// Handle reaction messages
	if msg.Metadata["type"] == "reaction" {
		if msgID, err := strconv.Atoi(msg.Metadata["message_id"]); err == nil {
			emoji := msg.Metadata["emoji"]
			if emoji == "" {
				emoji = "\U0001F440"
			}
			c.reactToMessage(ctx, chatID, msgID, emoji)
		}
		return nil
	}

	htmlContent := markdownToTelegramHTML(msg.Content)

	// Try to edit placeholder
	if pID, ok := c.placeholders.Load(msg.ChatID); ok {
		c.placeholders.Delete(msg.ChatID)
		editParams := &telego.EditMessageTextParams{
			ChatID:    tu.ID(chatID),
			MessageID: pID.(int),
			Text:      htmlContent,
			ParseMode: telego.ModeHTML,
		}

		editErr := c.sendWithRetry(func() error {
			_, e := c.bot.EditMessageText(ctx, editParams)
			return e
		})
		if editErr == nil {
			return nil
		}
		// Fallback to new message if edit fails
	}

	params := &telego.SendMessageParams{
		ChatID:    tu.ID(chatID),
		Text:      htmlContent,
		ParseMode: telego.ModeHTML,
	}

	sendErr := c.sendWithRetry(func() error {
		_, e := c.bot.SendMessage(ctx, params)
		return e
	})
	if sendErr != nil {
		log.Printf("HTML parse failed, falling back to plain text: %v", sendErr)
		plainParams := &telego.SendMessageParams{
			ChatID: tu.ID(chatID),
			Text:   msg.Content,
		}
		return c.sendWithRetry(func() error {
			_, e := c.bot.SendMessage(ctx, plainParams)
			return e
		})
	}

	return nil
}

func (c *TelegramChannel) handleMessage(ctx context.Context, update telego.Update) {
	message := update.Message
	if message == nil {
		return
	}

	user := message.From
	if user == nil {
		return
	}

	senderID := fmt.Sprintf("%d", user.ID)
	if user.Username != "" {
		senderID = fmt.Sprintf("%d|%s", user.ID, user.Username)
	}

	chatID := message.Chat.ID
	c.chatIDs.Store(senderID, chatID)

	content := ""
	mediaPaths := []string{}

	if message.Text != "" {
		content += message.Text
	}

	if message.Caption != "" {
		if content != "" {
			content += "\n"
		}
		content += message.Caption
	}

	if message.Photo != nil && len(message.Photo) > 0 {
		photo := message.Photo[len(message.Photo)-1]
		photoPath := c.downloadPhoto(ctx, photo.FileID)
		if photoPath != "" {
			mediaPaths = append(mediaPaths, photoPath)
			if content != "" {
				content += "\n"
			}
			content += fmt.Sprintf("[image: %s]", photoPath)
		}
	}

	if message.Voice != nil {
		voicePath := c.downloadFile(ctx, message.Voice.FileID, ".ogg")
		if voicePath != "" {
			mediaPaths = append(mediaPaths, voicePath)

			transcribedText := ""
			if c.transcriber != nil && c.transcriber.IsAvailable() {
				tctx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()

				result, err := c.transcriber.Transcribe(tctx, voicePath)
				if err != nil {
					log.Printf("Voice transcription failed: %v", err)
					transcribedText = fmt.Sprintf("[voice: %s (transcription failed)]", voicePath)
				} else {
					transcribedText = fmt.Sprintf("[voice transcription: %s]", result.Text)
					log.Printf("Voice transcribed successfully: %s", result.Text)
				}
			} else {
				transcribedText = fmt.Sprintf("[voice: %s]", voicePath)
			}

			if content != "" {
				content += "\n"
			}
			content += transcribedText
		}
	}

	if message.Audio != nil {
		audioPath := c.downloadFile(ctx, message.Audio.FileID, ".mp3")
		if audioPath != "" {
			mediaPaths = append(mediaPaths, audioPath)
			if content != "" {
				content += "\n"
			}
			content += fmt.Sprintf("[audio: %s]", audioPath)
		}
	}

	if message.Document != nil {
		docPath := c.downloadFile(ctx, message.Document.FileID, "")
		if docPath != "" {
			mediaPaths = append(mediaPaths, docPath)
			if content != "" {
				content += "\n"
			}
			content += fmt.Sprintf("[file: %s]", docPath)
		}
	}

	if content == "" {
		content = "[empty message]"
	}

	log.Printf("Telegram message from %s: %s...", senderID, truncateString(content, 50))

	isGroup := message.Chat.Type != "private"
	allowed := c.IsAllowed(senderID)

	if !allowed && !isGroup {
		log.Printf("Telegram message from %s: not in allow list, ignoring", senderID)
		return
	}

	// In groups, only respond if bot is @mentioned or message is a reply to the bot
	if isGroup {
		mentioned := false
		for _, e := range message.Entities {
			if e.Type == "mention" {
				name := extractEntityText(message.Text, e.Offset+1, e.Length-1)
				if strings.EqualFold(name, c.botUsername) {
					mentioned = true
					break
				}
			}
		}
		isReplyToBot := message.ReplyToMessage != nil &&
			message.ReplyToMessage.From != nil &&
			message.ReplyToMessage.From.ID == c.botID

		if !mentioned && !isReplyToBot {
			c.publishObserveOnly(senderID, chatID, message.MessageID, user, content, mediaPaths)
			return
		}

		// Bot is mentioned or replied to - check temp allow for non-allowed users
		if !allowed {
			if c.consumeTempAllow(chatID, user) {
				allowed = true
				log.Printf("Telegram message from %s: one-time temp allow granted", senderID)
			} else {
				log.Printf("Telegram message from %s: not in allow list, ignoring", senderID)
				c.publishObserveOnly(senderID, chatID, message.MessageID, user, content, mediaPaths)
				return
			}
		}

		// Strip @botname from content
		content = strings.ReplaceAll(content, "@"+c.botUsername, "")
		content = strings.TrimSpace(content)
		if content == "" {
			content = "[empty message]"
		}
	}

	// Grant one-time temp allow for other users mentioned in this message
	// Only permanently allowed users can grant temp access
	if isGroup && c.IsAllowed(senderID) {
		c.grantTempAllows(chatID, message)
	}

	// React to sender message to acknowledge receipt
	c.reactToMessage(ctx, chatID, message.MessageID, "\U0001F440")

	// Typing indicator + placeholder message (edited with final response)
	c.sendWithRetry(func() error {
		return c.bot.SendChatAction(ctx, tu.ChatAction(tu.ID(chatID), telego.ChatActionTyping))
	})

	chatIDStr := fmt.Sprintf("%d", chatID)
	var pMsg *telego.Message
	sendErr := c.sendWithRetry(func() error {
		var e error
		pMsg, e = c.bot.SendMessage(ctx, tu.Message(tu.ID(chatID), "Thinking..."))
		return e
	})
	if sendErr == nil && pMsg != nil {
		c.placeholders.Store(chatIDStr, pMsg.MessageID)
	}

	metadata := map[string]string{
		"message_id": fmt.Sprintf("%d", message.MessageID),
		"user_id":    fmt.Sprintf("%d", user.ID),
		"username":   user.Username,
		"first_name": user.FirstName,
		"is_group":   fmt.Sprintf("%t", isGroup),
	}

	sessionKey := fmt.Sprintf("%s:%s", c.name, chatIDStr)
	c.bus.PublishInbound(bus.InboundMessage{
		Channel:    c.name,
		SenderID:   senderID,
		ChatID:     chatIDStr,
		Content:    content,
		Media:      mediaPaths,
		SessionKey: sessionKey,
		Metadata:   metadata,
	})
}

func (c *TelegramChannel) downloadPhoto(ctx context.Context, fileID string) string {
	file, err := c.bot.GetFile(ctx, &telego.GetFileParams{FileID: fileID})
	if err != nil {
		log.Printf("Failed to get photo file: %v", err)
		return ""
	}

	return c.downloadFileWithInfo(file, ".jpg")
}

func (c *TelegramChannel) downloadFileWithInfo(file *telego.File, ext string) string {
	if file.FilePath == "" {
		return ""
	}

	url := c.bot.FileDownloadURL(file.FilePath)

	mediaDir := filepath.Join(os.TempDir(), "picoclaw_media")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		log.Printf("Failed to create media directory: %v", err)
		return ""
	}

	localPath := filepath.Join(mediaDir, file.FilePath[:min(16, len(file.FilePath))]+ext)

	if err := c.downloadFromURL(url, localPath); err != nil {
		log.Printf("Failed to download file: %v", err)
		return ""
	}

	return localPath
}

func (c *TelegramChannel) downloadFromURL(url, localPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	out, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	log.Printf("File downloaded successfully to: %s", localPath)
	return nil
}

func (c *TelegramChannel) downloadFile(ctx context.Context, fileID, ext string) string {
	file, err := c.bot.GetFile(ctx, &telego.GetFileParams{FileID: fileID})
	if err != nil {
		log.Printf("Failed to get file: %v", err)
		return ""
	}

	if file.FilePath == "" {
		return ""
	}

	url := c.bot.FileDownloadURL(file.FilePath)

	mediaDir := filepath.Join(os.TempDir(), "picoclaw_media")
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		log.Printf("Failed to create media directory: %v", err)
		return ""
	}

	localPath := filepath.Join(mediaDir, fileID[:16]+ext)

	if err := c.downloadFromURL(url, localPath); err != nil {
		log.Printf("Failed to download file: %v", err)
		return ""
	}

	return localPath
}

const tempAllowTTL = 30 * time.Minute

// grantTempAllows extracts @mentioned users from an allowed user's message
// and grants them one-time access to interact with the bot in this chat.
func (c *TelegramChannel) grantTempAllows(chatID int64, message *telego.Message) {
	for _, e := range message.Entities {
		switch e.Type {
		case "mention":
			name := extractEntityText(message.Text, e.Offset+1, e.Length-1)
			if strings.EqualFold(name, c.botUsername) {
				continue
			}
			key := fmt.Sprintf("%d:@%s", chatID, strings.ToLower(name))
			c.tempAllows.Store(key, time.Now().Add(tempAllowTTL))
			log.Printf("Temp allow granted for @%s in chat %d", name, chatID)
		case "text_mention":
			if e.User == nil {
				continue
			}
			key := fmt.Sprintf("%d:%d", chatID, e.User.ID)
			c.tempAllows.Store(key, time.Now().Add(tempAllowTTL))
			log.Printf("Temp allow granted for user %d in chat %d", e.User.ID, chatID)
		}
	}
}

// consumeTempAllow checks if a user has a one-time temp allow and consumes it.
func (c *TelegramChannel) consumeTempAllow(chatID int64, user *telego.User) bool {
	// Try by username first
	if user.Username != "" {
		key := fmt.Sprintf("%d:@%s", chatID, strings.ToLower(user.Username))
		if expiry, ok := c.tempAllows.LoadAndDelete(key); ok {
			if time.Now().Before(expiry.(time.Time)) {
				return true
			}
		}
	}
	// Try by user ID (for text_mention grants)
	key := fmt.Sprintf("%d:%d", chatID, user.ID)
	if expiry, ok := c.tempAllows.LoadAndDelete(key); ok {
		if time.Now().Before(expiry.(time.Time)) {
			return true
		}
	}
	return false
}

func (c *TelegramChannel) publishObserveOnly(senderID string, chatID int64, messageID int, user *telego.User, content string, mediaPaths []string) {
	chatIDStr := fmt.Sprintf("%d", chatID)
	sessionKey := fmt.Sprintf("%s:%s", c.name, chatIDStr)

	c.bus.PublishInbound(bus.InboundMessage{
		Channel:    c.name,
		SenderID:   senderID,
		ChatID:     chatIDStr,
		Content:    content,
		Media:      mediaPaths,
		SessionKey: sessionKey,
		Metadata: map[string]string{
			"message_id":   fmt.Sprintf("%d", messageID),
			"user_id":      fmt.Sprintf("%d", user.ID),
			"username":     user.Username,
			"first_name":   user.FirstName,
			"is_group":     "true",
			"observe_only": "true",
		},
	})
}

func (c *TelegramChannel) reactToMessage(ctx context.Context, chatID int64, messageID int, emoji string) {
	err := c.bot.SetMessageReaction(ctx, &telego.SetMessageReactionParams{
		ChatID:    tu.ID(chatID),
		MessageID: messageID,
		Reaction: []telego.ReactionType{
			&telego.ReactionTypeEmoji{
				Type:  "emoji",
				Emoji: emoji,
			},
		},
	})
	if err != nil {
		log.Printf("Failed to react to message: %v", err)
	}
}

func parseChatID(chatIDStr string) (int64, error) {
	var id int64
	_, err := fmt.Sscanf(chatIDStr, "%d", &id)
	return id, err
}

// extractEntityText extracts text from a Telegram message using UTF-16 offsets.
// Telegram entity Offset/Length are in UTF-16 code units, not UTF-8 bytes.
func extractEntityText(text string, offset, length int) string {
	units := utf16.Encode([]rune(text))
	if offset+length > len(units) {
		return ""
	}
	return string(utf16.Decode(units[offset : offset+length]))
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func markdownToTelegramHTML(text string) string {
	if text == "" {
		return ""
	}

	codeBlocks := extractCodeBlocks(text)
	text = codeBlocks.text

	inlineCodes := extractInlineCodes(text)
	text = inlineCodes.text

	text = regexp.MustCompile(`^#{1,6}\s+(.+)$`).ReplaceAllString(text, "$1")

	text = regexp.MustCompile(`^>\s*(.*)$`).ReplaceAllString(text, "$1")

	text = escapeHTML(text)

	text = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`).ReplaceAllString(text, `<a href="$2">$1</a>`)

	text = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllString(text, "<b>$1</b>")

	text = regexp.MustCompile(`__(.+?)__`).ReplaceAllString(text, "<b>$1</b>")

	reItalic := regexp.MustCompile(`_([^_]+)_`)
	text = reItalic.ReplaceAllStringFunc(text, func(s string) string {
		match := reItalic.FindStringSubmatch(s)
		if len(match) < 2 {
			return s
		}
		return "<i>" + match[1] + "</i>"
	})

	text = regexp.MustCompile(`~~(.+?)~~`).ReplaceAllString(text, "<s>$1</s>")

	text = regexp.MustCompile(`^[-*]\s+`).ReplaceAllString(text, "\u2022 ")

	for i, code := range inlineCodes.codes {
		escaped := escapeHTML(code)
		text = strings.ReplaceAll(text, fmt.Sprintf("\x00IC%d\x00", i), fmt.Sprintf("<code>%s</code>", escaped))
	}

	for i, code := range codeBlocks.codes {
		escaped := escapeHTML(code)
		text = strings.ReplaceAll(text, fmt.Sprintf("\x00CB%d\x00", i), fmt.Sprintf("<pre><code>%s</code></pre>", escaped))
	}

	return text
}

type codeBlockMatch struct {
	text  string
	codes []string
}

func extractCodeBlocks(text string) codeBlockMatch {
	re := regexp.MustCompile("```[\\w]*\\n?([\\s\\S]*?)```")
	matches := re.FindAllStringSubmatch(text, -1)

	codes := make([]string, 0, len(matches))
	for _, match := range matches {
		codes = append(codes, match[1])
	}

	text = re.ReplaceAllStringFunc(text, func(m string) string {
		return fmt.Sprintf("\x00CB%d\x00", len(codes)-1)
	})

	return codeBlockMatch{text: text, codes: codes}
}

type inlineCodeMatch struct {
	text  string
	codes []string
}

func extractInlineCodes(text string) inlineCodeMatch {
	re := regexp.MustCompile("`([^`]+)`")
	matches := re.FindAllStringSubmatch(text, -1)

	codes := make([]string, 0, len(matches))
	for _, match := range matches {
		codes = append(codes, match[1])
	}

	text = re.ReplaceAllStringFunc(text, func(m string) string {
		return fmt.Sprintf("\x00IC%d\x00", len(codes)-1)
	})

	return inlineCodeMatch{text: text, codes: codes}
}

func escapeHTML(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	return text
}
