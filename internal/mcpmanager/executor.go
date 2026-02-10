package mcpmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fandangolas/mimir/internal/llm"
	"github.com/fandangolas/mimir/internal/mcp/tools"
)

// ExecuteToolCall executes a tool call and returns the result.
func (m *Manager) ExecuteToolCall(ctx context.Context, toolCall llm.ToolCall) (string, error) {
	if !m.enabled {
		return "", fmt.Errorf("MCP not enabled")
	}

	switch toolCall.Function.Name {
	case "list_calendar_events":
		return m.executeListEvents(ctx, []byte(toolCall.Function.Arguments))
	case "create_calendar_event":
		return m.executeCreateEvent(ctx, []byte(toolCall.Function.Arguments))
	case "search_calendar_events":
		return m.executeSearchEvents(ctx, []byte(toolCall.Function.Arguments))
	case "get_current_time":
		return m.executeGetCurrentTime(ctx)
	default:
		return "", fmt.Errorf("unknown tool: %s", toolCall.Function.Name)
	}
}

func (m *Manager) executeListEvents(ctx context.Context, argsJSON []byte) (string, error) {
	var rawArgs map[string]interface{}
	if err := json.Unmarshal(argsJSON, &rawArgs); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	// Extract days_ahead
	daysAhead := 7 // default
	if v, ok := rawArgs["days_ahead"].(float64); ok {
		daysAhead = int(v)
	}

	// Calculate time range
	now := time.Now()
	timeMin := now.Truncate(24 * time.Hour) // Start of today
	if daysAhead > 0 {
		timeMin = timeMin.Add(time.Duration(daysAhead) * 24 * time.Hour)
	}
	timeMax := timeMin.Add(24 * time.Hour) // End of the day

	params := tools.ListEventsParams{
		CalendarID: "primary",
		TimeMin:    timeMin,
		TimeMax:    timeMax,
		MaxResults: 20,
	}

	return m.calendarTool.ListEvents(ctx, params)
}

func (m *Manager) executeCreateEvent(ctx context.Context, argsJSON []byte) (string, error) {
	var args struct {
		Summary     string `json:"summary"`
		Start       string `json:"start"`
		End         string `json:"end"`
		Description string `json:"description"`
		Location    string `json:"location"`
	}

	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	start, err := time.Parse(time.RFC3339, args.Start)
	if err != nil {
		return "", fmt.Errorf("parse start time: %w", err)
	}

	end, err := time.Parse(time.RFC3339, args.End)
	if err != nil {
		return "", fmt.Errorf("parse end time: %w", err)
	}

	params := tools.CreateEventParams{
		CalendarID:  "primary", // Always use primary calendar
		Summary:     args.Summary,
		Start:       start,
		End:         end,
		Description: args.Description,
		Location:    args.Location,
	}

	return m.calendarTool.CreateEvent(ctx, params)
}

func (m *Manager) executeSearchEvents(ctx context.Context, argsJSON []byte) (string, error) {
	var rawArgs map[string]interface{}
	if err := json.Unmarshal(argsJSON, &rawArgs); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	var args struct {
		Query      string `json:"query"`
		MaxResults int    `json:"max_results"`
	}

	if v, ok := rawArgs["query"].(string); ok {
		args.Query = v
	}

	// Handle max_results as either string or number
	if v, ok := rawArgs["max_results"].(float64); ok {
		args.MaxResults = int(v)
	} else if v, ok := rawArgs["max_results"].(string); ok {
		var maxRes int
		if _, err := fmt.Sscanf(v, "%d", &maxRes); err == nil {
			args.MaxResults = maxRes
		}
	}

	return m.calendarTool.SearchEvents(ctx, args.Query, args.MaxResults)
}

func (m *Manager) executeGetCurrentTime(ctx context.Context) (string, error) {
	return m.calendarTool.GetCurrentTime(ctx)
}
