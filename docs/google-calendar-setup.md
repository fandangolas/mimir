# Google Calendar Integration Setup

This guide walks you through setting up Google Calendar integration for Mimir via the Model Context Protocol (MCP).

## Overview

Mimir integrates with Google Calendar using the `google-calendar-mcp` server by nspady. This allows Mimir to:

- List and search calendar events
- Create new events
- Check free/busy availability
- Access current time for context

All calendar operations require explicit user consent via OAuth 2.0.

---

## Prerequisites

1. **Node.js and NPM** - Required to run the MCP server
   ```bash
   node --version  # Should be v18 or higher
   npm --version   # Should be v8 or higher
   ```

2. **Google Cloud Project** - For OAuth credentials
3. **Mimir installed** - This guide assumes you've completed the Quick Start

---

## Step 1: Create Google Cloud Project

### 1.1 Create Project

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Click "Select a project" → "New Project"
3. Name: `Mimir Personal Assistant`
4. Click "Create"

### 1.2 Enable Google Calendar API

1. Navigate to "APIs & Services" → "Library"
2. Search for "Google Calendar API"
3. Click "Enable"

### 1.3 Configure OAuth Consent Screen

1. Navigate to "APIs & Services" → "OAuth consent screen"
2. Select "External" (unless you have a Google Workspace)
3. Click "Create"

Fill in required fields:
- **App name:** Mimir Personal Assistant
- **User support email:** Your email
- **Developer contact email:** Your email

4. Click "Save and Continue"
5. **Scopes:** Click "Add or Remove Scopes"
   - Search for and add: `https://www.googleapis.com/auth/calendar`
   - Click "Update" → "Save and Continue"
6. **Test users:** Click "Add Users"
   - Add your Google email address
   - Click "Save and Continue"
7. Click "Back to Dashboard"

### 1.4 Create OAuth Credentials

1. Navigate to "APIs & Services" → "Credentials"
2. Click "Create Credentials" → "OAuth client ID"
3. Application type: "Desktop app"
4. Name: `Mimir Desktop Client`
5. Click "Create"
6. **Download JSON** - Click the download icon next to your newly created client ID
7. Save the file as `gcp-oauth.keys.json` in a secure location

**Important:** Never commit this file to version control!

---

## Step 2: Configure Mimir

### 2.1 Update Environment Variables

Edit your `.env` file:

```bash
# Enable MCP
MCP_ENABLED=true

# Path to your OAuth credentials file
GOOGLE_OAUTH_CREDENTIALS=/path/to/gcp-oauth.keys.json

# Optional: Custom token storage path (defaults to system config directory)
# GOOGLE_CALENDAR_MCP_TOKEN_PATH=/path/to/custom/tokens.json
```

**Recommended token storage locations:**
- **macOS/Linux:** `~/.config/mimir/google-calendar-tokens.json`
- **Custom:** Any secure directory with restricted permissions

### 2.2 Verify Node.js Installation

The MCP server will be automatically installed via `npx` when Mimir starts. Ensure `npx` is available:

```bash
npx --version
```

---

## Step 3: First-Time Authentication

### 3.1 Start Mimir

```bash
make run
```

You should see:
```
INFO mcp enabled, initializing manager
INFO starting google calendar mcp client credentials=/path/to/gcp-oauth.keys.json
```

### 3.2 Complete OAuth Flow

The first time Mimir starts with MCP enabled:

1. A browser window will open automatically
2. Sign in with your Google account (the one added as a test user)
3. You'll see a warning: "Google hasn't verified this app"
   - Click "Advanced"
   - Click "Go to Mimir Personal Assistant (unsafe)" - This is safe; it's your own app
4. Review permissions:
   - "See, edit, share, and permanently delete all the calendars you can access using Google Calendar"
5. Click "Allow"
6. You should see "Authentication successful!"
7. Return to the terminal - Mimir will continue starting

**Logs will show:**
```
INFO calendar mcp client initialized tools_count=5
INFO mcp manager initialized successfully
```

---

## Step 4: Test the Integration

### 4.1 Basic Test via Telegram

Send a message to your Mimir bot:

```
What's on my calendar today?
```

Mimir should respond with your calendar events.

### 4.2 Create an Event

```
Create a meeting tomorrow at 2pm for 1 hour called "Team Sync"
```

Verify the event was created in Google Calendar.

---

## Security Best Practices

### File Permissions

Ensure your credentials and tokens are secure:

```bash
# OAuth credentials (read-only)
chmod 600 /path/to/gcp-oauth.keys.json

# Token directory (read/write for owner only)
mkdir -p ~/.config/mimir
chmod 700 ~/.config/mimir
```

### Token Expiration

**Test Mode (default):**
- OAuth tokens expire after 7 days
- You'll need to re-authenticate weekly

**Production Mode (optional):**
1. Submit your app for verification (see Google Cloud Console)
2. Once verified, tokens won't expire

To re-authenticate manually:
```bash
# Clear tokens
rm ~/.config/google-calendar-mcp/tokens.json

# Restart Mimir (will trigger OAuth flow)
make run
```

### Restricted Tool Access

