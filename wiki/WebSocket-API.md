# WebSocket API

Real-time pub/sub messaging via WebSocket.

## Endpoint

```
ws://localhost:6336/live
wss://localhost:6443/live
```

## Authentication

Pass JWT token via query parameter, header, or cookie:

```javascript
const ws = new WebSocket(`ws://localhost:6336/live?token=${TOKEN}`);
```

**Token Extraction:** The server accepts JWT tokens from:
- Query parameter: `?token=JWT_TOKEN`
- Authorization header: `Bearer JWT_TOKEN`
- Cookie: `token=JWT_TOKEN`

## Wire Protocol

### Client → Server (request)

All client messages are JSON with `method`, optional `id` for request correlation, and `attributes`:

```json
{ "id": "req-1", "method": "subscribe", "attributes": { "topicName": "user_account" } }
```

If `id` is included, the server echoes it in the response. If omitted, the response has no `id` field.

### Server → Client (four message types)

All server messages have a `type` field that distinguishes the message category.

**Session** — sent once on connection:
```json
{ "type": "session", "status": "open", "data": { "user": "uuid-string", "groups": ["group-ref-1"], "sessionId": 42 } }
```

**Response** — reply to a client request:
```json
{ "id": "req-1", "type": "response", "method": "subscribe", "ok": true, "data": { "topic": "user_account" } }
{ "id": "req-1", "type": "response", "method": "subscribe", "ok": false, "error": "permission denied" }
```

**Event** — push from a subscription:
```json
{ "type": "event", "topic": "user_account", "event": "create", "data": { "id": 1, "name": "John", "__type": "user_account" }, "source": "database" }
```

**Pong** — reply to client ping:
```json
{ "type": "pong" }
```

Key difference from earlier protocol versions: `data` is always a proper JSON object, never base64-encoded.

## Connection Lifecycle

1. Client opens WebSocket with authentication
2. Server sends `{"type": "session", "status": "open", ...}` with user info
3. Client sends requests, server sends responses and push events
4. Client can send `{"method": "ping"}` at any time to check liveness

## Methods

### subscribe

Subscribe to one or more topics for real-time updates.

**Single topic:**
```json
{ "id": "req-1", "method": "subscribe", "attributes": { "topicName": "user_account" } }
```

**Multiple topics (comma-separated):**
```json
{ "id": "req-2", "method": "subscribe", "attributes": { "topicName": "user_account,document,order" } }
```

**Response (per topic):**
```json
{ "id": "req-1", "type": "response", "method": "subscribe", "ok": true, "data": { "topic": "user_account" } }
```

**Error (nonexistent or permission denied):**
```json
{ "id": "req-1", "type": "response", "method": "subscribe", "ok": false, "error": "permission denied: secret_table" }
```

After subscribing, you receive push events when data changes:
```json
{ "type": "event", "topic": "user_account", "event": "create", "data": { ... }, "source": "database" }
```

### unsubscribe

Stop receiving events from topics.

```json
{ "id": "req-3", "method": "unsubscribe", "attributes": { "topicName": "user_account" } }
```

**Response:**
```json
{ "id": "req-3", "type": "response", "method": "unsubscribe", "ok": true, "data": { "topic": "user_account" } }
```

### new-message

Publish a message to a topic.

```json
{
  "id": "req-4",
  "method": "new-message",
  "attributes": {
    "topicName": "chat-room-1",
    "message": { "text": "Hello everyone!", "from": "alice" }
  }
}
```

Subscribers on that topic receive:
```json
{ "type": "event", "topic": "chat-room-1", "event": "new-message", "data": { "text": "Hello everyone!", "from": "alice" }, "source": "user-uuid" }
```

**Permission:** Requires CanExecute on user topics (GuestExecute bit for non-owners), CanCreate on system topics.

### create-topicName

Create a custom PubSub topic for application-specific messaging.

```json
{ "id": "req-5", "method": "create-topicName", "attributes": { "name": "chat-room-1" } }
```

**Response:**
```json
{ "id": "req-5", "type": "response", "method": "create-topicName", "ok": true, "data": { "topicName": "chat-room-1", "created": true } }
```

The creating user becomes the topic owner. Default permission is owner-only (UserCRUD|UserExecute).

**Errors:**
- Cannot use names that match database table names (reserved for system topics)
- Cannot create a topic that already exists

### destroy-topicName

Delete a custom topic.

```json
{ "id": "req-6", "method": "destroy-topicName", "attributes": { "name": "chat-room-1" } }
```

**Permission:** Requires CanDelete (owner by default). Cannot delete system topics.

### set-topic-permission

Change the permission bitmask on a user-created topic. Only the topic owner or an admin can call this.

```json
{
  "id": "req-7",
  "method": "set-topic-permission",
  "attributes": {
    "topicName": "chat-room-1",
    "permission": 2097151
  }
}
```

**Response:**
```json
{ "id": "req-7", "type": "response", "method": "set-topic-permission", "ok": true, "data": { "topicName": "chat-room-1", "permission": 2097151 } }
```

Cannot modify system topic permissions.

### get-topic-permission

Read the permission bitmask and ownership info of a topic.

```json
{ "id": "req-8", "method": "get-topic-permission", "attributes": { "topicName": "chat-room-1" } }
```

**Response (user topic):**
```json
{ "id": "req-8", "type": "response", "method": "get-topic-permission", "ok": true, "data": { "topicName": "chat-room-1", "owner": "user-uuid", "permission": 2097151, "type": "user" } }
```

**Response (system topic):**
```json
{ "id": "req-8", "type": "response", "method": "get-topic-permission", "ok": true, "data": { "topicName": "user_account", "permission": 262206, "type": "system" } }
```

**Permission:** Requires CanPeek (GuestPeek bit for non-owners).

### ping

Check connection liveness.

```json
{ "method": "ping" }
```

**Response:**
```json
{ "type": "pong" }
```

## Topic Permissions

User-created topics have a permission bitmask that controls access for non-owners. The permission model uses three tiers: Guest (any authenticated user), User (owner), and Group.

### Permission Bits

| Bit | Name | Value | Description |
|-----|------|-------|-------------|
| 0 | GuestPeek | 1 | Non-owner can get-topic-permission |
| 1 | GuestRead | 2 | Non-owner can subscribe |
| 2 | GuestCreate | 4 | Non-owner can publish (system topics) |
| 3 | GuestUpdate | 8 | — |
| 4 | GuestDelete | 16 | Non-owner can destroy |
| 5 | GuestExecute | 32 | Non-owner can publish (user topics) |
| 6 | GuestRefer | 64 | — |
| 7 | UserPeek | 128 | Owner can get-topic-permission |
| 8 | UserRead | 256 | Owner can subscribe |
| 9 | UserCreate | 512 | Owner can publish (system topics) |
| 10 | UserUpdate | 1024 | — |
| 11 | UserDelete | 2048 | Owner can destroy |
| 12 | UserExecute | 4096 | Owner can publish (user topics) |
| 13 | UserRefer | 8192 | — |

### Default Permissions

| Topic Type | Default Permission | Effect |
|------------|-------------------|--------|
| User-created | UserCRUD \| UserExecute (16256) | Owner-only access |
| System | Table permission from database | Follows table-level permissions |

### Common Permission Values

| Value | Name | Effect |
|-------|------|--------|
| 16256 | Owner-only | Only creator can access |
| 16259 | Owner + public read | Anyone can subscribe, owner controls rest |
| 16291 | Owner + public read/write | Anyone can subscribe and publish |
| 2097151 | ALLOW_ALL | Full access for everyone |

### Permission Checks by Method

| Method | System Topic Check | User Topic Check |
|--------|-------------------|-----------------|
| subscribe | CanPeek (table permission) | CanRead (GuestRead for non-owner) |
| new-message | CanCreate (table permission) | CanExecute (GuestExecute for non-owner) |
| destroy-topicName | Always denied | CanDelete (GuestDelete for non-owner) |
| set-topic-permission | Always denied | Owner or admin only |
| get-topic-permission | CanPeek (table permission) | CanPeek (GuestPeek for non-owner) |

## System Topics

Each database table automatically has a topic:

- `user_account` — User account changes
- `document` — Document changes
- `order` — Order changes
- `world` — Schema/table definition changes
- `credential` — Credential changes
- `cloud_store` — Cloud storage changes
- Join tables also have topics

System topics:
- Cannot be created or destroyed via WebSocket
- Permissions come from the table-level permission in the database
- Events are filtered per-row: users only receive events for records they can read

## Event Types

| Event | Description |
|-------|-------------|
| `create` | New record created (system topics) |
| `update` | Record updated (system topics) |
| `delete` | Record deleted (system topics) |
| `new-message` | User-published message (user topics) |

## Complete Working Example

```javascript
const WebSocket = require('ws');
const TOKEN = process.env.DAPTIN_TOKEN;

