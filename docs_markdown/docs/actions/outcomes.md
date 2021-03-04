# List of inbuilt outcomes

An OutCome is one node in the chain of OutFields defined inside an [action](/actions/actions)

## Database OutComes

### Get

Get list of rows

Structure:

```yaml
- Method: GET
  Type: <table_name>/
  Attributes:
    page["number"]: 1
    page["size"]: 10
    query: "[Query]"
    filter: ""
    included_relations: relation_name,
```

### Get By Id

Get a single row by reference_id

Structure:

```yaml
- Method: GET_BY_ID
  Type: <table_name>/
  Attributes:
    reference_id: <reference_id>
    included_relations: <reference_id>
```

### Create

Create a new row

Structure:

```yaml
- Method: POST
  Type: <table_name>/
  Attributes:
    reference_id: <reference_id>
    ...ColumnNames: ...Values
```

### Update

Update a row

Structure:

```yaml
- Method: PUT
  Type: <table_name>/
  Attributes:
    reference_id: <reference_id>
    ...ColumnNames: ...Values
```

### Delete

Delete a row

Structure:

```yaml
- Method: DELETE
  Type: <table_name>/
  Attributes:
    reference_id: <reference_id>
```

## OpenAPI Specification OutComes

Operations defined in any [OpenAPI Spec](/integrations/spec.md) uploaded can be used as an OutCome in the action

Eg:

