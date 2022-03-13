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
curl --location --request POST 'localhost:6336/_config/backend/imps.enabled' \
    --header 'Content-Type: application/json' \
    --header 'Authorization: Bearer <TOKEN>' \
    --data-raw 'true'
     
curl --location --request POST 'localhost:6336/_config/backend/imap.listen_interface' \
    --header 'Content-Type: application/json' \
    --header 'Authorization: Bearer <TOKEN>' \
    --data-raw '0.0.0.0:465'
    

curl --location --request POST 'localhost:6336/_config/backend/hostname' \
    --header 'Content-Type: application/json' \
    --header 'Authorization: Bearer <TOKEN>' \
    --data-raw 'imps.example.com'
    


```

The mail account created earlier should be able to access the SMTP/IMAP interface to send and receive email.


## DKIM 

Make sure you have a SSL certificate created against the addresses you want to send mail from

Check the [certificate page](certificate.md).

DKIM Selector is: *d1*

Daptin will sign all external mails from the SMTP server using the key against the 

# Restart

Restart the server to start/update listening to as the SMTP server/IMAP server

```bash
curl 'http://localhost:8080/action/world/restart_daptin' -X POST \
    -H 'Authorization: Bearer <TOKEN>' \
    -H 'Content-Type: application/json;charset=utf-8' \
    --data-raw '{"attributes":{}}'
```