let reqCounter = 0;
function nextId() { return `req-${++reqCounter}`; }

const ws = new WebSocket(`ws://localhost:6336/live?token=${TOKEN}`);

ws.on('open', () => console.log('WebSocket open, waiting for session...'));

ws.on('message', (raw) => {
  const msg = JSON.parse(raw.toString());

  switch (msg.type) {
    case 'session':
      console.log(`Session opened: user=${msg.data.user} sessionId=${msg.data.sessionId}`);

      // Subscribe to user_account changes
      ws.send(JSON.stringify({
        id: nextId(), method: 'subscribe',
        attributes: { topicName: 'user_account' }
      }));

      // Create a custom topic
      ws.send(JSON.stringify({
        id: nextId(), method: 'create-topicName',
        attributes: { name: 'app-notifications' }
      }));
      break;

    case 'response':
      const status = msg.ok ? 'OK' : `ERROR: ${msg.error}`;
      console.log(`Response [${msg.id}] ${msg.method}: ${status}`);

      // After create, subscribe and publish
      if (msg.method === 'create-topicName' && msg.ok) {
        ws.send(JSON.stringify({
          id: nextId(), method: 'subscribe',
          attributes: { topicName: 'app-notifications' }
        }));
        setTimeout(() => {
          ws.send(JSON.stringify({
            id: nextId(), method: 'new-message',
            attributes: {
              topicName: 'app-notifications',
              message: { text: 'Hello from WebSocket!', ts: new Date().toISOString() }
            }
          }));
        }, 500);
      }
      break;

    case 'event':
      console.log(`Event: ${msg.event} on ${msg.topic}`);
      console.log('  Data:', JSON.stringify(msg.data));
      break;

    case 'pong':
      console.log('Pong received');
      break;
  }
});

