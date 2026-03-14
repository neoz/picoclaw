package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
)

// DockerExecutor runs commands inside a disposable Docker container with
// filesystem, network, and resource isolation.
type DockerExecutor struct {
	workspaceDir string
	timeout      time.Duration
	image        string
	memoryLimit  string
	cpuLimit     string
	pidsLimit    string
	network      string
	readOnlyRoot bool
	volume       string // Docker volume name; when set, mounts volume instead of bind-mount
	userSpec     string // "uid:gid" for --user flag, matched to workspace owner
}

// DockerConfig holds configuration for the Docker sandbox executor.
type DockerConfig struct {
	Image        string `json:"image" env:"PICOCLAW_TOOLS_EXEC_DOCKER_IMAGE"`
	MemoryLimit  string `json:"memory_limit" env:"PICOCLAW_TOOLS_EXEC_DOCKER_MEMORY_LIMIT"`
	CPULimit     string `json:"cpu_limit" env:"PICOCLAW_TOOLS_EXEC_DOCKER_CPU_LIMIT"`
	PidsLimit    string `json:"pids_limit" env:"PICOCLAW_TOOLS_EXEC_DOCKER_PIDS_LIMIT"`
	Network      string `json:"network" env:"PICOCLAW_TOOLS_EXEC_DOCKER_NETWORK"`
	ReadOnlyRoot *bool  `json:"read_only_root" env:"PICOCLAW_TOOLS_EXEC_DOCKER_READ_ONLY_ROOT"`
	Volume       string `json:"volume" env:"PICOCLAW_TOOLS_EXEC_SANDBOX_VOLUME"`
}

func NewDockerExecutor(workspaceDir string, timeout time.Duration, cfg DockerConfig) *DockerExecutor {
	image := cfg.Image
	if image == "" {
		image = "picoclaw-sandbox:latest"
	}
	memoryLimit := cfg.MemoryLimit
	if memoryLimit == "" {
		memoryLimit = "256m"
	}
	cpuLimit := cfg.CPULimit
	if cpuLimit == "" {
		cpuLimit = "0.5"
	}
	pidsLimit := cfg.PidsLimit
	if pidsLimit == "" {
		pidsLimit = "100"
	}
	network := cfg.Network
	if network == "" {
		network = "none"
	}
	readOnly := true
	if cfg.ReadOnlyRoot != nil {
		readOnly = *cfg.ReadOnlyRoot
	}

	return &DockerExecutor{
		workspaceDir: workspaceDir,
		timeout:      timeout,
		image:        image,
		memoryLimit:  memoryLimit,
		cpuLimit:     cpuLimit,
		pidsLimit:    pidsLimit,
		network:      network,
		readOnlyRoot: readOnly,
		volume:       cfg.Volume,
		userSpec:     detectWorkspaceOwner(workspaceDir),
	}
}

func (d *DockerExecutor) Execute(ctx context.Context, command, workingDir string) (string, error) {
	tmpDir := os.TempDir()
	args := []string{
		"run", "--rm",
		"--network", d.network,
		"--memory", d.memoryLimit,
		"--cpus", d.cpuLimit,
		"--pids-limit", d.pidsLimit,
		"-v", tmpDir + ":" + tmpDir + ":rw",
		"--tmpfs", "/home/sandbox:size=128m",
	}

	if d.readOnlyRoot {
		args = append(args, "--read-only")
	}

	// Mount workspace into sandbox container at the same path as the caller
	// (host or main container) so all tools see identical paths.
	// Volume mode: mount named volume at workspaceDir (not /workspace).
	// Bind-mount mode: mount host path at the same path inside container.
	containerWd := d.workspaceDir
	if d.volume != "" {
		args = append(args, "-v", d.volume+":"+d.workspaceDir+":rw")
	} else if d.workspaceDir != "" {
		args = append(args, "-v", d.workspaceDir+":"+d.workspaceDir+":rw")
	}
	if workingDir != "" {
		containerWd = workingDir
	}

	args = append(args, "-w", containerWd)

	// Run as the same UID:GID that owns the workspace so bind-mounted files
	// are readable/writable. HOME points to a writable tmpfs.
	// USER: set so Python/uv can resolve the current user without /etc/passwd lookup.
	// UV_CACHE_DIR: uv needs a writable cache dir (default ~/.cache/uv fails on read-only root)
	args = append(args, "--user", d.userSpec,
		"-e", "HOME=/home/sandbox",
		"-e", "USER=sandbox",
		"-e", "UV_CACHE_DIR=/tmp/uv-cache")

	args = append(args, d.image, "sh", "-c", command)

	logger.InfoCF("sandbox", fmt.Sprintf("docker %s", strings.Join(args, " ")),
		map[string]interface{}{
			"command":     command,
			"working_dir": containerWd,
			"image":       d.image,
			"network":     d.network,
			"volume":      d.volume,
		})

	cmdCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(cmdCtx, "docker", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)
	output := stdout.String()
	errOutput := stderr.String()
	if errOutput != "" {
		output += "\nSTDERR:\n" + errOutput
	}

	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			logger.InfoCF("sandbox", fmt.Sprintf("Command timed out after %v", d.timeout),
				map[string]interface{}{"command": command})
			return fmt.Sprintf("Error: Command timed out after %v", d.timeout), nil
		}
		output += fmt.Sprintf("\nExit code: %v", err)
		logger.InfoCF("sandbox", fmt.Sprintf("Command failed (took %dms)", duration.Milliseconds()),
			map[string]interface{}{
				"command":     command,
				"exit_error":  err.Error(),
				"stdout_len":  stdout.Len(),
				"stderr_len":  len(errOutput),
				"stderr":      truncate(errOutput, 500),
			})
	} else {
		logger.InfoCF("sandbox", fmt.Sprintf("Command completed (took %dms)", duration.Milliseconds()),
			map[string]interface{}{
				"command":    command,
				"stdout_len": stdout.Len(),
				"stderr_len": len(errOutput),
			})
	}

	// Hint the LLM to use web_fetch when network commands fail in the sandbox
	if d.network == "none" && err != nil && looksLikeNetworkCommand(command) {
		output += "\nNote: Network is disabled in the sandbox. Use the web_fetch tool instead for HTTP requests."
	}

	return output, nil
}

// looksLikeNetworkCommand checks if the command starts with a common network tool.
func looksLikeNetworkCommand(command string) bool {
	cmd := strings.TrimSpace(command)
	for _, prefix := range []string{"curl ", "curl\t", "wget ", "wget\t", "ping ", "nc ", "ncat "} {
		if strings.HasPrefix(cmd, prefix) {
			return true
		}
	}
	return false
}

// truncate returns the first n bytes of s, appending "..." if truncated.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// DockerAvailable returns true if Docker is installed and the daemon is reachable.
func DockerAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "docker", "info").Run() == nil
}
