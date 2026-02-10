package mcpmanager

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/fandangolas/mimir/internal/mcp"
	"github.com/fandangolas/mimir/internal/mcp/tools"
)

// Manager manages MCP clients and tools.
type Manager struct {
	calendarClient *mcp.Client
	calendarTool   *tools.CalendarTool
	enabled        bool
	logger         *slog.Logger
}

// Config holds MCP manager configuration.
type Config struct {
	Enabled                bool
	GoogleOAuthCredentials string
	GoogleCalendarTokens   string
}

// NewManager creates a new MCP manager.
func NewManager(cfg Config) *Manager {
	return &Manager{
		enabled: cfg.Enabled,
		logger:  slog.Default().With("component", "mcp_manager"),
	}
}

// Start initializes and starts all MCP clients.
func (m *Manager) Start(ctx context.Context, cfg Config) error {
	if !m.enabled {
		m.logger.Info("mcp disabled, skipping initialization")
		return nil
	}

	m.logger.Info("initializing mcp clients")

	// Start Google Calendar MCP client
	if err := m.startCalendarClient(ctx, cfg); err != nil {
		return fmt.Errorf("start calendar client: %w", err)
	}

	m.logger.Info("mcp clients initialized successfully")
	return nil
}

// startCalendarClient initializes the Google Calendar MCP client.
func (m *Manager) startCalendarClient(ctx context.Context, cfg Config) error {
	if cfg.GoogleOAuthCredentials == "" {
		return fmt.Errorf("GOOGLE_OAUTH_CREDENTIALS is required when MCP is enabled")
	}

	// Verify credentials file exists
	if _, err := os.Stat(cfg.GoogleOAuthCredentials); os.IsNotExist(err) {
		return fmt.Errorf("credentials file not found: %s", cfg.GoogleOAuthCredentials)
	}

	m.logger.Info("starting google calendar mcp client",
		"credentials", cfg.GoogleOAuthCredentials)

	// Create MCP client
	client := mcp.NewClient("npx", nil, nil)

	// Build environment variables
	env := []string{
		fmt.Sprintf("GOOGLE_OAUTH_CREDENTIALS=%s", cfg.GoogleOAuthCredentials),
	}

	// Add custom token path if specified
	if cfg.GoogleCalendarTokens != "" {
		// Ensure directory exists
		tokenDir := filepath.Dir(cfg.GoogleCalendarTokens)
		if err := os.MkdirAll(tokenDir, 0700); err != nil {
			return fmt.Errorf("create token directory: %w", err)
		}
		env = append(env, fmt.Sprintf("GOOGLE_CALENDAR_MCP_TOKEN_PATH=%s", cfg.GoogleCalendarTokens))
	}

	// Add enabled tools filter for security
	enabledTools := "list-events,create-event,search-events,get-freebusy,get-current-time"
	env = append(env, fmt.Sprintf("ENABLED_TOOLS=%s", enabledTools))

	// Start the MCP server process
	args := []string{"@cocal/google-calendar-mcp"}
	if err := client.StartWithCommand(ctx, "npx", args, env); err != nil {
		return fmt.Errorf("start mcp server: %w", err)
	}

	// Perform MCP initialization handshake
	if err := client.Initialize(ctx); err != nil {
		client.Close()
		return fmt.Errorf("initialize mcp client: %w", err)
	}

	// List available tools
	toolsList, err := client.ListTools(ctx)
	if err != nil {
		client.Close()
		return fmt.Errorf("list tools: %w", err)
	}

	m.logger.Info("calendar mcp client initialized",
		"tools_count", len(toolsList))

	// Create calendar tool wrapper
	m.calendarClient = client
	m.calendarTool = tools.NewCalendarTool(client)

	return nil
}

// CalendarTool returns the calendar tool if enabled.
func (m *Manager) CalendarTool() *tools.CalendarTool {
	return m.calendarTool
}

// IsEnabled returns whether MCP is enabled.
func (m *Manager) IsEnabled() bool {
	return m.enabled
}

// Close shuts down all MCP clients.
func (m *Manager) Close() error {
	if !m.enabled {
		return nil
	}

	m.logger.Info("shutting down mcp clients")

	if m.calendarClient != nil {
		if err := m.calendarClient.Close(); err != nil {
			m.logger.Error("error closing calendar client", "error", err)
		}
	}

	return nil
}
