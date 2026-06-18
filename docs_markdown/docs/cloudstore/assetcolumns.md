Asset columns
===

Blob-like columns can either be stored in the database column or persisted in a
configured [cloud store](cloudstore.md). When a column is backed by a
`cloud_store`, the table stores file metadata and the file body is stored in
the configured provider.

To enable cloud storage for a column, set `IsForeignKey: true` and point
`ForeignKeyData` at the `cloud_store` record:

```yaml
Tables:
  - TableName: <table_name>
    Columns:
      - Name: <column_name>
        ColumnName: <column_name>
        DataType: text
        ColumnType: file
        IsForeignKey: true
        ForeignKeyData:
          DataSource: cloud_store
          Namespace: <cloud_store_name>
          KeyName: <folder_or_prefix>
```

`Namespace` must match the `name` field on the `cloud_store` row. `KeyName` is
the folder or prefix under that cloud store.

File uploads use an array of file objects:

```json
{
  "attachment": [
    {
      "name": "report.pdf",
      "file": "data:application/pdf;base64,...",
      "type": "application/pdf"
    }
  ]
}
```

Cloud-backed files can be served over HTTP:

```text
/asset/<table_name>/<reference_id>/<column_name>.<extension>
```

The extension can be any value relevant to the file MIME type.

## Built-in mail columns

The built-in `mail.mail` and `outbox.mail` columns use the same cloud-store
configuration path. There is no separate SMTP or IMAP storage config key.

```yaml
Tables:
  - TableName: mail
    Columns:
      - Name: mail
        ColumnName: mail
        DataType: blob
        ColumnType: gzip
        IsForeignKey: true
        ForeignKeyData:
          DataSource: cloud_store
          Namespace: mail-storage
          KeyName: mail-messages

  - TableName: outbox
    Columns:
      - Name: mail
        ColumnName: mail
        DataType: blob
        ColumnType: gzip
        IsForeignKey: true
        ForeignKeyData:
          DataSource: cloud_store
          Namespace: mail-storage
          KeyName: outbox-messages
```

SMTP delivery, IMAP `FETCH`, `COPY`, `APPEND`, and outbox processing read and
write the raw RFC 822 message body through these columns. Mail metadata stays
in the SQL tables. API reads that need the message body should use:

```text
GET /api/mail/<id>?included_relations=mail
```
