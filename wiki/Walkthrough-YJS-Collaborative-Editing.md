# Walkthrough: Building Collaborative Editing with YJS

**A complete, step-by-step guide** to implement Google Docs-style collaborative editing in your application with Daptin and YJS.

By the end of this walkthrough, you'll have:
- ✅ Real-time collaborative text editing
- ✅ Multiple users editing simultaneously without conflicts
- ✅ User presence/awareness (see who's editing)
- ✅ Offline support with automatic synchronization
- ✅ Understanding of YJS and CRDT basics

**Time Required**: 30-40 minutes
**Difficulty**: Intermediate (JavaScript and basic editor knowledge helpful)

---

## What You'll Learn

This walkthrough teaches you:

1. **YJS Basics**: What are CRDTs and why they matter
2. **Daptin Integration**: Two YJS endpoint types
3. **Plain Text Editing**: Build a simple collaborative text editor
4. **Rich Text Editing**: Integrate with Quill editor
5. **Code Editing**: Integrate with Monaco editor
6. **User Awareness**: Show other users' cursors and selections
7. **Offline Mode**: How changes sync when reconnected

---

## The Scenario

**Application**: Document collaboration platform
**Feature**: Google Docs-style editing

**What We're Building**:
1. Plain text collaborative editor (browser-based)
2. Rich text editor with Quill
3. Code editor with Monaco
4. User presence indicators
5. Offline editing support

**Use Cases**:
- Team documentation
- Code pair programming
- Note-taking apps
- Content management systems
- Design collaboration

---

## Architecture Overview

```
┌──────────────────────────────────────────────────────────┐
│                   Client Applications                     │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐         │
│  │  Browser 1 │  │  Browser 2 │  │  Browser 3 │         │
│  │   Alice    │  │    Bob     │  │   Carol    │         │
│  └──────┬─────┘  └──────┬─────┘  └──────┬─────┘         │
│         │                │                │               │
│         │ Y.Doc (CRDT)   │ Y.Doc (CRDT)   │ Y.Doc (CRDT) │
│         │   ytext        │   ytext        │   ytext      │
│         │                │                │               │
│         └────────────────┼────────────────┘               │
│                          │                                │
│              WebsocketProvider (y-websocket)              │
│                          │                                │
└──────────────────────────┼────────────────────────────────┘
                           │
                           │ WebSocket
                           │ ws://localhost:6336/yjs/:documentName
                           ▼
┌──────────────────────────────────────────────────────────┐
│                   Daptin Server                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │           YJS Handler                              │  │
│  │  • Authenticates user from token                   │  │
│  │  • Broadcasts deltas to all connected clients      │  │
│  │  • Persists document state to disk                 │  │
│  └───────────────────┬────────────────────────────────┘  │
│                      │                                    │
│                      ▼                                    │
│  ┌──────────────────────────────────────────────────┐    │
│  │       YJS Document Storage                        │    │
│  │  • ZIP files with YJS binary state                │    │
│  │  • Plain text fallback                             │    │
│  │  • Automatic versioning                            │    │
│  └──────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────┐
│                  How CRDT Works                          │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  Alice types: "Hello"     →  [H, e, l, l, o]           │
│  Bob types: " World"      →  [H, e, l, l, o, _, W...]  │
│  Carol types "!" at end   →  [H, e, l, l, o, _, W...!] │
│                                                          │
│  ✨ No conflicts! CRDTs automatically merge changes    │
│     Each character has a unique ID and position         │
│     Operations commute (order doesn't matter)           │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

---

## Before You Begin

### Prerequisites Check

```bash
# 1. Daptin running
curl -s http://localhost:6336/api/world | head -c 50
# Expected: {"data":[...

# 2. Valid authentication token
cat /tmp/daptin-token.txt
# Expected: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...

# 3. Node.js and required packages
node --version
# Expected: v18.0.0 or higher

npm install yjs y-websocket ws
# Expected: success
```

### Understanding YJS and CRDTs

**What is a CRDT?**
Conflict-free Replicated Data Type - a data structure that can be updated independently on multiple devices and automatically merged without conflicts.

**Why use YJS?**
- **No conflicts**: Multiple users edit simultaneously without merge conflicts
- **Offline-first**: Works without internet, syncs when reconnected
- **Fast**: Efficient binary encoding and delta synchronization
- **Battle-tested**: Used by major apps like Linear, Notion, etc.

**Key concepts**:
- **Y.Doc**: The shared document (CRDT container)
- **Y.Text**: Shared text type (for text editing)
- **Y.Array**: Shared array type
- **Y.Map**: Shared key-value map
- **WebsocketProvider**: Connects Y.Doc to Daptin server
- **Awareness**: Tracks user presence (cursors, selections)

---

## Step 0: Enable YJS in Daptin

First, enable YJS and configure storage:

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

# Enable YJS
curl -X POST http://localhost:6336/_config/backend/yjs.enabled \
  -H "Authorization: Bearer $TOKEN" \
  -d 'true' | jq

# Set storage path (optional, defaults to ./yjs-documents)
curl -X POST http://localhost:6336/_config/backend/yjs.storage.path \
  -H "Authorization: Bearer $TOKEN" \
  -d '"./yjs-data"' | jq

# Verify configuration
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/_config/backend/yjs.enabled | jq

curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/_config/backend/yjs.storage.path | jq
```

**Expected output:**
```json
true
"./yjs-data"
```

---

## Step 1: Test YJS Connection

Let's verify YJS endpoints are working:

```bash
cat > test-yjs-connection.js << 'EOF'
const WebSocket = require('ws');
const TOKEN = process.argv[2];

console.log('Testing YJS endpoint...');
const ws = new WebSocket(
  `ws://localhost:6336/yjs/test-document?token=${TOKEN}`
);

