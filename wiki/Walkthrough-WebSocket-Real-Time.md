# Walkthrough: Building Real-Time Features with WebSocket

**A complete, step-by-step guide** to implement real-time pub/sub messaging in your application with Daptin's WebSocket API.

By the end of this walkthrough, you'll have:
- ✅ Live notifications when database records change
- ✅ Custom pub/sub topics for application-specific messaging
- ✅ Real-time chat or notification system
- ✅ Permission-aware event filtering
- ✅ Understanding of WebSocket connection and message format

**Time Required**: 15-20 minutes
**Difficulty**: Beginner (basic JavaScript knowledge helpful)

---

## What You'll Learn

This walkthrough teaches you:

1. **WebSocket Basics**: How to connect and authenticate
2. **Topic System**: System topics vs. custom topics
3. **Event Subscriptions**: Subscribing to database table changes
4. **Publishing Messages**: Sending custom messages to topics
5. **Permission Filtering**: How events are filtered by user permissions
6. **Client Integration**: Building a simple real-time notification system

---

## The Scenario

**Application**: Task Management System
**Feature**: Real-time task updates

**What We're Building**:
1. Connect to Daptin's WebSocket endpoint
2. Subscribe to the `ticket` topic to get real-time task updates
3. Create a custom `notifications` topic for app-specific alerts
4. Build a simple notification viewer
5. Test permission filtering (users only see tasks they can access)

**Use Cases**:
- Dashboard that updates when tasks are created/updated
- Real-time notifications for team members
- Live chat or messaging features
- Collaborative task management

---

## Architecture Overview

```
┌──────────────────────────────────────────────────────────┐
│                   Client Applications                     │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐         │
│  │ Dashboard  │  │   Mobile   │  │   Admin    │         │
│  │  Browser   │  │    App     │  │   Panel    │         │
│  └──────┬─────┘  └──────┬─────┘  └──────┬─────┘         │
│         │                │                │               │
│         └────────────────┼────────────────┘               │
│                          │                                │
└──────────────────────────┼────────────────────────────────┘
                           │ WebSocket
                           │ ws://localhost:6336/live?token=JWT
                           ▼
┌──────────────────────────────────────────────────────────┐
│                   Daptin Server                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │           WebSocket Handler (/live)                │  │
│  │  • Authenticates user from token                   │  │
│  │  • Manages subscriptions                           │  │
│  │  • Filters events by permissions                   │  │
│  └───────────────────┬────────────────────────────────┘  │
│                      │                                    │
│                      ▼                                    │
│  ┌──────────────────────────────────────────────────┐    │
│  │           Olric PubSub (Port 5336)               │    │
│  │  • Distributed topic-based messaging             │    │
│  │  • Cluster-wide event propagation                │    │
│  └───────────────────┬──────────────────────────────┘    │
│                      │                                    │
│                      ▼                                    │
│  ┌──────────────────────────────────────────────────┐    │
│  │              System Topics                        │    │
│  │  • ticket (task changes)                          │    │
│  │  • user_account (user changes)                    │    │
│  │  • document (document changes)                    │    │
│  │  • ... (one per database table)                   │    │
│  └──────────────────────────────────────────────────┘    │
│  ┌──────────────────────────────────────────────────┐    │
│  │             Custom Topics                         │    │
│  │  • notifications (app alerts)                     │    │
│  │  • chat-room-1 (team chat)                        │    │
│  │  • ... (user-created topics)                      │    │
│  └──────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────┘
```

---

## Before You Begin

### Prerequisites Check

Make sure you have:

```bash
# 1. Daptin running
curl -s http://localhost:6336/api/world | head -c 50
# Expected: {"data":[...

# 2. Valid authentication token
cat /tmp/daptin-token.txt
# Expected: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...

# 3. Node.js and ws package (for JavaScript examples)
node --version
# Expected: v18.0.0 or higher

npm list ws
# Expected: ws@X.X.X (or install with: npm install ws)

# 4. A test table with some data (we'll use ticket table)
curl -s -H "Authorization: Bearer $(cat /tmp/daptin-token.txt)" \
  http://localhost:6336/api/ticket | jq '.data | length'
# Expected: 0 or more
```

