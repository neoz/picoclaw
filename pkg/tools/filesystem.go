package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// checkAllowedDir validates that the resolved path is within the allowed directory.
func checkAllowedDir(path, allowedDir string) (string, error) {
	var resolvedPath string
	if filepath.IsAbs(path) {
		resolvedPath = filepath.Clean(path)
	} else {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("failed to resolve path: %w", err)
		}
		resolvedPath = abs
	}

	if allowedDir != "" {
		allowedAbs, err := filepath.Abs(allowedDir)
		if err != nil {
			return "", fmt.Errorf("failed to resolve allowed directory: %w", err)
		}
		if !strings.HasPrefix(resolvedPath, allowedAbs+string(filepath.Separator)) && resolvedPath != allowedAbs {
			return "", fmt.Errorf("path %s is outside allowed directory %s", path, allowedDir)
		}
	}

	return resolvedPath, nil
}

type ReadFileTool struct {
	allowedDir string
}

func NewReadFileTool(allowedDir string) *ReadFileTool {
	return &ReadFileTool{allowedDir: allowedDir}
}

func (t *ReadFileTool) Name() string {
	return "read_file"
}

func (t *ReadFileTool) Description() string {
	return "Read the contents of a file"
}

func (t *ReadFileTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the file to read",
			},
		},
		"required": []string{"path"},
	}
}

func (t *ReadFileTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path is required")
	}

	resolvedPath, err := checkAllowedDir(path, t.allowedDir)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}

type WriteFileTool struct {
	allowedDir string
}

func NewWriteFileTool(allowedDir string) *WriteFileTool {
	return &WriteFileTool{allowedDir: allowedDir}
}

func (t *WriteFileTool) Name() string {
	return "write_file"
}

func (t *WriteFileTool) Description() string {
	return "Write content to a file"
}

func (t *WriteFileTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the file to write",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Content to write to the file",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (t *WriteFileTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path is required")
	}

	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("content is required")
	}

	resolvedPath, err := checkAllowedDir(path, t.allowedDir)
	if err != nil {
		return "", err
	}

	dir := filepath.Dir(resolvedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(resolvedPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return "File written successfully", nil
}

type ListDirTool struct {
	allowedDir string
}

func NewListDirTool(allowedDir string) *ListDirTool {
	return &ListDirTool{allowedDir: allowedDir}
}

func (t *ListDirTool) Name() string {
	return "list_dir"
}

func (t *ListDirTool) Description() string {
	return "List files and directories in a path"
}

func (t *ListDirTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to list",
			},
		},
		"required": []string{"path"},
	}
}

func (t *ListDirTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		path = "."
	}

	resolvedPath, err := checkAllowedDir(path, t.allowedDir)
	if err != nil {
		return "", err
	}

	entries, err := os.ReadDir(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %w", err)
	}

	result := ""
	for _, entry := range entries {
		if entry.IsDir() {
			result += "DIR:  " + entry.Name() + "\n"
		} else {
			result += "FILE: " + entry.Name() + "\n"
		}
	}

	return result, nil
}
