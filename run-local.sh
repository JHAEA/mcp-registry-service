#!/bin/bash
# Local development script for MCP Registry Server

set -e

cd "$(dirname "$0")"

# Export environment variables (using proper quoting)
export REGISTRY_REPO_URL="https://github.com/JHAEA/askjack-mcp-registry"
export GITHUB_APP_ID="2664202"
export GITHUB_INSTALLATION_ID="104401507"
export GITHUB_APP_PRIVATE_KEY_PATH="./secrets/github-app-key.pem"
export WEBHOOK_SECRET='f9R*xoLkwej0M<3jushPwF>r'
export REGISTRY_BRANCH="main"
export PORT="8080"
export DATA_PATH="./data/registry"
export CLONE_TIMEOUT="2m"
export POLL_INTERVAL="1m"
export CACHE_SIZE="1000"

echo "Starting MCP Registry Server..."
echo "  Repository: $REGISTRY_REPO_URL"
echo "  Port: $PORT"
echo ""

go run ./cmd/registry