If you don't have a ticket table, create one:
```bash
cat > schema_ticket.yaml << 'EOF'
Tables:
  - TableName: ticket
    Columns:
      - Name: title
        DataType: varchar(500)
        ColumnType: name
      - Name: status
        DataType: varchar(50)
        ColumnType: label
        DefaultValue: "open"
      - Name: priority
        DataType: varchar(20)
        ColumnType: label
        DefaultValue: "medium"
EOF

# Restart Daptin to load schema
./scripts/testing/test-runner.sh stop && ./scripts/testing/test-runner.sh start
```

---

## Step 0: Understanding WebSocket Basics

**What is WebSocket?**
WebSocket is a protocol for real-time, bidirectional communication between client and server. Unlike HTTP where the client requests and server responds, WebSocket keeps a persistent connection where both sides can send messages anytime.

**Why use WebSocket for real-time features?**
- **Instant updates**: No polling, events arrive immediately
- **Efficient**: One connection, not hundreds of HTTP requests
- **Bidirectional**: Both client and server can initiate messages
- **Low latency**: Perfect for live dashboards, chat, notifications

**Daptin's WebSocket endpoint**: `ws://localhost:6336/live`
**Authentication**: Pass JWT token as query parameter: `?token=YOUR_JWT_TOKEN`

---

## Step 1: Test Connection (Quick Verification)

Let's verify WebSocket is working with a simple test script:

```bash
# Create test script
cat > test-ws-connection.js << 'EOF'
const WebSocket = require('ws');
const TOKEN = process.argv[2];

console.log('Connecting to WebSocket...');
const ws = new WebSocket(`ws://localhost:6336/live?token=${TOKEN}`);

ws.on('open', function open() {
  console.log('✓ Connected successfully!');

  // List available topics
  ws.send(JSON.stringify({
    method: 'list-topicName',
    attributes: {}
  }));
});

ws.on('message', function incoming(data) {
  const msg = JSON.parse(data.toString());

  if (msg.ObjectType === 'topicName-list') {
    const topics = JSON.parse(
      Buffer.from(msg.EventData, 'base64').toString()
    );
    console.log(`\n✓ Received ${topics.topics.length} available topics:`);
    console.log('  Sample:', topics.topics.slice(0, 5).join(', '), '...');
    process.exit(0);
  }
});

ws.on('error', (err) => {
  console.error('✗ Connection failed:', err.message);
  process.exit(1);
});
EOF

# Run test
node test-ws-connection.js "$(cat /tmp/daptin-token.txt)"
```

**Expected output:**
```
Connecting to WebSocket...
✓ Connected successfully!

✓ Received 69 available topics:
  Sample: mail_box, ticket_state, certificate, user_otp_account, timeline ...
```

**What just happened?**
- Connected to WebSocket with JWT authentication
- Listed all available topics (one per database table)
- Confirmed WebSocket endpoint is working

---

## Step 2: Subscribe to Database Table Changes

Now let's subscribe to the `ticket` topic to receive real-time updates when tickets are created, updated, or deleted.

### 2.1 Create Subscription Test Script

```bash
cat > test-ticket-subscription.js << 'EOF'
const WebSocket = require('ws');
const TOKEN = process.argv[2];

const ws = new WebSocket(`ws://localhost:6336/live?token=${TOKEN}`);

ws.on('open', function open() {
  console.log('Connected to WebSocket\n');

  // Subscribe to ticket topic
  console.log('Subscribing to ticket topic...');
  ws.send(JSON.stringify({
    method: 'subscribe',
    attributes: {
      topicName: 'ticket'
    }
  }));
});

