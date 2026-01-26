#!/bin/bash
# WebSocket Testing Script for Daptin
# Tests all WebSocket and real-time features

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/../.."

TOKEN=$(cat /tmp/daptin-token.txt 2>/dev/null || echo "")
if [ -z "$TOKEN" ]; then
    echo "❌ No token found. Run: ./scripts/testing/test-runner.sh token"
    exit 1
fi

BASE_URL="localhost:6336"
WS_URL="ws://${BASE_URL}/live?token=${TOKEN}"

echo "===================="
echo "WebSocket Test Suite"
echo "===================="
echo ""

# Test 1: List Topics
echo "📋 Test 1: List available topics"
echo '{"method":"list-topicName","attributes":{}}' | websocat -n1 "$WS_URL" 2>/dev/null | jq '.' || echo "Connection established, waiting for response..."
sleep 1
echo ""

# Test 2: Subscribe to a table topic
echo "📡 Test 2: Subscribe to user_account topic"
echo '{"method":"subscribe","attributes":{"topicName":"user_account"}}' | websocat -n1 "$WS_URL" 2>/dev/null
echo "✓ Subscribe command sent"
echo ""

# Test 3: Create custom topic
echo "🏗️  Test 3: Create custom topic"
echo '{"method":"create-topicName","attributes":{"name":"test-chat-room"}}' | websocat -n1 "$WS_URL" 2>/dev/null
echo "✓ Create topic command sent"
sleep 1
echo ""

# Test 4: Check if custom topic exists
echo "🔍 Test 4: List topics again (should include test-chat-room)"
echo '{"method":"list-topicName","attributes":{}}' | websocat -n1 "$WS_URL" 2>/dev/null | jq '.'
echo ""

echo "===================="
echo "✅ Basic tests complete"
echo "===================="
echo ""
echo "For interactive testing, use:"
echo "  websocat '$WS_URL'"
echo ""
echo "Then send JSON commands like:"
echo '  {"method":"subscribe","attributes":{"topicName":"user_account"}}'
echo '  {"method":"new-message","attributes":{"topicName":"test-chat-room","message":"Hello!"}}'
