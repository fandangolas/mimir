package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/fandangolas/mimir/internal/mcp"
)

// CalendarTool provides Google Calendar integration via MCP.
type CalendarTool struct {
	client *mcp.Client
}

// NewCalendarTool creates a new calendar tool.
func NewCalendarTool(client *mcp.Client) *CalendarTool {
	return &CalendarTool{
		client: client,
	}
}

// Event represents a calendar event.
type Event struct {
	ID          string    `json:"id"`
	Summary     string    `json:"summary"`
	Description string    `json:"description,omitempty"`
	Location    string    `json:"location,omitempty"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	Attendees   []string  `json:"attendees,omitempty"`
}

// ListEventsParams are parameters for listing calendar events.
type ListEventsParams struct {
	CalendarID string    `json:"calendarId,omitempty"` // Default: "primary"
	TimeMin    time.Time `json:"timeMin,omitempty"`
	TimeMax    time.Time `json:"timeMax,omitempty"`
	MaxResults int       `json:"maxResults,omitempty"` // Default: 10
	Query      string    `json:"q,omitempty"`          // Free-text search
}

// CreateEventParams are parameters for creating a calendar event.
type CreateEventParams struct {
	CalendarID  string    `json:"calendarId,omitempty"` // Default: "primary"
	Summary     string    `json:"summary"`
	Description string    `json:"description,omitempty"`
	Location    string    `json:"location,omitempty"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	Attendees   []string  `json:"attendees,omitempty"`
}

// GetFreeBusyParams are parameters for checking availability.
type GetFreeBusyParams struct {
	CalendarIDs []string  `json:"calendarIds"`
	TimeMin     time.Time `json:"timeMin"`
	TimeMax     time.Time `json:"timeMax"`
}

// FreeBusyResult represents availability information.
type FreeBusyResult struct {
	CalendarID string              `json:"calendarId"`
	Busy       []FreeBusyTimeRange `json:"busy"`
}

// FreeBusyTimeRange represents a busy time slot.
type FreeBusyTimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// ListEvents retrieves calendar events.
func (ct *CalendarTool) ListEvents(ctx context.Context, params ListEventsParams) (string, error) {
	args := make(map[string]interface{})

	if params.CalendarID != "" {
		args["calendarId"] = params.CalendarID
	}
	if !params.TimeMin.IsZero() {
		args["timeMin"] = params.TimeMin.Format(time.RFC3339)
	}
	if !params.TimeMax.IsZero() {
		args["timeMax"] = params.TimeMax.Format(time.RFC3339)
	}
	if params.MaxResults > 0 {
		args["maxResults"] = params.MaxResults
	}
	if params.Query != "" {
		args["q"] = params.Query
	}

	result, err := ct.client.CallTool(ctx, "list-events", args)
	if err != nil {
		return "", fmt.Errorf("list events: %w", err)
	}

	return formatToolResult(result), nil
}

// CreateEvent creates a new calendar event.
func (ct *CalendarTool) CreateEvent(ctx context.Context, params CreateEventParams) (string, error) {
	args := map[string]interface{}{
		"summary": params.Summary,
		"start":   params.Start.Format(time.RFC3339),
		"end":     params.End.Format(time.RFC3339),
	}

	if params.CalendarID != "" {
		args["calendarId"] = params.CalendarID
	}
	if params.Description != "" {
		args["description"] = params.Description
	}
	if params.Location != "" {
		args["location"] = params.Location
	}
	if len(params.Attendees) > 0 {
		args["attendees"] = params.Attendees
	}

	result, err := ct.client.CallTool(ctx, "create-event", args)
	if err != nil {
		return "", fmt.Errorf("create event: %w", err)
	}

	return formatToolResult(result), nil
}

// SearchEvents searches for calendar events.
func (ct *CalendarTool) SearchEvents(ctx context.Context, query string, maxResults int) (string, error) {
	args := map[string]interface{}{
		"q": query,
	}

	if maxResults > 0 {
		args["maxResults"] = maxResults
	}

	result, err := ct.client.CallTool(ctx, "search-events", args)
	if err != nil {
		return "", fmt.Errorf("search events: %w", err)
	}

	return formatToolResult(result), nil
}

// GetFreeBusy retrieves free/busy information.
func (ct *CalendarTool) GetFreeBusy(ctx context.Context, params GetFreeBusyParams) (string, error) {
	args := map[string]interface{}{
		"calendarIds": params.CalendarIDs,
		"timeMin":     params.TimeMin.Format(time.RFC3339),
		"timeMax":     params.TimeMax.Format(time.RFC3339),
	}

	result, err := ct.client.CallTool(ctx, "get-freebusy", args)
	if err != nil {
		return "", fmt.Errorf("get free/busy: %w", err)
	}

	return formatToolResult(result), nil
}

// GetCurrentTime retrieves the current time (useful for the LLM context).
func (ct *CalendarTool) GetCurrentTime(ctx context.Context) (string, error) {
	result, err := ct.client.CallTool(ctx, "get-current-time", nil)
	if err != nil {
		return "", fmt.Errorf("get current time: %w", err)
	}

	return formatToolResult(result), nil
}

// formatToolResult extracts text content from tool result.
func formatToolResult(result *mcp.ToolResult) string {
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