ws.on('open', () => {
  console.log('✓ YJS WebSocket connected successfully!');
  console.log('  Endpoint: ws://localhost:6336/yjs/test-document');
  setTimeout(() => {
    ws.close();
    process.exit(0);
  }, 2000);
});

ws.on('message', (data) => {
  console.log('← Received YJS protocol message (binary)');
});

ws.on('error', (err) => {
  console.error('✗ Connection failed:', err.message);
  process.exit(1);
});
EOF

node test-yjs-connection.js "$(cat /tmp/daptin-token.txt)"
```

**Expected output:**
```
Testing YJS endpoint...
✓ YJS WebSocket connected successfully!
  Endpoint: ws://localhost:6336/yjs/test-document
← Received YJS protocol message (binary)
```

---

## Step 2: Build a Simple Plain Text Collaborative Editor

Let's create a basic HTML page with collaborative text editing:

```bash
cat > collaborative-editor.html << 'EOF'
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Collaborative Text Editor</title>
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
      max-width: 900px;
      margin: 50px auto;
      padding: 20px;
      background: #f5f5f5;
    }
    .container {
      background: white;
      border-radius: 8px;
      padding: 30px;
      box-shadow: 0 2px 10px rgba(0,0,0,0.1);
    }
    h1 {
      margin-top: 0;
      color: #333;
    }
    .status {
      padding: 10px;
      margin-bottom: 20px;
      border-radius: 4px;
      font-size: 14px;
    }
    .status.connected {
      background: #d4edda;
      color: #155724;
    }
    .status.connecting {
      background: #fff3cd;
      color: #856404;
    }
    .users {
      padding: 10px;
      margin-bottom: 20px;
      background: #e7f3ff;
      border-radius: 4px;
      font-size: 14px;
    }
    .user-tag {
      display: inline-block;
      padding: 4px 10px;
      margin: 4px;
      border-radius: 12px;
      color: white;
      font-size: 12px;
    }
    textarea {
      width: 100%;
      min-height: 400px;
      padding: 15px;
      border: 2px solid #ddd;
      border-radius: 4px;
      font-family: 'Monaco', 'Courier New', monospace;
      font-size: 14px;
      resize: vertical;
      box-sizing: border-box;
    }
    textarea:focus {
      outline: none;
      border-color: #4CAF50;
    }
    .instructions {
      margin-top: 20px;
      padding: 15px;
      background: #f8f9fa;
      border-left: 4px solid #4CAF50;
      font-size: 14px;
    }
  </style>
