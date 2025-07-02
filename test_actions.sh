#!/bin/bash

TOKEN=$(cat fresh_token.txt)

echo "=== Testing Daptin Actions ==="
echo

# Test random data generation
echo "1. Testing random data generation action:"
curl -X POST http://localhost:6336/action/world/generate_random_data \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "count": 5,
      "entity": "user"
    }
  }' -s | jq '.'

echo
echo "2. Testing JWT token generation:"
curl -X POST http://localhost:6336/action/user_account/generate_jwt_token \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {}
  }' -s | jq '.'

echo
echo "3. Testing OTP generation:"
curl -X POST http://localhost:6336/action/user_account/otp_generate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {}
  }' -s | jq '.'

echo
echo "4. Testing export data action:"
curl -X POST http://localhost:6336/action/world/export_data \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "table_name": "user_account"
    }
  }' -s | jq '.'

echo
echo "5. Testing mail send action (will fail without mail server):"
curl -X POST http://localhost:6336/action/mail/send \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "to": ["test@example.com"],
      "from": "admin@test.com",
      "subject": "Test Email",
      "body": "This is a test email from Daptin actions"
    }
  }' -s | jq '.'