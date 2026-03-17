package heartbeat

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/logger"
)

type HeartbeatService struct {
	workspace      string
	onHeartbeat    func(string) (string, error)
	interval       time.Duration
	enabled        bool
	started        bool
	mu             sync.RWMutex
	stopChan       chan struct{}
	deliverChannel string
	deliverChatID  string
	bus            *bus.MessageBus
}

func NewHeartbeatService(workspace string, intervalS int, enabled bool) *HeartbeatService {
	return &HeartbeatService{
		workspace: workspace,
		interval:  time.Duration(intervalS) * time.Second,
		enabled:   enabled,
		stopChan:  make(chan struct{}),
	}
}

func (hs *HeartbeatService) SetOnHeartbeat(fn func(string) (string, error)) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.onHeartbeat = fn
}

func (hs *HeartbeatService) SetDelivery(msgBus *bus.MessageBus, channel, chatID string) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.bus = msgBus
	hs.deliverChannel = channel
	hs.deliverChatID = chatID
}

func (hs *HeartbeatService) Start() error {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if hs.started {
		return nil
	}

	if !hs.enabled {
		return fmt.Errorf("heartbeat service is disabled")
	}

	hs.started = true
	go hs.runLoop()

	return nil
}

func (hs *HeartbeatService) Stop() {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if !hs.started {
		return
	}

	hs.started = false
	close(hs.stopChan)
}

func (hs *HeartbeatService) running() bool {
	select {
	case <-hs.stopChan:
		return false
	default:
		return true
	}
}

func (hs *HeartbeatService) runLoop() {
	// Calculate remaining time from last heartbeat to avoid resetting on restart
	remaining := hs.interval
	if last := hs.loadLastHeartbeat(); !last.IsZero() {
		elapsed := time.Since(last)
		if elapsed < hs.interval {
			remaining = hs.interval - elapsed
		} else {
			remaining = 0
		}
	} else {
		// First start: save current time as reference so restarts before the
		// first heartbeat fires still know when the interval began counting.
		hs.saveLastHeartbeat()
	}

	// Wait for the remaining partial interval first
	if remaining > 0 && remaining < hs.interval {
		logger.InfoCF("heartbeat", "Resuming partial interval", map[string]interface{}{
			"remaining": remaining.Round(time.Second).String(),
			"interval":  hs.interval.String(),
		})
	}
	if remaining > 0 {
		timer := time.NewTimer(remaining)
		select {
		case <-hs.stopChan:
			timer.Stop()
			return
		case <-timer.C:
			hs.checkHeartbeat()
		}
	} else {
		hs.checkHeartbeat()
	}

	// Then switch to regular ticker
	ticker := time.NewTicker(hs.interval)
	defer ticker.Stop()

	for {
		select {
		case <-hs.stopChan:
			return
		case <-ticker.C:
			hs.checkHeartbeat()
		}
	}
}

func (hs *HeartbeatService) checkHeartbeat() {
	hs.mu.RLock()
	if !hs.enabled || !hs.running() {
		hs.mu.RUnlock()
		return
	}
	onHeartbeat := hs.onHeartbeat
	msgBus := hs.bus
	channel := hs.deliverChannel
	chatID := hs.deliverChatID
	hs.mu.RUnlock()

	if onHeartbeat == nil {
		return
	}

	hs.saveLastHeartbeat()

	prompt := hs.buildPrompt()

	response, err := onHeartbeat(prompt)
	if err != nil {
		logger.ErrorCF("heartbeat", "Heartbeat callback error", map[string]interface{}{
			"error": err.Error(),
		})
		hs.log(fmt.Sprintf("Heartbeat error: %v", err))
		return
	}

	// If response contains HEARTBEAT_OK sentinel, no action needed
	if strings.Contains(response, "HEARTBEAT_OK") {
		logger.InfoCF("heartbeat", "Heartbeat OK, no action needed", nil)
		return
	}

	// Deliver response to configured channel
	if msgBus != nil && channel != "" && chatID != "" {
		msgBus.PublishOutbound(bus.OutboundMessage{
			Channel: channel,
			ChatID:  chatID,
			Content: response,
		})
		logger.InfoCF("heartbeat", "Heartbeat response delivered", map[string]interface{}{
			"channel": channel,
			"chat_id": chatID,
		})
	}
}

func (hs *HeartbeatService) buildPrompt() string {
	notesDir := filepath.Join(hs.workspace, "memory")
	notesFile := filepath.Join(notesDir, "HEARTBEAT.md")

	var notes string
	if data, err := os.ReadFile(notesFile); err == nil {
		notes = string(data)
	}

	now := time.Now().Format("2006-01-02 15:04")

	prompt := fmt.Sprintf(`# Heartbeat Check

Current time: %s

Check if there are any tasks I should be aware of or actions I should take.
Review the memory file for any important updates or changes.
Be proactive in identifying potential issues or improvements.

If there is nothing to report, respond with exactly: HEARTBEAT_OK

%s
`, now, notes)

	return prompt
}

func (hs *HeartbeatService) lastHeartbeatFile() string {
	return filepath.Join(hs.workspace, "memory", "heartbeat_last.txt")
}

func (hs *HeartbeatService) loadLastHeartbeat() time.Time {
	data, err := os.ReadFile(hs.lastHeartbeatFile())
	if err != nil {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(string(data)))
	if err != nil {
		return time.Time{}
	}
	return t
}

func (hs *HeartbeatService) saveLastHeartbeat() {
	dir := filepath.Dir(hs.lastHeartbeatFile())
	os.MkdirAll(dir, 0755)
	os.WriteFile(hs.lastHeartbeatFile(), []byte(time.Now().Format(time.RFC3339Nano)), 0644)
}

func (hs *HeartbeatService) log(message string) {
	logFile := filepath.Join(hs.workspace, "memory", "heartbeat.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	f.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, message))
}
