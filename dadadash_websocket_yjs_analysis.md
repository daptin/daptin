# Dadadash WebSocket and YJS Real-Time Collaboration Analysis

## Overview

Dadadash implements real-time collaborative editing features using WebSocket connections and YJS (a CRDT-based library for building collaborative applications). This analysis documents the key patterns, connection setup, message formats, and integration approaches found in the codebase.

## Key Technologies Used

- **YJS** (v13.5.3): A CRDT (Conflict-free Replicated Data Type) framework for building collaborative applications
- **y-websocket** (v1.3.11): WebSocket provider for YJS
- **y-quill** (v0.1.4): YJS binding for Quill editor
- **y-codemirror** (v2.1.0): YJS binding for CodeMirror editor
- **quill-cursors** (v3.0.1): Cursor presence for collaborative editing

## WebSocket Connection Patterns

### 1. WebSocket URL Construction

The WebSocket connection is established using a consistent pattern across all collaborative editors:

```javascript
// From DocumentEditor.vue
let endpoint = that.endpoint();
endpoint = endpoint.substring(endpoint.indexOf("//"))
const provider = new WebsocketProvider(
  (window.location.protocol === "https:" ? "wss:" : "ws:")
  + '//' + endpoint + '/live/document/'
  + that.baseItem.reference_id + "/document_content",
  "yjs?token=" + token,
  ydoc
);
```

Key points:
- Protocol is automatically selected based on the current page protocol (ws:// or wss://)
- URL pattern: `/live/document/{document_id}/{field_name}`
- Authentication token is passed as a query parameter
- The room name is "yjs" with token appended

### 2. YJS Document Setup

Each editor creates a YJS document and connects it to the WebSocket provider:

```javascript
// Create YJS document
const ydoc = new Y.Doc()

// Create WebSocket provider
const provider = new WebsocketProvider(url, room, ydoc)

// Get shared data type (text for editors)
const ytext = ydoc.getText('quill')  // or 'codemirror' for CodeMirror
```

## Implementation Examples

### 1. Document Editor (Quill-based)

Location: `/src/pages/UserApps/DocumentEditor.vue`

Key features:
- Rich text collaborative editing using Quill
- User presence with colored cursors
- State persistence and recovery
- Auto-save with debouncing

```javascript
// Initialize Quill with collaborative features
const editor = new Quill(editorContainer, {
  modules: {
    cursors: true,
    toolbar: [...],
    history: {
      userOnly: true
    }
  },
  theme: 'snow'
})

// Bind YJS to Quill
const binding = new QuillBinding(ytext, editor, provider.awareness)

// Set user presence
provider.awareness.setLocalStateField('user', {
  name: that.decodedAuthToken().name,
  color: randomColor()
})

// Handle local changes
ytext.observe(function (event, transaction) {
  if (transaction.local) {
    that.saveDebounced();
  }
})
```

### 2. Mermaid Graph Editor (CodeMirror-based)

Location: `/src/pages/UserApps/MermaidGraphEditor.vue`

Key features:
- Collaborative diagram editing
- Real-time preview updates
- State synchronization

```javascript
// Create CodeMirror binding
const binding = new CodemirrorBinding(ytext, that.$refs.editor.codemirror, provider.awareness)

// Set user presence
if (that.decodedAuthToken && that.decodedAuthToken.name) {
  provider.awareness.setLocalStateField('user', {
    name: that.decodedAuthToken.name,
    color: randomColor()
  })
}

// Observe changes
ytext.observe(function () {
  that.debouncedUpdate();
})
```

## Data Persistence Pattern

Both editors use a consistent pattern for persisting collaborative state:

```javascript
// Saving state
let dataToSave = JSON.stringify({
  content: ytext.toString(),  // Plain text content
  encodedStateVector: fromUint8Array(Y.encodeStateAsUpdate(ydoc))  // YJS state
});

// Restoring state
if (persistedData.encodedStateVector) {
  Y.applyUpdate(ydoc, toUint8Array(persistedData.encodedStateVector))
}
```

## Authentication Integration

Authentication is handled by:
1. Retrieving the auth token from localStorage
2. Passing it as a query parameter in the WebSocket URL
3. Using the decoded token for user identification in presence

```javascript
let token = that.authToken();
const provider = new WebsocketProvider(
  wsUrl,
  "yjs?token=" + token,
  ydoc
);
```

## Auto-Save Implementation

Both editors implement debounced auto-save:

```javascript
// Create debounced save function
that.saveDebounced = debounce(that.saveDocument, 3 * 1000, false)

// Trigger on local changes
ytext.observe(function (event, transaction) {
  if (transaction.local) {
    that.saveDebounced();
  }
})
```

## File Storage Format

Documents are stored as ZIP files containing:
1. Content file (JSON with plain text and YJS state)
2. Settings file (page settings, configurations)

```javascript
var zip = new JSZip();
zip.file("contents.json", JSON.stringify({
  plaintext: ytext.toString(),
  encodedStateVector: fromUint8Array(Y.encodeStateAsUpdate(ydoc))
}));
zip.file("page-setting.json", JSON.stringify(pageSetting));
```

## Key Integration Points

### 1. Backend WebSocket Endpoint
- Pattern: `/live/{table_name}/{reference_id}/{field_name}`
- Authentication via token query parameter
- YJS protocol for message handling

### 2. Frontend Components
- Vue components with YJS integration
- Editor bindings (Quill, CodeMirror)
- Presence/awareness for user cursors
- State persistence and recovery

### 3. Data Flow
1. User makes edit → YJS captures change
2. YJS syncs via WebSocket to server
3. Server broadcasts to other connected clients
4. Remote changes applied to local YJS document
5. Editor UI updates automatically

## Best Practices Observed

1. **Debounced Saving**: Prevents excessive save operations
2. **State Recovery**: Handles reconnection gracefully
3. **User Presence**: Shows who's editing with colored cursors
4. **Protocol Selection**: Automatically uses ws:// or wss:// based on page protocol
5. **Error Handling**: Graceful fallback when state recovery fails

## Potential Improvements

1. **Offline Support**: Could add IndexedDB persistence (y-indexeddb is already a dependency)
2. **Conflict Resolution UI**: Visual indication of merge conflicts
3. **Version History**: Track document versions over time
4. **Permission Management**: Real-time permission updates
5. **Performance Monitoring**: Track sync latency and connection quality