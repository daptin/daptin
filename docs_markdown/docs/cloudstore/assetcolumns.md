Asset columns
===

Column types of blob types can either be stored in the database itself (not recommended) or persist in a persistent storage.

After we have created a [cloud store](cloudstore.md), we can point the column to a folder on the cloud store. The column will only contain metadata and the actual file will be persisted on the cloud store.

To enable this, Update the ForeignKey config of the column as follows:

