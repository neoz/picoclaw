package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mockExecutor is a fake CommandExecutor for testing ExecTool delegation.
type mockExecutor struct {
	lastCommand    string
	lastWorkingDir string
	output         string
	err            error
}

func (m *mockExecutor) Execute(_ context.Context, command, workingDir string) (string, error) {
	m.lastCommand = command
	m.lastWorkingDir = workingDir
	return m.output, m.err
}

// --- ExecTool metadata ---

func TestExecTool_Name(t *testing.T) {
	tool := NewExecTool("/tmp")
	if tool.Name() != "exec" {
		t.Errorf("Name() = %q, want exec", tool.Name())
	}
}

func TestExecTool_Description(t *testing.T) {
	tool := NewExecTool("/tmp")
	if tool.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestExecTool_Parameters(t *testing.T) {
	tool := NewExecTool("/tmp")
	params := tool.Parameters()

	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected properties map")
	}
	if _, ok := props["command"]; !ok {
		t.Error("expected 'command' parameter")
	}
	if _, ok := props["working_dir"]; !ok {
		t.Error("expected 'working_dir' parameter")
	}

	required, ok := params["required"].([]string)
	if !ok {
		t.Fatal("expected required array")
	}
	if len(required) != 1 || required[0] != "command" {
		t.Errorf("required = %v, want [command]", required)
	}
}

// --- ExecTool delegation to executor ---

func TestExecTool_DelegatesToExecutor(t *testing.T) {
	mock := &mockExecutor{output: "mock output"}
	tool := NewExecTool("/workspace")
	tool.SetExecutor(mock)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if mock.lastCommand != "echo test" {
		t.Errorf("executor received command %q, want 'echo test'", mock.lastCommand)
	}
	if result != "mock output" {
		t.Errorf("result = %q, want 'mock output'", result)
	}
}

func TestExecTool_PassesWorkingDir(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub")
	// Create subdir so it's valid
	mock := &mockExecutor{output: "ok"}
	tool := NewExecTool(dir)
	tool.SetExecutor(mock)
	tool.SetRestrictToWorkspace(false)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"command":     "ls",
		"working_dir": subdir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if mock.lastWorkingDir != subdir {
		t.Errorf("working dir = %q, want %q", mock.lastWorkingDir, subdir)
	}
}

func TestExecTool_MissingCommand(t *testing.T) {
	tool := NewExecTool("/tmp")
	_, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing command")
	}
}

func TestExecTool_CommandNotString(t *testing.T) {
	tool := NewExecTool("/tmp")
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": 123,
	})
	if err == nil {
		t.Error("expected error for non-string command")
	}
}

// --- Empty output ---

func TestExecTool_EmptyOutputBecomesNoOutput(t *testing.T) {
	mock := &mockExecutor{output: ""}
	tool := NewExecTool("/workspace")
	tool.SetExecutor(mock)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "true",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != "(no output)" {
		t.Errorf("result = %q, want '(no output)'", result)
	}
}

// --- Output truncation ---

func TestExecTool_OutputTruncation(t *testing.T) {
	// Generate output longer than 10000 chars
	longOutput := strings.Repeat("x", 15000)
	mock := &mockExecutor{output: longOutput}
	tool := NewExecTool("/workspace")
	tool.SetExecutor(mock)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo long",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) > 10200 { // 10000 + truncation message
		t.Errorf("result length = %d, expected truncation", len(result))
	}
	if !strings.Contains(result, "truncated") {
		t.Error("expected truncation message")
	}
	if !strings.Contains(result, "5000 more chars") {
		t.Errorf("expected '5000 more chars' in truncation message, got %q", result[10000:])
	}
}

func TestExecTool_OutputExactlyMaxLen(t *testing.T) {
	output := strings.Repeat("y", 10000)
	mock := &mockExecutor{output: output}
	tool := NewExecTool("/workspace")
	tool.SetExecutor(mock)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo exact",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result, "truncated") {
		t.Error("output of exactly maxLen should not be truncated")
	}
}