ws.on('message', function incoming(data) {
  const msg = JSON.parse(data.toString());

  if (msg.ObjectType === 'subscription-response') {
    console.log('✓ Subscription confirmed!\n');
    console.log('Now create/update/delete a ticket in another terminal.');
    console.log('You will see real-time events here.\n');
    console.log('Watching for events...\n');
  } else {
    console.log('─────────────────────────────────────');
    console.log('Event Type:', msg.EventType);
    console.log('Object Type:', msg.ObjectType);

    if (msg.EventData) {
      const eventData = JSON.parse(
        Buffer.from(msg.EventData, 'base64').toString()
      );
      console.log('Event Data:', JSON.stringify(eventData, null, 2));
    }
    console.log('─────────────────────────────────────\n');
  }
});

ws.on('error', (err) => {
  console.error('WebSocket error:', err.message);
});

console.log('Press Ctrl+C to exit\n');
EOF

# Make executable
chmod +x test-ticket-subscription.js
```

### 2.2 Test Real-Time Updates

**Terminal 1** - Run the subscription listener:
```bash
node test-ticket-subscription.js "$(cat /tmp/daptin-token.txt)"
```

**Terminal 2** - Create a ticket:
```bash
TOKEN=$(cat /tmp/daptin-token.txt)
curl -X POST http://localhost:6336/api/ticket \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "ticket",
      "attributes": {
        "title": "Test real-time notification",
        "status": "open",
        "priority": "high"
      }
    }
  }'
```

**What you should see in Terminal 1:**
```
─────────────────────────────────────
Event Type: create
Object Type: ticket
Event Data: {
  "data": {
    "type": "ticket",
    "id": "...",
    "attributes": {
      "title": "Test real-time notification",
      "status": "open",
      "priority": "high"
    }
  }
}
─────────────────────────────────────
```

**Success!** You're now receiving real-time notifications when tickets are created.

---

## Step 3: Subscribe to Multiple Topics

You can subscribe to multiple topics at once, either with separate subscriptions or a comma-separated list.

```bash
cat > test-multiple-topics.js << 'EOF'
const WebSocket = require('ws');
const TOKEN = process.argv[2];

const ws = new WebSocket(`ws://localhost:6336/live?token=${TOKEN}`);

ws.on('open', function open() {
  console.log('Connected to WebSocket\n');

  // Subscribe to multiple topics at once (comma-separated)
  console.log('Subscribing to ticket, user_account, and document topics...\n');
  ws.send(JSON.stringify({
    method: 'subscribe',
    attributes: {
      topicName: 'ticket,user_account,document'
    }
  }));
});

ws.on('message', function incoming(data) {
  const msg = JSON.parse(data.toString());

  if (msg.ObjectType === 'subscription-response') {
    console.log('✓ Subscriptions confirmed!\n');
    console.log('Watching for events on all subscribed topics...\n');
  } else {
    console.log(`[${msg.ObjectType}] ${msg.EventType}`);
  }
});

ws.on('error', (err) => {
  console.error('WebSocket error:', err.message);
});
EOF

node test-multiple-topics.js "$(cat /tmp/daptin-token.txt)"
```

---

## Step 4: Create Custom Topics for Application Messaging

System topics (like `ticket`, `user_account`) are automatic. For custom application messaging (chat, notifications, etc.), create custom topics.

### 4.1 Create a Notifications Topic

```bash
cat > test-custom-topic.js << 'EOF'
const WebSocket = require('ws');
const TOKEN = process.argv[2];

const ws = new WebSocket(`ws://localhost:6336/live?token=${TOKEN}`);

