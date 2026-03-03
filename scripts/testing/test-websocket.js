#!/usr/bin/env node
/**
 * WebSocket Testing Client for Daptin
 * Tests real-time features including subscriptions, custom topics, and event filtering
 *
 * Wire protocol (v2):
 *   Client→Server: { id, method, attributes }
 *   Server→Client: { type:"response"|"event"|"session"|"pong", ... }
 */

const WebSocket = require('ws');
const http = require('http');

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

let reqCounter = 0;
function nextId() {
  return `req-${++reqCounter}`;
}

class DaptinWebSocketTester {
  constructor(token) {
    this.token = token;
    this.ws = null;
    this.receivedMessages = [];
    this.testResults = {};
    this.sessionInfo = null;
  }

  connect() {
    return new Promise((resolve, reject) => {
      const url = `${WS_URL}?token=${this.token}`;
      log('test', `Connecting to ${url}`);

      this.ws = new WebSocket(url);

      this.ws.on('open', () => {
        log('success', 'WebSocket connection established');
        this.testResults['connection'] = true;
        // don't resolve yet — wait for session-open
      });

      this.ws.on('message', (data) => {
        const message = JSON.parse(data.toString());
        this.receivedMessages.push(message);

        // handle session-open as first message
        if (message.type === 'session' && message.status === 'open' && !this.sessionInfo) {
          this.sessionInfo = message.data;
          log('success', 'Session opened', message.data);
          this.testResults['session_open'] = true;
          resolve();
          return;
        }

        log('event', `Message received [type=${message.type}]`, message);
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
        if (!this.sessionInfo) {
          reject(new Error('Connection/session timeout'));
        }
      }, 5000);
    });
  }

  send(method, attributes) {
    const id = nextId();
    const payload = { id, method, attributes };
    log('info', `Sending: ${method} (id=${id})`, payload);
    this.ws.send(JSON.stringify(payload));
    return id;
  }

  async waitForResponse(sentId, timeout = 5000) {
    const start = Date.now();
    while (Date.now() - start < timeout) {
      const msg = this.receivedMessages.find(m =>
        m.type === 'response' && m.id === sentId
      );
      if (msg) return msg;
      await this.sleep(50);
    }
    return null;
  }

  async testPingPong() {
    log('test', 'Test: Ping/Pong');
    const beforeCount = this.receivedMessages.length;
    this.ws.send(JSON.stringify({ method: 'ping' }));
    await this.waitForMessage(beforeCount, 3000);
    const response = this.receivedMessages[this.receivedMessages.length - 1];
    if (response && response.type === 'pong') {
      log('success', 'Pong received');
      this.testResults['ping_pong'] = true;
    } else {
      log('error', 'Failed to get pong', response);
      this.testResults['ping_pong'] = false;
    }
  }

  async testSubscribe(topicName) {
    log('test', `Test: Subscribe to topic '${topicName}'`);
    const id = this.send('subscribe', { topicName });
    const response = await this.waitForResponse(id);
    if (response && response.ok === true && response.method === 'subscribe') {
      log('success', `Subscribed to ${topicName}`, response.data);
      this.testResults[`subscribe_${topicName}`] = true;
    } else {
      log('error', `Subscribe failed for ${topicName}`, response);
      this.testResults[`subscribe_${topicName}`] = false;
    }
  }

  async testSubscribeMultiple(topics) {
    log('test', `Test: Subscribe to multiple topics: ${topics.join(', ')}`);
    const id = this.send('subscribe', { topicName: topics.join(',') });
    // expect one response per topic
    await this.sleep(2000);
    const responses = this.receivedMessages.filter(m =>
      m.type === 'response' && m.method === 'subscribe'
    );
    this.testResults['subscribe_multiple'] = responses.length >= topics.length;
    log(responses.length >= topics.length ? 'success' : 'error',
      `Got ${responses.length} subscribe responses for ${topics.length} topics`);
  }

  async testCreateTopic(topicName) {
    log('test', `Test: Create custom topic '${topicName}'`);
    const id = this.send('create-topicName', { name: topicName });
    const response = await this.waitForResponse(id);
    if (response && response.ok === true) {
      log('success', `Topic ${topicName} created`, response.data);
      this.testResults[`create_topic_${topicName}`] = true;
    } else {
      log('error', `Create topic failed`, response);
      this.testResults[`create_topic_${topicName}`] = false;
    }
  }

  async testPublishMessage(topicName, message) {
    log('test', `Test: Publish message to '${topicName}'`);
    const id = this.send('new-message', { topicName, message });
    await this.sleep(1000);
    this.testResults[`publish_${topicName}`] = true;
  }

  async testUnsubscribe(topicName) {
    log('test', `Test: Unsubscribe from '${topicName}'`);
    const id = this.send('unsubscribe', { topicName });
    const response = await this.waitForResponse(id);
    if (response && response.ok === true && response.method === 'unsubscribe') {
      log('success', `Unsubscribed from ${topicName}`);
      this.testResults[`unsubscribe_${topicName}`] = true;
    } else {
      log('error', `Unsubscribe failed`, response);
      this.testResults[`unsubscribe_${topicName}`] = false;
    }
  }

  async testDestroyTopic(topicName) {
    log('test', `Test: Destroy custom topic '${topicName}'`);
    const id = this.send('destroy-topicName', { name: topicName });
    const response = await this.waitForResponse(id);
    if (response && response.ok === true) {
      log('success', `Topic ${topicName} destroyed`);
      this.testResults[`destroy_topic_${topicName}`] = true;
    } else {
      log('error', `Destroy topic failed`, response);
      this.testResults[`destroy_topic_${topicName}`] = false;
    }
  }

  async testSubscribeWithFilter(topicName, filters) {
    log('test', `Test: Subscribe to '${topicName}' with filters`, filters);
    const id = this.send('subscribe', { topicName, filters });
    const response = await this.waitForResponse(id);
    if (response && response.ok === true) {
      log('success', `Subscribed with filter to ${topicName}`);
      this.testResults[`subscribe_filter_${topicName}`] = true;
    } else {
      log('error', `Subscribe with filter failed`, response);
      this.testResults[`subscribe_filter_${topicName}`] = false;
    }
  }

  async testNoSuchMethod() {
    log('test', 'Test: Unknown method returns error');
    const id = this.send('nonexistent-method', {});
    const response = await this.waitForResponse(id);
    if (response && response.ok === false && response.error === 'no such method') {
      log('success', 'Got expected error for unknown method');
      this.testResults['no_such_method'] = true;
    } else {
      log('error', 'Unexpected response for unknown method', response);
      this.testResults['no_such_method'] = false;
    }
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

// Main test suite
async function runTests() {
  const token = process.env.TOKEN || process.argv[2];

  if (!token) {
    log('error', 'No token provided. Usage: TOKEN=xxx node test-websocket.js or node test-websocket.js <token>');
    process.exit(1);
  }

  log('info', 'Starting WebSocket test suite (v2 wire protocol)');
  log('info', `Server: ${BASE_URL}`);

  const tester = new DaptinWebSocketTester(token);

  try {
    // Test 1: Connect + session-open
    await tester.connect();
    await tester.sleep(500);

    // Test 2: Ping/Pong
    await tester.testPingPong();

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

    // Test 8: Subscribe with filter
    await tester.testSubscribeWithFilter('world', { EventType: 'create' });

    // Test 9: Unknown method
    await tester.testNoSuchMethod();

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