</head>
<body>
  <div class="container">
    <h1>📝 Collaborative Text Editor</h1>

    <div id="status" class="status connecting">
      🔄 Connecting to server...
    </div>

    <div id="users" class="users">
      👥 Online users: <span id="user-count">0</span>
      <div id="user-list"></div>
    </div>

    <textarea id="editor" placeholder="Start typing... Changes sync in real-time!"></textarea>

    <div class="instructions">
      <strong>💡 Try this:</strong><br>
      1. Open this page in multiple browser windows<br>
      2. Type in one window and watch it appear in others<br>
      3. All changes sync instantly, no conflicts!
    </div>
  </div>

  <script src="https://cdn.jsdelivr.net/npm/yjs@13/dist/yjs.mjs" type="module"></script>
  <script src="https://cdn.jsdelivr.net/npm/y-websocket@1/dist/y-websocket.mjs" type="module"></script>

  <script type="module">
    import * as Y from 'https://cdn.jsdelivr.net/npm/yjs@13/dist/yjs.mjs';
    import { WebsocketProvider } from 'https://cdn.jsdelivr.net/npm/y-websocket@1/dist/y-websocket.mjs';

    // *** REPLACE THIS WITH YOUR TOKEN ***
    const TOKEN = 'YOUR_JWT_TOKEN_HERE';

    // Create YJS document
    const ydoc = new Y.Doc();
    const ytext = ydoc.getText('content');

    // Connect to Daptin
    const provider = new WebsocketProvider(
      `ws://localhost:6336/yjs/my-document?token=${TOKEN}`,
      'my-document',
      ydoc
    );

    // Get DOM elements
    const textarea = document.getElementById('editor');
    const statusDiv = document.getElementById('status');
    const userCount = document.getElementById('user-count');
    const userList = document.getElementById('user-list');

    // Set this user's info
    const userName = 'User ' + Math.floor(Math.random() * 1000);
    const userColor = '#' + Math.floor(Math.random()*16777215).toString(16);

    provider.awareness.setLocalStateField('user', {
      name: userName,
      color: userColor
    });

    // Connection status
    provider.on('status', event => {
      if (event.status === 'connected') {
        statusDiv.className = 'status connected';
        statusDiv.textContent = '✅ Connected - Changes sync in real-time';
      } else {
        statusDiv.className = 'status connecting';
        statusDiv.textContent = '🔄 Connecting...';
      }
    });

    // Track online users
    provider.awareness.on('change', () => {
      const states = Array.from(provider.awareness.getStates().values());
      const users = states.filter(s => s.user).map(s => s.user);

      userCount.textContent = users.length;
      userList.innerHTML = users.map(user =>
        `<span class="user-tag" style="background: ${user.color}">${user.name}</span>`
      ).join('');
    });

    // Sync textarea with YJS
    let updating = false;

    // Update textarea when YJS changes
    ytext.observe(() => {
      if (!updating) {
        const cursorPos = textarea.selectionStart;
        const newValue = ytext.toString();

        if (textarea.value !== newValue) {
          updating = true;
          textarea.value = newValue;
          textarea.setSelectionRange(cursorPos, cursorPos);
          updating = false;
        }
      }
    });

    // Update YJS when textarea changes
    textarea.addEventListener('input', (e) => {
      if (!updating) {
        updating = true;
        const newValue = textarea.value;
        const oldValue = ytext.toString();

        // Simple diff and update
        if (newValue !== oldValue) {
          ydoc.transact(() => {
            ytext.delete(0, oldValue.length);
            ytext.insert(0, newValue);
          });
        }
        updating = false;
      }
    });

    // Initialize with current content
    textarea.value = ytext.toString();

    console.log('✅ Collaborative editor initialized');
    console.log('   Your name:', userName);
    console.log('   Your color:', userColor);
  </script>
</body>
</html>
EOF

echo "✓ Created collaborative-editor.html"
echo ""
echo "Next steps:"
echo "1. Open the file and replace YOUR_JWT_TOKEN_HERE with your token"
echo "2. Serve it with: python3 -m http.server 8000"
echo "3. Open http://localhost:8000/collaborative-editor.html in multiple browsers"
```

**Replace the token in the HTML file:**
```bash
# Auto-replace token
TOKEN=$(cat /tmp/daptin-token.txt)
sed -i.bak "s/YOUR_JWT_TOKEN_HERE/$TOKEN/" collaborative-editor.html

