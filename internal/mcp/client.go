package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"sync/atomic"
)

// Client manages communication with an MCP server via stdio.
type Client struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	scanner *bufio.Scanner

	mu          sync.Mutex
	nextID      atomic.Int32
	pending     map[int]chan Response
	initialized bool
	tools       []Tool

	logger *slog.Logger
}

// NewClient creates a new MCP client.
func NewClient(command string, args []string, env []string) *Client {
	return &Client{
		pending: make(map[int]chan Response),
		logger:  slog.Default().With("component", "mcp_client"),
	}
}

// Start launches the MCP server process.
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd != nil {
		return fmt.Errorf("client already started")
	}

	// This will be set by the caller with actual command/args
	// For now, we'll add a method to configure this
	return fmt.Errorf("command not configured")
}

// Configure sets the command and arguments for the MCP server.
func (c *Client) Configure(command string, args []string, env []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cmd = exec.Command(command, args...)
	if len(env) > 0 {
		c.cmd.Env = append(c.cmd.Environ(), env...)
	}
}

// StartWithCommand launches the MCP server with the given command configuration.
func (c *Client) StartWithCommand(ctx context.Context, command string, args []string, env []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd != nil {
		return fmt.Errorf("client already started")
	}

	c.cmd = exec.Command(command, args...)
	if len(env) > 0 {
		c.cmd.Env = append(c.cmd.Environ(), env...)
	}

	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("create stdin pipe: %w", err)
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create stdout pipe: %w", err)
	}

	c.scanner = bufio.NewScanner(c.stdout)

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("start command: %w", err)
	}

	// Start reading responses in background
	go c.readLoop(ctx)

	c.logger.Info("mcp server started", "command", command)
	return nil
}

// Initialize performs the MCP initialization handshake.
func (c *Client) Initialize(ctx context.Context) error {
	c.mu.Lock()
	if c.initialized {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	params := InitializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities: Capabilities{
			Roots: &RootsCapability{
				ListChanged: false,
			},
		},
		ClientInfo: ClientInfo{
			Name:    "mimir",
			Version: "1.0.0",
		},
	}

	var result InitializeResult
	if err := c.call(ctx, "initialize", params, &result); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}

	c.mu.Lock()
	c.initialized = true
	c.mu.Unlock()

	c.logger.Info("mcp server initialized",
		"server", result.ServerInfo.Name,
		"version", result.ServerInfo.Version,
		"protocol", result.ProtocolVersion)

	// Send initialized notification
	if err := c.notify(ctx, "notifications/initialized", nil); err != nil {
		return fmt.Errorf("send initialized notification: %w", err)
	}

	return nil
}

// ListTools retrieves available tools from the MCP server.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	var result ToolsListResult
	if err := c.call(ctx, "tools/list", nil, &result); err != nil {
		return nil, fmt.Errorf("list tools: %w", err)
	}

	c.mu.Lock()
	c.tools = result.Tools
	c.mu.Unlock()

	c.logger.Debug("tools listed", "count", len(result.Tools))
	return result.Tools, nil
}

// CallTool invokes a tool on the MCP server.
func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*ToolResult, error) {
	params := CallToolParams{
		Name:      name,
		Arguments: arguments,
	}

	var result ToolResult
	if err := c.call(ctx, "tools/call", params, &result); err != nil {
		return nil, fmt.Errorf("call tool %q: %w", name, err)
	}

	if result.IsError {
		return &result, fmt.Errorf("tool error: %s", c.formatToolResult(&result))
	}

	c.logger.Debug("tool called", "name", name)
	return &result, nil
}

// Close terminates the MCP server process.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}

	// Close stdin to signal shutdown
	if c.stdin != nil {
		c.stdin.Close()
	}

	// Wait for process to exit
	if err := c.cmd.Wait(); err != nil {
		c.logger.Warn("mcp server exit error", "error", err)
	}

	c.logger.Info("mcp server stopped")
	return nil
}

// call sends a JSON-RPC request and waits for the response.
func (c *Client) call(ctx context.Context, method string, params interface{}, result interface{}) error {
	id := int(c.nextID.Add(1))

	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	// Register response channel
	respChan := make(chan Response, 1)
	c.mu.Lock()
	c.pending[id] = respChan
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	// Send request
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	c.mu.Lock()
	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		c.mu.Unlock()
		return fmt.Errorf("write request: %w", err)
	}
	c.mu.Unlock()

	// Wait for response
	select {
	case <-ctx.Done():
		return ctx.Err()
	case resp := <-respChan:
		if resp.Error != nil {
			return fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}

		// Unmarshal result if provided
		if result != nil && resp.Result != nil {
			data, err := json.Marshal(resp.Result)
			if err != nil {
				return fmt.Errorf("marshal result: %w", err)
			}
			if err := json.Unmarshal(data, result); err != nil {
				return fmt.Errorf("unmarshal result: %w", err)
			}
		}

		return nil
	}
}

// notify sends a JSON-RPC notification (no response expected).
func (c *Client) notify(ctx context.Context, method string, params interface{}) error {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		req["params"] = params
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write notification: %w", err)
	}

	return nil
}

// readLoop continuously reads responses from the MCP server.
func (c *Client) readLoop(ctx context.Context) {
	for c.scanner.Scan() {
		line := c.scanner.Bytes()

		var resp Response
		if err := json.Unmarshal(line, &resp); err != nil {
			c.logger.Error("failed to parse response", "error", err, "line", string(line))
			continue
		}

		c.mu.Lock()
		ch, ok := c.pending[resp.ID]
		c.mu.Unlock()

		if ok {
			select {
			case ch <- resp:
			case <-ctx.Done():
				return
			}
		}
	}

	if err := c.scanner.Err(); err != nil {
		c.logger.Error("scanner error", "error", err)
	}
}

// formatToolResult formats tool result content as a string.
func (c *Client) formatToolResult(result *ToolResult) string {
	if len(result.Content) == 0 {
		return ""
	}

	var text string
	for _, content := range result.Content {
		if content.Type == "text" {
			text += content.Text
		}
	}
	return text
}
