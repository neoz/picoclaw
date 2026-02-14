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

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/voice"
)

type TelegramChannel struct {
	*BaseChannel
	bot          *tgbotapi.BotAPI
	config       config.TelegramConfig
	chatIDs      sync.Map // string -> int64
	updates      tgbotapi.UpdatesChannel
	transcriber  *voice.GroqTranscriber
	placeholders sync.Map // chatID -> messageID
	tempAllows   sync.Map // "chatID:username" -> time.Time (expiry)
	botUsername  string
	botID        int64
}

func NewTelegramChannel(cfg config.TelegramConfig, bus *bus.MessageBus) (*TelegramChannel, error) {
	bot, err := tgbotapi.NewBotAPI(cfg.Token)
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

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	updates := c.bot.GetUpdatesChan(u)
	c.updates = updates

	c.setRunning(true)

	botInfo, err := c.bot.GetMe()
	if err != nil {
		return fmt.Errorf("failed to get bot info: %w", err)
	}
	c.botUsername = botInfo.UserName
	c.botID = botInfo.ID
	log.Printf("Telegram bot @%s connected", botInfo.UserName)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case update, ok := <-updates:
				if !ok {
					log.Printf("Updates channel closed, reconnecting...")
					return
				}
				if update.Message != nil {
					c.handleMessage(update)
				}
			}
		}
	}()

	return nil
}

func (c *TelegramChannel) Stop(ctx context.Context) error {
	log.Println("Stopping Telegram bot...")
	c.setRunning(false)

	if c.updates != nil {
		c.bot.StopReceivingUpdates()
		c.updates = nil
	}

	return nil
}

// sendWithRetry sends a Telegram Chattable and retries on rate limit (429) errors.
func (c *TelegramChannel) sendWithRetry(msg tgbotapi.Chattable) (tgbotapi.Message, error) {
	const maxRetries = 3
	for i := 0; i <= maxRetries; i++ {
		resp, err := c.bot.Send(msg)
		if err == nil {
			return resp, nil
		}
		var tgErr *tgbotapi.Error
		if errors.As(err, &tgErr) && tgErr.RetryAfter > 0 {
			wait := time.Duration(tgErr.RetryAfter) * time.Second
			log.Printf("Telegram rate limited, retrying after %d seconds (attempt %d/%d)", tgErr.RetryAfter, i+1, maxRetries)
			time.Sleep(wait)
			continue
		}
		return resp, err
	}
	return tgbotapi.Message{}, fmt.Errorf("telegram rate limit: max retries exceeded")
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
			c.reactToMessage(chatID, msgID, emoji)
		}
		return nil
	}

	htmlContent := markdownToTelegramHTML(msg.Content)

	// Try to edit placeholder
	if pID, ok := c.placeholders.Load(msg.ChatID); ok {
		c.placeholders.Delete(msg.ChatID)
		editMsg := tgbotapi.NewEditMessageText(chatID, pID.(int), htmlContent)
		editMsg.ParseMode = tgbotapi.ModeHTML

		if _, err := c.sendWithRetry(editMsg); err == nil {
			return nil
		}
		// Fallback to new message if edit fails
	}

	tgMsg := tgbotapi.NewMessage(chatID, htmlContent)
	tgMsg.ParseMode = tgbotapi.ModeHTML

	if _, err := c.sendWithRetry(tgMsg); err != nil {
		log.Printf("HTML parse failed, falling back to plain text: %v", err)
		tgMsg = tgbotapi.NewMessage(chatID, msg.Content)
		tgMsg.ParseMode = ""
		_, err = c.sendWithRetry(tgMsg)
		return err
	}

	return nil
}

func (c *TelegramChannel) handleMessage(update tgbotapi.Update) {
	message := update.Message
	if message == nil {
		return
	}

	user := message.From
	if user == nil {
		return
	}

	senderID := fmt.Sprintf("%d", user.ID)
	if user.UserName != "" {
		senderID = fmt.Sprintf("%d|%s", user.ID, user.UserName)
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
		photoPath := c.downloadPhoto(photo.FileID)
		if photoPath != "" {
			mediaPaths = append(mediaPaths, photoPath)
			if content != "" {
				content += "\n"
			}
			content += fmt.Sprintf("[image: %s]", photoPath)
		}
	}

	if message.Voice != nil {
		voicePath := c.downloadFile(message.Voice.FileID, ".ogg")
		if voicePath != "" {
			mediaPaths = append(mediaPaths, voicePath)

			transcribedText := ""
			if c.transcriber != nil && c.transcriber.IsAvailable() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				result, err := c.transcriber.Transcribe(ctx, voicePath)
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
		audioPath := c.downloadFile(message.Audio.FileID, ".mp3")
		if audioPath != "" {
			mediaPaths = append(mediaPaths, audioPath)
			if content != "" {
				content += "\n"
			}
			content += fmt.Sprintf("[audio: %s]", audioPath)
		}
	}

	if message.Document != nil {
		docPath := c.downloadFile(message.Document.FileID, "")
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
	c.reactToMessage(chatID, message.MessageID, "\U0001F440")

	// Typing indicator + placeholder message (edited with final response)
	c.sendWithRetry(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))

	chatIDStr := fmt.Sprintf("%d", chatID)
	pMsg, err := c.sendWithRetry(tgbotapi.NewMessage(chatID, "Thinking..."))
	if err == nil {
		c.placeholders.Store(chatIDStr, pMsg.MessageID)
	}

	metadata := map[string]string{
		"message_id": fmt.Sprintf("%d", message.MessageID),
		"user_id":    fmt.Sprintf("%d", user.ID),
		"username":   user.UserName,
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

func (c *TelegramChannel) downloadPhoto(fileID string) string {
	file, err := c.bot.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		log.Printf("Failed to get photo file: %v", err)
		return ""
	}

	return c.downloadFileWithInfo(&file, ".jpg")
}

func (c *TelegramChannel) downloadFileWithInfo(file *tgbotapi.File, ext string) string {
	if file.FilePath == "" {
		return ""
	}

	url := file.Link(c.bot.Token)

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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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

func (c *TelegramChannel) downloadFile(fileID, ext string) string {
	file, err := c.bot.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		log.Printf("Failed to get file: %v", err)
		return ""
	}

	if file.FilePath == "" {
		return ""
	}

	url := file.Link(c.bot.Token)

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
func (c *TelegramChannel) grantTempAllows(chatID int64, message *tgbotapi.Message) {
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
func (c *TelegramChannel) consumeTempAllow(chatID int64, user *tgbotapi.User) bool {
	// Try by username first
	if user.UserName != "" {
		key := fmt.Sprintf("%d:@%s", chatID, strings.ToLower(user.UserName))
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

func (c *TelegramChannel) publishObserveOnly(senderID string, chatID int64, messageID int, user *tgbotapi.User, content string, mediaPaths []string) {
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
			"username":     user.UserName,
			"first_name":   user.FirstName,
			"is_group":     "true",
			"observe_only": "true",
		},
	})
}

func (c *TelegramChannel) reactToMessage(chatID int64, messageID int, emoji string) {
	params := tgbotapi.Params{}
	params.AddNonZero64("chat_id", chatID)
	params.AddNonZero("message_id", messageID)
	params.AddInterface("reaction", []map[string]string{
		{"type": "emoji", "emoji": emoji},
	})
	if _, err := c.bot.MakeRequest("setMessageReaction", params); err != nil {
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

	text = regexp.MustCompile(`^[-*]\s+`).ReplaceAllString(text, "• ")

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