echo "✓ Token inserted"
```

**Run it:**
```bash
# Serve the HTML file
python3 -m http.server 8000 &
echo "✓ Server started at http://localhost:8000"

# Open in browser
open http://localhost:8000/collaborative-editor.html
# Or manually open: http://localhost:8000/collaborative-editor.html
```

**Test it:**
1. Open the URL in two browser windows side-by-side
2. Type in one window
3. Watch the text appear in real-time in the other window!
4. Try typing simultaneously in both - no conflicts!

---

## Step 3: Integrate with Quill Rich Text Editor

Now let's build a professional rich text editor with formatting:

```bash
npm install quill y-quill

cat > rich-text-editor.html << 'EOF'
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Collaborative Rich Text Editor</title>
  <link href="https://cdn.quilljs.com/1.3.6/quill.snow.css" rel="stylesheet">
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
      max-width: 1000px;
      margin: 30px auto;
      padding: 20px;
      background: #f5f5f5;
    }
    .container {
      background: white;
      border-radius: 8px;
      box-shadow: 0 2px 10px rgba(0,0,0,0.1);
    }
    .header {
      padding: 20px 30px;
      border-bottom: 1px solid #e0e0e0;
    }
    h1 {
      margin: 0 0 10px 0;
      font-size: 24px;
    }
    .status {
      font-size: 14px;
      color: #666;
    }
    .status.connected { color: #4CAF50; }
    .users {
      padding: 15px 30px;
      background: #f8f9fa;
      border-bottom: 1px solid #e0e0e0;
      font-size: 14px;
    }
    .user-cursor {
      display: inline-block;
      width: 12px;
      height: 12px;
      border-radius: 50%;
      margin-right: 8px;
    }
    #editor {
      min-height: 500px;
      padding: 30px;
    }
    .ql-editor {
      font-size: 16px;
      line-height: 1.6;
    }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>📄 Collaborative Document</h1>
      <div id="status" class="status">⏳ Connecting...</div>
    </div>

    <div class="users" id="users-container">
      <span id="user-count">0</span> editors online
    </div>

    <div id="editor"></div>
  </div>

  <script src="https://cdn.quilljs.com/1.3.6/quill.min.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/yjs@13/dist/yjs.mjs" type="module"></script>
  <script src="https://cdn.jsdelivr.net/npm/y-websocket@1/dist/y-websocket.mjs" type="module"></script>
  <script src="https://cdn.jsdelivr.net/npm/y-quill@0.1.5/dist/y-quill.min.js"></script>

  <script type="module">
    import * as Y from 'https://cdn.jsdelivr.net/npm/yjs@13/dist/yjs.mjs';
    import { WebsocketProvider } from 'https://cdn.jsdelivr.net/npm/y-websocket@1/dist/y-websocket.mjs';

    // *** REPLACE WITH YOUR TOKEN ***
    const TOKEN = 'YOUR_JWT_TOKEN_HERE';

    // Initialize YJS
    const ydoc = new Y.Doc();
    const ytext = ydoc.getText('quill');

    // Connect to Daptin
    const provider = new WebsocketProvider(
      `ws://localhost:6336/yjs/rich-document?token=${TOKEN}`,
      'rich-document',
      ydoc
    );

    // Set user info
    const userName = prompt('Enter your name:') || 'Anonymous';
    const userColor = '#' + Math.floor(Math.random()*16777215).toString(16);

    provider.awareness.setLocalStateField('user', {
      name: userName,
      color: userColor
    });

    // Initialize Quill
    const quill = new Quill('#editor', {
      theme: 'snow',
      modules: {
        toolbar: [
          [{ 'header': [1, 2, 3, false] }],
          ['bold', 'italic', 'underline', 'strike'],
          ['blockquote', 'code-block'],
          [{ 'list': 'ordered'}, { 'list': 'bullet' }],
          [{ 'indent': '-1'}, { 'indent': '+1' }],
          ['link', 'image'],
          ['clean']
        ]
      }
    });

    // Bind Quill to YJS
    const binding = new window.QuillBinding(ytext, quill, provider.awareness);

    // Update status
    const statusDiv = document.getElementById('status');
    provider.on('status', event => {
      if (event.status === 'connected') {
        statusDiv.className = 'status connected';
        statusDiv.textContent = '✅ Connected';
      } else {
        statusDiv.className = 'status';
        statusDiv.textContent = '⏳ Connecting...';
      }
    });

    // Track users
    const usersContainer = document.getElementById('users-container');
    const userCountSpan = document.getElementById('user-count');

    provider.awareness.on('change', () => {
      const states = Array.from(provider.awareness.getStates().values());
      const users = states.filter(s => s.user).map(s => s.user);

      userCountSpan.textContent = users.length;

      const userHTML = users.map(user => `
        <span style="margin-right: 15px;">
          <span class="user-cursor" style="background: ${user.color}"></span>
          ${user.name}
        </span>
      `).join('');

      usersContainer.innerHTML = `${users.length} editors online: ${userHTML}`;
    });

    console.log('✅ Rich text editor initialized');
  </script>
