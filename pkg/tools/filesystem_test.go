package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// --- checkAllowedDir ---

func TestCheckAllowedDir_InWorkspace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	os.WriteFile(path, []byte("test"), 0644)

	resolved, err := checkAllowedDir(path, dir)
	if err != nil {
		t.Fatalf("expected allowed, got error: %v", err)
	}
	if resolved == "" {
		t.Error("expected non-empty resolved path")
	}
}

func TestCheckAllowedDir_OutsideWorkspace(t *testing.T) {
	// Use non-temp paths since checkAllowedDir allows anything under os.TempDir()
	_, err := checkAllowedDir("/etc/passwd", "/home/user/workspace")
	if err == nil {
		t.Error("expected error for path outside workspace")
	}
}

func TestCheckAllowedDir_TraversalBlocked(t *testing.T) {
	_, err := checkAllowedDir("/home/user/workspace/../escape.txt", "/home/user/workspace")
	if err == nil {
		t.Error("expected error for path traversal outside workspace")
	}
}

func TestCheckAllowedDir_SymlinkEscapeBlocked(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows")
	}

	workspace := t.TempDir()
	outside := t.TempDir()

	// Create a secret file outside workspace
	secretPath := filepath.Join(outside, "secret.txt")
	os.WriteFile(secretPath, []byte("secret data"), 0644)

	// Create a symlink inside workspace pointing outside
	symlinkPath := filepath.Join(workspace, "escape_link")
	if err := os.Symlink(secretPath, symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// checkAllowedDir should resolve the symlink and reject
	_, err := checkAllowedDir(symlinkPath, workspace)
	if err == nil {
		t.Error("expected error: symlink target is outside workspace")
	}
	if err != nil && !strings.Contains(err.Error(), "outside allowed directory") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckAllowedDir_SymlinkWithinWorkspaceAllowed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows")
	}

	workspace := t.TempDir()

	// Create a real file inside workspace
	realPath := filepath.Join(workspace, "real.txt")
	os.WriteFile(realPath, []byte("data"), 0644)

	// Create a symlink inside workspace pointing to another file in workspace
	symlinkPath := filepath.Join(workspace, "link.txt")
	if err := os.Symlink(realPath, symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	resolved, err := checkAllowedDir(symlinkPath, workspace)
	if err != nil {
		t.Fatalf("expected allowed for symlink within workspace, got error: %v", err)
	}
	if resolved == "" {
		t.Error("expected non-empty resolved path")
	}
}

func TestCheckAllowedDir_SymlinkDirEscapeBlocked(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows")
	}

	workspace := t.TempDir()
	outside := t.TempDir()

	// Create a dir symlink inside workspace pointing outside
	symlinkDir := filepath.Join(workspace, "escape_dir")
	if err := os.Symlink(outside, symlinkDir); err != nil {
		t.Fatalf("failed to create dir symlink: %v", err)
	}

	// Try to access a file through the symlink dir
	filePath := filepath.Join(symlinkDir, "file.txt")

	_, err := checkAllowedDir(filePath, workspace)
	if err == nil {
		t.Error("expected error: symlink directory target is outside workspace")
	}
}

func TestCheckAllowedDir_NewFileInWorkspaceAllowed(t *testing.T) {
	workspace := t.TempDir()

	// Path to a file that doesn't exist yet (write_file use case)
	newPath := filepath.Join(workspace, "new_file.txt")

	resolved, err := checkAllowedDir(newPath, workspace)
	if err != nil {
		t.Fatalf("expected allowed for new file in workspace, got error: %v", err)
	}
	if resolved == "" {
		t.Error("expected non-empty resolved path")
	}
}

// --- ProtectFiles ---

func TestReadFileTool_ProtectedFilesBlocked(t *testing.T) {
	workspace := t.TempDir()

	// Create a bootstrap file
	soulPath := filepath.Join(workspace, "SOUL.md")
	os.WriteFile(soulPath, []byte("secret system prompt"), 0644)

	tool := NewReadFileTool(workspace)
	tool.ProtectFiles([]string{soulPath})

	_, err := tool.Execute(nil, map[string]interface{}{"path": soulPath})
	if err == nil {
		t.Error("expected error for protected file")
	}
	if err != nil && !strings.Contains(err.Error(), "protected system file") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReadFileTool_NonProtectedFileAllowed(t *testing.T) {
	workspace := t.TempDir()

	normalPath := filepath.Join(workspace, "readme.txt")
	os.WriteFile(normalPath, []byte("hello"), 0644)

	soulPath := filepath.Join(workspace, "SOUL.md")

	tool := NewReadFileTool(workspace)
	tool.ProtectFiles([]string{soulPath})

	result, err := tool.Execute(nil, map[string]interface{}{"path": normalPath})
	if err != nil {
		t.Fatalf("expected allowed, got error: %v", err)
	}
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}
