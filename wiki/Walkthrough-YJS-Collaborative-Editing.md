# Walkthrough: YJS collaborative editing

The maintained setup, schema, permission behavior, endpoint forms, and client
example are in [YJS Collaboration](YJS-Collaboration.md).

That guide is the canonical workflow. In particular, declare a `file.` column
such as `file.md|txt` or `file.*`; restricted suffixes enforce uploaded file
extensions and require declared MIME types. They do not select an editor.
Editor selection belongs to the YJS client.
