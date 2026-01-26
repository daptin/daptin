#!/usr/bin/env python3
"""
WebSocket Interactive Test for Daptin
Tests real-time pub/sub features
"""

import asyncio
import json
import sys
import signal

try:
    import websockets
except ImportError:
    print("❌ websockets package not found. Install with: pip3 install websockets")
    sys.exit(1)

# Read token
try:
    with open('/tmp/daptin-token.txt', 'r') as f:
        TOKEN = f.read().strip()
except FileNotFoundError:
    print("❌ No token found. Run: ./scripts/testing/test-runner.sh token")
    sys.exit(1)

WS_URL = f"ws://localhost:6336/live?token={TOKEN}"

test_results = {
    'connection': False,
    'list_topics': False,
    'create_topic': False,
    'subscribe': False,
    'publish': False,
    'receive_event': False
}

received_topics = []
test_step = 0

async def test_websocket():
    global test_step, received_topics

    print("🚀 Daptin WebSocket Test")
    print("========================\n")

    try:
        async with websockets.connect(WS_URL) as ws:
            print("✅ WebSocket connected")
            test_results['connection'] = True

            # Test 1: List topics
            print("\n📋 Test 1: Listing available topics...")
            await ws.send(json.dumps({
                "method": "list-topicName",
                "attributes": {}
            }))

            # Start receiving messages
            test_step = 0
            async for message in ws:
                try:
                    msg = json.loads(message)
                    print(f"\n📨 Received: {json.dumps(msg, indent=2)}")

                    # Handle list-topic response
                    if msg.get('ObjectType') == 'topicName-list' or msg.get('EventType') == 'response':
                        test_results['list_topics'] = True
                        event_data = json.loads(msg['EventData'])
                        received_topics = event_data.get('topics', [])
                        print(f"✅ Topics available: {', '.join(received_topics)}")

                        if test_step == 0:
                            test_step = 1
                            # Test 2: Create custom topic
                            print("\n🏗️  Test 2: Creating custom topic 'ws-test-room'...")
                            await ws.send(json.dumps({
                                "method": "create-topicName",
                                "attributes": {"name": "ws-test-room"}
                            }))
                            test_results['create_topic'] = True

                            # Wait a bit then list topics again
                            await asyncio.sleep(0.5)
                            print("\n🔍 Verifying topic creation...")
                            await ws.send(json.dumps({
                                "method": "list-topicName",
                                "attributes": {}
                            }))
                            test_step = 2

                        elif test_step == 2:
                            test_step = 3
                            # Test 3: Subscribe to topic
                            print("\n📡 Test 3: Subscribing to ws-test-room...")
                            await ws.send(json.dumps({
                                "method": "subscribe",
                                "attributes": {"topicName": "ws-test-room"}
                            }))
                            test_results['subscribe'] = True

                            # Test 4: Publish message
                            await asyncio.sleep(0.5)
                            print("\n📤 Test 4: Publishing message to ws-test-room...")
                            await ws.send(json.dumps({
                                "method": "new-message",
                                "attributes": {
                                    "topicName": "ws-test-room",
                                    "message": {
                                        "text": "Hello from Python test!",
                                        "timestamp": "2026-01-26T12:00:00Z"
                                    }
                                }
                            }))
                            test_results['publish'] = True

                            # Test 5: Subscribe to user_account (table topic)
                            await asyncio.sleep(1)
                            print("\n📡 Test 5: Subscribing to user_account table...")
                            await ws.send(json.dumps({
                                "method": "subscribe",
                                "attributes": {"topicName": "user_account"}
                            }))

                            # Finish test after 3 seconds
                            await asyncio.sleep(3)
                            print_results()
                            break

                    elif msg.get('EventType') == 'new-message':
                        print("✅ Received published message!")
                        test_results['receive_event'] = True
                        event_data = json.loads(msg['EventData'])
                        print(f"   Message content: {event_data}")

                    elif msg.get('EventType') in ['create', 'update', 'delete']:
                        print(f"✅ Received {msg['EventType']} event from {msg.get('ObjectType')}")
                        test_results['receive_event'] = True

                except json.JSONDecodeError as e:
                    print(f"❌ Error parsing message: {e}")
                    print(f"   Raw data: {message}")
                except KeyError as e:
                    print(f"⚠️  Message missing expected field: {e}")

    except websockets.exceptions.WebSocketException as e:
        print(f"❌ WebSocket error: {e}")
    except Exception as e:
        print(f"❌ Unexpected error: {e}")
        import traceback
        traceback.print_exc()

def print_results():
    print("\n")
    print("========================")
    print("📊 Test Results Summary")
    print("========================")
    for test, passed in test_results.items():
        status = "✅ PASSED" if passed else "❌ FAILED"
        print(f"{status}: {test}")
    print("========================\n")

def signal_handler(sig, frame):
    print("\n\n⚠️  Test interrupted")
    print_results()
    sys.exit(0)

signal.signal(signal.SIGINT, signal_handler)

if __name__ == "__main__":
    asyncio.run(test_websocket())
