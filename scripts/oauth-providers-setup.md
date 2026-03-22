# 100xbot OAuth Provider Setup

Replicable setup for all OAuth providers. Use these exact API calls on any Daptin instance.

Base URL: `https://<daptin-host>` (local: `http://localhost:6336`)

---

## 1. Google

**Dev console**: https://console.cloud.google.com/apis/credentials

```bash
TOKEN=$(cat /tmp/daptin-token.txt)
curl -X POST $BASE_URL/api/oauth_connect \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "oauth_connect",
      "attributes": {
        "name": "google",
        "client_id": "641171012177-83pvcdmprl2f0crvb75u63cdqqbdd9st.apps.googleusercontent.com",
        "client_secret": "Mx72yi2gSO8pgetHG9xoSjPVneapsFxOlflaaguUx9cO4VKaNLDP8WXpMu1wzWtX08mx",
        "scope": "https://www.googleapis.com/auth/spreadsheets https://www.googleapis.com/auth/drive https://www.googleapis.com/auth/documents https://www.googleapis.com/auth/gmail.send https://www.googleapis.com/auth/gmail.readonly https://www.googleapis.com/auth/calendar https://www.googleapis.com/auth/tasks",
        "auth_url": "https://accounts.google.com/o/oauth2/auth",
        "token_url": "https://oauth2.googleapis.com/token",
        "profile_url": "https://www.googleapis.com/oauth2/v1/userinfo?alt=json",
        "profile_email_path": "email",
        "redirect_uri": "https://kipkglfnhnpbogckhlmikjlfpbngnioc.chromiumapp.org/",
        "response_type": "code",
        "allow_login": true,
        "access_type_offline": true
      }
    }
  }'
```

---

## 2. GitHub

**Dev console**: https://github.com/settings/developers

```bash
TOKEN=$(cat /tmp/daptin-token.txt)
curl -X POST $BASE_URL/api/oauth_connect \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "oauth_connect",
      "attributes": {
        "name": "github",
        "client_id": "Ov23liCvxqBa0tBcTSIG",
        "client_secret": "1d135ba9691eaa1251908833ccaa5de8a3650cbf",
        "scope": "repo gist user read:org",
        "auth_url": "https://github.com/login/oauth/authorize",
        "token_url": "https://github.com/login/oauth/access_token",
        "profile_url": "https://api.github.com/user",
        "profile_email_path": "email",
        "redirect_uri": "https://kipkglfnhnpbogckhlmikjlfpbngnioc.chromiumapp.org/",
        "response_type": "code",
        "allow_login": true,
        "access_type_offline": false
      }
    }
  }'
```

---

## 3. Microsoft (Azure AD)

**Dev console**: https://portal.azure.com → App registrations

```bash
TOKEN=$(cat /tmp/daptin-token.txt)
curl -X POST $BASE_URL/api/oauth_connect \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "oauth_connect",
      "attributes": {
        "name": "microsoft",
        "client_id": "",
        "client_secret": "",
        "scope": "openid email profile Mail.ReadWrite Calendars.ReadWrite Files.ReadWrite User.Read offline_access",
        "auth_url": "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
        "token_url": "https://login.microsoftonline.com/common/oauth2/v2.0/token",
        "profile_url": "https://graph.microsoft.com/v1.0/me",
        "profile_email_path": "mail",
        "redirect_uri": "/oauth/response",
        "response_type": "code",
        "allow_login": true,
        "access_type_offline": true
      }
    }
  }'
```

---

## 4. Notion

**Dev console**: https://www.notion.so/my-integrations

```bash
TOKEN=$(cat /tmp/daptin-token.txt)
curl -X POST $BASE_URL/api/oauth_connect \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "oauth_connect",
      "attributes": {
        "name": "notion",
        "client_id": "32bd872b-594c-8119-ab21-00372e6558d0",
        "client_secret": "secret_OC9Mj3b0F66yxwW7wKFduwyw0b75QkHmWH38KSn7fC",
        "scope": "",
        "auth_url": "https://api.notion.com/v1/oauth/authorize",
        "token_url": "https://api.notion.com/v1/oauth/token",
        "profile_url": "https://api.notion.com/v1/users/me",
        "profile_email_path": "person.email",
        "redirect_uri": "https://kipkglfnhnpbogckhlmikjlfpbngnioc.chromiumapp.org/",
        "response_type": "code",
        "allow_login": false,
        "access_type_offline": false
      }
    }
  }'
```

---

## 5. Slack

**Dev console**: https://api.slack.com/apps

```bash
TOKEN=$(cat /tmp/daptin-token.txt)
curl -X POST $BASE_URL/api/oauth_connect \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "oauth_connect",
      "attributes": {
        "name": "slack",
        "client_id": "",
        "client_secret": "",
        "scope": "chat:write channels:read channels:history users:read",
        "auth_url": "https://slack.com/oauth/v2/authorize",
        "token_url": "https://slack.com/api/oauth.v2.access",
        "profile_url": "https://slack.com/api/auth.test",
        "profile_email_path": "user_id",
        "redirect_uri": "/oauth/response",
        "response_type": "code",
        "allow_login": false,
        "access_type_offline": false
      }
    }
  }'
```

---

## 6. Linear

**Dev console**: https://linear.app/settings → API → OAuth Applications

```bash
TOKEN=$(cat /tmp/daptin-token.txt)
curl -X POST $BASE_URL/api/oauth_connect \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "oauth_connect",
      "attributes": {
        "name": "linear",
        "client_id": "",
        "client_secret": "",
        "scope": "read write",
        "auth_url": "https://linear.app/oauth/authorize",
        "token_url": "https://api.linear.app/oauth/token",
        "profile_url": "https://api.linear.app/graphql",
        "profile_email_path": "data.viewer.email",
        "redirect_uri": "/oauth/response",
        "response_type": "code",
        "allow_login": false,
        "access_type_offline": false
      }
    }
  }'
```

---

## 7. Spotify

**Dev console**: https://developer.spotify.com/dashboard

```bash
TOKEN=$(cat /tmp/daptin-token.txt)
curl -X POST $BASE_URL/api/oauth_connect \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "oauth_connect",
      "attributes": {
        "name": "spotify",
        "client_id": "",
        "client_secret": "",
        "scope": "user-read-playback-state user-modify-playback-state playlist-modify-public playlist-read-private",
        "auth_url": "https://accounts.spotify.com/authorize",
        "token_url": "https://accounts.spotify.com/api/token",
        "profile_url": "https://api.spotify.com/v1/me",
        "profile_email_path": "email",
        "redirect_uri": "/oauth/response",
        "response_type": "code",
        "allow_login": false,
        "access_type_offline": true
      }
    }
  }'
```

---

## Verification

After creating each provider:

```bash
# List all providers
./scripts/testing/test-runner.sh get /api/oauth_connect | jq '[.data[].attributes.name]'

# Test OAuth flow (gets redirect URL)
./scripts/testing/test-runner.sh action oauth_connect oauth_login_begin '{}' <record-id>

# After completing OAuth in browser, check token
./scripts/testing/test-runner.sh get /api/oauth_token

# Retrieve decrypted token
./scripts/testing/test-runner.sh action oauth_token get_token '{}' <token-record-id>
```
