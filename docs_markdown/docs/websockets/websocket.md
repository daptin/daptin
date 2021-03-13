## Websockets

Clients can connect to a websocket endpoint 

Endpoint

```ws://localhost:6336/live?token=<auth_token>```

!note WSS
    Use wss:// if you have enabled SSL

Client request payload structure

```json
{
    "method": "", // one of list-topic, create-topic, destroy-topic, subscribe, unsubscribe, new-message
    "type": "",   // required when method is subscribe
    "payload": {} // attributes depending on the method
}
```

### Websocket requests

#### List topics

List all the available topics
    - System topics
    - User created topics

```json
{
    "method": "list-topics"
}
```

#### Create topic

List all the available topics
    - System topics
    - User created topics

```json
{
    "method": "create-topics"
}
```

#### Destroy topic

List all the available topics
    - System topics
    - User created topics

```json
{
    "method": "destroy-topic"
}
```

#### Subscribe topic

List all the available topics
    - System topics
    - User created topics

```json
{
    "method": "subscribe"
}
```

#### Unsubscribe topic


List all the available topics
    - System topics
    - User created topics

```json
{
    "method": "unsubscribe"
}
```

#### New message for a user-created topic

List all the available topics
    - System topics
    - User created topics

```json
{
    "method": "new-message"
}
```

