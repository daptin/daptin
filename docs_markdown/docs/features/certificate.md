# SSL certificates

- * Authentication required

```-H "Authorization: <TOKEN>"```

List all certificates


```bash 
curl http://localhost:8080/api/certificate?sort=&page[number]=1&page[size]=10
```

Create new certificate entry

```bash
curl 'http://localhost:8080/api/certificate' -X POST \
    -H 'Content-Type: application/vnd.api+json' \
    -H 'Authorization: Bearer <TOKEN>' \
    --data-raw '{"data":{"type":"certificate","attributes":{"hostname":"example.com","issuer":"self"}},"meta":{}}'
```

Creates a new entry and does not generate any certificate

## Self generated


```bash
curl 'http://localhost:8080/action/certificate/generate_self_certificate' -X POST \
    -H 'Authorization: Bearer <TOKEN>' -H 'Content-Type: application/json;charset=utf-8' \
    --data-raw '{"attributes":{"certificate_id":"8036429f-8935-4cab-9c5c-42261a451905"}}'
```

This will create a new self-signed certificate for the selected certificate hostname


## ACME generated

Make sure the domain is pointed to this instance.

```bash
curl 'http://localhost:8080/action/certificate/generate_acme_certificate' -X POST \
    -H 'Authorization: Bearer <TOKEN>' -H 'Content-Type: application/json;charset=utf-8' \
    --data-raw '{"attributes":{"certificate_id":"8036429f-8935-4cab-9c5c-42261a451905","email":"<YOUR_EMAIL>"}}'
```

This will issue a new certificate from ACME and store the key in the database

## Import existing

You can directly do a PATCH on the entry and import your existing certificates and have them served.