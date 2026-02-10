# MCP (Model Context Protocol) Architecture

## Overview

Mimir integrates external tools via the Model Context Protocol (MCP), a standardized protocol for connecting AI assistants to external data sources and tools. Currently implemented: **Google Calendar**.

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Mimir (Go)                           │
│                                                         │
│  ┌──────────────┐         ┌──────────────────────────┐ │
│  │ Orchestrator │────────▶│   MCP Manager            │ │
│  └──────────────┘         │   (mcpmanager pkg)       │ │
│                           │                          │ │
│                           │  ┌─────────────────────┐ │ │
│                           │  │ Calendar Tool       │ │ │
│                           │  │ (tools/calendar.go) │ │ │
│                           │  └─────────────────────┘ │ │
│                           │          │               │ │
│                           │          ▼               │ │
│                           │  ┌─────────────────────┐ │ │
│                           │  │ MCP Client          │ │ │
│                           │  │ (mcp pkg)           │ │ │
│                           │  └─────────────────────┘ │ │
│                           └──────────┬───────────────┘ │
└───────────────────────────────────────┼─────────────────┘
                                        │
                           stdio (JSON-RPC 2.0)
                                        │
┌───────────────────────────────────────▼─────────────────┐
│        google-calendar-mcp (Node.js/TypeScript)         │
│                                                          │
│  ┌──────────────┐      ┌─────────────┐                  │
│  │ OAuth 2.0    │──────│ Calendar    │                  │
│  │ Handler      │      │ API Client  │                  │
│  └──────────────┘      └─────────────┘                  │
└───────────────────────────────────────┬─────────────────┘
                                        │
                                    HTTPS API
                                        │
                                        ▼
                            ┌───────────────────────┐
                            │ Google Calendar API   │
                            └───────────────────────┘
```

---

## Package Structure

### `internal/mcp/`
Core MCP protocol implementation (JSON-RPC 2.0 over stdio).

**Files:**
- `client.go` - MCP client with stdio communication
- `types.go` - MCP protocol types (Request, Response, Tool, etc.)
- `client_test.go` - Unit tests

**Key components:**
- `Client` - Manages subprocess lifecycle and JSON-RPC communication
- `Initialize()` - MCP handshake with server
- `ListTools()` - Discover available tools
- `CallTool()` - Invoke a tool by name

### `internal/mcp/tools/`
Tool-specific wrappers that expose MCP functionality.

**Files:**
- `calendar.go` - Google Calendar tool wrapper
- `calendar_test.go` - Unit tests

**Key components:**
- `CalendarTool` - Wraps MCP client with typed methods
- `ListEvents()`, `CreateEvent()`, `SearchEvents()`, etc.
- Type-safe parameters (e.g., `ListEventsParams`, `CreateEventParams`)

### `internal/mcpmanager/`
High-level manager that initializes and coordinates MCP clients.

**Files:**
- `manager.go` - MCP manager implementation

**Key components:**
- `Manager` - Owns all MCP client instances
- `Start()` - Launches MCP server subprocesses
- `CalendarTool()` - Exposes calendar functionality to orchestrator
- `Close()` - Graceful shutdown

---

## Communication Flow

### 1. Initialization (Startup)

```
main.go
  │
  ├─▶ NewManager(config)
  │
  ├─▶ manager.Start(ctx, config)
  │     │
  │     ├─▶ spawn: npx @cocal/google-calendar-mcp
  │     │         (with GOOGLE_OAUTH_CREDENTIALS env)
  │     │
  │     ├─▶ client.Initialize(ctx)
  │     │     └─▶ JSON-RPC: { "method": "initialize", ... }
  │     │         ◀─ { "result": { "protocolVersion": "2024-11-05", ... } }
  │     │
  │     └─▶ client.ListTools(ctx)
  │           └─▶ JSON-RPC: { "method": "tools/list" }
  │               ◀─ { "result": { "tools": [...] } }
  │
  └─▶ orch.WithMCP(manager)
```

### 2. Tool Call (Runtime)

```
User: "What's on my calendar today?"
  │
  ▼
orchestrator.Handle()
  │
  ├─▶ LLM decides to call calendar tool
  │
  └─▶ mcpManager.CalendarTool().ListEvents(ctx, params)
        │
        └─▶ client.CallTool(ctx, "list-events", args)
              │
              ├─▶ JSON-RPC over stdin:
              │   {
              │     "jsonrpc": "2.0",
              │     "id": 1,
              │     "method": "tools/call",
              │     "params": {
              │       "name": "list-events",
              │       "arguments": { "timeMin": "2025-02-09T00:00:00Z", ... }
              │     }
              │   }
              │
              ├─▶ MCP server processes request:
              │   - Reads token from ~/.config/google-calendar-mcp/tokens.json
              │   - Calls Google Calendar API
              │   - Formats response
              │
              └◀─ JSON-RPC response over stdout:
                  {
                    "jsonrpc": "2.0",
                    "id": 1,
                    "result": {
                      "content": [
                        {
                          "type": "text",
                          "text": "Events for today:\n1. Team Meeting (10:00 AM)\n..."
                        }
                      ]
                    }
                  }
