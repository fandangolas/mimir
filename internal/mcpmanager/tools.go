package mcpmanager

import (
	"github.com/fandangolas/mimir/internal/llm"
)

// GetLLMTools returns tool definitions for the LLM.
func (m *Manager) GetLLMTools() []llm.Tool {
	if !m.enabled || m.calendarTool == nil {
		return nil
	}

	return []llm.Tool{
		{
			Type: "function",
			Function: llm.FunctionDef{
				Name:        "list_calendar_events",
				Description: "List events from Google Calendar. Always call this function when user asks about their schedule, calendar, meetings, or appointments.",
				Parameters: map[string]interface{}{
					"type":     "object",
					"required": []string{"days_ahead"},
					"properties": map[string]interface{}{
						"days_ahead": map[string]interface{}{
							"type":        "integer",
							"description": "Number of days to look ahead. Use 0 for today, 1 for tomorrow, 7 for this week.",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: llm.FunctionDef{
				Name:        "create_calendar_event",
				Description: "Create a new event in Google Calendar. Use this when the user asks to schedule something or create an appointment.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"summary": map[string]interface{}{
							"type":        "string",
							"description": "Event title/summary (e.g., 'Team Meeting', 'Doctor Appointment').",
						},
						"start": map[string]interface{}{
							"type":        "string",
							"description": "Start time in RFC3339 format (e.g., '2025-02-09T14:00:00Z').",
						},
						"end": map[string]interface{}{
							"type":        "string",
							"description": "End time in RFC3339 format (e.g., '2025-02-09T15:00:00Z').",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Optional detailed description of the event.",
						},
						"location": map[string]interface{}{
							"type":        "string",
							"description": "Optional location (e.g., 'Conference Room A', 'Zoom').",
						},
					},
					"required": []string{"summary", "start", "end"},
				},
			},
		},
		{
			Type: "function",
			Function: llm.FunctionDef{
				Name:        "search_calendar_events",
				Description: "Search for events in Google Calendar by keyword. Use this to find specific meetings or appointments.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Search query (e.g., 'standup', 'John', 'dentist').",
						},
						"max_results": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum number of results. Defaults to 10.",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		{
			Type: "function",
			Function: llm.FunctionDef{
				Name:        "get_current_time",
				Description: "Get the current date and time. Use this to understand what 'today', 'tomorrow', 'this week' means in context.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
	}
}
