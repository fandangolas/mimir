package mcp

// JSON-RPC 2.0 types for MCP protocol

// Request represents a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC 2.0 error.
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// InitializeParams are parameters for the initialize request.
type InitializeParams struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ClientInfo      ClientInfo   `json:"clientInfo"`
}

// Capabilities declares client capabilities.
type Capabilities struct {
	Roots    *RootsCapability    `json:"roots,omitempty"`
	Sampling *SamplingCapability `json:"sampling,omitempty"`
}

// RootsCapability indicates support for roots.
type RootsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// SamplingCapability indicates support for sampling.
type SamplingCapability struct{}

// ClientInfo provides information about the client.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult is the result of an initialize request.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// ServerCapabilities declares server capabilities.
type ServerCapabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
}

// ToolsCapability indicates support for tools.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability indicates support for resources.
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptsCapability indicates support for prompts.
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ServerInfo provides information about the server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Tool represents an MCP tool.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// ToolsListResult is the result of a tools/list request.
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolParams are parameters for calling a tool.
type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// ToolResult is the result of a tool call.
type ToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// Content represents tool result content.
type Content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}
