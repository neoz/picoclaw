//go:build !windows

package tools

import (
	"fmt"
	"os"
	"syscall"
)

// detectWorkspaceOwner returns the "uid:gid" of the workspace directory owner.
// The sandbox container runs as this user so bind-mounted files are accessible.
// Falls back to "1000:1000" if detection fails.
func detectWorkspaceOwner(workspaceDir string) string {
	if workspaceDir == "" {
		return "1000:1000"
	}
	info, err := os.Stat(workspaceDir)
	if err != nil {
		return "1000:1000"
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "1000:1000"
	}
	return fmt.Sprintf("%d:%d", stat.Uid, stat.Gid)
}
