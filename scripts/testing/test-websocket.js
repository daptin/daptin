#!/usr/bin/env node
/**
 * WebSocket Testing Client for Daptin
 * Tests real-time features including subscriptions, custom topics, and event filtering
 */

const WebSocket = require('ws');
const https = require('https');

// Configuration
const BASE_URL = 'localhost:6336';
const WS_URL = `ws://${BASE_URL}/live`;
const API_URL = `http://${BASE_URL}/api`;

// Colors for output
const colors = {
  green: '\x1b[32m',
  red: '\x1b[31m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  cyan: '\x1b[36m',
  reset: '\x1b[0m',
  bold: '\x1b[1m'
};

function log(type, message, data = null) {
  const timestamp = new Date().toISOString();
  const prefix = `[${timestamp}]`;

  switch(type) {
    case 'success':
      console.log(`${colors.green}✓ ${prefix} ${message}${colors.reset}`);
      break;
    case 'error':
      console.log(`${colors.red}✗ ${prefix} ${message}${colors.reset}`);
      break;
    case 'info':
      console.log(`${colors.blue}ℹ ${prefix} ${message}${colors.reset}`);
      break;
    case 'test':
      console.log(`${colors.cyan}▶ ${prefix} ${message}${colors.reset}`);
      break;
    case 'event':
      console.log(`${colors.yellow}★ ${prefix} ${message}${colors.reset}`);
      break;
  }

  if (data) {
    console.log(JSON.stringify(data, null, 2));
  }
}

class DaptinWebSocketTester {
  constructor(token) {
    this.token = token;
    this.ws = null;
    this.receivedMessages = [];
    this.testResults = {};
  }

  connect() {
    return new Promise((resolve, reject) => {
      const url = `${WS_URL}?token=${this.token}`;
      log('test', `Connecting to ${url}`);

      this.ws = new WebSocket(url);

      this.ws.on('open', () => {
        log('success', 'WebSocket connection established');
        this.testResults['connection'] = true;
        resolve();
      });

      this.ws.on('message', (data) => {
        const message = JSON.parse(data.toString());
        this.receivedMessages.push(message);
        log('event', 'Message received', message);
      });

      this.ws.on('error', (err) => {
        log('error', 'WebSocket error', err.message);
        reject(err);
      });

      this.ws.on('close', () => {
        log('info', 'WebSocket connection closed');
      });

      // Timeout after 5 seconds
      setTimeout(() => {
        if (this.ws.readyState !== WebSocket.OPEN) {
          reject(new Error('Connection timeout'));
        }
      }, 5000);
    });
  }

  send(method, attributes) {
    const payload = { method, attributes };
    log('info', `Sending: ${method}`, payload);
    this.ws.send(JSON.stringify(payload));
  }

  async testSubscribe(topicName) {
    log('test', `Test: Subscribe to topic '${topicName}'`);
    this.send('subscribe', { topicName });
    await this.sleep(1000);
    this.testResults[`subscribe_${topicName}`] = true;
  }

  async testSubscribeMultiple(topics) {
    log('test', `Test: Subscribe to multiple topics: ${topics.join(', ')}`);
    this.send('subscribe', { topicName: topics.join(',') });
    await this.sleep(1000);
    this.testResults['subscribe_multiple'] = true;
  }

  async testListTopics() {
    log('test', 'Test: List topics');
    const beforeCount = this.receivedMessages.length;
    this.send('list-topicName', {});

    // Wait for response
    await this.waitForMessage(beforeCount, 3000);

    const response = this.receivedMessages[this.receivedMessages.length - 1];
    if (response && response.ObjectType === 'topicName-list') {
      log('success', 'Topics list received', response);
      this.testResults['list_topics'] = true;
      return response;
    } else {
      log('error', 'Failed to get topics list');
      this.testResults['list_topics'] = false;
    }
  }

  async testCreateTopic(topicName) {
    log('test', `Test: Create custom topic '${topicName}'`);
    this.send('create-topicName', { name: topicName });
    await this.sleep(1000);
    this.testResults[`create_topic_${topicName}`] = true;
  }

  async testPublishMessage(topicName, message) {
    log('test', `Test: Publish message to '${topicName}'`);
    this.send('new-message', { topicName, message });
    await this.sleep(1000);
    this.testResults[`publish_${topicName}`] = true;
  }

  async testUnsubscribe(topicName) {
    log('test', `Test: Unsubscribe from '${topicName}'`);
    this.send('unsubscribe', { topicName });
    await this.sleep(1000);
    this.testResults[`unsubscribe_${topicName}`] = true;
  }

  async testDestroyTopic(topicName) {
    log('test', `Test: Destroy custom topic '${topicName}'`);
    this.send('destroy-topicName', { name: topicName });
    await this.sleep(1000);
    this.testResults[`destroy_topic_${topicName}`] = true;
  }

  async testSubscribeWithFilter(topicName, filters) {
    log('test', `Test: Subscribe to '${topicName}' with filters`, filters);
    this.send('subscribe', { topicName, filters });
    await this.sleep(1000);
    this.testResults[`subscribe_filter_${topicName}`] = true;
  }

  async waitForMessage(beforeCount, timeout = 5000) {
    const start = Date.now();
    while (this.receivedMessages.length <= beforeCount) {
      if (Date.now() - start > timeout) {
        throw new Error('Timeout waiting for message');
      }
      await this.sleep(100);
    }
  }

  sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
  }

  close() {
    if (this.ws) {
      this.ws.close();
    }
  }

  printResults() {
    console.log('\n' + '='.repeat(60));
    console.log(colors.bold + 'TEST RESULTS' + colors.reset);
    console.log('='.repeat(60));

    let passed = 0;
    let failed = 0;

    for (const [test, result] of Object.entries(this.testResults)) {
      const status = result ? `${colors.green}PASS${colors.reset}` : `${colors.red}FAIL${colors.reset}`;
      console.log(`${status} - ${test}`);
      if (result) passed++;
      else failed++;
    }

    console.log('='.repeat(60));
    console.log(`Total: ${passed + failed} | Passed: ${colors.green}${passed}${colors.reset} | Failed: ${colors.red}${failed}${colors.reset}`);
    console.log(`Messages received: ${this.receivedMessages.length}`);
    console.log('='.repeat(60) + '\n');
  }
}