ws.on('open', function open() {
  console.log('Connected to WebSocket\n');

  // Step 1: Create custom topic
  console.log('Creating custom "notifications" topic...');
  ws.send(JSON.stringify({
    method: 'create-topicName',
    attributes: {
      name: 'notifications'
    }
  }));

  // Step 2: Subscribe to it
  setTimeout(() => {
    console.log('Subscribing to notifications topic...');
    ws.send(JSON.stringify({
      method: 'subscribe',
      attributes: {
        topicName: 'notifications'
      }
    }));
  }, 1000);

  // Step 3: Publish a test message
  setTimeout(() => {
    console.log('Publishing test notification...\n');
    ws.send(JSON.stringify({
      method: 'new-message',
      attributes: {
        topicName: 'notifications',
        message: JSON.stringify({
          type: 'alert',
          title: 'Welcome!',
          body: 'Your real-time notification system is working!',
          timestamp: new Date().toISOString()
        })
      }
    }));
  }, 2000);
});

ws.on('message', function incoming(data) {
  const msg = JSON.parse(data.toString());

  if (msg.ObjectType === 'subscription-response') {
    console.log('✓ Subscription confirmed');
  } else if (msg.ObjectType === 'notifications') {
    console.log('─────────────────────────────────────');
    console.log('📬 Notification Received:');
    const notification = JSON.parse(
      Buffer.from(msg.EventData, 'base64').toString()
    );
    console.log(JSON.stringify(notification, null, 2));
    console.log('─────────────────────────────────────');
    setTimeout(() => process.exit(0), 1000);
  }
});

ws.on('error', (err) => {
  console.error('WebSocket error:', err.message);
});
EOF

node test-custom-topic.js "$(cat /tmp/daptin-token.txt)"
```

**Expected output:**
```
Connected to WebSocket

Creating custom "notifications" topic...
Subscribing to notifications topic...
✓ Subscription confirmed
Publishing test notification...

─────────────────────────────────────
📬 Notification Received:
{
  "type": "alert",
  "title": "Welcome!",
  "body": "Your real-time notification system is working!",
  "timestamp": "2026-01-26T12:34:56.789Z"
}
─────────────────────────────────────
```

---

## Step 5: Build a Complete Notification System

Now let's build a practical notification viewer that displays all events in a formatted way.

```bash
cat > notification-viewer.js << 'EOF'
const WebSocket = require('ws');
const TOKEN = process.argv[2];

if (!TOKEN) {
  console.error('Usage: node notification-viewer.js <JWT_TOKEN>');
  process.exit(1);
}

class NotificationViewer {
  constructor(token) {
    this.token = token;
    this.ws = null;
    this.eventCount = 0;
  }

  connect() {
    console.log('🔌 Connecting to Daptin WebSocket...\n');
    this.ws = new WebSocket(
      `ws://localhost:6336/live?token=${this.token}`
    );

    this.ws.on('open', () => this.onOpen());
    this.ws.on('message', (data) => this.onMessage(data));
    this.ws.on('error', (err) => this.onError(err));
    this.ws.on('close', () => this.onClose());
  }

  onOpen() {
    console.log('✅ Connected successfully!\n');
    console.log('🔍 Listing available topics...');

    this.send('list-topicName', {});
  }

  onMessage(data) {
    const msg = JSON.parse(data.toString());

    // Handle topic list
    if (msg.ObjectType === 'topicName-list') {
      const topics = JSON.parse(
        Buffer.from(msg.EventData, 'base64').toString()
      );
      console.log(`📋 ${topics.topics.length} topics available\n`);

      // Subscribe to key topics
      console.log('📡 Subscribing to: ticket, user_account, notifications\n');
      this.send('subscribe', {
        topicName: 'ticket,user_account,notifications'
      });
    }

    // Handle subscription confirmation
    else if (msg.ObjectType === 'subscription-response') {
      console.log('✓ Subscriptions active');
      console.log('👀 Watching for events...\n');
      console.log('═══════════════════════════════════════════\n');
    }

    // Handle events
    else {
      this.eventCount++;
      this.displayEvent(msg);
    }
  }

