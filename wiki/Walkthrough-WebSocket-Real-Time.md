# Walkthrough: Building Real-Time Features with WebSocket

A step-by-step guide to real-time pub/sub messaging with Daptin's WebSocket API.

By the end of this walkthrough, you'll have:
- Live notifications when database records change
- Custom pub/sub topics for application messaging
- Permission-controlled topic access
- A working real-time notification system

**Time Required**: 15-20 minutes
**Difficulty**: Beginner

---

## What You'll Learn

1. **Connection & Session**: How to connect, authenticate, and receive the session handshake
2. **Topic System**: System topics (database tables) vs. custom topics
3. **Subscriptions**: Subscribing to database changes with request correlation
4. **Publishing**: Sending messages to custom topics
5. **Permissions**: Controlling who can subscribe, publish, and manage topics
6. **Client Integration**: Building a notification system

---

## The Scenario

**Application**: Task Management System
**Feature**: Real-time task updates and team notifications

**What We're Building**:
1. Connect to Daptin's WebSocket and verify the session handshake
2. Subscribe to the `ticket` topic for real-time task updates
3. Create a custom `notifications` topic with controlled permissions
4. Build a notification viewer
5. Test permission filtering

---

## Architecture Overview

```
┌──────────────────────────────────────────────────────────┐
│                   Client Applications                     │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐         │
│  │ Dashboard  │  │   Mobile   │  │   Admin    │         │
│  └──────┬─────┘  └──────┬─────┘  └──────┬─────┘         │
│         └────────────────┼────────────────┘               │
│                          │ WebSocket                      │
│                          │ ws://localhost:6336/live        │
│                          ▼                                │
├──────────────────────────────────────────────────────────┤
│                   Daptin Server                           │
│                                                           │
│  Session-open → { type:"session", data:{user,groups} }   │
│  Responses    → { type:"response", id, method, ok }      │
│  Events       → { type:"event", topic, event, data }     │
│  Pong         → { type:"pong" }                          │
│                                                           │
│  System Topics (auto): ticket, user_account, world, ...  │
│  User Topics (custom): notifications, chat-room-1, ...   │
└──────────────────────────────────────────────────────────┘
```

---

## Before You Begin

```bash
# 1. Daptin running
curl -s http://localhost:6336/api/world | head -c 50
# Expected: {"data":[...

# 2. Valid authentication token
cat /tmp/daptin-token.txt

# 3. Node.js and ws package
npm list ws || npm install ws
```

---

## Step 1: Connect and Receive Session

The first message after connecting is always a session-open message with your user info.

```javascript
// save as test-ws-connect.js
const WebSocket = require('ws');
const TOKEN = process.argv[2];

const ws = new WebSocket(`ws://localhost:6336/live?token=${TOKEN}`);

ws.on('message', (raw) => {
  const msg = JSON.parse(raw.toString());

  if (msg.type === 'session' && msg.status === 'open') {
    console.log('Session opened!');
    console.log('  User:', msg.data.user);
    console.log('  Groups:', msg.data.groups);
    console.log('  Session ID:', msg.data.sessionId);
    process.exit(0);
  }
});

ws.on('error', (err) => {
  console.error('Connection failed:', err.message);
  process.exit(1);
});

setTimeout(() => { console.error('Timeout'); process.exit(1); }, 5000);
```

```bash
node test-ws-connect.js "$(cat /tmp/daptin-token.txt)"
```

**Expected output:**
```
Session opened!
  User: a1b2c3d4-e5f6-...
  Groups: ["group-ref-1"]
  Session ID: 1
```

---

## Step 2: Subscribe to Database Changes

Subscribe to the `ticket` topic. Every request includes an `id` for correlation.

```javascript
// save as test-ticket-sub.js
const WebSocket = require('ws');
const TOKEN = process.argv[2];

let reqCounter = 0;
function nextId() { return `req-${++reqCounter}`; }

const ws = new WebSocket(`ws://localhost:6336/live?token=${TOKEN}`);

