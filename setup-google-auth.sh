#!/bin/bash
set -e

echo "🔐 Mimir Google Calendar Authentication Setup"
echo "=============================================="
echo ""

PROJECT_DIR="/Users/silveira.nic/dev/personal/golang-tests/personal-assistant"
CREDS_FILE="$PROJECT_DIR/gcp-oauth.keys.json"

# Step 1: Check if credentials file exists
echo "Step 1: Checking for OAuth credentials..."
if [ -f "$CREDS_FILE" ]; then
    echo "✅ Found credentials file: $CREDS_FILE"
else
    echo "❌ Credentials file not found!"
    echo ""
    echo "Please download your OAuth credentials from Google Cloud Console:"
    echo "1. Go to: https://console.cloud.google.com/apis/credentials"
    echo "2. Download the JSON file for your OAuth 2.0 Client ID"
    echo "3. Move it to: $CREDS_FILE"
    echo ""
    echo "Then run this script again."
    exit 1
fi

# Step 2: Verify file has correct structure
echo ""
echo "Step 2: Verifying credentials file format..."
if grep -q "client_id" "$CREDS_FILE" && grep -q "client_secret" "$CREDS_FILE"; then
    echo "✅ Credentials file format looks good"
else
    echo "❌ Credentials file doesn't look right"
    echo "Please re-download from Google Cloud Console"
    exit 1
fi

# Step 3: Check Node.js/NPM
echo ""
echo "Step 3: Checking Node.js and NPM..."
if command -v node &> /dev/null; then
    NODE_VERSION=$(node --version)
    echo "✅ Node.js installed: $NODE_VERSION"
else
    echo "❌ Node.js not found!"
    echo ""
    echo "Install Node.js:"
    echo "  brew install node"
    exit 1
fi

if command -v npx &> /dev/null; then
    NPX_VERSION=$(npx --version)
    echo "✅ NPX installed: $NPX_VERSION"
else
    echo "❌ NPX not found (should come with Node.js)"
    exit 1
fi

# Step 4: Test MCP server
echo ""
echo "Step 4: Testing google-calendar-mcp installation..."
echo "(This will download the MCP server if needed)"
echo ""

# Quick test - just check if it can be resolved
if npx --yes @cocal/google-calendar-mcp --help 2>/dev/null | head -1; then
    echo "✅ MCP server can be installed"
else
    echo "⚠️  MCP server not found, but will be installed on first run"
fi

# Step 5: Summary
echo ""
echo "=============================================="
echo "✅ Setup complete! You're ready to authenticate."
echo ""
echo "Next steps:"
echo "1. Make sure your database and Ollama are running:"
echo "   make up"
echo ""
echo "2. Start Mimir:"
echo "   make run"
echo ""
echo "3. A browser window will open for OAuth authentication"
echo "4. Sign in with your Google account"
echo "5. Grant calendar permissions"
echo "6. Done!"
echo ""
echo "After authentication, test with:"
echo '   "What'\''s on my calendar today?"'
echo ""