  displayEvent(msg) {
    const timestamp = new Date().toLocaleTimeString();
    const eventType = this.getEventIcon(msg.EventType);

    console.log(`[${timestamp}] ${eventType} ${msg.EventType.toUpperCase()}`);
    console.log(`📦 Type: ${msg.ObjectType}`);

    if (msg.EventData) {
      try {
        const eventData = JSON.parse(
          Buffer.from(msg.EventData, 'base64').toString()
        );

        // Display relevant fields based on object type
        if (msg.ObjectType === 'ticket') {
          const attrs = eventData.data?.attributes || eventData;
          console.log(`   Title: ${attrs.title || 'N/A'}`);
          console.log(`   Status: ${attrs.status || 'N/A'}`);
          console.log(`   Priority: ${attrs.priority || 'N/A'}`);
        } else if (msg.ObjectType === 'user_account') {
          const attrs = eventData.data?.attributes || eventData;
          console.log(`   Name: ${attrs.name || 'N/A'}`);
          console.log(`   Email: ${attrs.email || 'N/A'}`);
        } else {
          console.log(`   Data: ${JSON.stringify(eventData).substring(0, 100)}...`);
        }
      } catch (e) {
        console.log(`   (Could not parse event data)`);
      }
    }

    console.log(`   Event #${this.eventCount}`);
    console.log('───────────────────────────────────────────\n');
  }

  getEventIcon(eventType) {
    const icons = {
      'create': '✨',
      'update': '📝',
      'delete': '🗑️',
      'new-message': '💬'
    };
    return icons[eventType] || '📊';
  }

  send(method, attributes) {
    this.ws.send(JSON.stringify({ method, attributes }));
  }

  onError(err) {
    console.error('❌ WebSocket error:', err.message);
    process.exit(1);
  }

  onClose() {
    console.log('\n🔌 Connection closed');
    process.exit(0);
  }
}

// Start the viewer
const viewer = new NotificationViewer(TOKEN);
viewer.connect();

console.log('Press Ctrl+C to exit\n');
EOF

node notification-viewer.js "$(cat /tmp/daptin-token.txt)"
```

**Test it**: In another terminal, create/update tickets and watch them appear in real-time!

---

## Step 6: Understanding Permission Filtering

**Important**: WebSocket events are automatically filtered by user permissions. Users only receive events for records they can read.

### 6.1 Test with Different Users

Let's verify permission filtering works:

**As Admin** (sees all tickets):
```bash
# Terminal 1: Admin viewer
node notification-viewer.js "$(cat /tmp/daptin-token.txt)"

# Terminal 2: Create a ticket as admin
TOKEN=$(cat /tmp/daptin-token.txt)
curl -X POST http://localhost:6336/api/ticket \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{"data":{"type":"ticket","attributes":{"title":"Admin ticket"}}}' | jq
```
**Result**: Event appears in Terminal 1 ✓

**As Regular User** (sees only their tickets):
```bash
# Sign in as regular user (e.g., mary@techgear.com from product walkthrough)
MARY_TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"mary@techgear.com","password":"password123"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')

# Terminal 1: Mary's viewer
node notification-viewer.js "$MARY_TOKEN"

# Terminal 2: Create a ticket as admin (Mary won't see it unless it's shared)
TOKEN=$(cat /tmp/daptin-token.txt)
curl -X POST http://localhost:6336/api/ticket \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{"data":{"type":"ticket","attributes":{"title":"Private admin ticket"}}}' | jq
```
**Result**: Event does NOT appear in Mary's viewer (no permission) ✓

---

## Step 7: Advanced Features

### 7.1 Filter Events by Type

Subscribe to only CREATE events:

```javascript
ws.send(JSON.stringify({
  method: 'subscribe',
  attributes: {
    topicName: 'ticket',
    EventType: 'create'  // Only receive CREATE events
  }
}));
```

### 7.2 Reconnection Logic

Add automatic reconnection for production:

```javascript
class RobustWebSocket {
  constructor(url, token) {
    this.url = `${url}?token=${token}`;
    this.reconnectDelay = 5000;
    this.connect();
  }

