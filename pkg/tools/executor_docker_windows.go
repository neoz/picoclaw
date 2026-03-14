//go:build windows

package tools

// detectWorkspaceOwner on Windows falls back to 1000:1000.
// Docker Desktop on Windows runs Linux containers in a VM where
// file ownership is handled by the Docker engine.
func detectWorkspaceOwner(workspaceDir string) string {
	return "1000:1000"
}
