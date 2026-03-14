package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// sandboxDockerfile is the embedded Dockerfile for the sandbox image.
// Kept as a constant so the binary is self-contained (no filesystem access needed).
const sandboxDockerfile = `FROM alpine:latest
RUN apk add --no-cache bash coreutils curl jq python3 git && adduser -D -s /bin/bash sandbox
RUN curl -LsSf https://astral.sh/uv/install.sh | sh && mv /root/.local/bin/uv /usr/local/bin/ && mv /root/.local/bin/uvx /usr/local/bin/
WORKDIR /home/sandbox
`

// InitSandbox initializes the global CommandExecutor based on config.
// It blocks until the sandbox is ready (image built + health check).
// Returns the executor and an error if sandbox: "docker" is set but Docker is unavailable.
func InitSandbox(workspaceDir string, timeout time.Duration, cfg config.ExecToolsConfig) (CommandExecutor, error) {
	mode := strings.ToLower(cfg.Sandbox)
	if mode == "" {
		mode = "auto"
	}

	if mode == "host" {
		logger.InfoCF("sandbox", "Sandbox mode: host (Docker disabled by config)", nil)
		return NewHostExecutor(workspaceDir, timeout, true), nil
	}

	// mode is "docker" or "auto"
	if !DockerAvailable() {
		if mode == "docker" {
			return nil, fmt.Errorf("sandbox mode is 'docker' but Docker is not available (not installed or daemon not running)")
		}
		logger.InfoCF("sandbox", "Docker not available, falling back to host executor", nil)
		return NewHostExecutor(workspaceDir, timeout, true), nil
	}

	// Docker is available -- ensure sandbox image exists
	image := cfg.Docker.Image
	if image == "" {
		image = "picoclaw-sandbox:latest"
	}

	if !imageExists(image) {
		logger.InfoCF("sandbox", fmt.Sprintf("Building sandbox image %s...", image), nil)
		if err := buildSandboxImage(image); err != nil {
			if mode == "docker" {
				return nil, fmt.Errorf("failed to build sandbox image: %w", err)
			}
			logger.ErrorCF("sandbox", "Failed to build sandbox image, falling back to host executor",
				map[string]interface{}{"error": err.Error()})
			return NewHostExecutor(workspaceDir, timeout, true), nil
		}
		logger.InfoCF("sandbox", fmt.Sprintf("Sandbox image %s built successfully", image), nil)
	}

	// Health check
	if err := healthCheck(image); err != nil {
		if mode == "docker" {
			return nil, fmt.Errorf("sandbox health check failed: %w", err)
		}
		logger.ErrorCF("sandbox", "Sandbox health check failed, falling back to host executor",
			map[string]interface{}{"error": err.Error()})
		return NewHostExecutor(workspaceDir, timeout, true), nil
	}

	// Auto-detect volume mode
	volume := cfg.Docker.Volume
	if volume == "" {
		volume = detectVolumeMode(workspaceDir)
	}

	dockerCfg := DockerConfig{
		Image:        cfg.Docker.Image,
		MemoryLimit:  cfg.Docker.MemoryLimit,
		CPULimit:     cfg.Docker.CPULimit,
		PidsLimit:    cfg.Docker.PidsLimit,
		Network:      cfg.Docker.Network,
		ReadOnlyRoot: cfg.Docker.ReadOnlyRoot,
		Volume:       volume,
	}

	executor := NewDockerExecutor(workspaceDir, timeout, dockerCfg)

	if volume != "" {
		logger.InfoCF("sandbox", fmt.Sprintf("Sandbox mode: docker (image=%s, volume=%s)", image, volume), nil)
	} else {
		logger.InfoCF("sandbox", fmt.Sprintf("Sandbox mode: docker (image=%s, bind-mount=%s)", image, workspaceDir), nil)
	}

	return executor, nil
}

// imageExists checks if a Docker image exists locally.
func imageExists(image string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "docker", "image", "inspect", image).Run() == nil
}

// buildSandboxImage builds the sandbox Docker image from the embedded Dockerfile.
func buildSandboxImage(image string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "build", "-t", image, "-")
	cmd.Stdin = strings.NewReader(sandboxDockerfile)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String())
	}
	return nil
}

// healthCheck verifies the sandbox image works by running a trivial command.
func healthCheck(image string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", image, "echo", "ok")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("health check failed: %w: %s", err, stderr.String())
	}

	if !strings.Contains(stdout.String(), "ok") {
		return fmt.Errorf("health check: unexpected output %q", stdout.String())
	}

	logger.InfoCF("sandbox", "Sandbox health check passed", nil)
	return nil
}

// detectVolumeMode checks if PicoClaw is running inside a Docker container.
// If so, inspects the container's own mounts to find the named volume at workspaceDir.
// Otherwise returns empty string for host bind-mount.
func detectVolumeMode(workspaceDir string) string {
	if _, err := os.Stat("/.dockerenv"); err != nil {
		return "" // not in Docker, use bind-mount
	}

	// Get our container ID from hostname (Docker sets hostname to short container ID)
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}

	// Inspect our own mounts to find the volume name at workspaceDir
	format := fmt.Sprintf(`{{range .Mounts}}{{if eq .Destination "%s"}}{{.Name}}{{end}}{{end}}`, workspaceDir)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "docker", "inspect", "--format", format, hostname).Output()
	if err != nil {
		logger.InfoCF("sandbox", "Could not detect volume name via docker inspect, falling back to bind-mount",
			map[string]interface{}{"error": err.Error()})
		return ""
	}

	volume := strings.TrimSpace(string(out))
	if volume != "" {
		logger.InfoCF("sandbox", fmt.Sprintf("Detected volume %q mounted at %s", volume, workspaceDir), nil)
	}
	return volume
}
