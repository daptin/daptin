# Template Rendering

Dynamic content generation in Daptin using database-backed Go HTML templates.

Templates can be used in two ways:

1. As an internal action outcome with `template.render`.
2. As HTTP routes through the `template.url_pattern` field.

Both paths use the same `template` table and the same renderer, but they differ in how input is supplied, how output is returned, and how routing/cache configuration is applied.

## Core Concepts

- Templates are rows in the `template` table.
- Template content is rendered with Go `html/template`.
- Daptin adds the [soha](https://github.com/flysnow-org/soha) template function map.
- `template.render` is an internal performer, not a direct REST action endpoint.
- Rendered content is returned as base64 when used inside an action chain.
- Routed templates decode the rendered base64 content before writing the HTTP response.
- `url_pattern`, `action_config`, and `cache_config` are used only for routed templates.

## Template Table

| Field | Required | Used by | Description |
|-------|----------|---------|-------------|
| `name` | Yes | Actions and routes | Unique template name. `template.render` looks up templates by this value. |
| `content` | Yes | Actions and routes | Template source, base64-encoded source, or a file reference such as `subsite://...`. |
| `mime_type` | Yes | Actions and routes | HTTP content type for rendered output, for example `text/html`, `text/plain`, or `application/json`. |
| `url_pattern` | Yes | Routes | JSON array of Gin route patterns. Use `[]` for templates used only from actions. |
| `headers` | No | Actions and routes | JSON object of response headers, for example `{"X-Frame-Options":"DENY"}`. |
| `action_config` | No | Routes | JSON action request run before the template is rendered. |
| `cache_config` | No | Routes | JSON cache policy for routed template responses. |

## Creating a Template

Use `/api/template` like any other JSON:API resource. The `url_pattern` field is required by the table schema.

For action-only templates, use an empty JSON array:

```bash
curl -X POST http://localhost:6336/api/template \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "template",
      "attributes": {
        "name": "welcome_email",
        "content": "<h1>Hello {{.name}}</h1><p>Welcome to {{.service}}.</p>",
        "mime_type": "text/html",
        "url_pattern": "[]",
        "headers": "{}",
        "action_config": "{}",
        "cache_config": "{}"
      }
    }
  }'
```

For routed templates, `url_pattern` must be a JSON array of path patterns:

```json
{
  "name": "public_profile",
  "content": "<h1>{{.username}}</h1>",
  "mime_type": "text/html",
  "url_pattern": "[\"/profiles/:username\"]"
}
```

Do not use `{}` for `url_pattern`. The route loader parses this field as `[]string`; `{}` fails route registration. Use `[]` when the template should not register any HTTP route.

## Content Sources

The `content` field supports these sources.

### Direct Content

```json
{
  "content": "Hello {{.name}}, your order {{.order_id}} is ready."
}
```

### Base64-Encoded Content

If `content` is valid standard base64, Daptin decodes it before parsing it as a template.

```json
{
  "content": "SGVsbG8ge3submFtZX19"
}
```

This detection is automatic. Avoid storing short plain strings that accidentally form valid base64 if you expect them to render literally.

### Subsite File Reference

```json
{
  "content": "subsite://<site_reference_id>/templates/welcome.html"
}
```

Daptin resolves the site reference id to the site's local synced folder and loads the file on render.

### Site File Reference

```json
{
  "content": "site://<site_reference_id>/emails/notification.html"
}
```

`site://` uses the same site/subsite file cache lookup path as `subsite://`.

## Template Syntax

Daptin uses Go `html/template`, so values are HTML-escaped according to context.

### Variables

```go
Hello {{.name}}
Order: {{.order_id}}
```

### Conditions

```go
{{if .is_premium}}
  <p>Premium member</p>
{{else}}
  <p>Standard member</p>
{{end}}
```

### Loops

```go
<ul>
{{range .items}}
  <li>{{.name}}: {{.price}}</li>
{{end}}
</ul>
```

### Map Keys and Query Arrays

Some input names are not valid dot-notation identifiers, such as `tag[]`. Use the built-in `index` function:

```go
{{range index . "tag[]"}}
  <span>{{.}}</span>
{{end}}
```

### soha Functions

The renderer registers soha template functions:

```go
{{.name | upper}}
{{.name | lower}}
{{.created_at | date "2006-01-02"}}
{{.trusted_html | safe}}
```

Use `safe` only for content you trust. User-supplied HTML should normally be escaped by `html/template`.

## Using Templates in Custom Actions

`template.render` is an internal performer. It is not exposed as `/action/world/template.render` or `/action/<entity>/template.render`.

Use it inside a custom action `OutFields` entry:

```yaml
Actions:
  - Name: render_welcome_email
    OnType: user_account
    InstanceOptional: true
    InFields:
      - Name: name
        ColumnName: name
        ColumnType: label
      - Name: service
        ColumnName: service
        ColumnType: label
    OutFields:
      - Type: template.render
        Method: EXECUTE
        Reference: rendered
        SkipInResponse: true
        Attributes:
          template: welcome_email
          name: "~name"
          service: "~service"
```

The `template` attribute is the template row's `name`. Every other attribute is passed to the template as data.

Inside the template above, use:

```go
Hello {{.name}}
Welcome to {{.service}}
```

### Rendered Action Output

`template.render` returns a render model with these attributes:

```json
{
  "content": "BASE64_ENCODED_RENDERED_OUTPUT",
  "mime_type": "text/html",
  "headers": {}
}
```

When you set `Reference: rendered`, later outcomes can use:

- `$rendered.content`
- `$rendered.mime_type`
- `$rendered.headers`

The `content` value is base64. Decode it when passing rendered text to performers that expect plain text:

```yaml
body: "!atob(rendered.content)"
```

The `atob` helper is available in action JavaScript expressions.

### Sending a Templated Email

Create a template:

```bash
curl -X POST http://localhost:6336/api/template \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "template",
      "attributes": {
        "name": "order_confirmation_email",
        "content": "<h1>Order confirmed</h1><p>Hi {{.customer_name}}, order {{.order_id}} is confirmed.</p>",
        "mime_type": "text/html",
        "url_pattern": "[]",
        "headers": "{}"
      }
    }
  }'
```

Use it from a custom action:

```yaml
Actions:
  - Name: send_order_confirmation
    OnType: order
    InstanceOptional: false
    InFields:
      - Name: customer_name
        ColumnName: customer_name
        ColumnType: label
      - Name: customer_email
        ColumnName: customer_email
        ColumnType: email
    OutFields:
      - Type: template.render
        Method: EXECUTE
        Reference: rendered_email
        SkipInResponse: true
        Attributes:
          template: order_confirmation_email
          customer_name: "~customer_name"
          order_id: "$subject.reference_id"

      - Type: mail.send
        Method: EXECUTE
        Attributes:
          from: "orders@example.com"
          to: "~customer_email"
          subject: "Order confirmation"
          body: "!atob(rendered_email.content)"
          mail_server_hostname: "mail.example.com"
          send_immediately: true
```

`mail.send` reads `body` as a plain string. Passing `$rendered_email.content` directly sends the base64 string, not the rendered HTML.

### Returning Rendered Content From an Action Endpoint

If an action endpoint should return the rendered template body directly, create a `render` response with `response.create`:

```yaml
Actions:
  - Name: preview_invoice
    OnType: invoice
    InstanceOptional: false
    OutFields:
      - Type: template.render
        Method: EXECUTE
        Reference: rendered_invoice
        SkipInResponse: true
        Attributes:
          template: invoice_html
          invoice_id: "$subject.reference_id"
          customer_name: "$subject.customer_name"
          total: "$subject.total"

      - Type: response.create
        Method: EXECUTE
        Attributes:
          response_type: render
          content: "$rendered_invoice.content"
          mime_type: "$rendered_invoice.mime_type"
          headers: "$rendered_invoice.headers"
```

Daptin's action HTTP handler recognizes response type `render`, decodes the base64 `content`, applies `mime_type` and `headers`, and writes the rendered body as the HTTP response.

## Routed Templates

A template row can register HTTP routes with `url_pattern`.

```json
{
  "name": "profile_page",
  "content": "<h1>{{.username}}</h1><p>Tab: {{.tab}}</p>",
  "mime_type": "text/html",
  "url_pattern": "[\"/profiles/:username\", \"/u/:username\"]"
}
```

Daptin registers each pattern with `router.Any`, so the same template can answer GET, POST, PUT, PATCH, DELETE, and other methods. Design route paths carefully and avoid conflicts with Daptin's built-in endpoints such as `/api`, `/action`, `/openapi.yaml`, `/graphql`, `/live`, `/asset`, and existing subsite routes.

### Route Parameters

Gin path parameters become template variables:

| URL pattern | Request | Template variable |
|-------------|---------|-------------------|
| `/profiles/:username` | `/profiles/alice` | `{{.username}}` = `alice` |
| `/docs/:section/:slug` | `/docs/guides/start` | `{{.section}}` = `guides`, `{{.slug}}` = `start` |

Gin wildcard patterns also work:

```json
{
  "url_pattern": "[\"/docs/*path\"]"
}
```

Use `{{.path}}` in the template.

### Query Parameters

Query parameters are added as top-level template variables.

For `/search?q=daptin`:

```go
Search: {{.q}}
```

For repeated query parameters such as `/search?tag=go&tag=api`, Daptin sets both `tag` and `tag[]` to the value array. Use `index` for the `tag[]` key:

```go
{{range index . "tag[]"}}
  <span>{{.}}</span>
{{end}}
```

### Route Lifecycle

Template routes are loaded when Daptin creates template hooks during server startup.

- Changing `content`, `mime_type`, or `headers` is picked up by `template.render` on the next render because the renderer looks up the template row by `name`.
- Changing `url_pattern`, `action_config`, or `cache_config` requires a restart or system reload so routes and route configuration are rebuilt.
- If in-memory caching is enabled, cached rendered responses may continue to be served until cache expiry or process restart.

## action_config for Routed Templates

`action_config` lets a routed template run one Daptin action before rendering.

The field stores an `ActionRequest` JSON object:

```json
{
  "Type": "product",
  "Action": "load_public_product"
}
```

When a routed template receives a request:

1. Daptin builds route input from path and query parameters.
2. Daptin overwrites `action_config.Attributes` with that route input.
3. Daptin runs the configured action.
4. Daptin adds the action responses to template input.
5. Daptin renders the template.

Because step 2 overwrites `Attributes`, keep route-specific values in path/query parameters or compute them inside the action. Do not rely on static `Attributes` stored in `action_config`.

Example template row:

```json
{
  "name": "product_page",
  "content": "<h1>{{.product.name}}</h1><p>{{.product.description}}</p>",
  "mime_type": "text/html",
  "url_pattern": "[\"/products/:slug\"]",
  "action_config": "{\"Type\":\"product\",\"Action\":\"load_public_product\"}"
}
```

If the action returns an action response with response type `product`, that response's attributes are available as `.product`.

The full list of action responses is also available as `.actionResponses`.

If multiple action responses use the same response type, the last one wins for the top-level `.response_type` variable. Use `.actionResponses` when you need all responses.

## cache_config for Routed Templates

`cache_config` applies only to routed templates. It does not change how `template.render` behaves inside a custom action unless that action returns a routed/render response through separate logic.

Default behavior:

```json
{}
```

Caching is disabled unless `enable` is true.

### Common Cache Configurations

No caching for sensitive or per-user content:

```json
{
  "enable": true,
  "no_store": true,
  "private": true,
  "etag_strategy": "none",
  "enable_in_memory_cache": false
}
```

Short browser cache for dynamic public content:

```json
{
  "enable": true,
  "max_age": 60,
  "revalidate": true,
  "etag_strategy": "weak",
  "vary_by_query_params": ["page", "sort"],
  "enable_in_memory_cache": false
}
```

Server-side in-memory cache for public pages:

```json
{
  "enable": true,
  "max_age": 300,
  "revalidate": true,
  "etag_strategy": "strong",
  "cache_key_prefix": "product-pages-v1",
  "vary_by_query_params": ["currency"],
  "vary_by_headers": ["Accept-Language"],
  "enable_in_memory_cache": true
}
```

### cache_config Fields

| Field | Type | Effect |
|-------|------|--------|
| `enable` | bool | Enables route cache handling. Required for all other cache behavior. |
| `max_age` | int | Adds `max-age=<seconds>` and an `Expires` header. |
| `revalidate` | bool | Adds `must-revalidate`. Defaults to true when config is parsed. |
| `no_cache` | bool | Adds `no-cache`; disables validator shortcut checks. |
| `no_store` | bool | Adds `no-store` and returns before other cache directives. |
| `private` | bool | Adds `private`; otherwise Daptin adds `public`. |
| `vary_by_headers` | string array | Adds a `Vary` header and includes those header values in the in-memory cache key. |
| `vary_by_query_params` | string array | Adds `X-Vary-By-Query-Params` and includes those query values in the in-memory cache key. |
| `vary_by_path` | bool | Parsed, but the current cache key always starts with the request path. |
| `stale_while_revalidate` | int | Adds `stale-while-revalidate=<seconds>` to `Cache-Control`. |
| `custom_headers` | object | Adds custom headers during cache header application. |
| `expires_at` | RFC3339 time | Parsed into an absolute `Expires` value when provided. |
| `etag_strategy` | string | `weak`, `strong`, or `none`. Defaults to `weak`. |
| `cache_key_prefix` | string | Prefix for in-memory cache keys. Useful for deployment/version invalidation. |
| `enable_in_memory_cache` | bool | Enables server-side cached rendered responses. |
| `in_memory_cache_ttl` | int | Parsed with default 300 seconds; current route storage uses the file cache expiry calculation. |
| `in_memory_cache_max_size` | int | Parsed with default 100; enforcement depends on the underlying cache implementation. |
| `in_memory_cache_strategy` | string | Parsed as `lru` or `lfu`; enforcement depends on the underlying cache implementation. |
| `in_memory_cache_compression` | bool | Parsed; route cache compression is currently decided from MIME type and content size. |

Important cache notes:

- `X-Cache: MISS` is sent when a routed template is freshly rendered.
- `X-Cache: HIT` is sent when Daptin serves the in-memory cached response.
- In-memory cache keys always include request path.
- Configure `vary_by_query_params` and `vary_by_headers` for anything that changes rendered output.
- If `etag_strategy` is not `none`, Daptin generates an ETag from rendered content for fresh responses.
- The current validator shortcut treats a present `If-None-Match` header as cache-valid before re-rendering. Use `etag_strategy: "none"` if that behavior is not appropriate for a route.

## Headers and MIME Types

`mime_type` controls `Content-Type`:

```json
{
  "mime_type": "application/json"
}
```

`headers` must be a JSON object string:

```json
{
  "headers": "{\"X-Frame-Options\":\"DENY\",\"Cache-Control\":\"no-store\"}"
}
```

For routed templates, `cache_config` may also set cache-related headers. If both `headers` and `cache_config` set the same header, the later header assignment can replace earlier values.

## JSON Responses

Templates can render JSON. Set the MIME type and make sure the template produces valid JSON:

```json
{
  "name": "product_json",
  "content": "{\"slug\":\"{{.slug}}\",\"name\":\"{{.product.name}}\"}",
  "mime_type": "application/json",
  "url_pattern": "[\"/public-api/products/:slug\"]"
}
```

Use `html/template` escaping rules carefully in JSON templates. For complex JSON responses, consider generating structured JSON in an action and returning it directly instead of manually composing JSON text.

## Security and Access Control

- Creating and editing template rows is controlled by normal table permissions.
- A routed template itself is an HTTP route and does not automatically require authentication.
- If a routed template exposes private data, enforce access through the configured action, route design, or surrounding authentication middleware.
- Do not use `safe` with untrusted user input.
- Avoid route patterns that overlap built-in Daptin endpoints.
- Treat template content as trusted code. A user who can edit templates can affect rendered output and headers.
- Be careful with `action_config`: it can trigger actions for every route request.

## Operational Checklist

1. Create the template row.
2. Use `url_pattern: "[]"` for action-only templates.
3. Use `url_pattern: "[\"/path/:param\"]"` for routed templates.
4. Set the correct `mime_type`.
5. Set `headers` to `{}` or a JSON object string.
6. For action chains, decode `$reference.content` with `!atob(reference.content)` when plain text is needed.
7. For direct HTTP action output, create a `response.create` response with `response_type: render`.
8. Restart or reload Daptin after changing `url_pattern`, `action_config`, or `cache_config`.
9. Configure `vary_by_query_params` and `vary_by_headers` before enabling in-memory cache.
10. Test the route or custom action with representative path/query inputs.

## Troubleshooting

### Template creation fails with NOT NULL on url_pattern

Use a JSON array string:

```json
{
  "url_pattern": "[]"
}
```

Use a non-empty array to register routes:

```json
{
  "url_pattern": "[\"/hello/:name\"]"
}
```

### Route does not appear after creating or editing a template

Restart or reload Daptin. Route patterns are registered when template hooks are created during startup.

### Route logs show failed to parse url pattern as string array

`url_pattern` is not a JSON array. Replace `{}` with `[]` or `["/route"]`.

### Template variables are empty

Check the input source:

- For action templates, variables come from `Attributes`.
- For routed templates, variables come from path/query parameters and optional `action_config` responses.
- Use `{{.name}}`, not `{{name}}`.
- For keys such as `tag[]`, use `{{index . "tag[]"}}`.

### Email contains base64 text

Decode rendered content before sending:

```yaml
body: "!atob(rendered_email.content)"
```

### Direct action response returns JSON instead of rendered HTML

`template.render` alone stores the render result for references. Add `response.create` with `response_type: render`.

### Cached route returns stale content

Disable in-memory cache, change `cache_key_prefix`, wait for expiry, or restart Daptin.

### Headers are missing or invalid

`headers` must be a JSON object string. Empty string and `{}` are accepted patterns; malformed JSON causes render errors.

## Related

- [[Custom-Actions|Custom Actions]] - Defining actions that use `template.render`
- [[Email-Actions|Email Actions]] - Sending templated mail through `mail.send`
- [[Subsites|Subsites]] - Site/subsite storage used by `subsite://` and `site://`
- [[Action-Reference|Action Reference]] - Internal performers and built-in actions