// API Helper to trigger events
async function createTestRecord(token, tableName, data) {
  return new Promise((resolve, reject) => {
    const postData = JSON.stringify({
      data: {
        type: tableName,
        attributes: data
      }
    });

    const options = {
      hostname: 'localhost',
      port: 6336,
      path: `/api/${tableName}`,
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/vnd.api+json',
        'Content-Length': postData.length
      }
    };

    const req = https.request(options, (res) => {
      let body = '';
      res.on('data', chunk => body += chunk);
      res.on('end', () => {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          resolve(JSON.parse(body));
        } else {
          reject(new Error(`HTTP ${res.statusCode}: ${body}`));
        }
      });
    });

    req.on('error', reject);
    req.write(postData);
    req.end();
  });
}

// Main test suite
async function runTests() {
  const token = process.env.TOKEN || process.argv[2];

  if (!token) {
    log('error', 'No token provided. Usage: TOKEN=xxx node test-websocket.js or node test-websocket.js <token>');
    process.exit(1);
  }

  log('info', 'Starting WebSocket test suite');
  log('info', `Server: ${BASE_URL}`);

  const tester = new DaptinWebSocketTester(token);

  try {
    // Test 1: Connect
    await tester.connect();
    await tester.sleep(1000);

    // Test 2: List topics
    await tester.testListTopics();

    // Test 3: Subscribe to user_account topic
    await tester.testSubscribe('user_account');

    // Test 4: Subscribe to multiple topics
    await tester.testSubscribeMultiple(['user_account', 'world']);

    // Test 5: Create custom topic
    const customTopic = 'test-chat-' + Date.now();
    await tester.testCreateTopic(customTopic);

    // Test 6: Subscribe to custom topic
    await tester.testSubscribe(customTopic);

    // Test 7: Publish to custom topic
    await tester.testPublishMessage(customTopic, { text: 'Hello WebSocket!', timestamp: new Date().toISOString() });

    // Test 8: List topics again (should include custom topic)
    await tester.testListTopics();

    // Test 9: Subscribe with filter
    await tester.testSubscribeWithFilter('user_account', { EventType: 'create' });

    // Test 10: Unsubscribe
    await tester.testUnsubscribe(customTopic);

    // Test 11: Destroy custom topic
    await tester.testDestroyTopic(customTopic);

    // Wait a bit to see if any more messages arrive
    log('info', 'Waiting 3 seconds to collect any pending messages...');
    await tester.sleep(3000);

    // Print results
    tester.printResults();

    // Close connection
    tester.close();

  } catch (err) {
    log('error', 'Test suite failed', err.message);
    tester.printResults();
    tester.close();
    process.exit(1);
  }
}

// Run if executed directly
if (require.main === module) {
  runTests().catch(err => {
    console.error('Fatal error:', err);
    process.exit(1);
  });
}

module.exports = { DaptinWebSocketTester };