```

### 3. Shutdown

```
ctx.Done() (SIGINT/SIGTERM)
  │
  └─▶ manager.Close()
        │
        └─▶ client.Close()
              │
              ├─▶ Close stdin pipe (signals MCP server to exit)
              │
              └─▶ cmd.Wait() (blocks until subprocess exits)
```

---

## JSON-RPC 2.0 Protocol

### Request Format
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "list-events",
    "arguments": {
      "calendarId": "primary",
      "timeMin": "2025-02-09T00:00:00Z",
      "maxResults": 10
    }
  }
}
```

### Response Format (Success)
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Event list here..."
      }
    ],
    "isError": false
  }
}
```

### Response Format (Error)
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32602,
    "message": "Invalid params",
    "data": "timeMin is required"
  }
}
```

---

## Security

### Sandbox Model

**MCP server runs in separate process:**
- No direct access to Mimir's memory
- Communication only via stdio
- Subprocess killed on Mimir shutdown

**OAuth tokens:**
- Stored in `~/.config/google-calendar-mcp/tokens.json`
- Permissions: `600` (owner read/write only)
- Never transmitted over network (used locally by MCP server)

### Tool Filtering

Mimir restricts which tools are exposed via `ENABLED_TOOLS` environment variable:

```go
enabledTools := "list-events,create-event,search-events,get-freebusy,get-current-time"
```

**Disabled tools:**
- `update-event` - Prevents unintended modifications
- `delete-event` - Prevents data loss
- `respond-to-event` - RSVP changes could be unwanted
- `manage-accounts` - Multi-account not needed yet

### Input Validation

**MCP client validates:**
- Required parameters (e.g., `timeMin`, `timeMax` for `get-freebusy`)
- Type safety via Go structs (compile-time checks)
- Error handling for malformed responses

**MCP server validates:**
- OAuth token expiration (7 days in test mode)
- API rate limits (enforced by Google)
- Scope restrictions (only `calendar` scope granted)

---

## Error Handling

### Initialization Failures

**Scenario:** OAuth credentials file not found

```go
if _, err := os.Stat(cfg.GoogleOAuthCredentials); os.IsNotExist(err) {
    return fmt.Errorf("credentials file not found: %s", cfg.GoogleOAuthCredentials)
}
```

**Result:** Mimir startup fails with clear error message

### Runtime Failures

**Scenario:** MCP server crashes or becomes unresponsive

```go
// readLoop detects broken pipe
if err := c.scanner.Err(); err != nil {
    c.logger.Error("scanner error", "error", err)
}
```

**Result:**
- Pending tool calls return context deadline errors
- Orchestrator falls back to text-only responses
- Restart Mimir to restore MCP functionality

### OAuth Token Expiration

**Scenario:** Token expires after 7 days (test mode)

**Symptom:**
```
ERROR tool error: invalid_grant: Token has been expired or revoked
```

**Fix:**
```bash
# Clear tokens
rm ~/.config/google-calendar-mcp/tokens.json

# Restart Mimir (triggers re-auth)
make run
```

---

## Performance

### Latency Breakdown

Typical `list-events` call:
- MCP protocol overhead: <5ms
- Google Calendar API: 100-300ms
- Total: ~150-350ms

**Optimization:** Results are not cached (calendar data changes frequently)

### Resource Usage

**Memory:**
- MCP client: ~5 MB (Go structs + JSON buffers)
- MCP server (Node.js): ~30-50 MB

**CPU:**
- Negligible during idle
- ~10-20ms CPU time per tool call (JSON marshaling)

---

## Future Enhancements

### Phase 3 (In Progress)
- ✅ Google Calendar integration
- ⏳ Google Drive RAG (MCP server: `google-drive-mcp`)
- ⏳ Scheduled reminders (cron-style tool calls)

### Phase 4 (Planned)
- Multi-calendar support (work/personal)
- Event conflict detection (cross-calendar)
- Smart scheduling (find free slots)

### MCP Protocol Evolution
- **Bidirectional notifications** - Server can push events to Mimir
- **Streaming responses** - Long-running operations (file uploads)
- **Resource subscriptions** - Watch for calendar changes

---

## Debugging

### Enable MCP Debug Logs

```bash
# .env
LOG_LEVEL=debug
```

**Output:**
```
DEBUG mcp server started command=npx
DEBUG tools listed count=5
DEBUG tool called name=list-events
```

### Inspect JSON-RPC Traffic

Add logging to `client.go`:

```go
// In client.call()
log.Printf("→ %s", string(data))  // Request
log.Printf("← %s", string(line))  // Response
```

### Test MCP Server Manually

```bash
# Run standalone
npx @cocal/google-calendar-mcp

# Send JSON-RPC request
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{...}}' | npx @cocal/google-calendar-mcp
```

---

## References

- [Model Context Protocol Specification](https://modelcontextprotocol.io/)
- [google-calendar-mcp GitHub](https://github.com/nspady/google-calendar-mcp)
- [JSON-RPC 2.0 Specification](https://www.jsonrpc.org/specification)
- [Google Calendar API](https://developers.google.com/calendar/api)
