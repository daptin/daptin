# Cloud Actions

Daptin exposes cloud operations as actions on `cloud_store` and `site`
resources. These routes enter the same action permission and resource context
as other Daptin operations.

## v0.13.14 boundaries

- Set `cloud_store.credential_name` to the credential row's exact `name` **and**
  link the row through the `credential_id` relationship. Runtime provider
  lookup uses the name; the relationship is the persisted Daptin association
  and permission boundary. Omitting the name can cause HTTP 500 instead of a
  configuration validation error.
- An HTTP 200 action response may mean that asynchronous rclone work was
  accepted, not that it completed. In particular, `move_path` and
  `delete_path` can report success when the provider later reports a missing
  object. Verify the resulting object state with `list_files` or the provider
  API.
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

Verify both that the destination exists and that the source no longer exists.

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
CRED_ID=$(curl -sS -X POST http://localhost:6336/api/credential \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  --data-binary '{"data":{"type":"credential","attributes":{"name":"minio-creds","content":"{\"type\":\"s3\",\"provider\":\"Minio\",\"env_auth\":\"false\",\"access_key_id\":\"ACCESS_KEY\",\"secret_access_key\":\"SECRET_KEY\",\"endpoint\":\"http://minio:9000\",\"region\":\"us-east-1\"}"}}}' | jq -r '.data.id')

STORE_ID=$(curl -sS -X POST http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  --data-binary '{"data":{"type":"cloud_store","attributes":{"name":"minio-store","store_type":"s3","store_provider":"s3","credential_name":"minio-creds","root_path":"minio-store:bucket-name","store_parameters":"{}"}}}' | jq -r '.data.id')

curl -X PATCH "http://localhost:6336/api/cloud_store/$STORE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  --data-binary '{"data":{"type":"cloud_store","id":"'"$STORE_ID"'","relationships":{"credential_id":{"data":{"type":"credential","id":"'"$CRED_ID"'"}}}}}'
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
