Daptin handles users, permissions, data, and APIs. Now you can use it to run your own LLM router too. Connect different LLM providers, expose the models you want, and give your users access through the same accounts and groups they use in your application. Each account can have a plan with request limits, a token allowance, and a spending budget. Providers, models, routes, plans, and usage records are all available through Daptin's resource APIs, so you can inspect and change the setup as you go.

We'll build this up one piece at a time:

1. **Get Daptin running**: Start a Docker container with persistent storage and check that the server is ready.

2. **Set up authentication**: Create the first account, sign in, claim administrator access, and get the token we'll use for configuration.

3. **Connect our first model**: Configure an OpenRouter credential, create a provider, and expose GPT through a model named `assistant`. This introduces the three resources behind routing: providers, models, and deployments.

4. **Create a regular user**: Create an user account and a group, grant that group permission to invoke `assistant`, and check that guests remain denied. We'll also revoke access and try the request again.

5. **Add a fallback**: Connect Gemini to the same public model name, then disable the primary deployment to see the fallback handle a request.

6. **Try streaming**: Perform the same request with and without streaming so you can see how the responses differ.

7. **Save a conversation**: Conversation continuation: retain a response and continue from its ID, with Daptin loading the earlier conversation for us.

8. **Send an image to LLM**: Enable image input and make a request containing both text and an image URL.

9. **Configure budget for user**: Create a plan, assign its membership to the account, configure model prices, and inspect the recorded usage. Also test exhausted a limit and check that Daptin rejects the next request.

10. **Set operational limits**: Configure capacity limits and per-IP HTTP rate protection, and explain how these differ from a user's plan.

We will use curl in this walkthru (but all of it can be configured using the built in web dashboard as well) with actual responses and checks along the way. You can follow each configuration change top to bottom on your local daptin instance.

Response examples show the fields relevant for that step. Expand **Full response** to inspect the complete response. JWTs are replaced with placeholders throughout; copy your own token from your sign-in response.


## 1. Get Daptin running


Pull the daptin docker image 


```bash
docker pull daptin/daptin:latest
```

Create a docker volume for persisting the sqlite db that we will use for the local setup (prefer postgres for a production setup)

```sh
docker volume create daptin-router-data

docker run -d \
  --name daptin-router \
  --restart unless-stopped \
  -p 127.0.0.1:6336:8080 \
  -v daptin-router-data:/var/lib/daptin \
  -e DAPTIN_DB_TYPE=sqlite3 \
  -e DAPTIN_DB_CONNECTION_STRING=/var/lib/daptin/daptin.db \
  -e DAPTIN_DISABLE_SMTP=true \
  daptin/daptin:latest
```

Docker logs should give you something like this

```text
INFO[2026-11-13 06:51:48] Our admin is [No one]
INFO[2026-11-13 06:51:48] Listening at: [:8080]
```

<details>
<summary>Full startup log excerpt</summary>

```text
. (truncated for brevity)
.
.
INFO[2026-11-13 06:51:48] [/live/contact/:referenceId/content/yjs] YJS websocket endpoint for contact[content]
INFO[2026-11-13 06:51:48] Listening websocket server at ... /live
WARN[2026-11-13 06:51:48] Failed to execute query 789: SELECT `u`.`email` FROM `user_account_user_account_id_has_usergroup_usergroup_id` AS `uu` LEFT JOIN `user_account` AS `u` ON (`uu`.`user_account_id` = `u`.`id`) WHERE (`uu`.`usergroup_id` = ?) ORDER BY `uu`.`created_at` ASC LIMIT ? == [2 1]
WARN[2026-11-13 06:51:48] Failed to scan user group id from the result 830: sql: no rows in result set
INFO[2026-11-13 06:51:48] Our admin is [No one]
INFO[2026-11-13 06:51:48] [ProcessId=1] Listening at port: :8080
INFO[2026-11-13 06:51:48] [198] No config value set for [enable_https]: sql: no rows in result set
INFO[2026-11-13 06:51:48] [198] No config value set for [http.read_header_timeout_seconds]: sql: no rows in result set
INFO[2026-11-13 06:51:48] [198] No config value set for [http.idle_timeout_seconds]: sql: no rows in result set
INFO[2026-11-13 06:51:48] HTTPS is disabled
INFO[2026-11-13 06:51:48] Listening at: [:8080]
```

</details>

You can call the /ready endpoint also for checking over api if the server is up and ready to handle requests.

```sh
curl http://localhost:6336/ready
```

> Ready

## 2. Set up authentication

### Create your account

Signup and setup the administrator account first:


```bash
curl http://localhost:6336/action/user_account/signup \
 -H 'Content-Type: application/json' \
 -d '{
   "attributes": {
     "name": "Administrator",
     "email": "admin@example.com",
     "password": "YOUR_PASSWORD",
     "passwordConfirm": "YOUR_PASSWORD"
   }
 }'
```

The response to signup will be something like this

```json
[
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "message": "Sign-up successful. Redirecting to sign in",
      "type": "success"
    }
  }
]
```

<details>
<summary>Full response</summary>

```json
[
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "__type": "client.notify",
      "message": "Sign-up successful. Redirecting to sign in",
      "title": "Success",
      "type": "success"
    }
  },
  {
    "ResponseType": "client.redirect",
    "Attributes": {
      "__type": "client.redirect",
      "delay": 2000,
      "location": "/auth/signin",
      "window": "self"
    }
  }
]
```

</details>

### Sign in and save the administrator token

 ```bash
 curl http://localhost:6336/action/user_account/signin \
   -H 'Content-Type: application/json' \
   -d '{
     "attributes": {
       "email": "admin@example.com",
       "password": "YOUR_PASSWORD"
     }
   }'
 ```
 
```json
[
  {
    "ResponseType": "client.store.set",
    "Attributes": {
      "key": "token",
      "value": "YOUR_ADMIN_JWT"
    }
  }
]
```

<details>
<summary>Full response</summary>

```json
[
  {
    "ResponseType": "client.store.set",
    "Attributes": {
      "key": "token",
      "value": "YOUR_ADMIN_JWT"
    }
  },
  {
    "ResponseType": "client.cookie.set",
    "Attributes": {
      "key": "token",
      "value": "YOUR_ADMIN_JWT; SameSite=Strict"
    }
  },
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "message": "Logged in",
      "title": "Success",
      "type": "success"
    }
  },
  {
    "ResponseType": "client.redirect",
    "Attributes": {
      "delay": 2000,
      "location": "/",
      "window": "self"
    }
  }
]
```

</details>

In the response, find the `client.store.set` item whose `Attributes.key` is `token`. Copy its `Attributes.value`.

Use that value wherever the following commands say `ADMIN_TOKEN`.

```sh
ADMIN_TOKEN=YOUR_ADMIN_JWT
```

### Claim administrator access

Only the first user can claim admin, and only 1 user can claim admin, After 1 administrator is claimed more administrators can be added via the dashboard directly.

```bash
curl http://localhost:6336/action/world/become_an_administrator \
 -H "Authorization: Bearer $ADMIN_TOKEN" \
 -H 'Content-Type: application/json' \
 -d '{}'
```

A successful response will look like this

```json
[
  {
    "ResponseType": "Restart",
    "Attributes": null
  }
]
```

<details>
<summary>Full response</summary>

```json
[
  {
    "ResponseType": "client.redirect",
    "Attributes": {
      "delay": 7000,
      "location": "/",
      "window": "self"
    }
  },
  {
    "ResponseType": "Restart",
    "Attributes": null
  }
]
```

</details>

A failed response will be as follows (if admin already exists)

```json
[
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "message": "Unauthorized",
      "type": "error"
    }
  }
]
```

<details>
<summary>Full response</summary>

```json
[
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "message": "Unauthorized",
      "title": "failed",
      "type": "error"
    }
  }
]
```

</details>

## 3. Connect our first model

We will use OpenRouter for this guide. The router configuration uses four Daptin resources:

| Resource | What it represents | Example in this guide |
|---|---|---|
| `credential` | The API key used to authenticate with the upstream provider | `openrouter-key` |
| `llm_provider` | A provider account and the credential it uses | `openrouter` |
| `llm_model` | The name clients request, its allowed operations, and its access policy | `assistant` |
| `llm_deployment` | A route from that public model to a specific upstream model and provider account | `assistant-primary` → `openai/gpt-4o-mini` |

A model can have several deployments. Clients keep requesting `assistant`, while
Daptin selects a route from the deployments attached to it.

### How models and deployments fit together

`llm_model` defines **what your user requests**. `llm_deployment` defines **where Daptin sends that request**.

| | `llm_model` | `llm_deployment` |
|---|---|---|
| Purpose | Public model and its access policy | Concrete upstream route |
| Name example | `assistant` | `assistant-primary` |
| Model name | Name clients use: `assistant` | Provider's name: `openai/gpt-4o-mini` |
| Configuration | Allowed operations, capabilities, defaults, execute permissions | Provider account, priority, weight, timeouts, concurrency, pricing |
| Relationships | Can have multiple deployments | Links one public model to one provider account |

The setup we will build looks like this. We will add the primary route in this
section and the fallback route in section 5:

```text
Application requests: "assistant"       ← llm_model
                         │
                         ├── assistant-primary
                         │   OpenRouter → openai/gpt-4o-mini
                         │   Priority 0
                         │
                         └── assistant-fallback
                             OpenRouter → google/gemini-2.5-flash
                             Priority 1
                                      ↑
                               llm_deployment rows
```

Your application keeps sending `"model": "assistant"`. You can change upstream models, replace provider credentials, or add fallback routes by changing deployments.

Permissions belong to the public model: **who may invoke `assistant`**. Provider and Deployment configuration controls execution: **which upstream handles it and under what operational limits**.


### Save your OpenRouter key

