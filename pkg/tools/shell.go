package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ExecTool struct {
	workingDir          string
	timeout             time.Duration
	restrictToWorkspace bool
	executor            CommandExecutor
}

func NewExecTool(workingDir string) *ExecTool {
	return &ExecTool{
		workingDir:          workingDir,
		timeout:             60 * time.Second,
		restrictToWorkspace: true,
		executor:            NewHostExecutor(workingDir, 60*time.Second, true),
	}
}

func (t *ExecTool) Name() string {
	return "exec"
}

func (t *ExecTool) Description() string {
	return "Execute a shell command in workspace. Paths outside workspace are blocked. Prefer web_fetch for URLs."
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
		if wd, err := os.Getwd(); err == nil {
			cwd = wd
		}
	}

	executor := t.getExecutor()
	output, err := executor.Execute(ctx, command, cwd)
	if err != nil {
		return "", err
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

func (t *ExecTool) SetExecutor(e CommandExecutor) {
	t.executor = e
}

func (t *ExecTool) getExecutor() CommandExecutor {
	return t.executor
}

func (t *ExecTool) SetTimeout(timeout time.Duration) {
	t.timeout = timeout
}

func (t *ExecTool) SetRestrictToWorkspace(restrict bool) {
	t.restrictToWorkspace = restrict
}

func (t *ExecTool) SetAllowPatterns(patterns []string) error {
	if h, ok := t.executor.(*HostExecutor); ok {
		return h.SetAllowPatterns(patterns)
	}
	return nil
}
