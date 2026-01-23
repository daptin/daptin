# RSS and Atom Feeds

Auto-generated feeds from any entity.

## Feed Endpoints

| Format | Endpoint | Content-Type |
|--------|----------|--------------|
| RSS 2.0 | `/feed/{entity}.rss` | application/rss+xml |
| Atom | `/feed/{entity}.atom` | application/atom+xml |
| JSON Feed | `/feed/{entity}.json` | application/json |

## Basic Usage

### RSS Feed

```bash
curl http://localhost:6336/feed/blog_post.rss
```

### Atom Feed

```bash
curl http://localhost:6336/feed/blog_post.atom
```

### JSON Feed

```bash
curl http://localhost:6336/feed/blog_post.json
```

## RSS Example Output

```xml
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>blog_post Feed</title>
    <link>http://localhost:6336/api/blog_post</link>
    <description>Feed for blog_post</description>
    <item>
      <title>My First Post</title>
      <link>http://localhost:6336/api/blog_post/abc123</link>
      <description>Post content here...</description>
      <pubDate>Mon, 15 Jan 2024 10:00:00 GMT</pubDate>
      <guid>abc123</guid>
    </item>
  </channel>
</rss>
```

## Atom Example Output

```xml
<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>blog_post Feed</title>
  <link href="http://localhost:6336/api/blog_post"/>
  <updated>2024-01-15T10:00:00Z</updated>
  <entry>
    <title>My First Post</title>
    <link href="http://localhost:6336/api/blog_post/abc123"/>
    <id>abc123</id>
    <updated>2024-01-15T10:00:00Z</updated>
    <summary>Post content here...</summary>
  </entry>
</feed>
```

## JSON Feed Example

```json
{
  "version": "https://jsonfeed.org/version/1",
  "title": "blog_post Feed",
  "home_page_url": "http://localhost:6336/api/blog_post",
  "feed_url": "http://localhost:6336/feed/blog_post.json",
  "items": [
    {
      "id": "abc123",
      "url": "http://localhost:6336/api/blog_post/abc123",
      "title": "My First Post",
      "content_text": "Post content here...",
      "date_published": "2024-01-15T10:00:00Z"
    }
  ]
}
```

## Feed Configuration

### Schema Definition

```yaml
Tables:
  - TableName: blog_post
    Columns:
      - Name: title
        DataType: varchar(500)
        ColumnType: label
        FeedField: title  # Maps to feed title

      - Name: content
        DataType: text
        ColumnType: content
        FeedField: description  # Maps to description

      - Name: published_at
        DataType: datetime
        ColumnType: datetime
        FeedField: pubDate  # Maps to publication date
```

## Field Mapping

Default field mapping:

| Feed Field | Column Priority |
|------------|-----------------|
| title | title, name, label |
| description | content, description, body |
| pubDate | published_at, created_at |
| link | URL to record |
| guid | reference_id |

## Filtering

Feed behavior is controlled by the `feed` table configuration. Page size is set per-feed in the database.

## Authentication

Public feeds (if entity allows guest read):

```bash
curl http://localhost:6336/feed/blog.rss
```

Authenticated feeds:

```bash
curl http://localhost:6336/feed/private.rss \
  -H "Authorization: Bearer $TOKEN"
```

## Feed Configuration

Feeds are configured via the `feed` table. The page size and other settings are defined there, not via URL parameters.

### View Feed Settings

```bash
curl http://localhost:6336/api/feed \
  -H "Authorization: Bearer $TOKEN"
```

### Update Feed

```bash
curl -X PATCH http://localhost:6336/api/feed/FEED_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "feed",
      "id": "FEED_ID",
      "attributes": {
        "page_size": 50
      }
    }
  }'
```

## Subscribe in Readers

### Feedly

1. Add new source
2. URL: `http://your-server:6336/feed/entity.rss`

### RSS Reader Apps

Most RSS readers support all three formats.

## Caching

Feeds are cached for performance:
- Default: 5 minutes
- Respects HTTP cache headers
