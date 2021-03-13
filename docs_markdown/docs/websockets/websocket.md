## Websockets

Clients can connect to a websocket endpoint

Endpoint

```ws://localhost:6336/live?token=<auth_token>```

!note WSS 
    Use wss:// if you have enabled SSL

Client request payload structure

```json
{
  "method": "",
  // one of list-topic, create-topic, destroy-topic, subscribe, unsubscribe, new-message
  "type": "",
  // required when method is subscribe
  "payload": {}
  // attributes depending on the method
}
```

### Websocket requests

#### List topics

List all the available topics - System topics - User created topics

```json
{
  "method": "list-topic"
}
```

Sample Response

```json
{
  "MessageSource": "system",
  "EventType": "response",
  "ObjectType": "topic-list",
  "EventData": {
    "topics": [
      "task_task_id_has_usergroup_usergroup_id",
      "timeline",
      "application",
      "json_schema_json_schema_id_has_usergroup_usergroup_id",
      "plan_plan_id_has_usergroup_usergroup_id",
      "mail_account",
      "stream",
      "user_otp_account",
      "outbox_outbox_id_has_usergroup_usergroup_id",
      "oauth_token_oauth_token_id_has_usergroup_usergroup_id",
      "stream_stream_id_has_usergroup_usergroup_id",
      "deployment_deployment_id_has_usergroup_usergroup_id",
      "feed_feed_id_has_usergroup_usergroup_id",
      "smd_smd_id_has_usergroup_usergroup_id",
      "site_site_id_has_usergroup_usergroup_id",
      "timeline_timeline_id_has_usergroup_usergroup_id",
      "tab_eygurbe",
      "screen_screen_id_has_usergroup_usergroup_id",
      "mail_server",
      "certificate",
      "data_exchange",
      "document_document_id_has_usergroup_usergroup_id",
      "oauth_token",
      "calendar_calendar_id_has_usergroup_usergroup_id",
      "tab_nuqymzy",
      "outbox",
      "screen",
      "integration_integration_id_has_usergroup_usergroup_id",
      "oauth_connect",
      "user_payment_user_payment_id_has_usergroup_usergroup_id",
      "mail",
      "world",
      "mail_account_mail_account_id_has_usergroup_usergroup_id",
      "mail_box",
      "world_world_id_has_usergroup_usergroup_id",
      "user_account_user_account_id_has_usergroup_usergroup_id",
      "tab_nuqymzy_tab_nuqymzy_id_has_usergroup_usergroup_id",
      "calendar",
      "smd",
      "user_payment",
      "action",
      "task",
      "site",
      "integration",
      "starter_app_starter_app_id_has_usergroup_usergroup_id",
      "plan",
      "usergroup",
      "tab_eygurbe_tab_eygurbe_id_has_usergroup_usergroup_id",
      "deployment",
      "starter_app",
      "json_schema",
      "user_otp_account_user_otp_account_id_has_usergroup_usergroup_id",
      "feed",
      "cloud_store_cloud_store_id_has_usergroup_usergroup_id",
      "world_world_id_has_smd_smd_id",
      "certificate_certificate_id_has_usergroup_usergroup_id",
      "mail_mail_id_has_usergroup_usergroup_id",
      "action_action_id_has_usergroup_usergroup_id",
      "data_exchange_data_exchange_id_has_usergroup_usergroup_id",
      "user_account",
      "application_application_id_has_usergroup_usergroup_id",
      "document",
      "oauth_connect_oauth_connect_id_has_usergroup_usergroup_id",
      "cloud_store",
      "mail_server_mail_server_id_has_usergroup_usergroup_id",
      "mail_box_mail_box_id_has_usergroup_usergroup_id"
    ]
  }
}    
```

#### Create topic

Create a new topic

```json
{
  "method": "create-topic",
  "attributes": {
    "name": "<new_topic_name>"
  }
}
```

#### Destroy topic

Delete a user created topic

```json
{
  "method": "destroy-topic",
  "attributes": {
    "name": "<topic_name>"
  }
}
```

#### Subscribe topic

Listen to create/update/delete events in any table

```json
{
  "method": "subscribe",
  "attributes": {
    "topic": "user_account"
  }
}
```

Create event sample payload

```json
{
  "MessageSource": "database",
  "EventType": "create",
  "ObjectType": "user_account",
  "EventData": {
    "__type": "user_account",
    "confirmed": false,
    "created_at": "2021-03-13T13:47:07.954634Z",
    "email": "asdf@asfd.cm",
    "name": "asdf",
    "password": "$2a$11$HbH5o1s6ThsMJ9ft/.uljO9.T.od/tR0RFtep50Ef5mzymI6kNlW.",
    "permission": 2097057,
    "reference_id": "004cc6b6-8b9b-4d51-936a-128133b21d04",
    "updated_at": null,
    "user_account_id": "ee655e01-98a5-4761-bc93-b7a15e2b5847",
    "version": 1
  }
}    
```

Update event sample payload

```json
{
  "MessageSource": "database",
  "EventType": "update",
  "ObjectType": "user_account",
  "EventData": {
    "__type": "user_account",
    "confirmed": false,
    "created_at": "2021-03-13T13:47:07.954634Z",
    "email": "asdf@asfd.cm",
    "name": "asdf",
    "password": "$2a$11$U83vQU5A3Xq2Gcphb52XOej8H9p1GbFKerpkoSesbx674qZfBjJdu",
    "permission": 2097057,
    "reference_id": "004cc6b6-8b9b-4d51-936a-128133b21d04",
    "updated_at": "2021-03-13T13:48:21.258962Z",
    "user_account_id": "ee655e01-98a5-4761-bc93-b7a15e2b5847",
    "version": 2
  }
}    
```

#### Subscribe with filters

```json
{
  "method": "subscribe",
  "attributes": {
    "topic": "user_account",
    "filters": {
      // filter on type of event: create/update/delete
      "EventType": "update",
      // filter on column data, rows with unmatched column value will not be sent
      "<column_name>": "<filter_value>"
    }
  }
}    
```

#### Unsubscribe topic

Unsubscribe to an subscribed topic (this is required if you want to subscribe with new filters)

```json
{
  "method": "unsubscribe",
  "attributes": {
    "topic": "<topic_name>"
  }
}
```

#### New message for a user-created topic

Send a message on a user created topic, broad-casted to all subscribers of this topic

```json
{
  "method": "new-message",
  "attributes": {
    "topic": "test",
    "message": {
      "hello": "world"
    }
  }
}	
```

