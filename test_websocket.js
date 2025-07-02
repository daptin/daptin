const WebSocket = require('ws');

// Test WebSocket connection to Daptin
const token = process.argv[2];
if (!token) {
  console.error('Usage: node test_websocket.js <JWT_TOKEN>');
  process.exit(1);
}

const ws = new WebSocket('ws://localhost:6336/live', {
  headers: {
    'Authorization': `Bearer ${token}`
  }
});

ws.on('open', () => {
  console.log('✅ Connected to Daptin WebSocket server');
  
  // Test 1: List topics
  console.log('\n📋 Testing: List topics');
  ws.send(JSON.stringify({
    method: 'list-topicName',
    attributes: {}
  }));
  
  // Test 2: Subscribe to a topic
  setTimeout(() => {
    console.log('\n📢 Testing: Subscribe to user_account topic');
    ws.send(JSON.stringify({
      method: 'subscribe',
      attributes: {
        topicName: 'user_account'
      }
    }));
  }, 1000);
  
  // Test 3: Create custom topic
  setTimeout(() => {
    console.log('\n➕ Testing: Create custom topic');
    ws.send(JSON.stringify({
      method: 'create-topicName',
      attributes: {
        name: 'test-channel'
      }
    }));
  }, 2000);
  
  // Test 4: Publish to custom topic
  setTimeout(() => {
    console.log('\n✉️ Testing: Publish message');
    ws.send(JSON.stringify({
      method: 'new-message',
      attributes: {
        topicName: 'test-channel',
        message: 'Hello from Daptin WebSocket!'
      }
    }));
  }, 3000);
  
  // Close after tests
  setTimeout(() => {
    console.log('\n👋 Closing connection');
    ws.close();
  }, 5000);
});

ws.on('message', (data) => {
  console.log('\n📨 Received:', JSON.parse(data.toString()));
});

ws.on('error', (error) => {
  console.error('❌ WebSocket error:', error.message);
});

ws.on('close', () => {
  console.log('🔌 Connection closed');
});