// --- Executor error propagation ---

func TestExecTool_ExecutorError(t *testing.T) {
	mock := &mockExecutor{err: fmt.Errorf("executor failed")}
	tool := NewExecTool("/workspace")
	tool.SetExecutor(mock)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "fail",
	})
	if err == nil {
		t.Error("expected error to propagate from executor")
	}
	if !strings.Contains(err.Error(), "executor failed") {
		t.Errorf("error = %v, want 'executor failed'", err)
	}
}

// --- working_dir workspace restriction ---

func TestExecTool_WorkingDirOutsideWorkspace(t *testing.T) {
	dir := t.TempDir()
	mock := &mockExecutor{output: "ok"}
	tool := NewExecTool(dir)
	tool.SetExecutor(mock)
	tool.SetRestrictToWorkspace(true)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command":     "ls",
		"working_dir": "/some/outside/path",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "working_dir outside workspace") {
		t.Errorf("expected workspace restriction error, got %q", result)
	}
}

func TestExecTool_WorkingDirInsideWorkspace(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "subdir")
	mock := &mockExecutor{output: "ok"}
	tool := NewExecTool(dir)
	tool.SetExecutor(mock)
	tool.SetRestrictToWorkspace(true)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command":     "ls",
		"working_dir": subdir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result, "blocked") {
		t.Errorf("expected subdir to be allowed, got %q", result)
	}
}

func TestExecTool_WorkingDirUnrestrictedAllowsAnything(t *testing.T) {
	dir := t.TempDir()
	mock := &mockExecutor{output: "ok"}
	tool := NewExecTool(dir)
	tool.SetExecutor(mock)
	tool.SetRestrictToWorkspace(false)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command":     "ls",
		"working_dir": "/any/path",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result, "blocked") {
		t.Errorf("expected unrestricted to allow any working_dir, got %q", result)
	}
}

// --- Default executor fallback ---

func TestExecTool_DefaultExecutorIsHost(t *testing.T) {
	tool := NewExecTool(t.TempDir())

	// Without SetExecutor, getExecutor should return a HostExecutor
	executor := tool.getExecutor()
	if _, ok := executor.(*HostExecutor); !ok {
		t.Errorf("default executor type = %T, want *HostExecutor", executor)
	}
}

// --- SetTimeout ---

func TestExecTool_SetTimeout(t *testing.T) {
	tool := NewExecTool("/tmp")
	tool.SetTimeout(5 * time.Second)
	if tool.timeout != 5*time.Second {
		t.Errorf("timeout = %v, want 5s", tool.timeout)
	}
}

// --- SetAllowPatterns delegation ---

func TestExecTool_SetAllowPatterns_WithHostExecutor(t *testing.T) {
	tool := NewExecTool(t.TempDir())
	// Default executor is Host, SetAllowPatterns should work
	err := tool.SetAllowPatterns([]string{`^echo\b`})
	if err != nil {
		t.Fatal(err)
	}
}

func TestExecTool_SetAllowPatterns_WithMockExecutor(t *testing.T) {
	mock := &mockExecutor{}
	tool := NewExecTool("/tmp")
	tool.SetExecutor(mock)

	// Non-host executor should silently accept (no-op)
	err := tool.SetAllowPatterns([]string{`^echo\b`})
	if err != nil {
		t.Errorf("expected no error for non-host executor, got %v", err)
	}
}

// --- Integration: ExecTool with real HostExecutor ---

func TestExecTool_Integration_RealExecution(t *testing.T) {
	dir := t.TempDir()
	tool := NewExecTool(dir)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo integration_test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "integration_test") {
		t.Errorf("expected output to contain 'integration_test', got %q", result)
	}
}

func TestExecTool_Integration_BlocksDangerous(t *testing.T) {
	dir := t.TempDir()
	tool := NewExecTool(dir)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "rm -rf /",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "blocked") {
		t.Errorf("expected dangerous command to be blocked, got %q", result)
	}
}