</body>
</html>
EOF

echo "✓ Created rich-text-editor.html"

# Replace token
TOKEN=$(cat /tmp/daptin-token.txt)
sed -i.bak "s/YOUR_JWT_TOKEN_HERE/$TOKEN/" rich-text-editor.html

# Open in browser
open http://localhost:8000/rich-text-editor.html
```

**Features:**
- ✅ Bold, italic, underline formatting
- ✅ Headers, lists, quotes
- ✅ Real-time collaboration
- ✅ User cursors and selections
- ✅ Images and links

---

## Step 4: Build a Collaborative Code Editor (Monaco)

For code editing with syntax highlighting:

```bash
cat > code-editor.html << 'EOF'
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Collaborative Code Editor</title>
  <style>
    body {
      margin: 0;
      padding: 0;
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
      overflow: hidden;
    }
    .header {
      background: #1e1e1e;
      color: white;
      padding: 15px 20px;
      display: flex;
      justify-content: space-between;
      align-items: center;
    }
    .title {
      font-size: 18px;
      font-weight: 500;
    }
    .status {
      font-size: 14px;
      opacity: 0.8;
    }
    .users {
      display: flex;
      gap: 10px;
    }
    .user-badge {
      padding: 4px 10px;
      border-radius: 12px;
      font-size: 12px;
      color: white;
    }
    #editor {
      width: 100vw;
      height: calc(100vh - 60px);
    }
  </style>
</head>
<body>
  <div class="header">
    <div class="title">👨‍💻 Collaborative Code Editor</div>
    <div class="status" id="status">⏳ Connecting...</div>
    <div class="users" id="users"></div>
  </div>
  <div id="editor"></div>

  <script src="https://cdn.jsdelivr.net/npm/monaco-editor@0.43.0/min/vs/loader.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/yjs@13/dist/yjs.mjs" type="module"></script>
  <script src="https://cdn.jsdelivr.net/npm/y-websocket@1/dist/y-websocket.mjs" type="module"></script>
  <script src="https://cdn.jsdelivr.net/npm/y-monaco@0.1.5/dist/y-monaco.min.js"></script>

  <script type="module">
    import * as Y from 'https://cdn.jsdelivr.net/npm/yjs@13/dist/yjs.mjs';
    import { WebsocketProvider } from 'https://cdn.jsdelivr.net/npm/y-websocket@1/dist/y-websocket.mjs';

    // *** REPLACE WITH YOUR TOKEN ***
    const TOKEN = 'YOUR_JWT_TOKEN_HERE';

    // Load Monaco
    require.config({
      paths: { 'vs': 'https://cdn.jsdelivr.net/npm/monaco-editor@0.43.0/min/vs' }
    });

    require(['vs/editor/editor.main'], function() {
      // Initialize YJS
      const ydoc = new Y.Doc();
      const ytext = ydoc.getText('monaco');

      // Connect to Daptin
      const provider = new WebsocketProvider(
        `ws://localhost:6336/yjs/code-session?token=${TOKEN}`,
        'code-session',
        ydoc
      );

      // Set user info
      const userName = prompt('Enter your name:') || 'Anonymous';
      const userColor = '#' + Math.floor(Math.random()*16777215).toString(16);

      provider.awareness.setLocalStateField('user', {
        name: userName,
        color: userColor
      });

      // Create Monaco editor
      const editor = monaco.editor.create(document.getElementById('editor'), {
        value: '',
        language: 'javascript',
        theme: 'vs-dark',
        automaticLayout: true
      });

      // Bind Monaco to YJS
      const binding = new window.MonacoBinding(
        ytext,
        editor.getModel(),
        new Set([editor]),
        provider.awareness
      );

      // Update status
      const statusDiv = document.getElementById('status');
      provider.on('status', event => {
        if (event.status === 'connected') {
          statusDiv.textContent = '✅ Connected';
        } else {
          statusDiv.textContent = '⏳ Connecting...';
        }
      });

      // Track users
      const usersDiv = document.getElementById('users');
      provider.awareness.on('change', () => {
        const states = Array.from(provider.awareness.getStates().values());
        const users = states.filter(s => s.user).map(s => s.user);

        usersDiv.innerHTML = users.map(user => `
          <div class="user-badge" style="background: ${user.color}">
            ${user.name}
          </div>
        `).join('');
      });

      console.log('✅ Code editor initialized');
    });
  </script>