ws.on('message', (raw) => {
  const msg = JSON.parse(raw.toString());

  // Wait for session, then subscribe
  if (msg.type === 'session') {
    console.log('Connected. Subscribing to ticket topic...\n');
    ws.send(JSON.stringify({
      id: nextId(),
      method: 'subscribe',
      attributes: { topicName: 'ticket' }
    }));
    return;
  }

  // Subscription confirmed
  if (msg.type === 'response' && msg.method === 'subscribe') {
    if (msg.ok) {
      console.log(`Subscribed to ${JSON.parse(msg.data ? JSON.stringify(msg.data) : '{}').topic || 'ticket'}`);
      console.log('Watching for events... (create/update/delete a ticket in another terminal)\n');
    } else {
      console.error('Subscribe failed:', msg.error);
    }
    return;
  }

  // Real-time event
  if (msg.type === 'event') {
    console.log('─────────────────────────────────────');
    console.log(`Event: ${msg.event} on ${msg.topic}`);
    console.log('Data:', JSON.stringify(msg.data, null, 2));
    console.log('Source:', msg.source);
    console.log('─────────────────────────────────────\n');
  }
});

ws.on('error', (err) => console.error('Error:', err.message));
console.log('Press Ctrl+C to exit\n');
```

**Terminal 1** — run the listener:
```bash
node test-ticket-sub.js "$(cat /tmp/daptin-token.txt)"
```

**Terminal 2** — create a ticket:
```bash
TOKEN=$(cat /tmp/daptin-token.txt)
curl -X POST http://localhost:6336/api/ticket \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{"data":{"type":"ticket","attributes":{"title":"Real-time test","status":"open","priority":"high"}}}'
```

**What you see in Terminal 1:**
```
─────────────────────────────────────
Event: create on ticket
Data: {
  "title": "Real-time test",
  "status": "open",
  "priority": "high",
  "__type": "ticket",
  ...
}
Source: database
─────────────────────────────────────
```

---

## Step 3: Custom Topics with Permissions

Create a topic, set permissions to allow other users, then publish.

```javascript
// save as test-custom-topic.js
const WebSocket = require('ws');
const TOKEN = process.argv[2];

let reqCounter = 0;
function nextId() { return `req-${++reqCounter}`; }

const ws = new WebSocket(`ws://localhost:6336/live?token=${TOKEN}`);

ws.on('message', (raw) => {
  const msg = JSON.parse(raw.toString());

  if (msg.type === 'session') {
    console.log('Session opened.\n');

    // Step 1: Create topic
    console.log('Creating "notifications" topic...');
    ws.send(JSON.stringify({
      id: nextId(), method: 'create-topicName',
      attributes: { name: 'notifications' }
    }));
    return;
  }

  if (msg.type === 'response') {
    const status = msg.ok ? 'OK' : `FAILED: ${msg.error}`;
    console.log(`[${msg.id}] ${msg.method}: ${status}`);

    // Step 2: After create, make it public and subscribe
    if (msg.method === 'create-topicName' && msg.ok) {
      console.log('\nSetting public permissions (ALLOW_ALL)...');
      ws.send(JSON.stringify({
        id: nextId(), method: 'set-topic-permission',
        attributes: { topicName: 'notifications', permission: 2097151 }
      }));
    }

    if (msg.method === 'set-topic-permission' && msg.ok) {
      console.log('\nSubscribing...');
      ws.send(JSON.stringify({
        id: nextId(), method: 'subscribe',
        attributes: { topicName: 'notifications' }
      }));
    }

    // Step 3: After subscribe, publish a test message
    if (msg.method === 'subscribe' && msg.ok) {
      console.log('\nPublishing test message...');
      ws.send(JSON.stringify({
        id: nextId(), method: 'new-message',
        attributes: {
          topicName: 'notifications',
          message: { type: 'alert', title: 'Welcome!', body: 'Notifications are working!' }
        }
      }));
    }
    return;
  }

  if (msg.type === 'event') {
    console.log('\nReceived event:');
    console.log('  Topic:', msg.topic);
    console.log('  Event:', msg.event);
    console.log('  Data:', JSON.stringify(msg.data, null, 2));
    setTimeout(() => process.exit(0), 500);
  }
});

ws.on('error', (err) => console.error('Error:', err.message));
```

```bash
node test-custom-topic.js "$(cat /tmp/daptin-token.txt)"
```

**Expected output:**
```
Session opened.

