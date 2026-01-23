# WebSocket API

Real-time pub/sub messaging via WebSocket.

## Endpoint

```
ws://localhost:6336/live
wss://localhost:6443/live
```

## Authentication

Pass JWT token as query parameter:

```javascript
const ws = new WebSocket(`ws://localhost:6336/live?token=${TOKEN}`);
```

## Message Format

All messages are JSON:

```javascript
{
  "method": "method_name",
  "attributes": {...}
}
```

## Methods

### subscribe

Subscribe to topics for real-time updates.

```javascript
// Subscribe to single topic
ws.send(JSON.stringify({
  "method": "subscribe",
  "attributes": {
    "topicName": "user_account"
  }
}));

// Subscribe to multiple topics
ws.send(JSON.stringify({
  "method": "subscribe",
  "attributes": {
    "topicName": "user_account,document,order"
  }
}));
```

### publish / new-message

Publish message to topic.

```javascript
ws.send(JSON.stringify({
  "method": "new-message",
  "attributes": {
    "topicName": "chat-room-1",
    "message": "Hello everyone!"
  }
}));
```

### create-topic

Create custom topic.

```javascript
ws.send(JSON.stringify({
  "method": "create-topic",
  "attributes": {
    "name": "custom-notifications"
  }
}));
```

### list-topic

List available topics.

```javascript
ws.send(JSON.stringify({
  "method": "list-topic",
  "attributes": {}
}));
```

### get-topic

Get topic information.

```javascript
ws.send(JSON.stringify({
  "method": "get-topic",
  "attributes": {
    "topicName": "user_account"
  }
}));
```

## System Topics

Each database table has an automatic topic:

- `user_account` - User changes
- `document` - Document changes
- `order` - Order changes
- etc.

## Event Messages

When data changes, subscribers receive:

```json
{
  "type": "event",
  "topic": "order",
  "event": "create",
  "data": {
    "type": "order",
    "id": "order-123",
    "attributes": {...}
  }
}
```

### Event Types

| Event | Description |
|-------|-------------|
| `create` | New record created |
| `update` | Record updated |
| `delete` | Record deleted |

## Permission-Aware

Events are filtered based on user permissions:
- Users only receive events for records they can read
- Admins receive all events

## JavaScript Client

```javascript
class DaptinWebSocket {
  constructor(baseUrl, token) {
    this.url = `${baseUrl}/live?token=${token}`;
    this.handlers = {};
    this.connect();
  }

  connect() {
    this.ws = new WebSocket(this.url);

    this.ws.onopen = () => {
      console.log('Connected to Daptin WebSocket');
    };

    this.ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      this.handleMessage(data);
    };

    this.ws.onclose = () => {
      // Reconnect after 5 seconds
      setTimeout(() => this.connect(), 5000);
    };
  }

  handleMessage(data) {
    if (data.type === 'event' && this.handlers[data.topic]) {
      this.handlers[data.topic](data);
    }
  }

  subscribe(topics, handler) {
    const topicList = Array.isArray(topics) ? topics : [topics];
    topicList.forEach(topic => {
      this.handlers[topic] = handler;
    });

    this.ws.send(JSON.stringify({
      method: 'subscribe',
      attributes: { topicName: topicList.join(',') }
    }));
  }

  publish(topic, message) {
    this.ws.send(JSON.stringify({
      method: 'new-message',
      attributes: { topicName: topic, message }
    }));
  }

  createTopic(name) {
    this.ws.send(JSON.stringify({
      method: 'create-topic',
      attributes: { name }
    }));
  }
}

// Usage
const ws = new DaptinWebSocket('ws://localhost:6336', TOKEN);

// Subscribe to order updates
ws.subscribe('order', (event) => {
  console.log('Order event:', event.event, event.data);
});

// Subscribe to multiple tables
ws.subscribe(['user_account', 'document'], (event) => {
  console.log('Event:', event.topic, event.event);
});

// Create custom chat topic
ws.createTopic('chat-room-1');

// Publish to custom topic
ws.publish('chat-room-1', 'Hello!');
```

## React Hook Example

```javascript
import { useEffect, useState, useCallback } from 'react';

function useDaptinWebSocket(token) {
  const [ws, setWs] = useState(null);
  const [connected, setConnected] = useState(false);
  const [messages, setMessages] = useState([]);

  useEffect(() => {
    const socket = new WebSocket(`ws://localhost:6336/live?token=${token}`);

    socket.onopen = () => setConnected(true);
    socket.onclose = () => setConnected(false);
    socket.onmessage = (e) => {
      const data = JSON.parse(e.data);
      setMessages(prev => [...prev, data]);
    };

    setWs(socket);
    return () => socket.close();
  }, [token]);

  const subscribe = useCallback((topics) => {
    if (ws && connected) {
      ws.send(JSON.stringify({
        method: 'subscribe',
        attributes: { topicName: Array.isArray(topics) ? topics.join(',') : topics }
      }));
    }
  }, [ws, connected]);

  return { connected, messages, subscribe };
}
```

## Distributed Architecture

WebSocket events are distributed across cluster nodes using Olric pub/sub:

- All nodes receive events
- Clients can connect to any node
- Messages propagate cluster-wide
