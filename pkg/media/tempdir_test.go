package media

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsAllowedPath_InMediaTempDir(t *testing.T) {
	path := filepath.Join(TempDir(), "photo.jpg")
	if !IsAllowedPath(path, "") {
		t.Error("path under media temp dir should be allowed")
	}
}

func TestIsAllowedPath_InMediaTempDirNested(t *testing.T) {
	path := filepath.Join(TempDir(), "sub", "photo.jpg")
	if !IsAllowedPath(path, "") {
		t.Error("nested path under media temp dir should be allowed")
	}
}

func TestIsAllowedPath_InSystemTempDir(t *testing.T) {
	// System temp (outside media temp dir) should be rejected to prevent
	// the exfiltration chain: write_file to /tmp -> send as media.
	path := filepath.Join(os.TempDir(), "other_file.tmp")
	if IsAllowedPath(path, "") {
		t.Error("path under system temp dir (outside media temp) should be rejected")
	}
}

func TestIsAllowedPath_InWorkspace(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, "output", "image.png")
	if !IsAllowedPath(path, workspace) {
		t.Error("path under workspace should be allowed")
	}
}

func TestIsAllowedPath_WorkspaceRoot(t *testing.T) {
	workspace := t.TempDir()
	if !IsAllowedPath(workspace, workspace) {
		t.Error("workspace root itself should be allowed")
	}
}

func TestIsAllowedPath_OutsideAllDirs(t *testing.T) {
	if IsAllowedPath("/etc/passwd", "/home/user/workspace") {
		t.Error("path outside all allowed dirs should be rejected")
	}
}

func TestIsAllowedPath_OutsideWithEmptyWorkspace(t *testing.T) {
	if IsAllowedPath("/etc/shadow", "") {
		t.Error("path outside temp dirs with empty workspace should be rejected")
	}
}

func TestIsAllowedPath_TraversalAttempt(t *testing.T) {
	// Try to escape media temp dir via ..
	path := filepath.Join(TempDir(), "..", "..", "etc", "passwd")
	if IsAllowedPath(path, "") {
		t.Errorf("traversal path should be rejected, cleaned to %s", filepath.Clean(path))
	}
}

func TestIsAllowedPath_TraversalFromWorkspace(t *testing.T) {
	// Use a non-temp workspace so traversal can't accidentally land in an allowed dir
	workspace := filepath.Join("C:", "workspace", "project")
	path := filepath.Join(workspace, "..", "secret.txt")
	if IsAllowedPath(path, workspace) {
		t.Errorf("traversal out of workspace should be rejected, cleaned to %s", filepath.Clean(path))
	}
}

func TestIsAllowedPath_PrefixSpoofing(t *testing.T) {
	// Verify separator-aware matching: /home/user/work should not match /home/user/workspace
	workspace := filepath.Join("C:", "home", "user", "work")
	spoofPath := filepath.Join("C:", "home", "user", "workspace", "file.jpg")
	if IsAllowedPath(spoofPath, workspace) {
		t.Error("path under sibling dir with similar prefix should be rejected")
	}
}

func TestIsAllowedPath_MediaTempDirRoot(t *testing.T) {
	if !IsAllowedPath(TempDir(), "") {
		t.Error("media temp dir root itself should be allowed")
	}
}

func TestIsAllowedPath_SystemTempDirRoot(t *testing.T) {
	if IsAllowedPath(os.TempDir(), "") {
		t.Error("system temp dir root should be rejected (only media subdir allowed)")
	}
}