Creating "notifications" topic...
[req-1] create-topicName: OK

Setting public permissions (ALLOW_ALL)...
[req-2] set-topic-permission: OK

Subscribing...
[req-3] subscribe: OK

Publishing test message...

Received event:
  Topic: notifications
  Event: new-message
  Data: {
    "type": "alert",
    "title": "Welcome!",
    "body": "Notifications are working!"
  }
```

---

## Step 4: Permission Scenarios

### Default: Owner-Only Access

When you create a topic, only you can access it. Other users get "permission denied":

```javascript
// User A creates topic
ws.send(JSON.stringify({
  id: nextId(), method: 'create-topicName',
  attributes: { name: 'private-room' }
}));

// User B tries to subscribe → denied
// Response: { "type": "response", "method": "subscribe", "ok": false, "error": "permission denied: private-room" }
```

### Grant Read Access (Allow Others to Subscribe)

Add `GuestRead` (bit 1, value 2) and `GuestPeek` (bit 0, value 1):

```javascript
// Owner sets permission: UserCRUD|UserExecute|GuestPeek|GuestRead = 16256 + 1 + 2 = 16259
ws.send(JSON.stringify({
  id: nextId(), method: 'set-topic-permission',
  attributes: { topicName: 'private-room', permission: 16259 }
}));
// Now any authenticated user can subscribe
```

### Grant Publish Access

Add `GuestExecute` (bit 5, value 32):

```javascript
// Owner sets: 16259 + 32 = 16291
ws.send(JSON.stringify({
  id: nextId(), method: 'set-topic-permission',
  attributes: { topicName: 'private-room', permission: 16291 }
}));
// Now any user can subscribe AND publish
```

### Make Fully Public

Use `ALLOW_ALL` (2097151):

```javascript
ws.send(JSON.stringify({
  id: nextId(), method: 'set-topic-permission',
  attributes: { topicName: 'private-room', permission: 2097151 }
}));
```

### Permission Quick Reference

| Goal | Permission Value |
|------|-----------------|
| Owner-only (default) | 16256 |
| + public subscribe | 16259 |
| + public subscribe & publish | 16291 |
| Full public access | 2097151 |

---

## Step 5: Build a Notification Viewer

```javascript
// save as notification-viewer.js
const WebSocket = require('ws');
const TOKEN = process.argv[2];

if (!TOKEN) {
  console.error('Usage: node notification-viewer.js <JWT_TOKEN>');
  process.exit(1);
}

let reqCounter = 0;
function nextId() { return `req-${++reqCounter}`; }

class NotificationViewer {
  constructor(token) {
    this.token = token;
    this.eventCount = 0;
  }

  connect() {
    console.log('Connecting...\n');
    this.ws = new WebSocket(`ws://localhost:6336/live?token=${this.token}`);
    this.ws.on('message', (raw) => this.onMessage(JSON.parse(raw.toString())));
    this.ws.on('error', (err) => console.error('Error:', err.message));
    this.ws.on('close', () => console.log('Connection closed'));
  }

  onMessage(msg) {
    switch (msg.type) {
      case 'session':
        console.log(`Session: user=${msg.data.user}\n`);
        // Subscribe to key topics
        this.ws.send(JSON.stringify({
          id: nextId(), method: 'subscribe',
          attributes: { topicName: 'ticket,user_account' }
        }));
        break;

      case 'response':
        if (msg.ok) {
          console.log(`[${msg.method}] OK`);
          if (msg.method === 'subscribe') {
            console.log('Watching for events...\n');
          }
        } else {
          console.log(`[${msg.method}] Error: ${msg.error}`);
        }
        break;

      case 'event':
        this.eventCount++;
        const icon = { create: '+', update: '~', delete: '-', 'new-message': '>' }[msg.event] || '?';
        console.log(`[${icon}] ${msg.event.toUpperCase()} on ${msg.topic} (#${this.eventCount})`);
        if (msg.data) {
          const preview = JSON.stringify(msg.data).substring(0, 120);
          console.log(`    ${preview}${preview.length >= 120 ? '...' : ''}`);
        }
        console.log();
        break;

      case 'pong':
        console.log('Pong');
        break;
    }
  }
}

