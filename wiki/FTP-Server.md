# FTP Server

Built-in FTP server for file management.

## Enable FTP

```bash
curl -X POST http://localhost:6336/_config/backend/ftp.enable \
  -H "Authorization: Bearer $TOKEN" \
  -d 'true'
```

## Configure Interface

```bash
curl -X POST http://localhost:6336/_config/backend/ftp.listen_interface \
  -H "Authorization: Bearer $TOKEN" \
  -d '"0.0.0.0:21"'
```

## Port

Default: **21** (standard FTP)

## Authentication

Use Daptin user credentials:
- Username: Email address
- Password: Daptin password

## Connecting

### Command Line

```bash
ftp localhost 21
# Enter username (email) and password
```

### FileZilla

1. Host: `localhost`
2. Port: `21`
3. Protocol: FTP
4. Username: Your Daptin email
5. Password: Your Daptin password

### WinSCP

1. File Protocol: FTP
2. Host name: `localhost`
3. Port: `21`
4. User name: Your Daptin email
5. Password: Your Daptin password

## FTP for Subsites

Enable FTP on specific sites:

```bash
curl -X PATCH http://localhost:6336/api/site/SITE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "site",
      "id": "SITE_ID",
      "attributes": {
        "ftp_enabled": true
      }
    }
  }'
```

## File Operations

Once connected via FTP:

| Command | Description |
|---------|-------------|
| `ls` | List files |
| `cd <dir>` | Change directory |
| `get <file>` | Download file |
| `put <file>` | Upload file |
| `mkdir <dir>` | Create directory |
| `rm <file>` | Delete file |
| `rmdir <dir>` | Delete directory |

## Storage Backend

FTP accesses the same storage as:
- Cloud storage (if configured)
- Local filesystem
- Subsites

## Permissions

FTP operations respect Daptin permissions:
- Users access their own files
- Admins access all files
- Site-specific access for subsites

## Security Considerations

1. **Use SFTP when possible** - FTP transmits passwords in cleartext
2. **Firewall rules** - Restrict FTP port access
3. **Strong passwords** - Use complex passwords
4. **TLS encryption** - Consider FTPS for sensitive data

## Passive Mode

Passive mode is supported for clients behind NAT/firewalls.

## Troubleshooting

### Cannot Connect

1. Check FTP is enabled
2. Verify port 21 is open
3. Check credentials

### Permission Denied

1. Verify user has access
2. Check file/folder permissions
3. Verify site FTP is enabled

### Timeout

1. Check firewall settings
2. Try passive mode
3. Verify server is running