</body>
</html>
EOF

echo "✓ Created code-editor.html"

# Replace token
TOKEN=$(cat /tmp/daptin-token.txt)
sed -i.bak "s/YOUR_JWT_TOKEN_HERE/$TOKEN/" code-editor.html

# Open in browser
open http://localhost:8000/code-editor.html
```

**Features:**
- ✅ Full Monaco editor (VS Code engine)
- ✅ Syntax highlighting
- ✅ IntelliSense autocompletion
- ✅ Real-time collaboration
- ✅ User cursors visible

---

## Step 5: Using File Column YJS Endpoints

For collaborative editing of database records with `file.*` columns:

### 5.1 Create Schema with File Column

```bash
cat > schema_document.yaml << 'EOF'
Tables:
  - TableName: document
    Columns:
      - Name: title
        DataType: varchar(500)
        ColumnType: name

      - Name: content
        DataType: text
        ColumnType: file.document  # YJS-enabled
EOF

# Restart to load schema
./scripts/testing/test-runner.sh stop && ./scripts/testing/test-runner.sh start
./scripts/testing/test-runner.sh token
```

### 5.2 Create a Document

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

curl -X POST http://localhost:6336/api/document \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "document",
      "attributes": {
        "title": "Team Project Notes",
        "content": [{
          "name": "notes.txt",
          "file": "data:text/plain;base64,V2VsY29tZSB0byBvdXIgdGVhbSBub3Rlcw==",
          "type": "text/plain"
        }]
      }
    }
  }' | jq '.data.id'

# Save the document ID
DOC_ID="<paste-id-here>"
```

### 5.3 Connect to Document's YJS Endpoint

```javascript
// YJS endpoint for this specific document:
// ws://localhost:6336/live/document/{DOC_ID}/content/yjs?token={TOKEN}

const ydoc = new Y.Doc();
const provider = new WebsocketProvider(
  `ws://localhost:6336/live/document/${DOC_ID}/content/yjs?token=${TOKEN}`,
  `doc-${DOC_ID}`,
  ydoc
);

const ytext = ydoc.getText('content');
// ... bind to your editor
```

**Benefit**: Changes are persisted to the database record automatically!

---

## Step 6: Offline Support and Synchronization

YJS works offline and syncs when reconnected:

```javascript
// Monitor connection status
provider.on('status', event => {
  if (event.status === 'connected') {
    console.log('✅ Online - syncing...');
  } else {
    console.log('⚠️ Offline - changes saved locally');
  }
});

// Monitor sync status
provider.on('sync', isSynced => {
  if (isSynced) {
    console.log('✅ All changes synced');
  } else {
    console.log('⏳ Syncing changes...');
  }
});
```

**How it works**:
1. You edit while offline → Changes stored in local Y.Doc
2. Connection restored → YJS automatically sends deltas
3. Server broadcasts to other clients
4. Everyone's documents converge to same state

---

## Troubleshooting

### Connection Fails with 403

**Cause**: Invalid or expired JWT token.

**Solution**:
```bash
# Get fresh token
TOKEN=$(./scripts/testing/test-runner.sh token)
cat /tmp/daptin-token.txt
```

### Changes Don't Sync

**Possible causes**:
1. **YJS not enabled**: Check config
2. **Wrong endpoint**: Use `/yjs/:documentName` for direct, `/live/{table}/{id}/{column}/yjs` for file columns
3. **Token not passed**: Must include `?token=JWT` in URL

**Debug**:
```bash
# Check YJS enabled
curl -s -H "Authorization: Bearer $(cat /tmp/daptin-token.txt)" \
  http://localhost:6336/_config/backend/yjs.enabled