  connect() {
    this.ws = new WebSocket(this.url);

    this.ws.on('open', () => {
      console.log('✓ Connected');
      this.reconnectDelay = 5000; // Reset delay on successful connection
      this.onOpen();
    });

    this.ws.on('close', () => {
      console.log(`Reconnecting in ${this.reconnectDelay/1000}s...`);
      setTimeout(() => this.connect(), this.reconnectDelay);
      this.reconnectDelay = Math.min(this.reconnectDelay * 2, 60000); // Exponential backoff
    });

    this.ws.on('message', (data) => this.onMessage(data));
  }

  onOpen() {
    // Re-subscribe to topics after reconnection
    this.ws.send(JSON.stringify({
      method: 'subscribe',
      attributes: { topicName: 'ticket' }
    }));
  }

  onMessage(data) {
    const msg = JSON.parse(data.toString());
    console.log('Event:', msg);
  }
}
```

### 7.3 Unsubscribe

Stop receiving events from a topic:

```javascript
ws.send(JSON.stringify({
  method: 'unsubscribe',
  attributes: {
    topicName: 'ticket,user_account'
  }
}));
```

### 7.4 Delete Custom Topics

Remove custom topics you created:

```javascript
ws.send(JSON.stringify({
  method: 'destroy-topicName',
  attributes: {
    name: 'notifications'
  }
}));
```

**Note**: Cannot delete system topics (database tables).

---

## Troubleshooting

### Connection Fails with "Unexpected response: 403"

**Cause**: Invalid or expired JWT token.

**Solution**:
```bash
# Get fresh token
TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')

echo "$TOKEN" > /tmp/daptin-token.txt
```

### Not Receiving Events

**Possible causes**:
1. **Not subscribed**: Verify subscription confirmation message
2. **Permission issue**: User doesn't have read permission on the records
3. **Wrong topic name**: Topic names are exact (case-sensitive)

**Debug**:
```bash
# List all topics
ws.send(JSON.stringify({
  method: 'list-topicName',
  attributes: {}
}));
```

### Events for Records I Can't See in API

**Cause**: Permission check timing - you might have permission at subscription time but not when querying API.

**Solution**: Check permissions on the world (table) and record level.

---

## Complete Example: Chat Application

Here's a complete chat application using WebSocket:

```javascript
const WebSocket = require('ws');
const readline = require('readline');

class ChatClient {
  constructor(token, username, room) {
    this.token = token;
    this.username = username;
    this.room = room;
    this.ws = null;

    // Setup CLI
    this.rl = readline.createInterface({
      input: process.stdin,
      output: process.stdout
    });
  }

  connect() {
    this.ws = new WebSocket(`ws://localhost:6336/live?token=${this.token}`);

    this.ws.on('open', () => {
      console.log(`\n💬 Joined chat room: ${this.room}\n`);

      // Create room topic if doesn't exist
      this.send('create-topicName', { name: this.room });

      // Subscribe to room
      setTimeout(() => {
        this.send('subscribe', { topicName: this.room });
        this.prompt();
      }, 500);
    });

    this.ws.on('message', (data) => {
      const msg = JSON.parse(data.toString());

      if (msg.ObjectType === this.room && msg.EventType !== 'subscription-response') {
        const chat = JSON.parse(Buffer.from(msg.EventData, 'base64').toString());
        if (chat.username !== this.username) {
          console.log(`\n${chat.username}: ${chat.message}`);
          this.prompt();
        }
      }
    });
  }

  send(method, attributes) {
    this.ws.send(JSON.stringify({ method, attributes }));
  }