```bash
curl http://localhost:6336/api/credential \
 -H "Authorization: Bearer $ADMIN_TOKEN" \
 -H 'Content-Type: application/vnd.api+json' \
 -d '{
   "data": {
     "type": "credential",
     "attributes": {
       "name": "openrouter-key",
       "content": "{\"api_key\":\"sk-or-v1-YOUR_OPENROUTER_KEY\"}"
     }
   }
 }'
```

The escaped quotes in `content` are required because this field contains a JSON string.


The response to a successful creation will be like this

```json
{
  "data": {
    "type": "credential",
    "id": "01a1d3bf-c173-79bb-ab9c-3583d7cae432",
    "attributes": {
      "name": "openrouter-key",
      "content": "ENCRYPTED_VALUE"
    }
  }
}
```

<details>
<summary>Full response</summary>

```json
{
  "data": {
    "type": "credential",
    "id": "01a1d3bf-c173-79bb-ab9c-3583d7cae432",
    "attributes": {
      "__type": "credential",
      "content": "kJdzKDH16Brnfp1mPZU5DQqK9ZCQqNjYvPHt063axp9JEx5C5ukZP7XFc-7SrCjFvM_0P1cSXXGxHlooXWnwRUPr_NxoN-yyokM-6bE23F9VdYbyn2665cRnt39dyvvnhdVDdhTLJg==",
      "created_at": "2026-11-13T07:12:01.909312051Z",
      "name": "openrouter-key",
      "permission": 2097151,
      "reference_id": "01a1d3bf-c173-79bb-ab9c-3583d7cae432",
      "updated_at": "2026-11-13T07:12:01.909313259Z",
      "user_account_id": "01a1d3b5-0c88-7a44-8571-1c8211790a0e",
      "version": 1
    },
    "relationships": {
      "llm_provider_id": {
        "data": null
      },
      "user_account_id": {
        "data": {
          "type": "user_account",
          "id": "01a1d3b5-0c88-7a44-8571-1c8211790a0e"
        }
      },
      "usergroup_id": {
        "data": []
      }
    }
  }
}
```

</details>

Note that since the "content" field of the credential entity is marked as "encrypted" column type, its value will never be returned or CRUD api response. The base64 value above is the encrypted payload. You can use encrypted columns in your own custom entities as well in the same way.

 Copy `data.id` from the response. That is your `$CREDENTIAL_ID`.

```bash
CREDENTIAL_ID=01a1d3bf-c173-79bb-ab9c-3583d7cae432
```
### Create the provider

```bash
curl http://localhost:6336/api/llm_provider \
 -H "Authorization: Bearer $ADMIN_TOKEN" \
 -H 'Content-Type: application/vnd.api+json' \
 -d '{
   "data": {
     "type": "llm_provider",
     "attributes": {
       "name": "openrouter",
       "provider_type": "openrouter",
       "provider_parameters": {},
       "credential_id": "01a1d3bf-c173-79bb-ab9c-3583d7cae432",
       "enable": true
     }
   }
 }'
```

The reponse will be something like this

```json
{
  "data": {
    "type": "llm_provider",
    "id": "01a1d3c5-8468-77e9-b962-25eb573dbd58",
    "attributes": {
      "name": "openrouter",
      "provider_type": "openrouter",
      "credential_id": "01a1d3bf-c173-79bb-ab9c-3583d7cae432",
      "provider_parameters": {},
      "enable": 1
    }
  }
}
```

<details>
<summary>Full response</summary>

```json
{
  "data": {
    "type": "llm_provider",
    "id": "01a1d3c5-8468-77e9-b962-25eb573dbd58",
    "attributes": {
      "__type": "llm_provider",
      "allow_insecure": 0,
      "allow_private_network": 0,
      "base_url": null,
      "created_at": "2026-11-13T07:18:19.497550961Z",
      "credential_id": "01a1d3bf-c173-79bb-ab9c-3583d7cae432",
      "enable": 1,
      "name": "openrouter",
      "permission": 2097151,
      "provider_parameters": {},
      "provider_type": "openrouter",
      "reference_id": "01a1d3c5-8468-77e9-b962-25eb573dbd58",
      "updated_at": "2026-11-13T07:18:19.497551503Z",
      "user_account_id": "01a1d3b5-0c88-7a44-8571-1c8211790a0e",
      "version": 1
    },
    "relationships": {
      "credential_id": {
        "data": {
          "type": "credential",
          "id": "01a1d3bf-c173-79bb-ab9c-3583d7cae432"
        }
      },
      "llm_deployment_id": {
        "data": []
      },
      "user_account_id": {
        "data": {
          "type": "user_account",
          "id": "01a1d3b5-0c88-7a44-8571-1c8211790a0e"
        }
      },
      "usergroup_id": {
        "data": []
      }
    }
  }
}
```

</details>

 Copy `data.id`. That is your `PROVIDER_ID`.



### Create the model your client will request

```bash
curl http://localhost:6336/api/llm_model \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "llm_model",
      "attributes": {
        "name": "assistant",
        "operations": "[\"chat\",\"responses\"]",
        "routing_strategy": "priority_weighted",
        "enable": true
      }
    }
  }'
```

Which gives us the response

```json
{
  "data": {
    "type": "llm_model",
    "id": "01a1d3c9-3c03-7ac3-a864-7b7cf5705b14",
    "attributes": {
      "name": "assistant",
      "operations": [
        "chat",
        "responses"
      ],
      "permission": 2097151
    }
  }
}
```

<details>
<summary>Full response</summary>

```json
{
  "data": {
    "type": "llm_model",
    "id": "01a1d3c9-3c03-7ac3-a864-7b7cf5705b14",
    "attributes": {
      "__type": "llm_model",
      "capabilities": {},
      "created_at": "2026-11-13T07:22:23.107978838Z",
      "default_parameters": {},
      "enable": 1,
      "fallback_models": [],
      "name": "assistant",
      "operations": [
        "chat",
        "responses"
      ],
      "permission": 2097151,
      "reference_id": "01a1d3c9-3c03-7ac3-a864-7b7cf5705b14",
      "routing_strategy": "priority_weighted",
      "unsupported_parameter_policy": "reject",
      "updated_at": "2026-11-13T07:22:23.107979755Z",
      "user_account_id": "01a1d3b5-0c88-7a44-8571-1c8211790a0e",
      "version": 1
    },
    "relationships": {
      "llm_deployment_id": {
        "data": []
      },
      "user_account_id": {
        "data": {
          "type": "user_account",
          "id": "01a1d3b5-0c88-7a44-8571-1c8211790a0e"
        }
      },
      "usergroup_id": {
        "data": []
      }
    }
  }
}
```

</details>

Copy `data.id`. That is your `MODEL_ID`.

### Restrict the model to administrator access

The permission value you are seeing in the respoonse above includes GUEST_EXECUTE so anyone will be able to use the model, lets lock it down to admin only for now and we will reopen it to users/guests later

```sh
curl -X PATCH http://localhost:6336/api/llm_model/01a1d3c9-3c03-7ac3-a864-7b7cf5705b14 \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "llm_model",
      "id": "01a1d3c9-3c03-7ac3-a864-7b7cf5705b14",
      "attributes": {"permission": 0}
    }
  }'
```



### Connect it to GPT-4o mini

```bash
curl http://localhost:6336/api/llm_deployment \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "llm_deployment",
      "attributes": {
        "name": "assistant-primary",
        "llm_model_id": "01a1d3c9-3c03-7ac3-a864-7b7cf5705b14",   
        "llm_provider_id": "01a1d3c5-8468-77e9-b962-25eb573dbd58",
        "upstream_model": "openai/gpt-4o-mini",
        "operations": "[\"chat\",\"responses\"]",
        "priority": 0,
        "weight": 1,
        "enable": true
      }
    }
  }'
```

A successful request will give the following response

```json
{
  "data": {
    "type": "llm_deployment",
    "id": "01a1d3ce-2c3c-7bfe-bb8e-6be92b322368",
    "attributes": {
      "name": "assistant-primary",
      "llm_model_id": "01a1d3c9-3c03-7ac3-a864-7b7cf5705b14",
      "llm_provider_id": "01a1d3c5-8468-77e9-b962-25eb573dbd58",
      "upstream_model": "openai/gpt-4o-mini",
      "priority": 0,
      "weight": 1
    }
  }
}
```

<details>
<summary>Full response</summary>

```json
{
  "data": {
    "type": "llm_deployment",
    "id": "01a1d3ce-2c3c-7bfe-bb8e-6be92b322368",
    "attributes": {
      "__type": "llm_deployment",
      "connect_timeout_ms": 10000,
      "created_at": "2026-11-13T07:27:46.749376377Z",
      "enable": 1,
      "health_check": {},
      "llm_model_id": "01a1d3c9-3c03-7ac3-a864-7b7cf5705b14",
      "llm_provider_id": "01a1d3c5-8468-77e9-b962-25eb573dbd58",
      "max_concurrency": -1,
      "name": "assistant-primary",
      "operations": [
        "chat",
        "responses"
      ],
      "parameters": {},
      "permission": 2097151,
      "pricing": {},
      "priority": 0,
      "reference_id": "01a1d3ce-2c3c-7bfe-bb8e-6be92b322368",
      "request_timeout_ms": 90000,
      "rpm": -1,
      "tpm": -1,
      "updated_at": "2026-11-13T07:27:46.749376793Z",
      "upstream_model": "openai/gpt-4o-mini",
      "user_account_id": "01a1d3b5-0c88-7a44-8571-1c8211790a0e",
      "version": 1,
      "weight": 1
    },
    "relationships": {
      "llm_model_id": {
        "data": {
          "type": "llm_model",
          "id": "01a1d3c9-3c03-7ac3-a864-7b7cf5705b14"
        }
      },
      "llm_provider_id": {
        "data": {
          "type": "llm_provider",
          "id": "01a1d3c5-8468-77e9-b962-25eb573dbd58"
        }
      },
      "user_account_id": {
        "data": {
          "type": "user_account",
          "id": "01a1d3b5-0c88-7a44-8571-1c8211790a0e"
        }
      },
      "usergroup_id": {
        "data": []
      }
    }
  }
}
```

