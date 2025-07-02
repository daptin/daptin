# WebSocket Authentication Solution

## The Problem
WebSocket connections were returning 403 Forbidden when trying to authenticate with headers:
```bash
# This DOESN'T work
curl -H "Authorization: Bearer $TOKEN" "ws://localhost:6336/live"
```

## The Solution
Pass the JWT token as a **query parameter**:
```bash
# This WORKS!
curl "ws://localhost:6336/live?token=$TOKEN"
```

## Working Examples

### Browser JavaScript
```javascript
const token = localStorage.getItem('token');
const ws = new WebSocket(`ws://localhost:6336/live?token=${token}`);

ws.onopen = () => {
  console.log('Connected!');
  
  // Subscribe to events
  ws.send(JSON.stringify({
    method: 'subscribe',
    attributes: {
      topicName: 'user_account'
    }
  }));
};
```

### Python
```python
import websocket
import json

token = "YOUR_JWT_TOKEN"
ws = websocket.WebSocketApp(
    f"ws://localhost:6336/live?token={token}",
    on_open=on_open,
    on_message=on_message
)
ws.run_forever()
```

### YJS Connection
```javascript
const provider = new WebsocketProvider(
  `ws://localhost:6336/live/document/${docId}/content/yjs?token=${token}`,
  'room-name',
  ydoc
);
```

## Why Query Parameters?
The Daptin WebSocket handler extracts authentication from the initial HTTP upgrade request. Since WebSocket libraries don't consistently support custom headers during the upgrade handshake, query parameters provide a reliable cross-platform solution.

## Security Note
While query parameters can appear in logs, this is acceptable for WebSocket connections because:
1. The connection upgrades to WebSocket protocol immediately
2. Subsequent messages use the established WebSocket connection
3. Tokens have limited lifetime (72 hours by default)