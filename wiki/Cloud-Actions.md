# Cloud Actions

Daptin exposes cloud operations as actions on `cloud_store` and `site`
resources. These routes enter the same action permission and resource context
as other Daptin operations.

## Operational boundaries

- Set `cloud_store.credential_name` to the credential row's exact `name`.
  Cloud-storage operations use this name as their sole credential selector and
  reject missing or unavailable remote-store credentials before invoking
  rclone.
- An HTTP 200 response from `delete_path` may mean that asynchronous rclone
  work was accepted, not that it completed. Verify the resulting object state
  with `list_files` or the provider API.
- The log line `rclone session exitcode - 1` is not sufficient by itself to
  classify an operation. Inspect the surrounding provider error and the actual
  object state.

The public action names below come from the action definitions. Names such as
`cloudstore.file.upload` are internal performers, not public routes.

## Cloud-store actions

All cloud-store actions are instance actions. Pass the store's public
`reference_id` as `cloud_store_id`; do not use an internal numeric ID.

### upload_file

```bash
curl -X POST http://localhost:6336/action/cloud_store/upload_file \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  --data-binary '{
    "attributes": {
      "cloud_store_id": "CLOUD_STORE_REFERENCE_ID",
      "path": "/uploads",
      "file": [{"name":"document.pdf","file":"data:application/pdf;base64,..."}]
    }
  }'
```

### delete_path

```bash
curl -X POST http://localhost:6336/action/cloud_store/delete_path \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  --data-binary '{"attributes":{"cloud_store_id":"CLOUD_STORE_REFERENCE_ID","path":"/uploads/old-file.pdf"}}'
```

The response is a queue/dispatch acknowledgement in v0.13.14. Verify that the
object is absent.

### create_folder

```bash
curl -X POST http://localhost:6336/action/cloud_store/create_folder \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  --data-binary '{"attributes":{"cloud_store_id":"CLOUD_STORE_REFERENCE_ID","path":"/uploads/2024","name":"january"}}'
```

### move_path

```bash
curl -X POST http://localhost:6336/action/cloud_store/move_path \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  --data-binary '{"attributes":{"cloud_store_id":"CLOUD_STORE_REFERENCE_ID","source":"/uploads/old-name.pdf","destination":"/archive/new-name.pdf"}}'
```

`move_path` completes before returning success. A failed or missing-source move
returns an error, and Daptin confirms the destination exists before reporting
success.

### create_site

```bash
curl -X POST http://localhost:6336/action/cloud_store/create_site \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  --data-binary '{"attributes":{"cloud_store_id":"CLOUD_STORE_REFERENCE_ID","site_type":"static","path":"static-website","hostname":"static.example.com"}}'
```

## Site actions

`get_file`, `list_files`, and `sync_site_storage` are instance actions on
`site`; pass `site_id` as a public reference ID.

```bash
curl -X POST http://localhost:6336/action/site/list_files \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  --data-binary '{"attributes":{"site_id":"SITE_REFERENCE_ID","path":"/assets"}}'

curl -X POST http://localhost:6336/action/site/get_file \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  --data-binary '{"attributes":{"site_id":"SITE_REFERENCE_ID","path":"/index.html"}}'

curl -X POST http://localhost:6336/action/site/sync_site_storage \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  --data-binary '{"attributes":{"site_id":"SITE_REFERENCE_ID"}}'
```

In v0.13.14 site sync copies the backing store to the site's temporary local
directory. It does not persist FTP edits back to the store; see [[FTP-Server]].

## Credential configuration

Credential content is an encrypted JSON string containing rclone-compatible
fields:

```bash
curl -sS -X POST http://localhost:6336/api/credential \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  --data-binary '{"data":{"type":"credential","attributes":{"name":"minio-creds","content":"{\"type\":\"s3\",\"provider\":\"Minio\",\"env_auth\":\"false\",\"access_key_id\":\"ACCESS_KEY\",\"secret_access_key\":\"SECRET_KEY\",\"endpoint\":\"http://minio:9000\",\"region\":\"us-east-1\"}"}}}'

curl -sS -X POST http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  --data-binary '{"data":{"type":"cloud_store","attributes":{"name":"minio-store","store_type":"s3","store_provider":"s3","credential_name":"minio-creds","root_path":"minio-store:bucket-name","store_parameters":"{}"}}}'
```

Create the bucket before invoking an action. For self-hosted HTTPS endpoints,
install the endpoint CA in Daptin's trust store; do not disable certificate
verification for production. Restart Daptin after adding or changing a cloud
store because runtime storage composition occurs at startup.

## Asset columns

`column.storage.sync` is an internal performer used by the resource lifecycle,
not a second public cloud API. Configure an asset column's
`ForeignKeyData.Namespace` and create/update rows through the resource API so
validation, ownership, permissions, audit, events, cache invalidation, and
metering stay on the canonical resource path.