</details>

Wait a few seconds for the catalog to reload (or you can restart the container/process).

At this point we have done all the configuration and we can start calling the llm apis, we will start with the model list which should list our public facing names (the one we created in llm_deployment)

### List available models

```bash
curl http://localhost:6336/v1/models \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

Response:
```json
{
  "data": [
    {
      "id": "assistant",
      "object": "model"
    }
  ],
  "object": "list"
}
```

<details>
<summary>Full response</summary>

```json
{
  "data": [
    {
      "created": 0,
      "id": "assistant",
      "llmgateway": {
        "capabilities": {},
        "operations": [
          "chat",
          "responses"
        ]
      },
      "object": "model",
      "owned_by": "llmgateway"
    }
  ],
  "object": "list"
}
```

</details>

### Send a chat request

```bash
curl http://localhost:6336/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "assistant",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ],
    "max_tokens": 128
  }'
```

This will giv permission denied error since there is no auth token and we had set the permission for the llm_model to 0 earlier.

```bash
curl http://localhost:6336/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "model": "assistant",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ],
    "max_tokens": 128
  }'
```

Response:

```json
{
  "model": "assistant",
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "Hello! How can I assist you today?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 9,
    "completion_tokens": 9,
    "total_tokens": 18
  }
}
```

<details>
<summary>Full response</summary>

```json
{
  "choices": [
    {
      "finish_reason": "stop",
      "index": 0,
      "logprobs": null,
      "message": {
        "content": "Hello! How can I assist you today?",
        "role": "assistant"
      }
    }
  ],
  "created": 1794555847,
  "id": "gen-1794555847-wSj38rh3abVghxCJDUsD",
  "model": "assistant",
  "object": "chat.completion",
  "usage": {
    "prompt_tokens": 9,
    "completion_tokens": 9,
    "total_tokens": 18
  }
}
```

</details>

We have a very basic setup for the LLM router now.  Admin can use the /v1/ apis to list models or invoke /completions
Next will do some advanced stuff, including adding a fallback model, changing permission to allow users/guests and also budgeting/token limiting.




## 4. Decide who gets access

### Choose execution permissions

   These execution grants have different meanings:

| Permission | Meaning |
|---|---|
| Model row `0` | No row-level execution grant; administrators and separately granted groups may still execute |
| Model row `32` | Allows guest invocation |
| Model row `4096` | Allows the record's owner—not every signed-in user |
| Model row `2097152` | Allows any authenticated account |
| Model's group relationship `524288` | `GroupExecute`: grants execution to members of that group |

Permission values are bitsets. PATCHing a permission value replaces that record's current bitset. Apply model permissions through a separate PATCH after creation.

**For the group-specific setup below, keep the model row's permission at `0`.**


### Create an application account

As administrator, create an application account if you do not have one (or you can enable the signup action to let guests signup):

```bash
curl http://localhost:6336/api/user_account \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "user_account",
      "attributes": {
        "name": "Router application",
        "email": "app@example.com",
        "password": "YOUR_USER_PASSWORD"
      }
    }
  }'
```

Response

```json
{
  "data": {
    "type": "user_account",
    "id": "01a1d46b-46f6-7b6c-a6ea-3f9fe8cc26fc"
  }
}
```


### Create an access group

And then we create a new usergroup

```bash
curl http://localhost:6336/api/usergroup \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "usergroup",
      "attributes": {"name": "assistant-users"}
    }
  }'
```


Response
```json
{
  "data": {
    "type": "usergroup",
    "id": "01a1d46c-2c35-7502-a00b-3244b16944a2"
  }
}
```


Copy the group response's `data.id` as `GROUP_ID`.


Add the newly created user account to this new usergroup

### Add the application account to the group

```bash
curl -X PATCH http://localhost:6336/api/user_account/01a1d46b-46f6-7b6c-a6ea-3f9fe8cc26fc \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "user_account",
      "id": "01a1d46b-46f6-7b6c-a6ea-3f9fe8cc26fc",
      "relationships": {
        "usergroup_id": {
          "data": [{"type": "usergroup", "id": "01a1d46c-2c35-7502-a00b-3244b16944a2"}]
        }
      }
    }
  }'
```
Expect HTTP 200. This adds membership and preserves existing memberships.

Response

```json
{
  "data": {
    "type": "user_account",
    "id": "01a1d46b-46f6-7b6c-a6ea-3f9fe8cc26fc"
  }
}
```


We can verify the actual group IDs with a separate read:


```bash
curl http://localhost:6336/api/user_account/01a1d46b-46f6-7b6c-a6ea-3f9fe8cc26fc/relationships/usergroup_id \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

Response

```json
{
  "data": [
    {
      "type": "usergroup",
      "id": "01a1d46c-2c35-7502-a00b-3244b16944a2"
    },
    {
      "type": "usergroup",
      "id": "01a1d45c-4675-7f29-985e-f7a678355c66"
    }
  ]
}
```

<details>
<summary>Full response</summary>

```json
{
  "links": {
    "current_page": 1,
    "from": 0,
    "last_page": 1,
    "per_page": 10,
    "to": 10,
    "total": 2
  },
  "data": [
    {
      "type": "usergroup",
      "id": "01a1d46c-2c35-7502-a00b-3244b16944a2"
    },
    {
      "type": "usergroup",
      "id": "01a1d45c-4675-7f29-985e-f7a678355c66"
    }
  ]
}
```

</details>

User is part of the usergroup. We will do the same with the llm_model entity we created earlier. Add it to the same usergroup

### Attach the group to the model

```bash
curl -X PATCH http://localhost:6336/api/llm_model/01a1d3c9-3c03-7ac3-a864-7b7cf5705b14 \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "llm_model",
      "id": "01a1d3c9-3c03-7ac3-a864-7b7cf5705b14",
      "relationships": {
        "usergroup_id": {
          "data": [{"type": "usergroup", "id": "01a1d46c-2c35-7502-a00b-3244b16944a2"}]
        }
      }
    }
  }'
```

Response

```json
{
  "data": {
    "type": "llm_model",
    "id": "01a1d3c9-3c03-7ac3-a864-7b7cf5705b14"
  }
}
```


### Grant the group permission to execute the model

The new additions to the usergroup are created with the default permissions, we will update the group permission of the llm_model and allow GROUP_EXECUTE so that members of that group can execute llm calls on this model


```bash
curl -X PATCH http://localhost:6336/api/usergroup/01a1d46c-2c35-7502-a00b-3244b16944a2/relationships/llm_model_id \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": [{
      "type": "llm_model",
      "id": "01a1d3c9-3c03-7ac3-a864-7b7cf5705b14",
      "attributes": {"permission": 524288}
    }]
  }'
```

This PATCH returns nothing in response body. This permission update replaces the relation's entire bitmask with `524288`.
It does not change the model row's own permission, which should remain `0`. 

Verify the permission persisted

### Read back the persisted grant

```bash
curl http://localhost:6336/api/llm_model/01a1d3c9-3c03-7ac3-a864-7b7cf5705b14/usergroup_id \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

In the response to this one of the usergroup will be the one we just modified 

```json
{
  "data": [
    {
      "type": "usergroup",
      "id": "01a1d482-1632-77ae-b3d3-e8cdd85b8b9d",
      "attributes": {
        "name": "assistant-users",
        "permission": 524288,
        "relation_reference_id": "01a1d46c-2c35-7502-a00b-3244b16944a2"
      }
    }
  ]
}
```


Now we should be able to login as the new user and use the llm model through that account

### Sign in as the application account

Sign in as the application account and copy the token as before:

```bash
curl http://localhost:6336/action/user_account/signin \
  -H 'Content-Type: application/json' \
  -d '{
    "attributes": {
      "email": "app@example.com",
      "password": "YOUR_USER_PASSWORD"
    }
  }'
```


Response

```json
[
  {
    "ResponseType": "client.store.set",
    "Attributes": {
      "key": "token",
      "value": "YOUR_APPLICATION_JWT"
    }
  }
]
```


Copy `Attributes.value` from the `client.store.set` item into a separate shell
variable for the application account:

```bash
USER_DAPTIN_TOKEN='YOUR_APPLICATION_JWT'
```

Use `$USER_DAPTIN_TOKEN` for model calls from this point onward. Keep
`$ADMIN_TOKEN` for configuration and inspecting administrative resources.

Try a request as the application account:

```sh
curl http://localhost:6336/v1/chat/completions \
  -H "Authorization: Bearer $USER_DAPTIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "assistant",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 128,
    "stream": false
  }'
```


```json
{
  "model": "assistant",
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "Hello! How can I assist you today?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 9,
    "completion_tokens": 9,
    "total_tokens": 18
  }
}
```

<details>
<summary>Full response</summary>

```json
{
  "choices": [
    {
      "finish_reason": "stop",
      "index": 0,
      "logprobs": null,
      "message": {
        "content": "Hello! How can I assist you today?",
        "role": "assistant"
      }
    }
  ],
  "created": 1794567109,
  "id": "gen-1794567109-Mnjrxj9MqH12b4rkmM75",
  "model": "assistant",
  "object": "chat.completion",
  "usage": {
    "prompt_tokens": 9,
    "completion_tokens": 9,
    "total_tokens": 18
  }
}
```

</details>

Now all users that belong to this new usergroup will be able to execute the /v1 llm apis for this model. We can set back the permission to restrict it again

### Test revocation

Set the existing model–group relation's permission to `0`:

```bash
curl -X PATCH http://localhost:6336/api/usergroup/01a1d46c-2c35-7502-a00b-3244b16944a2/relationships/llm_model_id \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": [{
      "type": "llm_model",
      "id": "01a1d3c9-3c03-7ac3-a864-7b7cf5705b14",
      "attributes": {"permission": 0}
    }]
  }'
