package tools

import "context"

// CommandExecutor abstracts how shell commands are run.
// HostExecutor runs directly on the host; DockerExecutor runs inside a
// disposable container for stronger isolation.
type CommandExecutor interface {
	Execute(ctx context.Context, command, workingDir string) (string, error)
}
