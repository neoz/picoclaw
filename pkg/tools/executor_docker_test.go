package tools

import (
	"testing"
	"time"
)

func TestNewDockerExecutor_Defaults(t *testing.T) {
	d := NewDockerExecutor("/workspace", 60*time.Second, DockerConfig{})

	if d.image != "picoclaw-sandbox:latest" {
		t.Errorf("image = %q, want picoclaw-sandbox:latest", d.image)
	}
	if d.memoryLimit != "256m" {
		t.Errorf("memoryLimit = %q, want 256m", d.memoryLimit)
	}
	if d.cpuLimit != "0.5" {
		t.Errorf("cpuLimit = %q, want 0.5", d.cpuLimit)
	}
	if d.pidsLimit != "100" {
		t.Errorf("pidsLimit = %q, want 100", d.pidsLimit)
	}
	if d.network != "none" {
		t.Errorf("network = %q, want none", d.network)
	}
	if !d.readOnlyRoot {
		t.Error("readOnlyRoot should default to true")
	}
	if d.volume != "" {
		t.Errorf("volume = %q, want empty", d.volume)
	}
	if d.workspaceDir != "/workspace" {
		t.Errorf("workspaceDir = %q, want /workspace", d.workspaceDir)
	}
	if d.timeout != 60*time.Second {
		t.Errorf("timeout = %v, want 60s", d.timeout)
	}
}

func TestNewDockerExecutor_CustomConfig(t *testing.T) {
	readOnly := false
	cfg := DockerConfig{
		Image:        "custom-image:v2",
		MemoryLimit:  "512m",
		CPULimit:     "1.0",
		PidsLimit:    "200",
		Network:      "bridge",
		ReadOnlyRoot: &readOnly,
		Volume:       "my-volume",
	}
	d := NewDockerExecutor("/mydir", 30*time.Second, cfg)

	if d.image != "custom-image:v2" {
		t.Errorf("image = %q", d.image)
	}
	if d.memoryLimit != "512m" {
		t.Errorf("memoryLimit = %q", d.memoryLimit)
	}
	if d.cpuLimit != "1.0" {
		t.Errorf("cpuLimit = %q", d.cpuLimit)
	}
	if d.pidsLimit != "200" {
		t.Errorf("pidsLimit = %q", d.pidsLimit)
	}
	if d.network != "bridge" {
		t.Errorf("network = %q", d.network)
	}
	if d.readOnlyRoot {
		t.Error("readOnlyRoot should be false")
	}
	if d.volume != "my-volume" {
		t.Errorf("volume = %q, want my-volume", d.volume)
	}
}

func TestNewDockerExecutor_ReadOnlyTrueExplicit(t *testing.T) {
	readOnly := true
	cfg := DockerConfig{ReadOnlyRoot: &readOnly}
	d := NewDockerExecutor("", 10*time.Second, cfg)
	if !d.readOnlyRoot {
		t.Error("expected readOnlyRoot = true when explicitly set")
	}
}

func TestNewDockerExecutor_PartialConfig(t *testing.T) {
	cfg := DockerConfig{
		Image:   "my-sandbox:1.0",
		Network: "host",
	}
	d := NewDockerExecutor("/ws", 45*time.Second, cfg)

	if d.image != "my-sandbox:1.0" {
		t.Errorf("image = %q", d.image)
	}
	if d.network != "host" {
		t.Errorf("network = %q", d.network)
	}
	if d.memoryLimit != "256m" {
		t.Errorf("memoryLimit = %q, want default 256m", d.memoryLimit)
	}
	if d.cpuLimit != "0.5" {
		t.Errorf("cpuLimit = %q, want default 0.5", d.cpuLimit)
	}
	if d.pidsLimit != "100" {
		t.Errorf("pidsLimit = %q, want default 100", d.pidsLimit)
	}
	if !d.readOnlyRoot {
		t.Error("readOnlyRoot should default to true")
	}
	if d.volume != "" {
		t.Errorf("volume = %q, want empty", d.volume)
	}
}

func TestDockerAvailable_NoError(t *testing.T) {
	_ = DockerAvailable()
}

// --- Volume mounting logic ---

func TestDockerExecutor_VolumeConfig(t *testing.T) {
	cfg := DockerConfig{Volume: "picoclaw-workspace"}
	d := NewDockerExecutor("/home/picoclaw/.picoclaw/workspace", 60*time.Second, cfg)

	if d.volume != "picoclaw-workspace" {
		t.Errorf("volume = %q, want picoclaw-workspace", d.volume)
	}
}