```

To continue the guide, restore the grant by repeating the group-side permission PATCH above with `"permission": 524288`.
Setting the relation's permission to `0` leaves the link in place but removes its group execution grant. 
It does not revoke owner, guest, authenticated, other group, or administrator access supplied independently.


### Optional: remove an account's membership

```bash
curl -X DELETE http://localhost:6336/api/user_account/<USER_ID>/relationships/usergroup_id \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": [{"type": "usergroup", "id": "<GROUP_ID>"}]
  }'
```

(same structure works if you want to remove the model from the usergroup)



## 5. Add a fallback

### Create the Gemini deployment

Use the same `MODEL_ID` and `PROVIDER_ID`:

Both deployments use the same OpenRouter account. The fallback gives us a second
upstream model, but both routes share that account's credit and OpenRouter's
availability.

```bash
curl http://localhost:6336/api/llm_deployment \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "llm_deployment",
      "attributes": {
        "name": "assistant-fallback",
        "llm_model_id": "01a1d3c9-3c03-7ac3-a864-7b7cf5705b14",
        "llm_provider_id": "01a1d3c5-8468-77e9-b962-25eb573dbd58",
        "upstream_model": "google/gemini-2.5-flash",
        "operations": "[\"chat\",\"responses\"]",
        "priority": 1,
        "weight": 1,
        "enable": true
      }
    }
  }'
```

Which gives us the response of successful creation

```json
{
  "data": {
    "type": "llm_deployment",
    "id": "01a1d3e1-4229-7adb-b7b6-417803e3f465",
    "attributes": {
      "name": "assistant-fallback",
      "llm_model_id": "01a1d3c9-3c03-7ac3-a864-7b7cf5705b14",
      "llm_provider_id": "01a1d3c5-8468-77e9-b962-25eb573dbd58",
      "upstream_model": "google/gemini-2.5-flash",
      "priority": 1,
      "weight": 1
    }
  }
}
```

<details>
<summary>Full response</summary>

```json
{
  "data": {
    "type": "llm_deployment",
    "id": "01a1d3e1-4229-7adb-b7b6-417803e3f465",
    "attributes": {
      "__type": "llm_deployment",
      "connect_timeout_ms": 10000,
      "created_at": "2026-11-13T07:48:37.548225595Z",
      "enable": 1,
      "health_check": {},
      "llm_model_id": "01a1d3c9-3c03-7ac3-a864-7b7cf5705b14",
      "llm_provider_id": "01a1d3c5-8468-77e9-b962-25eb573dbd58",
      "max_concurrency": -1,
      "name": "assistant-fallback",
      "operations": [
        "chat",
        "responses"
      ],
      "parameters": {},
      "permission": 2097151,
      "pricing": {},
      "priority": 1,
      "reference_id": "01a1d3e1-4229-7adb-b7b6-417803e3f465",
      "request_timeout_ms": 90000,
      "rpm": -1,
      "tpm": -1,
      "updated_at": "2026-11-13T07:48:37.548226262Z",
      "upstream_model": "google/gemini-2.5-flash",
      "user_account_id": "01a1d3b5-0c88-7a44-8571-1c8211790a0e",
      "version": 1,
      "weight": 1
    },
    "relationships": {
      "llm_model_id": {
        "data": {
          "type": "llm_model",
          "id": "01a1d3c9-3c03-7ac3-a864-7b7cf5705b14"
        }
      },
      "llm_provider_id": {
        "data": {
          "type": "llm_provider",
          "id": "01a1d3c5-8468-77e9-b962-25eb573dbd58"
        }
      },
      "user_account_id": {
        "data": {
          "type": "user_account",
          "id": "01a1d3b5-0c88-7a44-8571-1c8211790a0e"
        }
      },
      "usergroup_id": {
        "data": []
      }
    }
  }
}
```

</details>

### Understand when the fallback is used

Requests still use `"model": "assistant"`. Daptin prefers priority `0` and can use priority `1` when the primary is unavailable or encounters a retryable failure.

With GPT at priority `0` and Gemini at priority `1`:

| Situation | Result |
|---|---|
| GPT is eligible and succeeds | GPT answers; Gemini is not called |
| GPT is disabled, unhealthy, or excluded by a deployment protection limit | Daptin can select Gemini |
| GPT encounters a retryable upstream failure | Daptin can attempt Gemini within the remaining attempt/time budget |
| Daptin rejects model permission, validation, or quota admission | The request is rejected; fallback does not bypass the decision |
| Upstream returns a non-retryable error such as 400, 401, 403, or 404 | Daptin returns the error rather than automatically trying Gemini |
| A stream has already emitted its first client-visible event | Daptin stays with that deployment; it cannot switch midway |

Retryable upstream statuses include **408, 409, 429, and 5xx**. Connection failures and timeouts may also be retryable.

Giving both deployments priority `0` would make them weighted alternatives. Priority `0` followed by `1` expresses primary and standby.

### Test the fallback deliberately

Disable the primary deployment:

```bash
curl -X PATCH http://localhost:6336/api/llm_deployment/01a1d3ce-2c3c-7bfe-bb8e-6be92b322368 \
-H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "llm_deployment",
      "id": "01a1d3ce-2c3c-7bfe-bb8e-6be92b322368",
      "attributes": {
        "enable": false
      }
    }
  }'
```

Wait a few seconds for the catalog reload, then repeat your normal request:

```bash
curl http://localhost:6336/v1/chat/completions \
  -H "Authorization: Bearer $USER_DAPTIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "assistant",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ],
    "max_tokens": 128,
    "stream": false
  }'
```

Gemini should now handle it. The response still reports the public model name `assistant`.


Inspect usage as administrator:

```bash
curl -g 'http://localhost:6336/api/api_usage?page[size]=10&sort=-created_at' \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

Look at the relevant row's `attributes.metadata.attempts`. Compare `DeploymentID`
with `FALLBACK_DEPLOYMENT_ID`, copied from the fallback creation response.


Restore the primary by repeating the PATCH with `"enable": true`.

## 6. Try streaming

### Receive the complete answer

First, send a normal request with the application token and `stream: false`:

```bash
curl http://localhost:6336/v1/chat/completions \
  -H "Authorization: Bearer $USER_DAPTIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "assistant",
    "messages": [{"role":"user","content":"Write a short greeting."}],
    "max_tokens": 128,
    "stream": false
  }'
```

Expect HTTP 200 and one JSON response. The completed answer is in
`choices[0].message.content`.

### Receive the answer as it is generated

Now send the same prompt with `stream: true` and add `-N` to curl:

```bash
curl -N http://localhost:6336/v1/chat/completions \
  -H "Authorization: Bearer $USER_DAPTIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "assistant",
    "messages": [{"role":"user","content":"Write a short greeting."}],
    "max_tokens": 128,
    "stream": true
  }'
```

`-N` disables curl's output buffering, so text appears as it arrives. The response
is a stream of server-sent events: each `data:` line carries a chunk, and text
arrives in `choices[].delta.content`.

Here is a short example showing only the relevant fields. The actual text and
chunk boundaries will vary:

```text
data: {"choices":[{"index":0,"delta":{"role":"assistant","content":"Hello!"},"finish_reason":null}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

The last line, `data: [DONE]`, marks the end of a successful Chat Completions
stream.

<details>
<summary>Captured stream excerpt</summary>

The following events were captured from the beginning of a longer response:

```text
data: {"choices":[{"delta":{"content":"Hello","role":"assistant"},"finish_reason":null,"index":0}],"created":1794557487,"id":"gen-1794557487-CQwEErjHpcRPWReYlgvZ","model":"assistant","object":"chat.completion.chunk","usage":null}

data: {"choices":[{"delta":{"content":"!","role":"assistant"},"finish_reason":null,"index":0}],"created":1794557487,"id":"gen-1794557487-CQwEErjHpcRPWReYlgvZ","model":"assistant","object":"chat.completion.chunk","usage":null}

data: {"choices":[{"delta":{"content":" I","role":"assistant"},"finish_reason":null,"index":0}],"created":1794557487,"id":"gen-1794557487-CQwEErjHpcRPWReYlgvZ","model":"assistant","object":"chat.completion.chunk","usage":null}

data: {"choices":[{"delta":{"content":" hope","role":"assistant"},"finish_reason":null,"index":0}],"created":1794557487,"id":"gen-1794557487-CQwEErjHpcRPWReYlgvZ","model":"assistant","object":"chat.completion.chunk","usage":null}

data: {"choices":[{"delta":{"content":" this","role":"assistant"},"finish_reason":null,"index":0}],"created":1794557487,"id":"gen-1794557487-CQwEErjHpcRPWReYlgvZ","model":"assistant","object":"chat.completion.chunk","usage":null}

data: {"choices":[{"delta":{"content":" message","role":"assistant"},"finish_reason":null,"index":0}],"created":1794557487,"id":"gen-1794557487-CQwEErjHpcRPWReYlgvZ","model":"assistant","object":"chat.completion.chunk","usage":null}

data: {"choices":[{"delta":{"content":" finds","role":"assistant"},"finish_reason":null,"index":0}],"created":1794557487,"id":"gen-1794557487-CQwEErjHpcRPWReYlgvZ","model":"assistant","object":"chat.completion.chunk","usage":null}
```

</details>

Streaming controls how the answer is delivered. It does not save a conversation.
For Chat Completions, send previous messages yourself on subsequent requests.
The next section uses Responses to save and load that history through Daptin.

## 7. Save a conversation

### Save the first turn

Use Responses when you want Daptin-managed history. Both requests below use the
application token, so the saved turns belong to the application account:

```bash
curl http://localhost:6336/v1/responses \
  -H "Authorization: Bearer $USER_DAPTIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "assistant",
    "input": "Remember that my favorite color is blue.",
    "store": true,
    "max_output_tokens": 128
  }'
