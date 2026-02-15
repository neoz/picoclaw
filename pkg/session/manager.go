package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/providers"
)

const messageLogRetentionDays = 7

type MessageLogEntry struct {
	Content   string    `json:"content"`
	SenderID  string    `json:"sender_id"`
	Timestamp time.Time `json:"timestamp"`
}

type Session struct {
	Key        string              `json:"key"`
	Messages   []providers.Message `json:"messages"`
	MessageLog []MessageLogEntry   `json:"message_log,omitempty"`
	Summary    string              `json:"summary,omitempty"`
	Created    time.Time           `json:"created"`
	Updated    time.Time           `json:"updated"`
}

// SanitizeSessionKey replaces characters illegal in filenames (e.g. ':' on Windows).
func SanitizeSessionKey(key string) string {
	return strings.ReplaceAll(key, ":", "_")
}

type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	storage  string
}

func NewSessionManager(storage string) *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*Session),
		storage:  storage,
	}

	if storage != "" {
		os.MkdirAll(storage, 0755)
		sm.loadSessions()
	}

	return sm
}

func (sm *SessionManager) GetOrCreate(key string) *Session {
	sm.mu.RLock()
	session, ok := sm.sessions[key]
	sm.mu.RUnlock()

	if !ok {
		sm.mu.Lock()
		// Double-check after acquiring write lock
		session, ok = sm.sessions[key]
		if !ok {
			session = &Session{
				Key:      key,
				Messages: []providers.Message{},
				Created:  time.Now(),
				Updated:  time.Now(),
			}
			sm.sessions[key] = session
		}
		sm.mu.Unlock()
	}

	return session
}

func (sm *SessionManager) AddMessage(sessionKey, role, content string) {
	sm.AddFullMessage(sessionKey, providers.Message{
		Role:    role,
		Content: content,
	})
}

// AddFullMessage adds a complete message with tool calls and tool call ID to the session.
// This is used to save the full conversation flow including tool calls and tool results.
func (sm *SessionManager) AddFullMessage(sessionKey string, msg providers.Message) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[sessionKey]
	if !ok {
		session = &Session{
			Key:      sessionKey,
			Messages: []providers.Message{},
			Created:  time.Now(),
		}
		sm.sessions[sessionKey] = session
	}

	session.Messages = append(session.Messages, msg)
	session.Updated = time.Now()
}

func (sm *SessionManager) GetHistory(key string) []providers.Message {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, ok := sm.sessions[key]
	if !ok {
		return []providers.Message{}
	}

	history := make([]providers.Message, len(session.Messages))
	copy(history, session.Messages)
	return history
}

func (sm *SessionManager) GetSummary(key string) string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, ok := sm.sessions[key]
	if !ok {
		return ""
	}
	return session.Summary
}

func (sm *SessionManager) SetSummary(key string, summary string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[key]
	if ok {
		session.Summary = summary
		session.Updated = time.Now()
	}
}

func (sm *SessionManager) TruncateHistory(key string, keepLast int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[key]
	if !ok {
		return
	}

	if len(session.Messages) <= keepLast {
		return
	}

	session.Messages = session.Messages[len(session.Messages)-keepLast:]
	session.Updated = time.Now()
}

func (sm *SessionManager) Save(session *Session) error {
	if sm.storage == "" {
		return nil
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	return sm.persistSession(session)
}

// persistSession writes a session to disk. Caller must hold sm.mu.
func (sm *SessionManager) persistSession(session *Session) error {
	if sm.storage == "" {
		return nil
	}

	sessionPath := filepath.Join(sm.storage, SanitizeSessionKey(session.Key)+".json")

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(sessionPath, data, 0644)
}

// AddToLog appends a message to the session's MessageLog and persists.
func (sm *SessionManager) AddToLog(key, content, senderID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, ok := sm.sessions[key]
	if !ok {
		session = &Session{
			Key:     key,
			Created: time.Now(),
		}
		sm.sessions[key] = session
	}

	session.MessageLog = append(session.MessageLog, MessageLogEntry{
		Content:   content,
		SenderID:  senderID,
		Timestamp: time.Now(),
	})
	session.Updated = time.Now()
	sm.persistSession(session)
}

// RecentLog returns the last `limit` log entries filtered by days and senderID.
func (sm *SessionManager) RecentLog(key string, limit, days int, senderID string) []MessageLogEntry {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, ok := sm.sessions[key]
	if !ok {
		return nil
	}

	filtered := filterLogEntries(session.MessageLog, days, senderID)
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	return filtered
}

// GetLog returns all log entries filtered by days and senderID (for BM25 search).
func (sm *SessionManager) GetLog(key string, days int, senderID string) []MessageLogEntry {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, ok := sm.sessions[key]
	if !ok {
		return nil
	}

	return filterLogEntries(session.MessageLog, days, senderID)
}

func filterLogEntries(entries []MessageLogEntry, days int, senderID string) []MessageLogEntry {
	if days <= 0 || days > messageLogRetentionDays {
		days = messageLogRetentionDays
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	var filtered []MessageLogEntry
	for _, e := range entries {
		if e.Timestamp.After(cutoff) && (senderID == "" || e.SenderID == senderID) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func (sm *SessionManager) loadSessions() error {
	files, err := os.ReadDir(sm.storage)
	if err != nil {
		return err
	}

	cutoff := time.Now().AddDate(0, 0, -messageLogRetentionDays)

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if filepath.Ext(file.Name()) != ".json" {
			continue
		}

		sessionPath := filepath.Join(sm.storage, file.Name())
		data, err := os.ReadFile(sessionPath)
		if err != nil {
			continue
		}

		var session Session
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}

		// Prune old MessageLog entries
		if len(session.MessageLog) > 0 {
			pruned := make([]MessageLogEntry, 0, len(session.MessageLog))
			for _, e := range session.MessageLog {
				if e.Timestamp.After(cutoff) {
					pruned = append(pruned, e)
				}
			}
			session.MessageLog = pruned
		}

		sm.sessions[session.Key] = &session
	}

	return nil
}
