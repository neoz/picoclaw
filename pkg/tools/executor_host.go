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

// urlPattern matches URLs so they can be stripped before filesystem path
// checking in guardCommand. This prevents URL paths like
// https://example.com/path or wttr.in/path from being flagged as absolute
// filesystem paths.
var urlPattern = regexp.MustCompile(`(?:(?:https?|ftp)://|[a-zA-Z0-9][-a-zA-Z0-9]*(?:\.[a-zA-Z0-9][-a-zA-Z0-9]*)+/)[^\s"']*`)

// HostExecutor runs commands directly on the host with regex-based safety
// guards and workspace path restrictions.
type HostExecutor struct {
	workingDir          string
	timeout             time.Duration
	denyPatterns        []*regexp.Regexp
	allowPatterns       []*regexp.Regexp
	restrictToWorkspace bool
}

func NewHostExecutor(workingDir string, timeout time.Duration, restrictToWorkspace bool) *HostExecutor {
	denyPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\brm\s+-[rf]{1,2}\b`),
		regexp.MustCompile(`\bdel\s+/[fq]\b`),
		regexp.MustCompile(`\brmdir\s+/s\b`),
		regexp.MustCompile(`\b(format|mkfs|diskpart)\b\s`),
		regexp.MustCompile(`\bdd\s+if=`),
		regexp.MustCompile(`>\s*/dev/sd[a-z]\b`),
		regexp.MustCompile(`\b(shutdown|reboot|poweroff)\b`),
		regexp.MustCompile(`:\(\)\s*\{.*\};\s*:`),
		regexp.MustCompile(`\.picoclaw/config\b`),
		regexp.MustCompile(`/etc/(shadow|gshadow|master\.passwd)\b`),
		regexp.MustCompile(`/\.(ssh|gnupg)/`),
		regexp.MustCompile(`\.(pem|p12|pfx|key|keystore|jks)\b`),
		regexp.MustCompile(`\bcurl\b.*\s(--data|--upload-file|-d|-[fF]|-[tT])\b`),
		regexp.MustCompile(`\bwget\b.*\s--post-(data|file)\b`),
	}

	return &HostExecutor{
		workingDir:          workingDir,
		timeout:             timeout,
		denyPatterns:        denyPatterns,
		restrictToWorkspace: restrictToWorkspace,
	}
}

func (h *HostExecutor) Execute(ctx context.Context, command, workingDir string) (string, error) {
	cwd := workingDir
	if cwd == "" {
		cwd = h.workingDir
	}
	if cwd == "" {
		if wd, err := os.Getwd(); err == nil {
			cwd = wd
		}
	}

	if guardError := h.guardCommand(command, cwd); guardError != "" {
		return fmt.Sprintf("Error: %s", guardError), nil
	}

	cmdCtx, cancel := context.WithTimeout(ctx, h.timeout)
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
			return fmt.Sprintf("Error: Command timed out after %v", h.timeout), nil
		}
		output += fmt.Sprintf("\nExit code: %v", err)
	}

	return output, nil
}

func (h *HostExecutor) SetAllowPatterns(patterns []string) error {
	h.allowPatterns = make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return fmt.Errorf("invalid allow pattern %q: %w", p, err)
		}
		h.allowPatterns = append(h.allowPatterns, re)
	}
	return nil
}

func (h *HostExecutor) guardCommand(command, cwd string) string {
	cmd := strings.TrimSpace(command)
	lower := strings.ToLower(cmd)

	for _, pattern := range h.denyPatterns {
		if pattern.MatchString(lower) {
			return "Command blocked by safety guard (dangerous pattern detected)"
		}
	}

	if len(h.allowPatterns) > 0 {
		allowed := false
		for _, pattern := range h.allowPatterns {
			if pattern.MatchString(lower) {
				allowed = true
				break
			}
		}
		if !allowed {
			return "Command blocked by safety guard (not in allowlist)"
		}
	}

	if h.restrictToWorkspace {
		if strings.Contains(cmd, "..\\") || strings.Contains(cmd, "../") {
			return "Command blocked by safety guard (path traversal detected)"
		}

		cwdPath, err := filepath.Abs(cwd)
		if err != nil {
			return ""
		}

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

		strippedCmd := urlPattern.ReplaceAllString(expandedCmd, "")

		pathPattern := regexp.MustCompile(`[A-Za-z]:\\[^\\\"']+|/[^\s\"']+`)
		matches := pathPattern.FindAllString(strippedCmd, -1)

		for _, raw := range matches {
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
	// Allow access to temp directory (consistent with read_file/write_file)
	if isUnderTmpDir(filepath.Clean(path)) {
		return true
	}
	return false
}