```

```json
{
  "id": "01a1d4a0-b058-7848-a37a-83ce8c1ee112",
  "model": "assistant",
  "status": "completed",
  "store": true,
  "output": [
    {
      "role": "assistant",
      "content": [
        {
          "type": "output_text",
          "text": "Got it! Your favorite color is blue. If you have any questions or need assistance related to it, feel free to ask!"
        }
      ]
    }
  ],
  "usage": {
    "input_tokens": 15,
    "output_tokens": 26,
    "total_tokens": 41
  }
}
```

<details>
<summary>Full response</summary>

```json
{
  "id": "01a1d4a0-b058-7848-a37a-83ce8c1ee112",
  "model": "assistant",
  "object": "response",
  "output": [
    {
      "content": [
        {
          "annotations": [],
          "logprobs": [],
          "text": "Got it! Your favorite color is blue. If you have any questions or need assistance related to it, feel free to ask!",
          "type": "output_text"
        }
      ],
      "id": "msg_tmp_ofyyg6iexe",
      "role": "assistant",
      "status": "completed",
      "type": "message"
    }
  ],
  "status": "completed",
  "store": true,
  "usage": {
    "input_tokens": 15,
    "output_tokens": 26,
    "total_tokens": 41
  }
}
```

</details>

### Continue the conversation

Copy the top-level `id` returned by your request as `RESPONSE_ID`. Replace the
placeholder below with that ID, then continue using the same application token:

```bash
curl http://localhost:6336/v1/responses \
  -H "Authorization: Bearer $USER_DAPTIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "assistant",
    "previous_response_id": "RESPONSE_ID",
    "input": "What is my favorite color?",
    "store": true,
    "max_output_tokens": 128
  }'
```

Use the newest completed response ID for the next turn. Do not also resend the previous conversation.


```json
{
  "id": "01a1d4a1-143f-7ed6-a9af-04ddb7f80e5d",
  "model": "assistant",
  "status": "completed",
  "store": true,
  "previous_response_id": "01a1d4a0-b058-7848-a37a-83ce8c1ee112",
  "output": [
    {
      "role": "assistant",
      "content": [
        {
          "type": "output_text",
          "text": "Your favorite color is blue!"
        }
      ]
    }
  ],
  "usage": {
    "input_tokens": 55,
    "output_tokens": 6,
    "total_tokens": 61
  }
}
```

<details>
<summary>Full response</summary>

```json
{
  "id": "01a1d4a1-143f-7ed6-a9af-04ddb7f80e5d",
  "model": "assistant",
  "object": "response",
  "output": [
    {
      "content": [
        {
          "annotations": [],
          "logprobs": [],
          "text": "Your favorite color is blue!",
          "type": "output_text"
        }
      ],
      "id": "msg_tmp_4vreiwjwkcb",
      "role": "assistant",
      "status": "completed",
      "type": "message"
    }
  ],
  "previous_response_id": "01a1d4a0-b058-7848-a37a-83ce8c1ee112",
  "status": "completed",
  "store": true,
  "usage": {
    "input_tokens": 55,
    "output_tokens": 6,
    "total_tokens": 61
  }
}
```

</details>


Stored responses require a signed-in account (it does not store for guests). Each continuation independently checks access to the saved history and execute permission on the requested model. 
Use `store: true` on every turn whose ID you want to continue later; `store: false` does not produce a reusable stored turn. 
Only completed retained responses can be continued. 
Top-level `instructions` and other request controls belong to the current turn, so repeat them when needed.

## 8. Send an image

The initial `assistant` configuration enables text operations. Media requires additional configuration:

| Feature | Operation | Additional model capability |
|---|---|---|
| Image input | `chat` or `responses` | `vision` |
| File input | `responses` | `files` |
| Inline audio input | `chat` | `audio` |
| Speech-to-text | `audio_transcription` | Dedicated compatible upstream |
| Text-to-speech | `audio_speech` | Dedicated compatible upstream |
| Image generation | `image_generation` | Dedicated compatible upstream |

Declaring a capability permits a request; it does not add that ability to the upstream model. Check the primary and backup routes.

### Enable image input

As administrator, enable image input on the public model:

```bash
curl -X PATCH http://localhost:6336/api/llm_model/01a1d3c9-3c03-7ac3-a864-7b7cf5705b14 \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "llm_model",
      "id": "01a1d3c9-3c03-7ac3-a864-7b7cf5705b14",
      "attributes": {
        "capabilities": "{\"vision\":true}"
      }
    }
  }'
```

This overwrites the capability field, so include any other capabilities you already use.

### Send an image URL

Before running the request, replace `PUBLIC_IMAGE_URL` with a public HTTPS URL
to a JPEG or PNG image. Use a URL that returns the image itself and can be
accessed without signing in. A local file path or a URL served only on your
machine will not work for the upstream provider.

Send the request with the application token:

```bash
curl http://localhost:6336/v1/chat/completions \
  -H "Authorization: Bearer $USER_DAPTIN_TOKEN"   \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "assistant",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "Describe this image."},
        {
          "type": "image_url",
          "image_url": {
            "url": "PUBLIC_IMAGE_URL"
          }
        }
      ]
    }],
    "max_tokens": 256
  }'
```

Expect HTTP 200 and a completion describing your image. The wording will vary
with the image and the upstream model. Both the primary and fallback routes need
to support image input for this request to work through either route.

(I will keep audio/video examples in a separate blog post)


If you have been following till this point you have a base llm router, with user accounts and usergroups to allow/deny access to the llm models which we route back to different llm providers based on the priority of our llm deployment entities. Notice how everything is basically a bunch of entities defined in the database and effectly making it declarative implementation. All we did was create some db rows and set values of their columns.

## 9. Give the application account a budget

Next we will enable metering and cost accounting at a user level. Metering enables recording of request/response payloads. LLM models are metered by default and that is why we were able to query the records earlier by calling /api/api_usage endpoint. But we have not created any api_plan to define how many tokens/seconds is the user allowed. Lets define a api_plan and link it to a user account by the api_member entity.


| Control | Applies to | Configure with | Limit failure |
|---|---|---|---|
| Model permissions | Who may invoke `assistant` | Model row and model–group grants | 403 |
| Account plan | How much the active account may consume across metered operations | `api_plan` and owned `api_member` | 402 for a hard limit |
| Deployment protection | Capacity of one upstream route, shared by its callers | Deployment RPM, TPM, concurrency | Can select another eligible route; exhausted capacity can produce 429 or an availability error |
| Global HTTP limiter | Requests from one client IP to one path | Backend `limit.rate` | 429 |

### Create a plan

We create a plan with followinglimits: 60 metered requests per UTC
minute, one million total tokens per UTC calendar month, and ten million configured
cost micro-units per month. If you price in USD, the cost limit represents $10.

```bash
curl http://localhost:6336/api/api_plan \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "api_plan",
      "attributes": {
        "name": "router-basic",
        "price_monthly_cents": 0,
        "limits": [
          {"metric":"requests","window":"minute","maximum":60,"mode":"hard"},
          {"metric":"total_tokens","window":"month","maximum":1000000,"mode":"hard"},
          {"metric":"cost_micros","window":"month","maximum":10000000,"mode":"hard"}
        ],
        "metadata": {}
      }
    }
  }'
```

Response:

```json
{
  "data": {
    "type": "api_plan",
    "id": "01a1d8e0-e793-75f0-ba75-1d103fa8994a",
    "attributes": {
      "name": "router-basic",
      "price_monthly_cents": 0,
      "limits": [
        {
          "maximum": 60,
          "metric": "requests",
          "mode": "hard",
          "window": "minute"
        },
        {
          "maximum": 1000000,
          "metric": "total_tokens",
          "mode": "hard",
          "window": "month"
        },
        {
          "maximum": 10000000,
          "metric": "cost_micros",
          "mode": "hard",
          "window": "month"
        }
      ]
    }
  }
}
```

<details>
<summary>Full response</summary>

```json
{
  "data": {
    "type": "api_plan",
    "id": "01a1d8e0-e793-75f0-ba75-1d103fa8994a",
    "attributes": {
      "__type": "api_plan",
      "archived_at": null,
      "created_at": "2026-11-14T07:06:20.435512137Z",
      "limits": [
        {
          "maximum": 60,
          "metric": "requests",
          "mode": "hard",
          "window": "minute"
        },
        {
          "maximum": 1000000,
          "metric": "total_tokens",
          "mode": "hard",
          "window": "month"
        },
        {
          "maximum": 10000000,
          "metric": "cost_micros",
          "mode": "hard",
          "window": "month"
        }
      ],
      "metadata": {},
      "name": "router-basic",
      "permission": 561441,
      "price_monthly_cents": 0,
      "reference_id": "01a1d8e0-e793-75f0-ba75-1d103fa8994a",
      "updated_at": "2026-11-14T07:06:20.435517929Z",
      "user_account_id": "01a1d45c-8c11-7229-8520-93c53d474184",
      "version": 1
    },
    "relationships": {
      "api_member_id": {
        "data": null
      },
      "api_quota_id": {
        "data": null
      },
      "api_usage_id": {
        "data": null
      },
      "user_account_id": {
        "data": {
          "type": "user_account",
          "id": "01a1d45c-8c11-7229-8520-93c53d474184"
        }
      },
      "usergroup_id": {
        "data": []
      }
    }
  }
}
```

</details>

Copy `data.id` as `PLAN_ID`. Each metric/window pair must be unique. `month` is a UTC calendar month; 
for a subscription's own billing cycle, use `member_period` and set valid `period_start` and `period_end` on its membership.


`price_monthly_cents` is plan data, not an automatic charge. A zero price does not make upstream calls free. 
Paid checkout and verified payment provisioning need a billing workflow; see [Payments and Checkout](https://github.com/daptin/daptin/wiki/Payments-and-Checkout).

### Create and assign the membership

Next we need a way to create the api_member entities which link a user_account (via the user_account_id column) to a api_plan. When you create any entity in daptin the default user_account_id is set to the user_account whos auth token is being used. An `api_member` created with the administrator token initially belongs to the administrator. 
Create it as inactive, then PATCH its owner and activate it in the same update. 
This avoids applying the new plan to the administrator before ownership is assigned. We will not provision a custom action here to keep things simple for the guide, but ideally you want the signup action to provision the api_member entity row for the user.



Create the membership as administrator:

```bash
curl http://localhost:6336/api/api_member \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "api_member",
      "attributes": {
        "api_plan_id": "PLAN_ID",
        "status": "inactive",
        "metadata": {}
      }
    }
  }'