Mimir only enables these tools for security:
- `list-events` - View calendar events
- `create-event` - Create new events
- `search-events` - Search events by keyword
- `get-freebusy` - Check availability
- `get-current-time` - Get current time

**Disabled tools:**
- `update-event`
- `delete-event`
- `respond-to-event`
- `manage-accounts`

To modify enabled tools, edit `internal/mcp/manager.go:62`.

---

## Troubleshooting

### "GOOGLE_OAUTH_CREDENTIALS is required"

**Cause:** Environment variable not set or file doesn't exist

**Fix:**
```bash
# Verify file exists
ls -l /path/to/gcp-oauth.keys.json

# Check .env file
cat .env | grep GOOGLE_OAUTH_CREDENTIALS
```

### "credentials file not found"

**Cause:** Path in `.env` is incorrect

**Fix:** Use absolute path (not relative):
```bash
GOOGLE_OAUTH_CREDENTIALS=/Users/yourname/mimir/gcp-oauth.keys.json
```

### "Authentication failed"

**Cause:** Multiple possibilities

**Fix:**
1. Ensure your email is added as a test user in OAuth consent screen
2. Clear browser cookies and retry
3. Check that Calendar API is enabled
4. Verify `project_id` is present in `gcp-oauth.keys.json`:
   ```json
   {
     "installed": {
       "client_id": "...",
       "project_id": "mimir-personal-assistant",  // Must be present
       ...
     }
   }
   ```

### "User Rate Limit Exceeded"

**Cause:** Missing `project_id` in credentials file

**Fix:** Re-download credentials from Google Cloud Console (Step 1.4)

### Browser doesn't open automatically

**Cause:** Non-Chromium browser or headless environment

**Fix:**
1. Check terminal for URL
2. Manually copy/paste URL into browser
3. Complete OAuth flow

### MCP server process doesn't start

**Cause:** Node.js/NPM not installed or `npx` not in PATH

**Fix:**
```bash
# Install Node.js (macOS)
brew install node

# Verify
npx --version

# Test MCP server manually
npx @cocal/google-calendar-mcp
```

---

## Available Calendar Tools

Once configured, Mimir can perform these calendar operations:

### List Events
```
What's on my calendar today?
Show me my meetings this week
```

### Create Events
```
Create a meeting tomorrow at 2pm for 1 hour called "Team Sync"
Schedule "Doctor appointment" on Friday at 10am
```

### Search Events
```
Find all meetings with "John" this month
Search for "standup" in my calendar
```

### Check Availability
```
Am I free tomorrow at 3pm?
Check if I have any conflicts on Monday
```

### Current Time Context
Mimir automatically uses `get-current-time` to understand temporal context in your queries.

---

## Architecture

```
Mimir (Go)
    ↓
internal/mcp/manager.go
    ↓ (stdio)
google-calendar-mcp (Node.js/TypeScript)
    ↓ (HTTPS)
Google Calendar API
```

**Communication flow:**
1. Mimir spawns `npx @cocal/google-calendar-mcp` as subprocess
2. MCP protocol (JSON-RPC over stdio) for tool calls
3. MCP server handles OAuth and API calls to Google
4. Results returned to Mimir as structured text

---

## Data Privacy

- **All calendar data stays local** - No external LLM providers see your calendar
- **OAuth tokens stored locally** - In system config directory with 600 permissions
- **No data logging** - Only correlation IDs in Mimir logs
- **Explicit consent** - Every OAuth scope requires user approval

---

## Advanced Configuration

### Multiple Google Accounts (Future)

The `google-calendar-mcp` server supports multi-account mode. To enable:

1. Authenticate additional accounts
2. Specify account ID in tool calls
3. Query across accounts simultaneously

See [google-calendar-mcp docs](https://github.com/nspady/google-calendar-mcp) for details.

### Custom Token Path

If you want tokens stored in a specific location:

```bash
# .env
GOOGLE_CALENDAR_MCP_TOKEN_PATH=/path/to/secure/tokens.json
```

Mimir will create the directory with 700 permissions automatically.

---

## Uninstalling

To disable Google Calendar integration:

```bash
# .env
MCP_ENABLED=false
```

To completely remove:

```bash
# Stop Mimir
# Remove credentials
rm /path/to/gcp-oauth.keys.json

# Remove tokens
rm -rf ~/.config/google-calendar-mcp

# Revoke access (optional)
# Visit: https://myaccount.google.com/permissions
# Find "Mimir Personal Assistant" → Remove Access
```

---

## Next Steps

- **Google Drive RAG** (Phase 3) - Index personal documents
- **Scheduled Reminders** (Phase 3) - Proactive calendar notifications
- **Multiple Calendars** - Work/Personal separation

See [development-phases.md](development-phases.md) for the complete roadmap.

---

## References

- [google-calendar-mcp GitHub](https://github.com/nspady/google-calendar-mcp)
- [Google Calendar API Documentation](https://developers.google.com/calendar/api)
- [Model Context Protocol Specification](https://modelcontextprotocol.io/)
- [OAuth 2.0 for Desktop Apps](https://developers.google.com/identity/protocols/oauth2/native-app)