# Expected: true
```

### Document State Lost

**Cause**: YJS storage not persisted.

**Check storage**:
```bash
# View YJS documents
ls -lh ./yjs-data/
# Expected: ZIP files with document states
```

---

## Advanced Features

### Custom Y.Text Operations

```javascript
// Insert text at position
ytext.insert(0, 'Hello ');

// Delete characters
ytext.delete(0, 5);  // Delete first 5 characters

// Get full text
const text = ytext.toString();

// Observe changes
ytext.observe(event => {
  console.log('Text changed:', event);
});
```

### Using Y.Map for Key-Value Data

```javascript
const ymap = ydoc.getMap('metadata');

ymap.set('author', 'Alice');
ymap.set('lastEdit', Date.now());

ymap.observe(event => {
  event.keysChanged.forEach(key => {
    console.log(`${key} changed to:`, ymap.get(key));
  });
});
```

### Using Y.Array for Lists

```javascript
const yarray = ydoc.getArray('tasks');

yarray.push(['Task 1', 'Task 2']);
yarray.insert(1, ['New Task']);  // Insert at index 1
yarray.delete(0, 1);  // Remove first item

yarray.observe(event => {
  console.log('Array changed');
});
```

---

## Best Practices

### 1. Use Transactions for Bulk Updates

```javascript
// ❌ Bad: Multiple separate operations
ytext.insert(0, 'Hello');
ytext.insert(5, ' World');

// ✅ Good: Single transaction
ydoc.transact(() => {
  ytext.insert(0, 'Hello');
  ytext.insert(5, ' World');
});
```

### 2. Clean Up Resources

```javascript
// When user leaves
provider.destroy();
ydoc.destroy();
```

### 3. Handle Reconnection

```javascript
provider.on('connection-close', () => {
  console.log('Connection lost, will reconnect...');
});

provider.on('connection-error', (error) => {
  console.error('Connection error:', error);
});
```

### 4. Set Meaningful Room Names

```javascript
// ✅ Use unique identifiers
new WebsocketProvider(url, `doc-${documentId}`, ydoc);

// ❌ Avoid generic names
new WebsocketProvider(url, 'my-room', ydoc);
```

---

## Production Checklist

- [ ] Enable YJS in Daptin config
- [ ] Configure persistent storage path
- [ ] Implement authentication token refresh
- [ ] Add connection status UI
- [ ] Show user presence indicators
- [ ] Handle offline mode gracefully
- [ ] Clean up resources on component unmount
- [ ] Add error boundaries
- [ ] Test with multiple concurrent users
- [ ] Monitor YJS storage size

---

## Summary

You've learned:

✅ What CRDTs are and why they prevent conflicts
✅ How to enable and configure YJS in Daptin
✅ Building plain text collaborative editors
✅ Integrating with Quill for rich text
✅ Integrating with Monaco for code editing
✅ Using file column YJS endpoints
✅ Handling offline mode and synchronization
✅ Best practices for production deployment

YJS + Daptin provides a complete solution for collaborative editing with zero backend code. The CRDT technology ensures your users can work together seamlessly without conflicts, even when offline.

---

## Next Steps

1. **Add more editor types**: Spreadsheets (Handsontable), diagrams (Excalidraw)
2. **Version history**: Store snapshots for undo/redo across sessions
3. **Comments**: Use Y.Map for collaborative commenting
4. **Presence awareness**: Show user cursors and selections
5. **Access control**: Combine with Daptin permissions
6. **Mobile apps**: Integrate y-websocket in React Native
7. **Desktop apps**: Use with Electron

---

## Resources

- **YJS Docs**: https://docs.yjs.dev
- **y-websocket**: https://github.com/yjs/y-websocket
- **y-quill**: https://github.com/yjs/y-quill
- **y-monaco**: https://github.com/yjs/y-monaco
- **Daptin YJS Config**: See `wiki/YJS-Collaboration.md`