ws.on('error', (err) => console.error('Error:', err.message));
ws.on('close', () => console.log('Connection closed'));
```

**Run:**
```bash
DAPTIN_TOKEN=$(cat /tmp/daptin-token.txt) node example.js
```

## JavaScript Client

```javascript
class DaptinWebSocket {
  constructor(baseUrl, token) {
    this.url = `${baseUrl}/live?token=${token}`;
    this.handlers = {};
    this.reqCounter = 0;
    this.pendingRequests = {};
    this.connect();
  }

  nextId() { return `req-${++this.reqCounter}`; }

  connect() {
    this.ws = new WebSocket(this.url);

    this.ws.onmessage = (event) => {
      const msg = JSON.parse(event.data);

      if (msg.type === 'session') {
        console.log('Connected, session:', msg.data);
        return;
      }

      if (msg.type === 'response' && msg.id && this.pendingRequests[msg.id]) {
        this.pendingRequests[msg.id](msg);
        delete this.pendingRequests[msg.id];
        return;
      }

      if (msg.type === 'event' && this.handlers[msg.topic]) {
        this.handlers[msg.topic](msg);
      }
    };

    this.ws.onclose = () => {
      setTimeout(() => this.connect(), 5000);
    };
  }

  send(method, attributes) {
    const id = this.nextId();
    return new Promise((resolve) => {
      this.pendingRequests[id] = resolve;
      this.ws.send(JSON.stringify({ id, method, attributes }));
    });
  }

