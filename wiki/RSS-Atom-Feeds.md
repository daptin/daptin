# RSS, Atom, and JSON Feeds

A feed URL is not generated from an entity name alone. Daptin requires a
`stream` resource, a `feed` resource related to that stream, readable source
rows, and a restart so the startup-composed feed/stream maps include them.
Guessing `/feed/{entity}.rss` without this setup returns 404.

## Route

The route uses `feed.feed_name`, not the source table name:

| Format | Route |
|---|---|
| RSS 2.0 | `/feed/{feed_name}.rss` |
| Atom | `/feed/{feed_name}.atom` |
| JSON Feed | `/feed/{feed_name}.json` |

## Complete example

This example assumes an `article` resource with `title`, `link`,
`description`, `author_name`, and `author_email` columns. Feed rendering also
uses the standard `created_at` column. Grant the requesting guest/user read
permission on both the `article` table and rows; the stream reuses the normal
resource read path and does not bypass permissions.

Create sample source data through JSON:API:

```bash
curl -X POST http://localhost:6336/api/article \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  --data-binary '{
    "data": {
      "type": "article",
      "attributes": {
        "title": "Daptin feed example",
        "link": "https://example.test/articles/feed-example",
        "description": "A source row rendered by a Daptin stream.",
        "author_name": "Example Publisher",
        "author_email": "publisher@example.test"
      }
    }
  }'
```

Create a stream. `stream_contract` is a JSON string; `StreamName` must equal
the row's `stream_name`, and `RootEntityName` names the resource authority used
for reads:

```bash
STREAM_ID=$(curl -sS -X POST http://localhost:6336/api/stream \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  --data-binary '{
    "data": {
      "type": "stream",
      "attributes": {
        "stream_name": "article_feed_stream",
        "enable": true,
        "stream_contract": "{\"StreamName\":\"article_feed_stream\",\"RootEntityName\":\"article\",\"Columns\":[{\"Name\":\"title\",\"ColumnName\":\"title\"},{\"Name\":\"link\",\"ColumnName\":\"link\"},{\"Name\":\"description\",\"ColumnName\":\"description\"},{\"Name\":\"author_name\",\"ColumnName\":\"author_name\"},{\"Name\":\"author_email\",\"ColumnName\":\"author_email\"},{\"Name\":\"created_at\",\"ColumnName\":\"created_at\"}],\"QueryParams\":{}}"
      }
    }
  }' | jq -r '.data.id')
```

Create and relate the feed:

```bash
FEED_ID=$(curl -sS -X POST http://localhost:6336/api/feed \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  --data-binary '{
    "data": {
      "type": "feed",
      "attributes": {
        "feed_name": "articles",
        "title": "Example Articles",
        "description": "Recent example articles",
        "link": "https://example.test/articles",
        "author_name": "Example Publisher",
        "author_email": "publisher@example.test",
        "enable": true,
        "enable_rss": true,
        "enable_atom": true,
        "enable_json": true,
        "page_size": 50
      },
      "relationships": {
        "stream_id": {
          "data": {"type":"stream","id":"'"$STREAM_ID"'"}
        }
      }
    }
  }' | jq -r '.data.id')
```

Restart Daptin with its process supervisor, then verify all formats:

```bash
curl -fsS -H "Authorization: Bearer $TOKEN" http://localhost:6336/feed/articles.rss  -o articles.rss
curl -fsS -H "Authorization: Bearer $TOKEN" http://localhost:6336/feed/articles.atom -o articles.atom
curl -fsS -H "Authorization: Bearer $TOKEN" http://localhost:6336/feed/articles.json | jq .
```

You can omit the bearer token only if the source rows are guest-readable. A
feed being enabled does not grant access to source rows.

## Failure checks

- **404:** confirm `feed_name`, `enable`, the requested format's `enable_rss`,
  `enable_atom`, or `enable_json` field, the `stream_id` relationship, and that
  both rows existed before restart. Other extensions are not supported.
- **404 after creating rows:** restart; feed and stream maps are composed at
  startup.
- **JSON error while rendering:** confirm `page_size` is positive and the stream
  returns `title`, `link`, `description`, `author_name`, `author_email`, and
  `created_at` for each item.
- **Empty feed:** test the same identity against the source resource and check
  both table-level and row-level read permissions.

The `enable_rss`, `enable_atom`, and `enable_json` fields control which feed
formats are available. A disabled format returns 404.