Upload [Stripe OpenAPI Specification](https://github.com/stripe/openapi/blob/master/openapi/spec3.yaml) to integrartion
table

```yaml
Type: "stripeApi" # as defined when uploading openapi spec
Method: "<operation_id>"
Reference: operationResponseBody
Attributes:
  # ...OperationParameters as defined in request schema
  ParamName: Value
```

The response can be evaluated on later in further outcomes

```yaml
Attributes:
  response: $stripeApi.<operationNamee>.response
  statusCode: $stripeApi.<operationNamee>.statusCode
  responseBody: $operationResponseBody 
```

## System outcomes

System OutComes are set of independent useful functions to build a variety of workflows

```yaml
  - Method: EXECUTE
    Type: $network.request
    SkipInResponse: true
    Reference: validation
    Attributes:
      Url: https://ipnpb.sandbox.paypal.com/cgi-bin/webscr
      Method: POST
      Headers:
        Authorization: >
          !'Bearer '  + token[0].access_token
      FormData: >
        !attributes['cmd'] = '_notify-validate'; attributes
```

### cloud_store.files.import

```yaml

- Method: EXECUTE
  Type: cloud_store.files.import
  Attributes:
    table_name: "$.table_name"

```

### integration.install

```yaml

- Method: EXECUTE
  Type: integration.install
  Attributes:
    reference_id: "$.reference_id"

```

### client.file.download

```yaml

- Method: ACTIONRESPONSE
  Type: client.file.download
  Attributes:
    content: "!btoa(<file_content>)"
    contentType: <content_mimee_type>
    message: "!'A Message JS'"
    name: "<file_name>"

```

### acme.tls.generate

```yaml

- Method: EXECUTE
  Type: acme.tls.generate
  Attributes:
    certificate: "~subject"
    email: "~email"

```

### self.tls.generate

```yaml

- Method: EXECUTE
  Type: self.tls.generate
  Attributes:
    certificate: "~subject"

```

### otp.generate

```yaml

- Method: EXECUTE
  Type: otp.generate
  Attributes:
    email: "$.email"
    mobile: "~mobile_number"

```

### otp.login.verify

```yaml

- Method: EXECUTE
  Type: otp.login.verify
  Attributes:
    mobile: "~mobile_number"
    otp: "~otp"

```

### otp.generate

```yaml

- Method: EXECUTE
  Type: otp.generate
  Attributes:
    email: "~email"
    mobile: "~mobile_number"

```

### otp.login.verify

```yaml

- Method: EXECUTE
  Type: otp.login.verify
  Attributes:
    mobile: "~mobile_number"
    otp: "~otp"

```

### world.column.delete

```yaml

- Method: EXECUTE
  Type: world.column.delete
  Attributes:
    column_name: "~column_name"
    world_id: "$.reference_id"

```

### world.delete

```yaml

- Method: EXECUTE
  Type: world.delete
  Attributes:
    world_id: "$.reference_id"

```

### world.column.rename

```yaml

- Method: EXECUTE
  Type: world.column.rename
  Attributes:
    column_name: "~column_name"
    new_column_name: "~new_column_name"
    world_id: "$.reference_id"

```

### site.storage.sync

```yaml

- Method: EXECUTE
  Type: site.storage.sync
  Attributes:
    cloud_store_id: "$.cloud_store_id"
    path: "~path"
    site_id: "$.reference_id"

```

### column.storage.sync

```yaml

- Method: EXECUTE
  Type: column.storage.sync
  Attributes:
    column_name: "~column_name"
    table_name: "~table_name"

```

### mail.servers.sync

```yaml

- Method: EXECUTE
  Type: mail.servers.sync
  Attributes: { }

```

### system_json_schema_update

```yaml

- Method: EXECUTE
  Type: system_json_schema_update
  Attributes:
    json_schema: '!JSON.parse(''[{"name":"empty.json","file":"data:application/json;base64,e30K","type":"application/json"}]'')'

```

### generate.random.data

```yaml

- Method: EXECUTE
  Type: generate.random.data
  Attributes:
    count: "~count"
    table_name: "~table_name"
    user_account_id: "$user.id"
    user_reference_id: "$user.reference_id"

```

### __data_export

```yaml

- Method: EXECUTE
  Type: __data_export
  Attributes:
    table_name: "$.table_name"
    world_reference_id: "$.reference_id"

```

### __csv_data_export

```yaml

- Method: EXECUTE
  Type: __csv_data_export
  Attributes:
    table_name: "$.table_name"
    world_reference_id: "$.reference_id"

```

### __data_import

```yaml

- Method: EXECUTE
  Type: __data_import
  Attributes:
    dump_file: "~dump_file"
    table_name: "$.table_name"
    truncate_before_insert: "~truncate_before_insert"
    user: "~user"
    world_reference_id: "$.reference_id"

```

### cloudstore.file.upload

```yaml

- Method: EXECUTE
  Type: cloudstore.file.upload
  Attributes:
    file: "~file"
    oauth_token_id: "$.oauth_token_id"
    path: "~path"
    root_path: "$.root_path"
    store_provider: "$.store_provider"

```

### cloudstore.site.create

```yaml

- Method: EXECUTE
  Type: cloudstore.site.create
  Attributes:
    cloud_store_id: "$.reference_id"
    hostname: "~hostname"
    oauth_token_id: "$.oauth_token_id"
    path: "~path"
    root_path: "$.root_path"
    site_type: "~site_type"
    store_provider: "$.store_provider"
    user_account_id: "$user.reference_id"

```

### cloudstore.file.delete

```yaml

- Method: EXECUTE
  Type: cloudstore.file.delete
  Attributes:
    oauth_token_id: "$.oauth_token_id"
    path: "~path"
    root_path: "$.root_path"
    store_provider: "$.store_provider"

```

### cloudstore.folder.create

```yaml

- Method: EXECUTE
  Type: cloudstore.folder.create
  Attributes:
    name: "~name"
    oauth_token_id: "$.oauth_token_id"
    path: "~path"
    root_path: "$.root_path"
    store_provider: "$.store_provider"

```

### cloudstore.path.move

```yaml

- Method: EXECUTE
  Type: cloudstore.path.move
  Attributes:
    destination: "~destination"
    oauth_token_id: "$.oauth_token_id"
    root_path: "$.root_path"
    source: "~source"
    store_provider: "$.store_provider"

```

### site.file.list

```yaml

- Method: EXECUTE
  Type: site.file.list
  Attributes:
    path: "~path"
    site_id: "$.reference_id"

```

### site.file.get

```yaml

- Method: EXECUTE
  Type: site.file.get
  Attributes:
    path: "~path"
    site_id: "$.reference_id"

```

### site.file.delete

```yaml

- Method: EXECUTE
  Type: site.file.delete
  Attributes:
    path: "~path"
    site_id: "$.reference_id"

```

### system_json_schema_update

```yaml

- Method: EXECUTE
  Type: system_json_schema_update
  Attributes:
    json_schema: "~schema_file"

```

### __upload_xlsx_file_to_entity

```yaml

- Method: EXECUTE
  Type: __upload_xlsx_file_to_entity
  Attributes:
    add_missing_columns: "~add_missing_columns"
    create_if_not_exists: "~create_if_not_exists"
    data_xls_file: "~data_xls_file"
    entity_name: "~entity_name"

```

### __upload_csv_file_to_entity

```yaml

- Method: EXECUTE
  Type: __upload_csv_file_to_entity
  Attributes:
    add_missing_columns: "~add_missing_columns"
    create_if_not_exists: "~create_if_not_exists"
    data_csv_file: "~data_csv_file"
    entity_name: "~entity_name"

```

### __download_cms_config

```yaml

- Method: EXECUTE
  Type: __download_cms_config
  Attributes: { }

```

### __become_admin

```yaml

- Method: EXECUTE
  Type: __become_admin
  Attributes:
    user: "~user"
    user_account_id: "$user.id"

```

### otp.generate

```yaml

- Method: EXECUTE
  Type: otp.generate
  Attributes:
    email: "~email"
    mobile: "~mobile"

```

### client.notify

```yaml

- Method: ACTIONRESPONSE
  Type: client.notify
  Attributes:
    message: Sign-up successful. Redirecting to sign in
    ### Success
    type: success

```

### client.redirect

```yaml

- Method: ACTIONRESPONSE
  Type: client.redirect
  Attributes:
    delay: 2000
    location: "/auth/signin"
    window: self

```

### otp.generate

```yaml

- Method: EXECUTE
  Type: otp.generate
  Attributes:
    email: "$email"

```

### mail.send

```yaml

- Method: EXECUTE
  Type: mail.send
  Attributes:
    body: 'Your verification code is: $otp.otp'
    from: no-reply@localhost
    subject: Request for password reset
    to: "~email"

```

### otp.login.verify

```yaml

- Method: EXECUTE
  Type: otp.login.verify
  Attributes:
    email: "~email"
    otp: "~otp"

```

### random.generate

```yaml

- Method: EXECUTE
  Type: random.generate
  Attributes:
    type: password

```

### user_account

```yaml

- Method: EXECUTE
  Type: user_account
  Attributes:
    password: "!newPassword.value"
    reference_id: "$user[0].reference_id"

```

### mail.send

```yaml

- Method: EXECUTE
  Type: mail.send
  Attributes:
    body: 'Your new password is: $newPassword.value'
    from: no-reply@localhost
    subject: Request for password reset
    to: "~email"

```

### jwt.token

```yaml

- Method: EXECUTE
  Type: jwt.token
  Attributes:
    email: "~email"
    password: "~password"

```

### oauth.client.redirect

```yaml

- Method: EXECUTE
  Type: oauth.client.redirect
  Attributes:
    authenticator: "$.name"
    scope: "$.scope"

```

### oauth.login.response

```yaml

- Method: EXECUTE
  Type: oauth.login.response
  Attributes:
    authenticator: "~authenticator"
    code: "~code"
    state: "~state"
    user_account_id: "~user.id"
    user_reference_id: "~user.reference_id"

```

### oauth.profile.exchange

```yaml

- Method: EXECUTE
  Type: oauth.profile.exchange
  Attributes:
    authenticator: "~authenticator"
    profileUrl: "$connection[0].profile_url"
    token: "$auth.access_token"
    tokenInfoUrl: "$connection[0].token_url"

```

### jwt.token

```yaml

- Method: EXECUTE
  Type: jwt.token
  Attributes:
    email: "!profile.email || profile.emailAddress"
    skipPasswordCheck: true
```