const viewer = new NotificationViewer(TOKEN);
viewer.connect();
console.log('Press Ctrl+C to exit\n');
```

```bash
node notification-viewer.js "$(cat /tmp/daptin-token.txt)"
```

---

## Step 6: Ping/Pong for Connection Health

Send a ping at any time to verify the connection is alive:

```javascript
// Send ping
ws.send(JSON.stringify({ method: 'ping' }));

// Receive: { "type": "pong" }
```

Use this in a periodic health check:

```javascript
setInterval(() => {
  ws.send(JSON.stringify({ method: 'ping' }));
}, 30000);
```

---

## Step 7: Reconnection with Re-subscribe

```javascript
class RobustWebSocket {
  constructor(url, token) {
    this.url = `${url}?token=${token}`;
    this.subscriptions = [];
    this.reconnectDelay = 5000;
    this.reqCounter = 0;
    this.connect();
  }

  nextId() { return `req-${++this.reqCounter}`; }

  connect() {
    this.ws = new WebSocket(this.url);

    this.ws.on('message', (raw) => {
      const msg = JSON.parse(raw.toString());

      if (msg.type === 'session') {
        console.log('Connected. Re-subscribing...');
        this.reconnectDelay = 5000;
        // Re-subscribe to all topics
        for (const topic of this.subscriptions) {
          this.ws.send(JSON.stringify({
            id: this.nextId(), method: 'subscribe',
            attributes: { topicName: topic }
          }));
        }
      }

      if (msg.type === 'event') {
        this.onEvent(msg);
      }
    });

    this.ws.on('close', () => {
      console.log(`Reconnecting in ${this.reconnectDelay / 1000}s...`);
      setTimeout(() => this.connect(), this.reconnectDelay);
      this.reconnectDelay = Math.min(this.reconnectDelay * 2, 60000);
    });
  }

  subscribe(topic) {
    if (!this.subscriptions.includes(topic)) {
      this.subscriptions.push(topic);
    }
    this.ws.send(JSON.stringify({
      id: this.nextId(), method: 'subscribe',
      attributes: { topicName: topic }
    }));
  }

  onEvent(msg) {
    console.log(`Event: ${msg.event} on ${msg.topic}`, msg.data);
  }
}
```

---

## Troubleshooting

### Connection Fails with 403

**Cause**: Invalid or expired JWT token.

```bash
# Get fresh token
./scripts/testing/test-runner.sh token
```

### Not Receiving Events

1. Check the response to your subscribe — is `ok: true`?
2. Verify user has read permission on the table/topic
3. Check topic name is exact (case-sensitive)

### Permission Denied on Subscribe

For user topics, the owner must grant `GuestRead` (bit 1):
```javascript
ws.send(JSON.stringify({
  id: nextId(), method: 'set-topic-permission',
  attributes: { topicName: 'my-topic', permission: 16259 } // adds GuestPeek + GuestRead
}));
```

---

## Quick Reference

### Message Types (Server → Client)

| Type | When | Key Fields |
|------|------|------------|
| `session` | On connect | `status`, `data.user`, `data.sessionId` |
| `response` | Reply to request | `id`, `method`, `ok`, `error`, `data` |
| `event` | Push from subscription | `topic`, `event`, `data`, `source` |
| `pong` | Reply to ping | — |

### Methods (Client → Server)

| Method | Purpose | Attributes |
|--------|---------|------------|
| `subscribe` | Subscribe to topics | `{topicName: "t1,t2"}` |
| `unsubscribe` | Unsubscribe | `{topicName: "t1"}` |
| `create-topicName` | Create custom topic | `{name: "my-topic"}` |
| `destroy-topicName` | Delete custom topic | `{name: "my-topic"}` |
| `new-message` | Publish message | `{topicName: "t", message: {...}}` |
| `set-topic-permission` | Set permissions | `{topicName: "t", permission: N}` |
| `get-topic-permission` | Get permissions | `{topicName: "t"}` |
| `ping` | Health check | `{}` |

### Permission Values

| Value | Meaning |
|-------|---------|
| 16256 | Owner-only (default for new topics) |
| 16259 | + public subscribe |
| 16291 | + public subscribe & publish |
| 2097151 | Full public access |