```

Creates the following row

```json
{
  "data": {
    "type": "api_member",
    "id": "01a1d8eb-f034-7cd4-97fa-f8e6bfbe5cd9",
    "attributes": {
      "api_plan_id": "01a1d8e0-e793-75f0-ba75-1d103fa8994a",
      "user_account_id": "01a1d45c-8c11-7229-8520-93c53d474184",
      "status": "inactive"
    }
  }
}
```

<details>
<summary>Full response</summary>

```json
{
  "data": {
    "type": "api_member",
    "id": "01a1d8eb-f034-7cd4-97fa-f8e6bfbe5cd9",
    "attributes": {
      "__type": "api_member",
      "api_plan_id": "01a1d8e0-e793-75f0-ba75-1d103fa8994a",
      "created_at": "2026-11-14T07:18:23.543939597Z",
      "metadata": {},
      "period_end": null,
      "period_start": "0001-01-01T00:00:00Z",
      "permission": 561441,
      "reference_id": "01a1d8eb-f034-7cd4-97fa-f8e6bfbe5cd9",
      "status": "inactive",
      "updated_at": "2026-11-14T07:18:23.543941597Z",
      "user_account_id": "01a1d45c-8c11-7229-8520-93c53d474184",
      "version": 1
    },
    "relationships": {
      "api_plan_id": {
        "data": {
          "type": "api_plan",
          "id": "01a1d8e0-e793-75f0-ba75-1d103fa8994a"
        }
      },
      "api_quota_id": {
        "data": null
      },
      "api_usage_id": {
        "data": null
      },
      "user_account_id": {
        "data": {
          "type": "user_account",
          "id": "01a1d45c-8c11-7229-8520-93c53d474184"
        }
      },
      "usergroup_id": {
        "data": []
      }
    }
  }
}
```

</details>

Copy `data.id` as `MEMBER_ID`. Use the application account's reference ID from
section 4 as `USER_ID`. Assign ownership and activate this new membership:

```bash
curl -X PATCH http://localhost:6336/api/api_member/01a1d8eb-f034-7cd4-97fa-f8e6bfbe5cd9 \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "api_member",
      "id": "01a1d8eb-f034-7cd4-97fa-f8e6bfbe5cd9",
      "attributes": {
        "user_account_id": "01a1d46b-46f6-7b6c-a6ea-3f9fe8cc26fc",
        "status": "active"
      }
    }
  }'
```

### Verify the membership

Expect HTTP 200. Read back the membership:

```bash
curl http://localhost:6336/api/api_member/MEMBER_ID \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```


```json
{
  "data": {
    "type": "api_member",
    "id": "01a1d8eb-f034-7cd4-97fa-f8e6bfbe5cd9",
    "attributes": {
      "api_plan_id": "01a1d8e0-e793-75f0-ba75-1d103fa8994a",
      "user_account_id": "01a1d46b-46f6-7b6c-a6ea-3f9fe8cc26fc",
      "status": "active"
    }
  }
}
```

<details>
<summary>Full response</summary>

```json
{
  "data": {
    "type": "api_member",
    "id": "01a1d8eb-f034-7cd4-97fa-f8e6bfbe5cd9",
    "attributes": {
      "__type": "api_member",
      "api_plan_id": "01a1d8e0-e793-75f0-ba75-1d103fa8994a",
      "created_at": "2026-11-14T07:18:23.543939597Z",
      "metadata": {},
      "period_end": null,
      "period_start": "0001-01-01T00:00:00Z",
      "permission": 561441,
      "reference_id": "01a1d8eb-f034-7cd4-97fa-f8e6bfbe5cd9",
      "status": "active",
      "updated_at": "2026-11-14T07:20:25.334852959Z",
      "user_account_id": "01a1d46b-46f6-7b6c-a6ea-3f9fe8cc26fc",
      "version": 2
    },
    "relationships": {
      "api_plan_id": {
        "data": {
          "type": "api_plan",
          "id": "01a1d8e0-e793-75f0-ba75-1d103fa8994a"
        }
      },
      "api_quota_id": {
        "data": null
      },
      "api_usage_id": {
        "data": null
      },
      "user_account_id": {
        "data": {
          "type": "user_account",
          "id": "01a1d46b-46f6-7b6c-a6ea-3f9fe8cc26fc"
        }
      },
      "usergroup_id": {
        "data": []
      }
    }
  }
}
```

</details>


### Configure deployment pricing

The plan includes a monthly cost budget. To enforce it, Daptin needs a price for
the tokens used by each upstream model. Configure those prices on the deployments.

For this example, suppose a model costs:

- $1 per million input tokens.
- $2 per million output tokens.

These are example prices. Look up the prices for your upstream models before
using this as a real spending budget.

Daptin stores these rates in **micro-dollars per million tokens**, where
1 dollar = 1,000,000 micro-dollars. The example prices therefore become:

| Pricing field | Example price | Value to configure |
|---|---|---|
| `input_tokens` | $1 per million input tokens | `1000000` |
| `output_tokens` | $2 per million output tokens | `2000000` |

Set the primary deployment's prices:

```bash
curl -X PATCH http://localhost:6336/api/llm_deployment/01a1d3ce-2c3c-7bfe-bb8e-6be92b322368 \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "llm_deployment",
      "id": "01a1d3ce-2c3c-7bfe-bb8e-6be92b322368",
      "attributes": {
        "pricing": {"input_tokens":1000000,"output_tokens":2000000}
      }
    }
  }'
```


Response:

```json
{
  "data": {
    "type": "llm_deployment",
    "id": "01a1d45f-f05d-796a-ad14-78024e0c7102",
    "attributes": {
      "name": "assistant-primary",
      "pricing": {
        "input_tokens": 1000000,
        "output_tokens": 2000000
      }
    }
  }
}
```

<details>
<summary>Full response</summary>

```json
{
  "data": {
    "type": "llm_deployment",
    "id": "01a1d45f-f05d-796a-ad14-78024e0c7102",
    "attributes": {
      "__type": "llm_deployment",
      "connect_timeout_ms": 10000,
      "created_at": "2026-11-13T10:06:59.677975507Z",
      "enable": 1,
      "health_check": {},
      "llm_model_id": "01a1d3c9-3c03-7ac3-a864-7b7cf5705b14",
      "llm_provider_id": "01a1d45e-8115-70f1-b959-719c7c846cd5",
      "max_concurrency": -1,
      "name": "assistant-primary",
      "operations": [
        "chat",
        "responses"
      ],
      "parameters": {},
      "permission": 561441,
      "pricing": {
        "input_tokens": 1000000,
        "output_tokens": 2000000
      },
      "priority": 0,
      "reference_id": "01a1d45f-f05d-796a-ad14-78024e0c7102",
      "request_timeout_ms": 90000,
      "rpm": -1,
      "tpm": -1,
      "updated_at": "2026-11-13T11:11:15.239219125Z",
      "upstream_model": "openai/gpt-4o-mini",
      "user_account_id": "01a1d45c-8c11-7229-8520-93c53d474184",
      "version": 3,
      "weight": 1
    },
    "relationships": {
      "llm_model_id": {
        "data": {
          "type": "llm_model",
          "id": "01a1d3c9-3c03-7ac3-a864-7b7cf5705b14"
        }
      },
      "llm_provider_id": {
        "data": {
          "type": "llm_provider",
          "id": "01a1d45e-8115-70f1-b959-719c7c846cd5"
        }
      },
      "user_account_id": {
        "data": {
          "type": "user_account",
          "id": "01a1d45c-8c11-7229-8520-93c53d474184"
        }
      },
      "usergroup_id": {
        "data": []
      }
    }
  }
}
```

</details>

Repeat this for all the LLM deployment you have created and linked for usage. Wait a few seconds for the deployment changes to load.

With the example prices, a request using 1,000 input tokens and 100 output tokens costs $0.0012. 
Daptin records that as `cost_micros: 1200` and counts it toward the plan's `10000000` micro-dollar ($10) monthly limit.
Daptin calculates this amount using the rates you configure; it does not fetch OpenRouter prices or invoices. 
If pricing is empty, token usage can still be counted, but the token cost can remain zero. 
Set pricing on both routes so that using the fallback also counts toward the cost budget.

### Make a metered request

Use the user token so the request is charged against the user's plan:

```bash
curl http://localhost:6336/v1/chat/completions \
  -H "Authorization: Bearer $USER_DAPTIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "assistant",
    "messages": [{"role":"user","content":"Write a short greeting."}],
    "max_tokens": 128,
    "stream": false
  }'
```

Expect a successful completion while the account has enough budget. Keep an
output limit such as `max_tokens` in your requests so Daptin can estimate the
capacity needed before starting generation.

### Inspect the usage record

As administrator, fetch the latest usage records:

```bash
curl -g 'http://localhost:6336/api/api_usage?page[size]=1&sort=-created_at' \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```


- `state` should be `completed` after a successful request.
- `measures` contains the recorded quantities, including `requests`,
  `input_tokens`, `output_tokens`, `total_tokens`, and `cost_micros`.
