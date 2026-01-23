# YJS Collaboration

Real-time collaborative document editing with conflict-free replication.

## Overview

YJS enables multiple users to edit documents simultaneously:
- Conflict-free (CRDT-based)
- Works offline
- Automatic synchronization
- User presence/awareness

## Endpoint Pattern

```
ws://localhost:6336/live/{typename}/{referenceId}/{columnName}/yjs
```

**Example:**
```
ws://localhost:6336/live/document/abc123/content/yjs?token=JWT_TOKEN
```

## Enabling YJS

```bash
curl -X POST http://localhost:6336/_config/backend/yjs.enabled \
  -H "Authorization: Bearer $TOKEN" \
  -d 'true'
```

Configure storage path:

```bash
curl -X POST http://localhost:6336/_config/backend/yjs.storage.path \
  -H "Authorization: Bearer $TOKEN" \
  -d '"./yjs-documents"'
```

## File Column Types

YJS automatically works with file-type columns:

| Column Type | Editor |
|-------------|--------|
| `file.document` | Rich text (Quill) |
| `file.text` | Plain text |
| `file.code` | Code (Monaco/CodeMirror) |
| `file.markdown` | Markdown |
| `file.spreadsheet` | Spreadsheet |

## JavaScript Integration

### Basic Setup

```javascript
import * as Y from 'yjs';
import { WebsocketProvider } from 'y-websocket';

// Create YJS document
const ydoc = new Y.Doc();

// Connect to Daptin YJS endpoint
const provider = new WebsocketProvider(
  `ws://localhost:6336/live/document/${documentId}/content/yjs?token=${token}`,
  'document-room',
  ydoc
);

// Get shared text type
const ytext = ydoc.getText('content');

// Listen for changes
ytext.observe((event) => {
  console.log('Content changed:', ytext.toString());
});

// Make changes
ytext.insert(0, 'Hello, World!');
```

### With Quill Editor

```javascript
import * as Y from 'yjs';
import { WebsocketProvider } from 'y-websocket';
import { QuillBinding } from 'y-quill';
import Quill from 'quill';

const ydoc = new Y.Doc();
const provider = new WebsocketProvider(
  `ws://localhost:6336/live/document/${documentId}/content/yjs?token=${token}`,
  'document-room',
  ydoc,
  { awareness: provider.awareness }
);

const ytext = ydoc.getText('quill');

const quill = new Quill('#editor', {
  theme: 'snow',
  modules: {
    toolbar: [
      ['bold', 'italic', 'underline'],
      ['link', 'image'],
      [{ list: 'ordered' }, { list: 'bullet' }]
    ]
  }
});

// Bind Quill to YJS
const binding = new QuillBinding(ytext, quill, provider.awareness);
```

### With Monaco Editor

```javascript
import * as Y from 'yjs';
import { WebsocketProvider } from 'y-websocket';
import { MonacoBinding } from 'y-monaco';
import * as monaco from 'monaco-editor';

const ydoc = new Y.Doc();
const provider = new WebsocketProvider(
  `ws://localhost:6336/live/document/${documentId}/content/yjs?token=${token}`,
  'code-room',
  ydoc
);

const ytext = ydoc.getText('monaco');

const editor = monaco.editor.create(document.getElementById('editor'), {
  value: '',
  language: 'javascript'
});

// Bind Monaco to YJS
const binding = new MonacoBinding(
  ytext,
  editor.getModel(),
  new Set([editor]),
  provider.awareness
);
```

## User Awareness

Show other users' cursors and selections:

```javascript
const provider = new WebsocketProvider(url, room, ydoc);

// Set local user info
provider.awareness.setLocalStateField('user', {
  name: 'John Doe',
  color: '#ff0000'
});

// Listen for awareness changes
provider.awareness.on('change', () => {
  const states = provider.awareness.getStates();
  states.forEach((state, clientId) => {
    if (state.user) {
      console.log(`User ${state.user.name} is online`);
    }
  });
});
```

## Document Storage

YJS documents are stored as:
- ZIP archive containing:
  - YJS binary state
  - Plain text fallback
- Automatic versioning
- Conflict resolution built-in

## React Component Example

```javascript
import React, { useEffect, useRef, useState } from 'react';
import * as Y from 'yjs';
import { WebsocketProvider } from 'y-websocket';
import { QuillBinding } from 'y-quill';
import Quill from 'quill';

function CollaborativeEditor({ documentId, token }) {
  const editorRef = useRef(null);
  const [users, setUsers] = useState([]);

  useEffect(() => {
    const ydoc = new Y.Doc();

    const provider = new WebsocketProvider(
      `ws://localhost:6336/live/document/${documentId}/content/yjs?token=${token}`,
      `doc-${documentId}`,
      ydoc
    );

    // Set user identity
    provider.awareness.setLocalStateField('user', {
      name: 'Current User',
      color: '#' + Math.floor(Math.random()*16777215).toString(16)
    });

    // Track online users
    provider.awareness.on('change', () => {
      const states = Array.from(provider.awareness.getStates().values());
      setUsers(states.filter(s => s.user).map(s => s.user));
    });

    const ytext = ydoc.getText('quill');

    const quill = new Quill(editorRef.current, {
      theme: 'snow'
    });

    const binding = new QuillBinding(ytext, quill, provider.awareness);

    return () => {
      binding.destroy();
      provider.destroy();
      ydoc.destroy();
    };
  }, [documentId, token]);

  return (
    <div>
      <div className="users">
        {users.map((user, i) => (
          <span key={i} style={{ color: user.color }}>
            {user.name}
          </span>
        ))}
      </div>
      <div ref={editorRef} />
    </div>
  );
}
```

## Creating YJS-Enabled Documents

### Schema Definition

```yaml
Tables:
  - TableName: document
    Columns:
      - Name: title
        DataType: varchar(500)
        ColumnType: label
      - Name: content
        DataType: text
        ColumnType: file.document  # YJS-enabled
```

### Create Document

```bash
curl -X POST http://localhost:6336/api/document \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "document",
      "attributes": {
        "title": "Collaborative Doc"
      }
    }
  }'
```

Then connect to YJS endpoint using the returned reference_id.

## Offline Support

YJS works offline:

```javascript
// Changes are stored locally
ytext.insert(0, 'Offline edit');

// When reconnected, changes sync automatically
provider.on('sync', (isSynced) => {
  console.log('Synced:', isSynced);
});
```

## Performance

- Documents sync incrementally (deltas only)
- Efficient binary encoding
- Handles large documents
- Low latency updates
