# YJS collaboration

Daptin exposes the YJS WebSocket protocol for standalone rooms and for file
columns on resource records. Both endpoint forms for a resource-backed room
use the same room, resource permissions, and persistence path.

## Enable YJS

```bash
curl -X POST http://localhost:6336/_config/backend/yjs.enabled \
  -H "Authorization: Bearer $TOKEN" \
  -d 'true'
```

Restart Daptin after changing this startup setting.

## Resource-backed collaboration

Declare a file column with its extension filter. `file.*` declares any
extension; `file.md|txt` declares `.md` and `.txt`. Labels such as
`file.document` and `file.markdown` are not semantic editor types: they mean
the literal extensions `.document` and `.markdown`.

```yaml
Tables:
  - TableName: collaborative_note
    Columns:
      - Name: title
        DataType: varchar(200)
        ColumnType: label
      - Name: content
        DataType: text
        ColumnType: file.md|txt
```

Create a record through the normal resource API, then connect using its public
`reference_id`:

```text
ws://localhost:6336/live/collaborative_note/{reference_id}/content/yjs?token={jwt}
```

The equivalent canonical endpoint is:

```text
ws://localhost:6336/yjs/collaborative_note.{reference_id}.content?token={jwt}
```

The two URLs identify the same room. A canonical-looking room never falls back
to standalone storage.

Access follows the record's normal authorities:

- `CanUpdate` grants read/write collaboration.
- `CanRead` without `CanUpdate` grants a read-only connection; client edits are
  discarded.
- No read access, a missing record, an unknown table, or a non-file column is
  reported as not found.

Each accepted update runs through `DbResource.Update` with the authenticated
session user. Validation, ownership, row permissions, metering, file handling,
events, and optimistic resource versioning therefore remain the authorities.
An update is broadcast only after that resource update succeeds.

The file column remains the durable state. Daptin preserves ordinary assets in
the column and maintains exactly one reserved `x-crdt/yjs` asset containing the
raw base64-encoded YJS update history. That state stays inline in the resource
column even when ordinary assets use a cloud store. Malformed or duplicate
state prevents the WebSocket upgrade instead of creating another state path.

## JavaScript client

Editor choice is entirely client-side. Bind any YJS-compatible editor to the
shared type your application chooses.

```javascript
import * as Y from 'yjs'
import { WebsocketProvider } from 'y-websocket'

const doc = new Y.Doc()
const provider = new WebsocketProvider(
  `ws://localhost:6336/live/collaborative_note/${referenceId}/content`,
  'yjs',
  doc,
  { params: { token } }
)

const text = doc.getText('content')
text.observe(() => console.log(text.toString()))
text.insert(0, 'Collaborative text')
```

`WebsocketProvider` appends the room name, so the base URL above plus the room
`yjs` produces the generated endpoint.

## Standalone rooms

Use a non-canonical name when collaboration is not attached to a resource:

```text
ws://localhost:6336/yjs/{room_name}?token={jwt}
```

Standalone rooms are stored under the configured local storage path
(`DAPTIN_LOCAL_STORAGE_PATH` or `-local_storage_path`) in `yjs-documents`.
They are authenticated but have no resource record from which to derive
row-level permissions; authenticated users sharing a room name share that
room. Use an unguessable name when that is the intended boundary.

## Cluster and offline behavior

Connected nodes distribute document updates and awareness through Daptin's
Olric-backed YJS broadcaster. Durable truth remains the resource file column
for canonical rooms and the configured disk store for standalone rooms. YJS
clients can edit offline and exchange their missing updates after reconnecting.

See [Column Type Reference](Column-Type-Reference.md#file-type-patterns) for the file
column grammar and [Permissions](Permissions.md) for resource access rules.
