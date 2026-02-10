#!/bin/bash
set -e

echo "🔐 Google Calendar Authentication"
echo "=================================="
echo ""

CREDS_FILE="/Users/silveira.nic/dev/personal/golang-tests/personal-assistant/gcp-oauth.keys.json"

export GOOGLE_OAUTH_CREDENTIALS="$CREDS_FILE"

echo "Starting MCP server for authentication..."
echo "A browser window should open shortly."
echo ""
echo "Press Ctrl+C after you complete authentication in the browser."
echo ""

npx @cocal/google-calendar-mcp
