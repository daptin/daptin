# Manage site files with FTP

This guide explains how to let a site owner or a trusted team manage a Daptin site's files with an FTP application such as FileZilla or Cyberduck.

You do not need to understand the FTP protocol. You will create a storage location, create a site, decide who may use it, enable the FTP service, and connect an FTP application.

## Before you begin

You need:

- A Daptin administrator account.
- `daptin-cli` connected to your Daptin server.
- A domain or hostname for the site, such as `files.example.com`.
- Access to restart Daptin.
- An FTP application. FileZilla and Cyberduck are common choices.

FTP is disabled by default. Leave it disabled if nobody needs it.

!!! warning "Upgrade before enabling FTP"
    Daptin versions through `0.12.29` do not isolate FTP sites correctly. Do not expose FTP on those versions. Upgrade to a release containing the FTP authorization fix first.

## Understand the three parts

An FTP setup has three parts:

1. **Cloud store** — where the files ultimately belong. A local directory is the simplest option.
2. **Site** — gives that storage path a hostname and turns on FTP for it.
3. **Permissions** — decide who can see, read, add, change, or delete the site's files.

The FTP root is not a shared disk. Each visible folder represents one site. A user sees only sites for which they have permission.

## 1. Connect `daptin-cli`

Run these commands on an administrator's computer. Replace the address and credentials with your own:

```bash
daptin-cli context add production https://daptin.example.com
daptin-cli context set production
daptin-cli execute user_account signin \
  email=admin@example.com \
  password='your-password'
```

The context list should show `authenticated`:

```bash
daptin-cli context list
```

## 2. Create a storage location

For a local directory:

```bash
mkdir -p /srv/daptin/sites/my-site

daptin-cli storage add site-files \
  --type local \
  --store-provider local \
  --root-path /srv/daptin/sites
```

Get its ID:

```bash
STORE_ID=$(daptin-cli -q list cloud_store \
  --filter name=site-files \
  --page-size 1 | tail -1)
```

If your files belong in S3 or another provider, create that cloud store instead. The permission steps below stay the same.

## 3. Create the FTP-enabled site

```bash
daptin-cli create site \
  name='My site files' \
  hostname=files.example.com \
  path=my-site \
  enable=true \
  ftp_enabled=true \
  site_type=static \
  cloud_store_id="$STORE_ID"

SITE_ID=$(daptin-cli -q list site \
  --filter hostname=files.example.com \
  --page-size 1 | tail -1)
```

## 4. Choose who can use the site

FTP uses the site's ordinary Daptin permissions. There is no separate FTP user list.

The available abilities are:

| Ability | What it allows in FTP |
|---------|-----------------------|
| Peek | See the site and enter its folder |
| Read | List and download files |
| Create | Upload new files and create folders |
| Update | Replace, append, rename, chmod, or change timestamps |
| Delete | Delete files and folders |

### Owner-only access

For a site managed only by its owner, use owner permissions:

```bash
daptin-cli permission encode \
  +OwnerPeek +OwnerRead +OwnerCreate +OwnerUpdate +OwnerDelete
```

The command prints the permission value `3968`. Apply it:

```bash
daptin-cli update site "$SITE_ID" permission=3968
```

### Give a team read-only access

Relate the site to the team's Daptin usergroup. Then give that relationship `GroupPeek` and `GroupRead` permissions. Team members can see, enter, list, and download, but cannot upload or delete.

For a team that manages content, add only the additional group abilities they need:

- Add `GroupCreate` for new uploads and folders.
- Add `GroupUpdate` for overwrite and rename.
- Add `GroupDelete` only for people allowed to remove content.

Avoid guest permissions for private sites. `GuestPeek` makes the site name visible to every authenticated FTP user, while broader guest permissions expose file operations more widely.

## 5. Enable the FTP service

Set:

- `ftp.enable` to `true`.
- `ftp.listen_interface` to the desired bind address and port.

The default public bind is `0.0.0.0:2121`. For a private or same-machine service, prefer a private address such as `127.0.0.1:2121`. Restrict the port with your firewall or VPN.

Restart Daptin after enabling FTP. A restart is also required after changing `ftp_enabled`, site permissions, or site-usergroup relationships.

Confirm that the port is listening:

```bash
lsof -nP -iTCP:2121 -sTCP:LISTEN
```

## 6. Connect FileZilla or Cyberduck

Create a new connection with:

- **Protocol:** FTP with explicit TLS (FTPS), not SFTP.
- **Server:** your Daptin server address.
- **Port:** `2121`, unless you changed it.
- **Username:** the user's Daptin email address.
- **Password:** the user's Daptin password.

After login, the user sees the hostnames of the sites they may access. Open `files.example.com` to manage its files.

If the client warns about an unknown certificate, verify the certificate fingerprint before accepting it. Configure a trusted site certificate for production.

## Everyday tasks

- Drag a new file into the site to upload it. This requires **Create**.
- Drag an existing filename into the site to replace it. This requires **Update**.
- Download or preview a file. This requires **Read**.
- Rename a file. This requires **Update**.
- Delete a file. This requires **Delete**.

Daptin prevents FTP paths from leaving the selected site. Parent-directory traversal, symlink escape, cross-site rename, and deleting or renaming a site root are rejected.

## Troubleshooting

### Login works, but the root is empty

The account is valid but has no `Peek` permission on an FTP-enabled site. Check:

- The site has `ftp_enabled=true`.
- The user owns the site with `OwnerPeek`, or belongs to a related group with `GroupPeek`.
- Daptin was restarted after the permission change.

### The site is visible, but files cannot be listed

The user has `Peek` but not `Read`, or the site's initial storage synchronization has not completed. Check the site relationship permissions and server sync logs.

### Upload is denied

- A new filename requires `Create`.
- Replacing or appending to an existing filename requires `Update`.

### Delete is denied

The user needs `Delete`. Read or Update permission does not imply Delete.

### The server port is closed

Check that:

- `ftp.enable=true`.
- At least one site has `ftp_enabled=true`.
- Daptin was restarted.
- `ftp.listen_interface` uses the expected address and port.
- The firewall allows the control and passive data connections.

## Production checklist

- Upgrade past the affected FTP authorization releases.
- Use explicit TLS (FTPS) with a trusted certificate.
- Prefer a private bind address, VPN, or IP allowlist.
- Give each owner or group only the permissions it needs.
- Avoid guest permissions for private content.
- Disable public signup unless it is required by your application.
- Restart after site or permission changes and test with a non-owner account.
- Back up site storage and monitor FTP login and destructive-operation logs.

For configuration details and protocol examples, see the [FTP Server technical reference](https://github.com/daptin/daptin/blob/master/wiki/FTP-Server.md).