- `user_account_id` identifies the account that made the request.


```json
{
  "data": [
    {
      "type": "api_usage",
      "id": "01a1d901-b2e3-7694-8445-abb2c4a1475b",
      "attributes": {
        "api_member_id": "01a1d8eb-f034-7cd4-97fa-f8e6bfbe5cd9",
        "api_plan_id": "01a1d8e0-e793-75f0-ba75-1d103fa8994a",
        "user_account_id": "01a1d46b-46f6-7b6c-a6ea-3f9fe8cc26fc",
        "state": "completed",
        "status_code": 200,
        "measures": {
          "requests": 1,
          "input_tokens": 12,
          "output_tokens": 30,
          "total_tokens": 42,
          "cost_micros": 72
        },
        "metadata": {
          "attempts": [
            {
              "DeploymentID": "01a1d45f-f05d-796a-ad14-78024e0c7102",
              "Outcome": "succeeded",
              "HTTPStatus": 200
            }
          ]
        }
      }
    }
  ]
}
```

<details>
<summary>Full response</summary>

```json
{
  "links": {
    "current_page": 1,
    "from": 0,
    "last_page": 10,
    "last_page_url": "/api/api_usage?page[number]=10&page[size]=1&sort=-created_at",
    "next_page_url": "/api/api_usage?page[number]=2&page[size]=1&sort=-created_at",
    "per_page": 1,
    "to": 1,
    "total": 10
  },
  "data": [
    {
      "type": "api_usage",
      "id": "01a1d901-b2e3-7694-8445-abb2c4a1475b",
      "attributes": {
        "__type": "api_usage",
        "action_name": null,
        "api_key_id": null,
        "api_member_id": "01a1d8eb-f034-7cd4-97fa-f8e6bfbe5cd9",
        "api_plan_id": "01a1d8e0-e793-75f0-ba75-1d103fa8994a",
        "created_at": "2026-11-14T07:42:09.635806257Z",
        "endpoint": "/v1/chat",
        "entity_type": "llm_model",
        "error_message": null,
        "latency_ms": 0,
        "measures": {
          "bytes": 1994,
          "cache_read_tokens": 0,
          "cache_write_tokens": 0,
          "cost_micros": 72,
          "input_tokens": 12,
          "output_tokens": 30,
          "reasoning_tokens": 0,
          "requests": 1,
          "total_tokens": 42
        },
        "metadata": {
          "attempts": [
            {
              "DeploymentID": "01a1d45f-f05d-796a-ad14-78024e0c7102",
              "EndedAt": "2026-11-14T07:42:11.440781925Z",
              "ErrorCode": "",
              "FirstByteAt": "0001-01-01T00:00:00Z",
              "HTTPStatus": 200,
              "Number": 1,
              "Outcome": "succeeded",
              "OutputCommitted": false,
              "ProviderID": "01a1d45e-8115-70f1-b959-719c7c846cd5",
              "Retryable": false,
              "StartedAt": "2026-11-14T07:42:09.649547674Z",
              "UpstreamRequest": "",
              "Usage": {
                "CacheReadTokens": 0,
                "CacheWriteTokens": 0,
                "CostMicros": 72,
                "Estimated": false,
                "InputTokens": 12,
                "Measures": null,
                "OutputTokens": 30,
                "ReasoningTokens": 0,
                "TotalTokens": 42
              }
            }
          ],
          "cache_status": "",
          "capture_limit": 16777216,
          "ended_at": "2026-11-14T07:42:11.440781925Z",
          "error_code": "",
          "first_byte_at": "0001-01-01T00:00:00Z",
          "metering_enabled": true,
          "model_id": "01a1d3c9-3c03-7ac3-a864-7b7cf5705b14",
          "operation": "chat",
          "payload_format": "llm.canonical.v1",
          "status": "succeeded"
        },
        "method": "POST",
        "parent_usage_id": null,
        "permission": 12160,
        "reference_id": "01a1d901-b2e3-7694-8445-abb2c4a1475b",
        "request_body": "{\"ID\":\"req_d87e2d429a0d04e60795d21dc37e3711\",\"Operation\":\"chat\",\"PublicModel\":\"assistant\",\"Stream\":false,\"MaxOutputTokens\":128,\"EstimatedUsage\":{\"InputTokens\":145,\"OutputTokens\":128,\"CacheReadTokens\":0,\"CacheWriteTokens\":0,\"ReasoningTokens\":0,\"TotalTokens\":273,\"CostMicros\":0,\"Estimated\":true,\"Measures\":null},\"Chat\":{\"Messages\":[{\"Role\":\"user\",\"Name\":\"\",\"Content\":[{\"Type\":\"text\",\"Text\":\"Write a short greeting.\",\"Refusal\":\"\",\"Annotations\":null,\"Logprobs\":null,\"ImageURL\":null,\"Audio\":null,\"File\":null}],\"ToolCalls\":null,\"ToolCallID\":\"\"}],\"Tools\":null,\"ToolChoice\":null,\"ResponseFormat\":null,\"N\":1,\"Temperature\":null,\"TopP\":null,\"FrequencyPenalty\":null,\"PresencePenalty\":null,\"MaxCompletionTokens\":128,\"Stop\":null,\"User\":\"\",\"Seed\":null,\"Logprobs\":false,\"TopLogprobs\":0,\"ParallelToolCalls\":null,\"ReasoningEffort\":\"\",\"Store\":null,\"PromptCacheKey\":\"\"},\"TextCompletion\":null,\"Responses\":null,\"Embeddings\":null,\"ImageGeneration\":null,\"ImageEdit\":null,\"ImageVariation\":null,\"Moderation\":null,\"Rerank\":null,\"AudioSpeech\":null,\"Transcription\":null,\"Search\":null,\"OCR\":null,\"StartedAt\":\"2026-11-14T07:42:09.625990632Z\"}",
        "request_body_encoding": "utf8",
        "request_bytes": 1111,
        "request_id": "req_d87e2d429a0d04e60795d21dc37e3711",
        "request_type": "llm_chat",
        "reservation_buckets": {
          "1:1:cost_micros:month:1793491200": {
            "amount": 401,
            "bucket_key": "1:1:cost_micros:month:1793491200",
            "metric": "cost_micros"
          },
          "1:1:requests:minute:1794642120": {
            "amount": 1,
            "bucket_key": "1:1:requests:minute:1794642120",
            "metric": "requests"
          },
          "1:1:total_tokens:month:1793491200": {
            "amount": 546,
            "bucket_key": "1:1:total_tokens:month:1793491200",
            "metric": "total_tokens"
          }
        },
        "reservation_expires_at": "2026-11-14T07:47:09.635328882Z",
        "reservation_token": "01a1d901-b2df-7ee7-902a-736a150651f0",
        "reserved_measures": {
          "cache_read_tokens": 0,
          "cache_write_tokens": 0,
          "cost_micros": 401,
          "input_tokens": 290,
          "output_tokens": 256,
          "reasoning_tokens": 0,
          "requests": 1,
          "total_tokens": 546
        },
        "response_body": "{\"RequestID\":\"req_d87e2d429a0d04e60795d21dc37e3711\",\"Model\":\"assistant\",\"Chat\":{\"ID\":\"gen-1794642130-y5AWuU6XZPJ07Pgsttwy\",\"Created\":1794642130,\"Choices\":[{\"Index\":0,\"Message\":{\"Role\":\"assistant\",\"Name\":\"\",\"Content\":[{\"Type\":\"text\",\"Text\":\"Hello! I hope you're having a wonderful day. If there's anything you'd like to chat about or need help with, feel free to let me know!\",\"Refusal\":\"\",\"Annotations\":null,\"Logprobs\":null,\"ImageURL\":null,\"Audio\":null,\"File\":null}],\"ToolCalls\":null,\"ToolCallID\":\"\"},\"FinishReason\":\"stop\",\"Logprobs\":null}]},\"TextCompletion\":null,\"Responses\":null,\"Embeddings\":null,\"Images\":null,\"Moderation\":null,\"Rerank\":null,\"AudioSpeech\":null,\"Transcription\":null,\"Search\":null,\"OCR\":null,\"Usage\":{\"InputTokens\":12,\"OutputTokens\":30,\"CacheReadTokens\":0,\"CacheWriteTokens\":0,\"ReasoningTokens\":0,\"TotalTokens\":42,\"CostMicros\":72,\"Estimated\":false,\"Measures\":null}}",
        "response_body_encoding": "utf8",
        "response_bytes": 883,
        "state": "completed",
        "status_code": 200,
        "terminal_at": "2026-11-14T07:42:11.4654253Z",
        "updated_at": "2026-11-14T07:42:11.465579883Z",
        "user_account_id": "01a1d46b-46f6-7b6c-a6ea-3f9fe8cc26fc",
        "version": 2
      },
      "relationships": {
        "api_member_id": {
          "data": {
            "type": "api_member",
            "id": "01a1d8eb-f034-7cd4-97fa-f8e6bfbe5cd9"
          }
        },
        "api_plan_id": {
          "data": {
            "type": "api_plan",
            "id": "01a1d8e0-e793-75f0-ba75-1d103fa8994a"
          }
        },
        "parent_usage_id": {
          "data": null
        },
        "user_account_id": {
          "data": {
            "type": "user_account",
            "id": "01a1d46b-46f6-7b6c-a6ea-3f9fe8cc26fc"
          }
        },
        "usergroup_id": {
          "data": []
        }
      }
    }
  ]
}
```

</details>


### Inspect the quota totals

Next, read the quota totals:

