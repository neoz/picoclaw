package heartbeat

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func tempWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "memory"), 0755)
	return dir
}

func TestSaveAndLoadLastHeartbeat(t *testing.T) {
	ws := tempWorkspace(t)
	hs := NewHeartbeatService(ws, 60, true)

	// No file yet - should return zero time
	got := hs.loadLastHeartbeat()
	if !got.IsZero() {
		t.Fatalf("expected zero time, got %v", got)
	}

	// Save and reload
	hs.saveLastHeartbeat()
	got = hs.loadLastHeartbeat()
	if got.IsZero() {
		t.Fatal("expected non-zero time after save")
	}
	if time.Since(got) > 2*time.Second {
		t.Fatalf("loaded time too far in past: %v", got)
	}
}

func TestLoadLastHeartbeat_InvalidFile(t *testing.T) {
	ws := tempWorkspace(t)
	hs := NewHeartbeatService(ws, 60, true)

	// Write garbage to the file
	os.WriteFile(hs.lastHeartbeatFile(), []byte("not-a-timestamp"), 0644)

	got := hs.loadLastHeartbeat()
	if !got.IsZero() {
		t.Fatalf("expected zero time for invalid file, got %v", got)
	}
}

func TestRunLoop_ResumePartialInterval(t *testing.T) {
	ws := tempWorkspace(t)
	interval := 1 // 1 second

	// Simulate a heartbeat that happened 500ms ago
	hs := NewHeartbeatService(ws, interval, true)
	lastTime := time.Now().Add(-500 * time.Millisecond)
	os.WriteFile(hs.lastHeartbeatFile(), []byte(lastTime.Format(time.RFC3339Nano)), 0644)

	var callCount atomic.Int32
	hs.SetOnHeartbeat(func(prompt string) (string, error) {
		callCount.Add(1)
		return "HEARTBEAT_OK", nil
	})

	start := time.Now()
	hs.Start()
	defer hs.Stop()

	// Wait for first heartbeat - should fire after ~500ms, not 1s
	time.Sleep(800 * time.Millisecond)
	elapsed := time.Since(start)

	if callCount.Load() < 1 {
		t.Fatalf("expected heartbeat to fire within ~500ms (elapsed %v), but got %d calls", elapsed, callCount.Load())
	}
}

func TestRunLoop_ImmediateFireWhenOverdue(t *testing.T) {
	ws := tempWorkspace(t)
	interval := 1 // 1 second

	// Simulate a heartbeat that happened 5 seconds ago (well past interval)
	hs := NewHeartbeatService(ws, interval, true)
	lastTime := time.Now().Add(-5 * time.Second)
	os.WriteFile(hs.lastHeartbeatFile(), []byte(lastTime.Format(time.RFC3339Nano)), 0644)

	var callCount atomic.Int32
	hs.SetOnHeartbeat(func(prompt string) (string, error) {
		callCount.Add(1)
		return "HEARTBEAT_OK", nil
	})

	hs.Start()
	defer hs.Stop()

	// Should fire almost immediately
	time.Sleep(200 * time.Millisecond)

	if callCount.Load() < 1 {
		t.Fatal("expected immediate heartbeat when overdue, got 0 calls")
	}
}

func TestRunLoop_FullIntervalWithNoLastFile(t *testing.T) {
	ws := tempWorkspace(t)
	interval := 1 // 1 second

	hs := NewHeartbeatService(ws, interval, true)

	var callCount atomic.Int32
	hs.SetOnHeartbeat(func(prompt string) (string, error) {
		callCount.Add(1)
		return "HEARTBEAT_OK", nil
	})

	hs.Start()
	defer hs.Stop()

	// Should NOT fire before the full interval
	time.Sleep(500 * time.Millisecond)
	if callCount.Load() > 0 {
		t.Fatal("heartbeat fired too early with no last file")
	}

	// Should fire after the full interval
	time.Sleep(700 * time.Millisecond)
	if callCount.Load() < 1 {
		t.Fatal("expected heartbeat after full interval")
	}
}

func TestCheckHeartbeat_SavesTimestamp(t *testing.T) {
	ws := tempWorkspace(t)
	hs := NewHeartbeatService(ws, 60, true)

	hs.SetOnHeartbeat(func(prompt string) (string, error) {
		return "HEARTBEAT_OK", nil
	})

	// Verify no file before
	if !hs.loadLastHeartbeat().IsZero() {
		t.Fatal("expected no last heartbeat file initially")
	}

	hs.checkHeartbeat()

	// Verify file was created
	got := hs.loadLastHeartbeat()
	if got.IsZero() {
		t.Fatal("expected last heartbeat to be saved after checkHeartbeat")
	}
}

func TestStartDisabled(t *testing.T) {
	ws := tempWorkspace(t)
	hs := NewHeartbeatService(ws, 1, false)

	err := hs.Start()
	if err == nil {
		t.Fatal("expected error when starting disabled service")
	}
}

func TestStartIdempotent(t *testing.T) {
	ws := tempWorkspace(t)
	hs := NewHeartbeatService(ws, 60, true)
	hs.SetOnHeartbeat(func(string) (string, error) { return "HEARTBEAT_OK", nil })

	if err := hs.Start(); err != nil {
		t.Fatalf("first start failed: %v", err)
	}
	defer hs.Stop()

	// Second start should be a no-op
	if err := hs.Start(); err != nil {
		t.Fatalf("second start should not error: %v", err)
	}
}

func TestStopBeforeStart(t *testing.T) {
	ws := tempWorkspace(t)
	hs := NewHeartbeatService(ws, 60, true)

	// Should not panic
	hs.Stop()
}

func TestCheckHeartbeat_NilCallback(t *testing.T) {
	ws := tempWorkspace(t)
	hs := NewHeartbeatService(ws, 60, true)

	// No callback set - should not panic or save timestamp
	hs.checkHeartbeat()

	if !hs.loadLastHeartbeat().IsZero() {
		t.Fatal("should not save timestamp when callback is nil")
	}
}
