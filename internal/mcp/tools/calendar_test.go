package tools

import (
	"testing"
	"time"

	"github.com/fandangolas/mimir/internal/mcp"
)

func TestNewCalendarTool(t *testing.T) {
	client := mcp.NewClient("echo", nil, nil)
	tool := NewCalendarTool(client)

	if tool == nil {
		t.Fatal("expected non-nil calendar tool")
	}

	if tool.client != client {
		t.Error("expected client to be set")
	}
}

func TestFormatToolResult(t *testing.T) {
	tests := []struct {
		name     string
		result   *mcp.ToolResult
		expected string
	}{
		{
			name: "single text content",
			result: &mcp.ToolResult{
				Content: []mcp.Content{
					{Type: "text", Text: "Event created"},
				},
			},
			expected: "Event created",
		},
		{
			name: "multiple text contents",
			result: &mcp.ToolResult{
				Content: []mcp.Content{
					{Type: "text", Text: "Event 1\n"},
					{Type: "text", Text: "Event 2"},
				},
			},
			expected: "Event 1\nEvent 2",
		},
		{
			name:     "empty content",
			result:   &mcp.ToolResult{Content: []mcp.Content{}},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatToolResult(tt.result)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestListEventsParams(t *testing.T) {
	params := ListEventsParams{
		CalendarID: "primary",
		TimeMin:    time.Now(),
		TimeMax:    time.Now().Add(24 * time.Hour),
		MaxResults: 10,
		Query:      "meeting",
	}

	if params.CalendarID != "primary" {
		t.Error("expected CalendarID to be 'primary'")
	}

	if params.MaxResults != 10 {
		t.Error("expected MaxResults to be 10")
	}

	if params.Query != "meeting" {
		t.Error("expected Query to be 'meeting'")
	}
}

func TestCreateEventParams(t *testing.T) {
	now := time.Now()
	later := now.Add(1 * time.Hour)

	params := CreateEventParams{
		CalendarID:  "primary",
		Summary:     "Team Meeting",
		Description: "Discuss Q1 goals",
		Location:    "Conference Room A",
		Start:       now,
		End:         later,
		Attendees:   []string{"user1@example.com", "user2@example.com"},
	}

	if params.Summary != "Team Meeting" {
		t.Error("expected Summary to be 'Team Meeting'")
	}

	if params.Start != now {
		t.Error("expected Start time to match")
	}

	if params.End != later {
		t.Error("expected End time to match")
	}

	if len(params.Attendees) != 2 {
		t.Errorf("expected 2 attendees, got %d", len(params.Attendees))
	}
}

func TestGetFreeBusyParams(t *testing.T) {
	now := time.Now()
	later := now.Add(7 * 24 * time.Hour)

	params := GetFreeBusyParams{
		CalendarIDs: []string{"primary", "work@example.com"},
		TimeMin:     now,
		TimeMax:     later,
	}

	if len(params.CalendarIDs) != 2 {
		t.Errorf("expected 2 calendar IDs, got %d", len(params.CalendarIDs))
	}

	if params.TimeMin != now {
		t.Error("expected TimeMin to match")
	}

	if params.TimeMax != later {
		t.Error("expected TimeMax to match")
	}
}
