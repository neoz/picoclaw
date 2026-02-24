package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type ExecTool struct {
	workingDir          string
	timeout             time.Duration
	denyPatterns        []*regexp.Regexp
	allowPatterns       []*regexp.Regexp
	restrictToWorkspace bool
}

func NewExecTool(workingDir string) *ExecTool {
	denyPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\brm\s+-[rf]{1,2}\b`),
		regexp.MustCompile(`\bdel\s+/[fq]\b`),
		regexp.MustCompile(`\brmdir\s+/s\b`),
		regexp.MustCompile(`\b(format|mkfs|diskpart)\b\s`), // Match disk wiping commands (must be followed by space/args)
		regexp.MustCompile(`\bdd\s+if=`),
		regexp.MustCompile(`>\s*/dev/sd[a-z]\b`),            // Block writes to disk devices (but allow /dev/null)
		regexp.MustCompile(`\b(shutdown|reboot|poweroff)\b`),
		regexp.MustCompile(`:\(\)\s*\{.*\};\s*:`),
		// Sensitive file access patterns
		regexp.MustCompile(`\.picoclaw/config\b`),                             // picoclaw config (contains API keys)
		regexp.MustCompile(`/etc/(shadow|gshadow|master\.passwd)\b`),          // password databases
		regexp.MustCompile(`/\.(ssh|gnupg)/`),                                 // SSH and GPG keys
		regexp.MustCompile(`\.(pem|p12|pfx|key|keystore|jks)\b`),             // private key files
		regexp.MustCompile(`\bcurl\b.*\b(--data|--upload-file|-d|-F|-T)\b`),   // data exfiltration via curl
		regexp.MustCompile(`\bwget\b.*\b--post-(data|file)\b`),               // data exfiltration via wget
	}

	return &ExecTool{
		workingDir:          workingDir,
		timeout:             60 * time.Second,
		denyPatterns:        denyPatterns,
		allowPatterns:       nil,
		restrictToWorkspace: true,
	}
}

func (t *ExecTool) Name() string {
	return "exec"
}

func (t *ExecTool) Description() string {
	return "Execute a shell command within the workspace directory. Commands accessing paths outside the workspace are blocked."
}

func (t *ExecTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "The shell command to execute",
			},
			"working_dir": map[string]interface{}{
				"type":        "string",
				"description": "Optional working directory for the command",
			},
		},
		"required": []string{"command"},
	}
}

func (t *ExecTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	command, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("command is required")
	}

	cwd := t.workingDir
	if wd, ok := args["working_dir"].(string); ok && wd != "" {
		if t.restrictToWorkspace && t.workingDir != "" {
			// Validate working_dir is within the workspace
			absWd, err := filepath.Abs(wd)
			if err == nil {
				absWorkspace, err := filepath.Abs(t.workingDir)
				if err == nil {
					rel, err := filepath.Rel(absWorkspace, absWd)
					if err != nil || strings.HasPrefix(rel, "..") {
						return "Error: Command blocked by safety guard (working_dir outside workspace)", nil
					}
				}
			}
		}
		cwd = wd
	}

	if cwd == "" {
		wd, err := os.Getwd()
		if err == nil {
			cwd = wd
		}
	}

	if guardError := t.guardCommand(command, cwd); guardError != "" {
		return fmt.Sprintf("Error: %s", guardError), nil
	}

	cmdCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", command)
	if cwd != "" {
		cmd.Dir = cwd
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\nSTDERR:\n" + stderr.String()
	}

	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return fmt.Sprintf("Error: Command timed out after %v", t.timeout), nil
		}
		output += fmt.Sprintf("\nExit code: %v", err)
	}

	if output == "" {
		output = "(no output)"
	}

	maxLen := 10000
	if len(output) > maxLen {
		output = output[:maxLen] + fmt.Sprintf("\n... (truncated, %d more chars)", len(output)-maxLen)
	}

	return output, nil
}

func (t *ExecTool) guardCommand(command, cwd string) string {
	cmd := strings.TrimSpace(command)
	lower := strings.ToLower(cmd)

	for _, pattern := range t.denyPatterns {
		if pattern.MatchString(lower) {
			return "Command blocked by safety guard (dangerous pattern detected)"
		}
	}

	if len(t.allowPatterns) > 0 {
		allowed := false
		for _, pattern := range t.allowPatterns {
			if pattern.MatchString(lower) {
				allowed = true
				break
			}
		}
		if !allowed {
			return "Command blocked by safety guard (not in allowlist)"
		}
	}

	if t.restrictToWorkspace {
		if strings.Contains(cmd, "..\\") || strings.Contains(cmd, "../") {
			return "Command blocked by safety guard (path traversal detected)"
		}

		cwdPath, err := filepath.Abs(cwd)
		if err != nil {
			return ""
		}

		// Expand ~ and $HOME before path checking to prevent bypass
		expandedCmd := cmd
		if home, err := os.UserHomeDir(); err == nil {
			expandedCmd = strings.ReplaceAll(expandedCmd, "~/", home+"/")
			expandedCmd = strings.ReplaceAll(expandedCmd, "$HOME/", home+"/")
			expandedCmd = strings.ReplaceAll(expandedCmd, "${HOME}/", home+"/")
			expandedCmd = strings.ReplaceAll(expandedCmd, "$HOME\"", home+"\"")
			expandedCmd = strings.ReplaceAll(expandedCmd, "$HOME'", home+"'")
			expandedCmd = strings.ReplaceAll(expandedCmd, "${HOME}\"", home+"\"")
			expandedCmd = strings.ReplaceAll(expandedCmd, "${HOME}'", home+"'")
		}

		pathPattern := regexp.MustCompile(`[A-Za-z]:\\[^\\\"']+|/[^\s\"']+`)
		matches := pathPattern.FindAllString(expandedCmd, -1)

		for _, raw := range matches {
			// Allow read-only system virtual filesystems
			if isSafeSystemPath(raw) {
				continue
			}

			p, err := filepath.Abs(raw)
			if err != nil {
				continue
			}

			rel, err := filepath.Rel(cwdPath, p)
			if err != nil {
				continue
			}

			if strings.HasPrefix(rel, "..") {
				return "Command blocked by safety guard (path outside working dir)"
			}
		}
	}

	return ""
}

// safeSystemPrefixes are read-only virtual filesystem paths safe to access
// even when restrictToWorkspace is enabled.
var safeSystemPrefixes = []string{
	"/sys/class/",
	"/sys/devices/",
	"/proc/cpuinfo",
	"/proc/meminfo",
	"/proc/uptime",
	"/proc/loadavg",
	"/proc/version",
	"/proc/stat",
	"/proc/net/",
	"/dev/null",
}

func isSafeSystemPath(path string) bool {
	for _, prefix := range safeSystemPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func (t *ExecTool) SetTimeout(timeout time.Duration) {
	t.timeout = timeout
}

func (t *ExecTool) SetRestrictToWorkspace(restrict bool) {
	t.restrictToWorkspace = restrict
}

func (t *ExecTool) SetAllowPatterns(patterns []string) error {
	t.allowPatterns = make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return fmt.Errorf("invalid allow pattern %q: %w", p, err)
		}
		t.allowPatterns = append(t.allowPatterns, re)
	}
	return nil
}