  prompt() {
    this.rl.question('> ', (message) => {
      if (message === '/quit') {
        console.log('Goodbye!');
        process.exit(0);
      }

      this.send('new-message', {
        topicName: this.room,
        message: JSON.stringify({
          username: this.username,
          message: message,
          timestamp: Date.now()
        })
      });

      this.prompt();
    });
  }
}

// Usage
const token = process.argv[2];
const username = process.argv[3] || 'Anonymous';
const room = process.argv[4] || 'general';

const chat = new ChatClient(token, username, room);
chat.connect();
```

**Run it:**
```bash
# Terminal 1
node chat.js "$(cat /tmp/daptin-token.txt)" "Alice" "team-chat"

# Terminal 2
node chat.js "$(cat /tmp/daptin-token.txt)" "Bob" "team-chat"

# Now type messages and see them appear in real-time!
```

---

## Quick Reference

### WebSocket Methods

| Method | Purpose | Parameters |
|--------|---------|------------|
| `list-topicName` | List all topics | `{}` |
| `subscribe` | Subscribe to topics | `{topicName: "topic1,topic2"}` |
| `unsubscribe` | Unsubscribe from topics | `{topicName: "topic1"}` |
| `create-topicName` | Create custom topic | `{name: "my-topic"}` |
| `destroy-topicName` | Delete custom topic | `{name: "my-topic"}` |
| `new-message` | Publish to custom topic | `{topicName: "my-topic", message: "..."}` |

### Event Message Format

```javascript
{
  "EventType": "create",           // create, update, delete
  "ObjectType": "ticket",          // Topic name
  "EventData": "base64String..."  // Base64-encoded JSON
}
```

### Decode Event Data

```javascript
const msg = JSON.parse(data.toString());
if (msg.EventData) {
  const eventData = JSON.parse(
    Buffer.from(msg.EventData, 'base64').toString()
  );
  console.log(eventData);
}
```

### System Topics

Every database table has an automatic topic:
- `ticket` - Ticket changes
- `user_account` - User changes
- `document` - Document changes
- `world` - Schema changes
- etc.

### Browser Example

```html
<!DOCTYPE html>
<html>
<head>
  <title>Real-Time Notifications</title>
</head>
<body>
  <h1>Notifications</h1>
  <div id="notifications"></div>

  <script>
    const TOKEN = 'your-jwt-token-here';
    const ws = new WebSocket(`ws://localhost:6336/live?token=${TOKEN}`);

    ws.onopen = () => {
      console.log('Connected');
      ws.send(JSON.stringify({
        method: 'subscribe',
        attributes: { topicName: 'ticket' }
      }));
    };

    ws.onmessage = (event) => {
      const msg = JSON.parse(event.data);

      if (msg.ObjectType === 'ticket' && msg.EventData) {
        const data = JSON.parse(atob(msg.EventData));
        const div = document.createElement('div');
        div.textContent = `${msg.EventType}: ${data.data.attributes.title}`;
        document.getElementById('notifications').prepend(div);
      }
    };
  </script>
</body>
</html>
```

---

## Next Steps

After completing this walkthrough, you can:

1. **Build a live dashboard**: Subscribe to multiple topics and display real-time stats
2. **Add notifications**: Show toast/alerts when events occur
3. **Implement chat**: Use custom topics for team communication
4. **Activity feeds**: Show recent activity in real-time
5. **Presence system**: Track who's online using custom topics
6. **React integration**: Use hooks for WebSocket subscriptions
7. **Mobile apps**: Connect from iOS/Android apps

---

## Summary

You've learned:

✅ How to connect to Daptin's WebSocket endpoint with authentication
✅ Subscribing to system topics for database changes
✅ Creating and using custom topics for application messaging
✅ Understanding permission-based event filtering
✅ Building production-ready real-time features
✅ Handling reconnection and error scenarios

The WebSocket API provides the foundation for any real-time feature in your application. Combined with Daptin's permission system, you get secure, scalable real-time updates automatically.
