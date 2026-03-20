package mcp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sipeed/picoclaw/pkg/config"
)

// Client wraps an MCP client session with PicoClaw-specific lifecycle.
type Client struct {
	serverName string
	cfg        config.MCPServerConfig
	session    *mcp.ClientSession
	tools      []*mcp.Tool
	mu         sync.RWMutex
}

// NewClient creates a new MCP client for the given server configuration.
func NewClient(serverName string, cfg config.MCPServerConfig) *Client {
	return &Client{
		serverName: serverName,
		cfg:        cfg,
	}
}

// Connect establishes the connection and caches available tools.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	transport, err := c.createTransport()
	if err != nil {
		return fmt.Errorf("mcp server %q: %w", c.serverName, err)
	}

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "picoclaw",
		Version: "1.0.0",
	}, nil)

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("mcp server %q connect: %w", c.serverName, err)
	}

	result, err := session.ListTools(ctx, nil)
	if err != nil {
		session.Close()
		return fmt.Errorf("mcp server %q list tools: %w", c.serverName, err)
	}

	c.session = session
	c.tools = result.Tools
	return nil
}

// Tools returns the cached tools from the server.
func (c *Client) Tools() []*mcp.Tool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.tools
}

// CallTool invokes a tool on the MCP server. If disconnected, attempts one reconnect.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	c.mu.RLock()
	session := c.session
	c.mu.RUnlock()

	if session == nil {
		if err := c.Connect(ctx); err != nil {
			return "", err
		}
		c.mu.RLock()
		session = c.session
		c.mu.RUnlock()
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return "", fmt.Errorf("mcp tool %q call: %w", name, err)
	}

	if result.IsError {
		if errVal := result.GetError(); errVal != nil {
			return "", fmt.Errorf("mcp tool %q error: %w", name, errVal)
		}
	}

	return extractTextContent(result), nil
}

// Close shuts down the MCP session.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session != nil {
		err := c.session.Close()
		c.session = nil
		c.tools = nil
		return err
	}
	return nil
}

func (c *Client) createTransport() (mcp.Transport, error) {
	switch c.cfg.Transport {
	case "stdio":
		return c.createStdioTransport()
	case "sse":
		return c.createSSETransport()
	case "streamable-http":
		return c.createStreamableTransport()
	default:
		return nil, fmt.Errorf("unsupported transport %q", c.cfg.Transport)
	}
}

func (c *Client) createStdioTransport() (mcp.Transport, error) {
	if c.cfg.Command == "" {
		return nil, fmt.Errorf("stdio transport requires command")
	}
	cmd := exec.Command(c.cfg.Command, c.cfg.Args...)
	// Merge extra env vars with expanded values
	if len(c.cfg.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range c.cfg.Env {
			cmd.Env = append(cmd.Env, k+"="+os.ExpandEnv(v))
		}
	}
	return &mcp.CommandTransport{Command: cmd}, nil
}

func (c *Client) createSSETransport() (mcp.Transport, error) {
	if c.cfg.URL == "" {
		return nil, fmt.Errorf("sse transport requires url")
	}
	transport := &mcp.SSEClientTransport{
		Endpoint: os.ExpandEnv(c.cfg.URL),
	}
	if len(c.cfg.Headers) > 0 {
		transport.HTTPClient = &http.Client{
			Transport: &headerTransport{
				base:    http.DefaultTransport,
				headers: expandHeaders(c.cfg.Headers),
			},
		}
	}
	return transport, nil
}

func (c *Client) createStreamableTransport() (mcp.Transport, error) {
	if c.cfg.URL == "" {
		return nil, fmt.Errorf("streamable-http transport requires url")
	}
	transport := &mcp.StreamableClientTransport{
		Endpoint: os.ExpandEnv(c.cfg.URL),
	}
	if len(c.cfg.Headers) > 0 {
		transport.HTTPClient = &http.Client{
			Transport: &headerTransport{
				base:    http.DefaultTransport,
				headers: expandHeaders(c.cfg.Headers),
			},
		}
	}
	return transport, nil
}

// headerTransport injects custom headers into HTTP requests.
type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	return t.base.RoundTrip(req)
}

func expandHeaders(headers map[string]string) map[string]string {
	expanded := make(map[string]string, len(headers))
	for k, v := range headers {
		expanded[k] = os.ExpandEnv(v)
	}
	return expanded
}

// extractTextContent concatenates all TextContent parts from a CallToolResult.
func extractTextContent(result *mcp.CallToolResult) string {
	var parts []string
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}
