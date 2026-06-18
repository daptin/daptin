# SMTP


Checkout the classic dashboard ui for daptin

```docker run -p 8080:8080 daptin/dashboard-classic```

Create a mail server entry with a hostname

```bash
curl 'http://localhost:8080/api/mail_server' -X POST \
  -H 'Content-Type: application/vnd.api+json' \
  -H 'Authorization: Bearer <TOKEN>' \
  --data-raw '{"data":{"type":"mail_server","attributes":{"always_on_tls":true,"authentication_required":true,"hostname":"mail.example.com","is_enabled":true,"listen_interface":"0.0.0.0:465","max_clients":"20","max_size":"10000","permission":0,"xclient_on":false}},"meta":{}}'
```

Create a new mail account

```bash
curl 'http://localhost:8080/api/mail_account' -X POST \
  -H 'Content-Type: application/vnd.api+json' \
  -H 'Authorization: Bearer <TOKEN>' \
  --data-raw '{"data":{"type":"mail_account","attributes":{"password":"password","password_md5":"password","permission":0,"username":"email-address"},"relationships":{"mail_server_id":{"data":{"id":"e494c2d1-ff68-4ed5-bf9c-b4804aeec0fb","type":"mail_server"}}}},"meta":{}}'
```


# Enable IMAP


Three config entries

- imap.enabled
- imap.listen_interface
- hostname

```bash
curl --location --request POST 'localhost:6336/_config/backend/imap.enabled' \
    --header 'Content-Type: text/plain' \
    --header 'Authorization: Bearer <TOKEN>' \
    --data-raw 'true'
     
curl --location --request POST 'localhost:6336/_config/backend/imap.listen_interface' \
    --header 'Content-Type: text/plain' \
    --header 'Authorization: Bearer <TOKEN>' \
    --data-raw '0.0.0.0:993'
    

curl --location --request POST 'localhost:6336/_config/backend/hostname' \
    --header 'Content-Type: text/plain' \
    --header 'Authorization: Bearer <TOKEN>' \
    --data-raw 'imap.example.com'
    


```

The mail account created earlier should be able to access the SMTP/IMAP interface to send and receive email.

## Mail message storage

SMTP and IMAP store the raw message body in the built-in `mail.mail` column.
You can keep the default database-backed column storage, or point the column to
any configured cloud store with the same `ForeignKeyData` used by asset
columns:

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
```

For queued outgoing mail, configure `outbox.mail` the same way:

```yaml
Tables:
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

When configured, Daptin stores message bodies as `message/rfc822` `.eml`
objects in the selected cloud store. Mailbox metadata, flags, UID state, and
relationships remain in SQL. To read the message body through the API, include
the `mail` relation:

```text
GET /api/mail/<id>?included_relations=mail
```


## DKIM 

Make sure you have a SSL certificate created against the addresses you want to send mail from

Check the [certificate page](certificate.md).

DKIM Selector is: *d1*


DKIM DNS record example

<selector(s=)._domainkey.domain(d=)>.   TXT v=DKIM1; p=<public key>

    s= indicates the selector record name used with the domain to locate the public key in DNS. The value is a name or number created by the sender. s= is included in the DKIM signature.
    d= indicates the domain used with the selector record (s=) to locate the public key. The value is a domain name owned by the sender. d= is included in the DKIM signature.
    p= indicates the public key used by a mailbox provider to match to the DKIM signature.

Here is what the full DNS DKIM record looks like for example.com:

```
d1._domainkey.example.com. 600 IN TXT "v=DKIM1\; p=MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQC1TaNgLlSyQMNWVLNLvyY/neDgaL2oqQE8T5illKqCgDtFHc8eHVAU+nlcaGmrKmDMw9dbgiGk1ocgZ56NR4ycfUHwQhvQPMUZw0cveel/8EAGoi/UyPmqfcPibytH81NFtTMAxUeM4Op8A6iHkvAMj5qLf4YRNsTkKAV;"
```


    The selector (s=): d1
    The domain (d=): example.com
    The version (v=): DKIM1
    The public key (p=): MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQC1TaNgLlSyQMNWVLNLvyY/neDgaL2oqQE8T5illKqCgDtFHc8eHVAU+nlcaGmrKmDMw9dbgiGk1ocgZ56NR4ycfUHwQhvQPMUZw0cveel/8EAGoi/UyPmqfcPibytH81NFtTMAxUeM4Op8A6iHkvAMj5qLf4YRNsTkKAV

Required tag

    p= is the public key used by a mailbox provider to match to the DKIM signature generated using the private key. The value is a string of characters representing the public key. It is generated along with its corresponding private key during the DKIM set-up process.



Daptin will (try to) sign all external mails from the SMTP server using the key against the FromMail hostname

# Restart

Restart the server to start/update listening to as the SMTP server/IMAP server

```bash
curl 'http://localhost:8080/action/world/restart_daptin' -X POST \
    -H 'Authorization: Bearer <TOKEN>' \
    -H 'Content-Type: application/json;charset=utf-8' \
    --data-raw '{"attributes":{}}'
```