```bash
curl -g 'http://localhost:6336/api/api_quota?page[size]=100' \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

Response

```json
{
  "data": [
    {
      "type": "api_quota",
      "id": "01a1d901-b2e2-7c65-b5eb-b21e07a014b8",
      "attributes": {
        "api_member_id": "01a1d8eb-f034-7cd4-97fa-f8e6bfbe5cd9",
        "metric": "total_tokens",
        "maximum": 1000000,
        "consumed": 42,
        "reserved": 0,
        "window_start": "2026-11-01T00:00:00Z",
        "window_end": "2026-12-01T00:00:00Z"
      }
    },
    {
      "type": "api_quota",
      "id": "01a1d901-b2e2-73cf-afc4-4c2e125802e0",
      "attributes": {
        "api_member_id": "01a1d8eb-f034-7cd4-97fa-f8e6bfbe5cd9",
        "metric": "requests",
        "maximum": 60,
        "consumed": 1,
        "reserved": 0,
        "window_start": "2026-11-14T07:42:00Z",
        "window_end": "2026-11-14T07:43:00Z"
      }
    },
    {
      "type": "api_quota",
      "id": "01a1d901-b2e0-773c-9c21-11248d657707",
      "attributes": {
        "api_member_id": "01a1d8eb-f034-7cd4-97fa-f8e6bfbe5cd9",
        "metric": "cost_micros",
        "maximum": 10000000,
        "consumed": 72,
        "reserved": 0,
        "window_start": "2026-11-01T00:00:00Z",
        "window_end": "2026-12-01T00:00:00Z"
      }
    }
  ]
}
```

<details>
<summary>Full response</summary>

```json
{
  "links": {
    "current_page": 1,
    "from": 0,
    "last_page": 1,
    "per_page": 100,
    "to": 100,
    "total": 3
  },
  "data": [
    {
      "type": "api_quota",
      "id": "01a1d901-b2e2-7c65-b5eb-b21e07a014b8",
      "attributes": {
        "__type": "api_quota",
        "api_member_id": "01a1d8eb-f034-7cd4-97fa-f8e6bfbe5cd9",
        "api_plan_id": "01a1d8e0-e793-75f0-ba75-1d103fa8994a",
        "bucket_key": "1:1:total_tokens:month:1793491200",
        "consumed": 42,
        "created_at": "2026-11-14T07:42:09.635099007Z",
        "maximum": 1000000,
        "metric": "total_tokens",
        "permission": 561441,
        "reference_id": "01a1d901-b2e2-7c65-b5eb-b21e07a014b8",
        "reserved": 0,
        "updated_at": "2026-11-14T07:42:11.464126841Z",
        "user_account_id": "01a1d46b-46f6-7b6c-a6ea-3f9fe8cc26fc",
        "version": 2,
        "window_end": "2026-12-01T00:00:00Z",
        "window_start": "2026-11-01T00:00:00Z"
      },
      "relationships": {
        "api_member_id": {
          "data": {
            "type": "api_member",
            "id": "01a1d8eb-f034-7cd4-97fa-f8e6bfbe5cd9"
          }
        },
        "api_plan_id": {
          "data": {
            "type": "api_plan",
            "id": "01a1d8e0-e793-75f0-ba75-1d103fa8994a"
          }
        },
        "user_account_id": {
          "data": {
            "type": "user_account",
            "id": "01a1d46b-46f6-7b6c-a6ea-3f9fe8cc26fc"
          }
        },
        "usergroup_id": {
          "data": []
        }
      }
    },
    {
      "type": "api_quota",
      "id": "01a1d901-b2e2-73cf-afc4-4c2e125802e0",
      "attributes": {
        "__type": "api_quota",
        "api_member_id": "01a1d8eb-f034-7cd4-97fa-f8e6bfbe5cd9",
        "api_plan_id": "01a1d8e0-e793-75f0-ba75-1d103fa8994a",
        "bucket_key": "1:1:requests:minute:1794642120",
        "consumed": 1,
        "created_at": "2026-11-14T07:42:09.634545632Z",
        "maximum": 60,
        "metric": "requests",
        "permission": 561441,
        "reference_id": "01a1d901-b2e2-73cf-afc4-4c2e125802e0",
        "reserved": 0,
        "updated_at": "2026-11-14T07:42:11.44913905Z",
        "user_account_id": "01a1d46b-46f6-7b6c-a6ea-3f9fe8cc26fc",
        "version": 2,
        "window_end": "2026-11-14T07:43:00Z",
        "window_start": "2026-11-14T07:42:00Z"
      },
      "relationships": {
        "api_member_id": {
          "data": {
            "type": "api_member",
            "id": "01a1d8eb-f034-7cd4-97fa-f8e6bfbe5cd9"
          }
        },
        "api_plan_id": {
          "data": {
            "type": "api_plan",
            "id": "01a1d8e0-e793-75f0-ba75-1d103fa8994a"
          }
        },
        "user_account_id": {
          "data": {
            "type": "user_account",
            "id": "01a1d46b-46f6-7b6c-a6ea-3f9fe8cc26fc"
          }
        },
        "usergroup_id": {
          "data": []
        }
      }
    },
    {
      "type": "api_quota",
      "id": "01a1d901-b2e0-773c-9c21-11248d657707",
      "attributes": {
        "__type": "api_quota",
        "api_member_id": "01a1d8eb-f034-7cd4-97fa-f8e6bfbe5cd9",
        "api_plan_id": "01a1d8e0-e793-75f0-ba75-1d103fa8994a",
        "bucket_key": "1:1:cost_micros:month:1793491200",
        "consumed": 72,
        "created_at": "2026-11-14T07:42:09.63303309Z",
        "maximum": 10000000,
        "metric": "cost_micros",
        "permission": 561441,
        "reference_id": "01a1d901-b2e0-773c-9c21-11248d657707",
        "reserved": 0,
        "updated_at": "2026-11-14T07:42:11.443380383Z",
        "user_account_id": "01a1d46b-46f6-7b6c-a6ea-3f9fe8cc26fc",
        "version": 2,
        "window_end": "2026-12-01T00:00:00Z",
        "window_start": "2026-11-01T00:00:00Z"
      },
      "relationships": {
        "api_member_id": {
          "data": {
            "type": "api_member",
            "id": "01a1d8eb-f034-7cd4-97fa-f8e6bfbe5cd9"
          }
        },
        "api_plan_id": {
          "data": {
            "type": "api_plan",
            "id": "01a1d8e0-e793-75f0-ba75-1d103fa8994a"
          }
        },
        "user_account_id": {
          "data": {
            "type": "user_account",
            "id": "01a1d46b-46f6-7b6c-a6ea-3f9fe8cc26fc"
          }
        },
        "usergroup_id": {
          "data": []
        }
      }
    }
  ]
}
```

</details>

### Understand quota totals and reservations

Each row tracks one metric for one time window. 
With this plan, look for requests in the current minute, tokens in the current month, and cost in the current month.

| Field | Meaning |
|---|---|
| `metric` | What this row counts, such as `requests` or `total_tokens` |
| `maximum` | The plan's limit for this metric and window |
| `consumed` | Usage already counted against the limit |
| `reserved` | Capacity held for requests that are still running |
| `window_start`, `window_end` | The UTC time range covered by this row |

The capacity available for another request is:

```text
maximum - consumed - reserved
```

For example, a token quota with a maximum of 1,000,000, consumed usage of 200,000, and reservations of 10,000 has 790,000 tokens available for new work.
Before calling the provider, Daptin estimates how much a request could use and reserves that amount. When the request finishes, it records the usage and releases the unused part of the reservation. 
This prevents simultaneous requests from all spending the same remaining budget.

A new request must fit within the available capacity. 
If it needs a reservation of 20,000 tokens but only 15,000 are available, Daptin returns HTTP 402 without calling the provider—even though the account has not consumed its entire limit.
Fallback attempts can increase the reservation needed. 
A conversation continuation also includes the earlier messages replayed to the model, so its input usage grows with the conversation.


### Test a hard limit

Temporarily change this plan to a zero-request hard limit:

```bash
curl -X PATCH http://localhost:6336/api/api_plan/PLAN_ID \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "api_plan",
      "id": "PLAN_ID",
      "attributes": {
        "limits": [
          {"metric":"requests","window":"minute","maximum":0,"mode":"hard"}
        ]
      }
    }
  }'
```

And repeat the /completions call with the user token, expect a "insufficient quota" response

```bash
curl http://localhost:6336/v1/chat/completions \
  -H "Authorization: Bearer $USER_DAPTIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "assistant",
    "messages": [{"role":"user","content":"Write a short greeting."}],
    "max_tokens": 128,
    "stream": false
  }'
```

Expected response (HTTP 402):

```json
{"error":{"code":"insufficient_quota","message":"insufficient quota","request_id":"req_1a0a162ad2e6393984bb78afe1db5368","type":"insufficient_quota"}}
```

An inactive or missing membership does means no plan-defined maximum applies so all requests are let thru.

## 10. Set operational limits

### Protect upstream deployments from overload

You can also apply upstream limits on LLM Deployment to protect upstream from overloading/ratelimiting. For example:

```bash
curl -X PATCH http://localhost:6336/api/llm_deployment/PRIMARY_DEPLOYMENT_ID \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/vnd.api+json' \
  -d '{
    "data": {
      "type": "llm_deployment",
      "id": "PRIMARY_DEPLOYMENT_ID",
      "attributes": {"rpm":60,"tpm":100000,"max_concurrency":4}
    }
  }'
```


### Add per-IP HTTP rate protection

The global limiter runs before authentication and is separate from account quotas. 
Its buckets are client IP plus request path, with one-second UTC windows.
The default is 500 requests per second per IP/path. 
This example sets five per second for each inference path:

```bash
curl http://localhost:6336/_config/backend/limit.rate \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "version": "1",
    "limits": {
      "/v1/chat/completions": 5,
      "/v1/responses": 5
    }
  }'
```

Change in rate limits require restart

For more detail, see [API Metering](https://github.com/daptin/daptin/blob/master/wiki/API-Metering.md)
and [Rate Limiting](https://github.com/daptin/daptin/blob/master/wiki/Rate-Limiting.md).
