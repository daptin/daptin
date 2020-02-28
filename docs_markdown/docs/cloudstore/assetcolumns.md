Asset columns
===

Column types of blob types can either be stored in the database itself (not recommended) or persist in a persistent storage.

After we have created a [cloud store](cloudstore.md), we can point the column to a folder on the cloud store. The column will only contain metadata and the actual file will be persisted on the cloud store.

To enable this, update the ForeignKeyData config of the column as follows:

Create a file for the schema change:

```add_column_storage.yaml
Tables:
  - TableName: <TableNameHere>
  - Columns:
    - ColumnName: <ColumnNameHere>
      ForeignKeyData:
        DataSource: "cloud"
        KeyName: <Cloud store name here>
        Namespace: <Folder name inside that clouds store>
```


Upload it using the dashboard (You can alternatively just edit that from the dashboard). This will trigger a reconfiguration of the system and initiate a local sync of the cloud directory in a temporary location. The cloud directory will be synced down stream every 15 minutes while the uploads will be asynced but instantaneous.


Such columns like image./video./audio./markdown. will be served over HTTP in a simple GET call:

/asset/&lt;table_name&gt;/&lt;reference_id&gt;/&lt;column_name&gt;.&lt;extension&gt;

&lt;extension&gt; can be anything relevant to the mimetype of the file. The column file will be dumped as it is. Useful for using in `img` html tag.