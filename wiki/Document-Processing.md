# Document Processing

Daptin actions can extract and convert documents using the embedded
[Manifold](https://github.com/artpar/manifold) Go library. Application builders
choose the action inputs, permissions, resource reads, persistence and responses.
End users invoke those named actions through the usual REST or GraphQL API.
Scheduled tasks invoke the same actions.

## Performers

| Outcome type | Required attributes | Result available through `Reference` |
| --- | --- | --- |
| `document.extract` | `source_format` and `file` or `document` | `document` (versioned SF tree), `text`, `metadata`, `diagnostics` |
| `document.convert` | `source_format`, `target_format` and `file` or `document` | `file` (one attachment), `diagnostics` |

Use `Method: EXECUTE`. Formats are exactly `docx`, `pdf`, `sf` and `txt`.
SF is Manifold's `manydoc-prosemirror/1` ProseMirror profile, not arbitrary
ProseMirror JSON, HTML or Markdown. Unknown nodes and unsupported features reject.
`document` accepts an SF object with `source_format: sf`; it cannot be combined
with `file`. This lets later outcomes edit or convert extracted structure.

`file` contains exactly one attachment with canonical standard base64 `contents`:

```json
{
  "source_format": "txt",
  "target_format": "pdf",
  "file": [{"name": "hello.txt", "contents": "SGVsbG8="}]
}
```

GraphQL represents a `json` action argument as a string: JSON-encode the same
attachment array for that argument. Both representations enter the same
performer validation path.

Names and paths do not select a file to read. These performers never fetch a URL,
open a caller-selected path, resolve credentials or write storage. They consume
inline bytes. Output attachments have `name`, `type`, `size` and base64 `contents`,
so subsequent resource writes can use them as normal file values. To download:

```yaml
OutFields:
  - Type: document.convert
    Method: EXECUTE
    Reference: converted
    Attributes:
      source_format: ~source_format
      target_format: pdf
      file: ~file
  - Type: client.file.download
    Method: ACTIONRESPONSE
    Attributes:
      content: "!converted.file[0].contents"
      name: "!converted.file[0].name"
      contentType: "!converted.file[0].type"
      diagnostics: ~converted.diagnostics
```

The performer's result is available to later outcomes; add an `ACTIONRESPONSE`
outcome to return selected fields to the caller. See [[Actions-Overview]].

## Run the example

Load [schema_document_processing.yaml](../examples/document-processing/schema_document_processing.yaml)
through your normal schema-folder configuration (`DAPTIN_SCHEMA_FOLDER`). It
defines an ordinary `conversion_document` resource with private owner permissions
and three actions. Signed-in members of `users` can create records and invoke
the actions; other users cannot read or execute operations on private records.

Extract an inline file:

```bash
curl "$DAPTIN_URL/action/conversion_document/extract_document" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"attributes":{"source_format":"txt","file":[{"name":"hello.txt","contents":"SGVsbG8="}]}}'
```

Use `convert_document` with the same attributes plus `target_format: pdf` to
download a PDF. Its conversion defaults to strict fidelity.

For persistence, create a `conversion_document` through `/api/conversion_document`
with `title`, `source_format`, and `source_base64`. Then invoke `save_pdf` with
`attributes.conversion_document_id` set to the record's public reference ID.
It reads the authorized action subject, extracts SF, converts it, updates
`output_base64` through resource CRUD and returns a download. This example stores
bytes on the resource; it does not require an external storage provider.
The example meters `save_pdf` once through the existing action metering service.
Its usage belongs to the invoking account; plan limits apply when configured.

For cloud-backed assets, first use the existing authorized resource/asset API to
obtain the bytes, then submit the inline attachment. Do not build an action that
reads an arbitrary caller-selected resource using temporary action privileges.
Permission to execute an action is separate from document read/write access.
When a workflow requires an intentional change of account, use backend-defined
`SWITCH_USER` before the relevant operations.

## Fidelity and limits

Extraction reports import diagnostics without hiding them. DOCX/PDF conversion
rejects losses with HTTP 422 through the usual action error response. Run
extraction to review diagnostics before conversion. After reviewing them, an action may pass the
boolean `allow_lossy: true` to `document.convert`. It defaults to false and does
not authorize unsupported output. The example actions keep that strict default.

PDF input reconstructs recoverable structure; it does not perform OCR. PDF/DOCX
output reflows content and does not promise source-identical pagination. PDF
output uses Manifold's built-in fonts. Missing glyphs and unsupported scripts
reject rather than silently substituting fonts. Legacy DOC, HTML, Markdown,
compressed SF and VDOCS are not exposed by these performers.

PDF input optionally accepts a string `password`. Keep passwords in transient
inputs and disable request-body capture for actions handling secrets. Do not
enable `LogToConsole` on document outcomes containing sensitive content.

Backend limits are fixed: 8 MiB decoded input, 32 MiB expanded DOCX members,
2,048 DOCX members, 16 MiB serialized performer result and 30 seconds per operation.
Two document operations may run concurrently per Daptin runtime; saturation
returns HTTP 503 immediately. Oversized inputs/results return HTTP 413;
cancellation/deadline returns HTTP 408. No partial output is returned on failure.
Base64 and repeated representations count towards the result limit, so the
maximum output file is smaller than 16 MiB.

The HTTP request budget is independent: the default
`http.max_buffered_request_bytes` is 10 MiB, including base64 and JSON overhead.
An 8 MiB file exceeds that default wire budget. To admit files up to the decoded
performer limit, an administrator can set this existing backend setting to
16 MiB (`16777216`) and restart Daptin. GraphQL also applies its separate
`graphql.max_request_bytes` budget. Raising a wire budget does not raise the
performer's decoded, archive-expansion or result limits.

These bounds and cooperative cancellation are not process-level memory isolation
for arbitrary hostile PDFs. Manifold also applies its own parser guards. Size
limits are operational protection, not durable customer quotas.

Configure action metering and plans using [[API-Metering]] for usage limits.
There is no separate document counter or billing store. A later `$llm.chat`
outcome retains its own model permission and metering checks. Extraction can
supply context for chat or summarization; conversation storage, retrieval and
context-window management belong to the application workflow.