  async subscribe(topics, handler) {
    const topicList = Array.isArray(topics) ? topics : [topics];
    topicList.forEach(topic => { this.handlers[topic] = handler; });
    return this.send('subscribe', { topicName: topicList.join(',') });
  }

  async publish(topic, message) {
    return this.send('new-message', { topicName: topic, message });
  }

  async createTopic(name) {
    return this.send('create-topicName', { name });
  }

  async setPermission(topicName, permission) {
    return this.send('set-topic-permission', { topicName, permission });
  }

  ping() {
    this.ws.send(JSON.stringify({ method: 'ping' }));
  }
}

// Usage
const ws = new DaptinWebSocket('ws://localhost:6336', TOKEN);

ws.subscribe('order', (event) => {
  console.log('Order event:', event.event, event.data);
});

ws.subscribe(['user_account', 'document'], (event) => {
  console.log('Event:', event.topic, event.event);
});

// Create a public topic
const resp = await ws.createTopic('chat-room-1');
if (resp.ok) {
  await ws.setPermission('chat-room-1', 2097151); // ALLOW_ALL
  await ws.publish('chat-room-1', { text: 'Hello!' });
}
```

## React Hook Example

```javascript
import { useEffect, useState, useCallback, useRef } from 'react';

function useDaptinWebSocket(token) {
  const [connected, setConnected] = useState(false);
  const [sessionInfo, setSessionInfo] = useState(null);
  const [messages, setMessages] = useState([]);
  const wsRef = useRef(null);
  const reqCounter = useRef(0);

  useEffect(() => {
    const socket = new WebSocket(`ws://localhost:6336/live?token=${token}`);

    socket.onmessage = (e) => {
      const msg = JSON.parse(e.data);

      if (msg.type === 'session') {
        setSessionInfo(msg.data);
        setConnected(true);
        return;
      }

      if (msg.type === 'event') {
        setMessages(prev => [...prev, msg]);
      }
    };

    socket.onclose = () => setConnected(false);
    wsRef.current = socket;
    return () => socket.close();
  }, [token]);

  const subscribe = useCallback((topics) => {
    if (wsRef.current && connected) {
      const id = `req-${++reqCounter.current}`;
      wsRef.current.send(JSON.stringify({
        id,
        method: 'subscribe',
        attributes: { topicName: Array.isArray(topics) ? topics.join(',') : topics }
      }));
    }
  }, [connected]);

  return { connected, sessionInfo, messages, subscribe };
}
```

## Distributed Architecture

WebSocket events are distributed across cluster nodes using Olric pub/sub:

- All nodes receive events
- Clients can connect to any node
- Messages propagate cluster-wide

## Method Reference

| Method | Purpose | Attributes |
|--------|---------|------------|
| `subscribe` | Subscribe to topics | `{topicName: "topic1,topic2"}` |
| `unsubscribe` | Unsubscribe from topics | `{topicName: "topic1"}` |
| `create-topicName` | Create custom topic | `{name: "my-topic"}` |
| `destroy-topicName` | Delete custom topic | `{name: "my-topic"}` |
| `new-message` | Publish to topic | `{topicName: "my-topic", message: {...}}` |
| `set-topic-permission` | Set topic permissions | `{topicName: "my-topic", permission: 2097151}` |
| `get-topic-permission` | Get topic permissions | `{topicName: "my-topic"}` |
| `ping` | Connection health check | `{}` |
