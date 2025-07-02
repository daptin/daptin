#!/usr/bin/env python3
import json
import websocket
import sys
import time
import threading

def on_message(ws, message):
    print(f"\n📨 Received: {json.loads(message)}")

def on_error(ws, error):
    print(f"\n❌ WebSocket error: {error}")

def on_close(ws, close_status_code, close_msg):
    print("\n🔌 Connection closed")

def on_open(ws):
    print("✅ Connected to Daptin WebSocket server")
    
    def run_tests():
        # Test 1: List topics
        print("\n📋 Testing: List topics")
        ws.send(json.dumps({
            "method": "list-topicName",
            "attributes": {}
        }))
        time.sleep(1)
        
        # Test 2: Subscribe to a topic
        print("\n📢 Testing: Subscribe to user_account topic")
        ws.send(json.dumps({
            "method": "subscribe",
            "attributes": {
                "topicName": "user_account"
            }
        }))
        time.sleep(1)
        
        # Test 3: Create custom topic
        print("\n➕ Testing: Create custom topic")
        ws.send(json.dumps({
            "method": "create-topicName",
            "attributes": {
                "name": "test-channel"
            }
        }))
        time.sleep(1)
        
        # Test 4: Publish to custom topic
        print("\n✉️ Testing: Publish message")
        ws.send(json.dumps({
            "method": "new-message",
            "attributes": {
                "topicName": "test-channel",
                "message": "Hello from Daptin WebSocket!"
            }
        }))
        time.sleep(2)
        
        # Close connection
        print("\n👋 Closing connection")
        ws.close()
    
    # Run tests in a separate thread
    threading.Thread(target=run_tests).start()

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python test_websocket.py <JWT_TOKEN>")
        sys.exit(1)
    
    token = sys.argv[1]
    
    # Create WebSocket connection with authentication
    ws = websocket.WebSocketApp(
        "ws://localhost:6336/live",
        header={"Authorization": f"Bearer {token}"},
        on_open=on_open,
        on_message=on_message,
        on_error=on_error,
        on_close=on_close
    )
    
    ws.run_forever()