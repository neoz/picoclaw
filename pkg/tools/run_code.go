package tools

import (
	"context"
	"fmt"
	"strings"
)

// RunCodeTool executes code directly inside the sandbox without writing to workspace.
// The script is written to /tmp inside the container and disappears when it exits.
type RunCodeTool struct {
	executor CommandExecutor
}

func NewRunCodeTool(executor CommandExecutor) *RunCodeTool {
	return &RunCodeTool{executor: executor}
}

func (t *RunCodeTool) Name() string {
	return "run_code"
}

func (t *RunCodeTool) Description() string {
	return "Run Python or shell code in sandbox without saving to workspace. Use 'dependencies' for pip packages."
}

func (t *RunCodeTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"code": map[string]interface{}{
				"type":        "string",
				"description": "The source code to execute",
			},
			"language": map[string]interface{}{
				"type":        "string",
				"description": "Programming language: python or shell",
				"enum":        []string{"python", "shell"},
			},
			"dependencies": map[string]interface{}{
				"type":        "string",
				"description": "Comma-separated list of pip packages (Python only), e.g. 'requests,scrapling'",
			},
		},
		"required": []string{"code", "language"},
	}
}

func (t *RunCodeTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	code, ok := args["code"].(string)
	if !ok || code == "" {
		return "", fmt.Errorf("code is required")
	}

	language, ok := args["language"].(string)
	if !ok || language == "" {
		return "", fmt.Errorf("language is required")
	}

	deps, _ := args["dependencies"].(string)

	var command string
	switch language {
	case "python":
		command = buildPythonCommand(code, deps)
	case "shell":
		command = buildShellCommand(code)
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}

	output, err := t.executor.Execute(ctx, command, "")
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

// buildPythonCommand creates a shell command that writes code to /tmp and runs it with uv.
func buildPythonCommand(code, deps string) string {
	escaped := strings.ReplaceAll(code, "'", "'\\''")
	writeScript := fmt.Sprintf("cat > /tmp/script.py << 'PICOCLAW_EOF'\n%s\nPICOCLAW_EOF", escaped)

	if deps != "" {
		var withArgs []string
		for _, dep := range strings.Split(deps, ",") {
			dep = strings.TrimSpace(dep)
			if dep != "" {
				withArgs = append(withArgs, "--with", dep)
			}
		}
		return fmt.Sprintf("%s && uv run %s python /tmp/script.py", writeScript, strings.Join(withArgs, " "))
	}

	return fmt.Sprintf("%s && python3 /tmp/script.py", writeScript)
}

// buildShellCommand creates a shell command that writes code to /tmp and runs it.
func buildShellCommand(code string) string {
	escaped := strings.ReplaceAll(code, "'", "'\\''")
	return fmt.Sprintf("cat > /tmp/script.sh << 'PICOCLAW_EOF'\n%s\nPICOCLAW_EOF\nchmod +x /tmp/script.sh && /tmp/script.sh", escaped)
}
