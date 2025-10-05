#!/usr/bin/env node

/**
 * Load Testing Script for Splat Boston
 * 
 * Simulates multiple concurrent users painting tiles across different chunks.
 * Tests both single-chunk and multi-chunk scenarios.
 */

const WebSocket = require('ws');
const http = require('http');
const https = require('https');

// Simple fetch implementation using native http/https
function fetch(url, options = {}) {
  return new Promise((resolve, reject) => {
    const parsedUrl = new URL(url);
    const client = parsedUrl.protocol === 'https:' ? https : http;
    
    const req = client.request(url, {
      method: options.method || 'GET',
      headers: options.headers || {},
    }, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        resolve({
          ok: res.statusCode >= 200 && res.statusCode < 300,
          status: res.statusCode,
          statusText: res.statusMessage,
          text: () => Promise.resolve(data),
          json: () => Promise.resolve(JSON.parse(data)),
        });
      });
    });
    
    req.on('error', reject);
    
    if (options.body) {
      req.write(options.body);
    }
    
    req.end();
  });
}

// Configuration
const API_URL = process.env.API_URL || 'http://localhost:8080';
const WS_URL = process.env.WS_URL || 'ws://localhost:8080';
const NUM_USERS = parseInt(process.env.NUM_USERS || '10');
const PAINT_INTERVAL_MS = parseInt(process.env.PAINT_INTERVAL_MS || '6000');
const TEST_DURATION_MS = parseInt(process.env.TEST_DURATION_MS || '60000');
const BLOCK_SIZE = parseInt(process.env.BLOCK_SIZE || '5'); // Size of contiguous blocks to paint

// Greater Boston area locations
const TEST_LOCATIONS = {
  // Downtown Boston (chunk ~343, 612)
  downtown: [
    { lat: 42.3601, lon: -71.0589, name: 'Boston Common' },
    { lat: 42.3555, lon: -71.0605, name: 'Chinatown' },
    { lat: 42.3586, lon: -71.0567, name: 'Financial District' },
  ],
  
  // Cambridge (chunk ~342, 611)
  cambridge: [
    { lat: 42.3736, lon: -71.1097, name: 'Harvard Square' },
    { lat: 42.3656, lon: -71.1031, name: 'Central Square' },
    { lat: 42.3598, lon: -71.0927, name: 'Kendall Square' },
  ],
  
  // South Boston (chunk ~344, 613)
  southBoston: [
    { lat: 42.3334, lon: -71.0448, name: 'South Boston' },
    { lat: 42.3407, lon: -71.0382, name: 'Seaport' },
  ],
  
  // Back Bay (chunk ~343, 611)
  backBay: [
    { lat: 42.3505, lon: -71.0763, name: 'Copley Square' },
    { lat: 42.3467, lon: -71.0827, name: 'Fenway' },
  ],
  
  // Somerville (chunk ~342, 610)
  somerville: [
    { lat: 42.3875, lon: -71.0995, name: 'Davis Square' },
    { lat: 42.3954, lon: -71.1218, name: 'Union Square' },
  ],
};

// Color palette (1-8)
const COLORS = [1, 2, 3, 4, 5, 6, 7, 8];

// Statistics
const stats = {
  paintRequests: 0,
  paintSuccess: 0,
  paintFailed: 0,
  paintCooldown: 0,
  paintGeofence: 0,
  wsConnections: 0,
  wsMessages: 0,
  wsErrors: 0,
  latencies: [],
  errors: {},
};

// Coordinate conversion (simplified - matches frontend)
function latLonToTileXY(lat, lon) {
  const R = 6378137;
  const metersPerTile = 10;
  const originLat = 0;
  const originLon = 0;
  
  const latRad = (lat * Math.PI) / 180;
  const originLatRad = (originLat * Math.PI) / 180;
  
  const x = R * ((lon - originLon) * Math.PI / 180);
  const y = R * Math.log(Math.tan(Math.PI / 4 + latRad / 2) / Math.tan(Math.PI / 4 + originLatRad / 2));
  
  return {
    x: Math.floor(x / metersPerTile),
    y: Math.floor(y / metersPerTile),
  };
}

function chunkOf(x, y) {
  const CHUNK_SIZE = 256;
  return {
    cx: Math.floor(x / CHUNK_SIZE),
    cy: Math.floor(y / CHUNK_SIZE),
  };
}

function offsetOf(x, y) {
  const CHUNK_SIZE = 256;
  const localX = ((x % CHUNK_SIZE) + CHUNK_SIZE) % CHUNK_SIZE;
  const localY = ((y % CHUNK_SIZE) + CHUNK_SIZE) % CHUNK_SIZE;
  return localY * CHUNK_SIZE + localX;
}

// Simulate a single user
class SimulatedUser {
  constructor(id, location) {
    this.id = id;
    this.location = location;
    this.ws = null;
    this.paintCount = 0;
    this.active = true;
  }
  
  async start() {
    console.log(`[User ${this.id}] Starting at ${this.location.name} (${this.location.lat}, ${this.location.lon})`);
    
    // Calculate chunk and starting tile
    const { x, y } = latLonToTileXY(this.location.lat, this.location.lon);
    const { cx, cy } = chunkOf(x, y);
    
    // Pick a random starting position for this user's block
    this.blockStartX = x + Math.floor((Math.random() - 0.5) * 20);
    this.blockStartY = y + Math.floor((Math.random() - 0.5) * 20);
    this.blockIndex = 0;
    this.blockSize = BLOCK_SIZE;
    
    console.log(`[User ${this.id}] Chunk: (${cx}, ${cy}) - Will paint ${this.blockSize}x${this.blockSize} block starting at tile (${this.blockStartX}, ${this.blockStartY})`);
    
    // Subscribe to WebSocket
    this.subscribeToChunk(cx, cy);
    
    // Start painting at intervals
    this.paintInterval = setInterval(() => {
      if (this.active) {
        this.paint();
      }
    }, PAINT_INTERVAL_MS + Math.random() * 2000); // Add jitter
    
    // Paint immediately
    this.paint();
  }
  
  subscribeToChunk(cx, cy) {
    const wsUrl = `${WS_URL}/sub?cx=${cx}&cy=${cy}`;
    
    try {
      this.ws = new WebSocket(wsUrl);
      
      this.ws.on('open', () => {
        console.log(`[User ${this.id}] WebSocket connected to chunk (${cx}, ${cy})`);
        stats.wsConnections++;
      });
      
      this.ws.on('message', (data) => {
        const delta = JSON.parse(data.toString());
        stats.wsMessages++;
        
        // Check if it's our paint
        if (delta.o !== undefined) {
          // console.log(`[User ${this.id}] Received delta: offset=${delta.o}, color=${delta.color}, seq=${delta.seq}`);
        }
      });
      
      this.ws.on('error', (error) => {
        console.error(`[User ${this.id}] WebSocket error:`, error.message);
        stats.wsErrors++;
      });
      
      this.ws.on('close', () => {
        console.log(`[User ${this.id}] WebSocket closed`);
      });
    } catch (error) {
      console.error(`[User ${this.id}] Failed to create WebSocket:`, error.message);
      stats.wsErrors++;
    }
  }
  
  async paint() {
    // Paint tiles in a contiguous block pattern
    const totalTiles = this.blockSize * this.blockSize;
    
    if (this.blockIndex >= totalTiles) {
      // Finished current block, start a new one nearby
      const { x, y } = latLonToTileXY(this.location.lat, this.location.lon);
      this.blockStartX = x + Math.floor((Math.random() - 0.5) * 30);
      this.blockStartY = y + Math.floor((Math.random() - 0.5) * 30);
      this.blockIndex = 0;
      console.log(`[User ${this.id}] Starting new ${this.blockSize}x${this.blockSize} block at tile (${this.blockStartX}, ${this.blockStartY})`);
    }
    
    // Calculate position within the block
    const blockX = this.blockIndex % this.blockSize;
    const blockY = Math.floor(this.blockIndex / this.blockSize);
    
    const x = this.blockStartX + blockX;
    const y = this.blockStartY + blockY;
    
    const { cx, cy } = chunkOf(x, y);
    const o = offsetOf(x, y);
    
    // Use consistent color for this user's blocks
    const color = COLORS[this.id % COLORS.length];
    
    this.blockIndex++;
    
    const request = {
      lat: this.location.lat,
      lon: this.location.lon,
      cx,
      cy,
      o,
      color,
      turnstileToken: '',
    };
    
    stats.paintRequests++;
    const startTime = Date.now();
    
    try {
      const response = await fetch(`${API_URL}/paint`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(request),
      });
      
      const latency = Date.now() - startTime;
      stats.latencies.push(latency);
      
      if (response.ok) {
        const result = await response.json();
        stats.paintSuccess++;
        this.paintCount++;
        console.log(`[User ${this.id}] Paint success: offset=${o}, color=${color}, seq=${result.seq}, latency=${latency}ms`);
      } else {
        const errorText = await response.text();
        
        if (response.status === 429) {
          stats.paintCooldown++;
          // console.log(`[User ${this.id}] Cooldown active`);
        } else if (response.status === 403) {
          stats.paintGeofence++;
          console.log(`[User ${this.id}] Geofence error: ${errorText}`);
        } else {
          stats.paintFailed++;
          console.error(`[User ${this.id}] Paint failed (${response.status}): ${errorText}`);
          
          const errorKey = `${response.status}`;
          stats.errors[errorKey] = (stats.errors[errorKey] || 0) + 1;
        }
      }
    } catch (error) {
      stats.paintFailed++;
      console.error(`[User ${this.id}] Paint error:`, error.message);
      
      stats.errors[error.message] = (stats.errors[error.message] || 0) + 1;
    }
  }
  
  stop() {
    this.active = false;
    
    if (this.paintInterval) {
      clearInterval(this.paintInterval);
    }
    
    if (this.ws) {
      this.ws.close();
    }
    
    console.log(`[User ${this.id}] Stopped. Painted ${this.paintCount} tiles.`);
  }
}

// Main test function
async function runLoadTest() {
  console.log('='.repeat(60));
  console.log('Splat Boston Load Test');
  console.log('='.repeat(60));
  console.log(`API URL: ${API_URL}`);
  console.log(`WS URL: ${WS_URL}`);
  console.log(`Number of users: ${NUM_USERS}`);
  console.log(`Paint interval: ${PAINT_INTERVAL_MS}ms`);
  console.log(`Test duration: ${TEST_DURATION_MS}ms`);
  console.log(`Block size: ${BLOCK_SIZE}x${BLOCK_SIZE} (${BLOCK_SIZE * BLOCK_SIZE} tiles per block)`);
  console.log('='.repeat(60));
  console.log('');
  
  // Health check
  console.log('Checking backend health...');
  try {
    const response = await fetch(`${API_URL}/healthz`);
    if (response.ok) {
      console.log('✓ Backend is healthy\n');
    } else {
      console.error('✗ Backend health check failed');
      process.exit(1);
    }
  } catch (error) {
    console.error('✗ Backend is not reachable:', error.message);
    process.exit(1);
  }
  
  // Flatten all locations
  const allLocations = [];
  Object.values(TEST_LOCATIONS).forEach(locations => {
    allLocations.push(...locations);
  });
  
  // Create users distributed across locations
  const users = [];
  for (let i = 0; i < NUM_USERS; i++) {
    const location = allLocations[i % allLocations.length];
    const user = new SimulatedUser(i + 1, location);
    users.push(user);
  }
  
  // Start all users with staggered start times
  console.log(`Starting ${NUM_USERS} users...\n`);
  for (let i = 0; i < users.length; i++) {
    setTimeout(() => {
      users[i].start();
    }, i * 200); // Stagger by 200ms each
  }
  
  // Run for specified duration
  await new Promise(resolve => setTimeout(resolve, TEST_DURATION_MS));
  
  // Stop all users
  console.log('\nStopping all users...\n');
  users.forEach(user => user.stop());
  
  // Wait for cleanup
  await new Promise(resolve => setTimeout(resolve, 1000));
  
  // Print statistics
  console.log('');
  console.log('='.repeat(60));
  console.log('Load Test Results');
  console.log('='.repeat(60));
  console.log(`Total paint requests: ${stats.paintRequests}`);
  console.log(`Successful paints: ${stats.paintSuccess} (${(stats.paintSuccess / stats.paintRequests * 100).toFixed(1)}%)`);
  console.log(`Failed paints: ${stats.paintFailed}`);
  console.log(`Cooldown rejections: ${stats.paintCooldown}`);
  console.log(`Geofence rejections: ${stats.paintGeofence}`);
  console.log('');
  console.log(`WebSocket connections: ${stats.wsConnections}`);
  console.log(`WebSocket messages received: ${stats.wsMessages}`);
  console.log(`WebSocket errors: ${stats.wsErrors}`);
  console.log('');
  
  if (stats.latencies.length > 0) {
    stats.latencies.sort((a, b) => a - b);
    const p50 = stats.latencies[Math.floor(stats.latencies.length * 0.5)];
    const p95 = stats.latencies[Math.floor(stats.latencies.length * 0.95)];
    const p99 = stats.latencies[Math.floor(stats.latencies.length * 0.99)];
    const avg = stats.latencies.reduce((a, b) => a + b, 0) / stats.latencies.length;
    const min = stats.latencies[0];
    const max = stats.latencies[stats.latencies.length - 1];
    
    console.log('Paint Latencies:');
    console.log(`  Min: ${min}ms`);
    console.log(`  Avg: ${avg.toFixed(1)}ms`);
    console.log(`  p50: ${p50}ms`);
    console.log(`  p95: ${p95}ms`);
    console.log(`  p99: ${p99}ms`);
    console.log(`  Max: ${max}ms`);
    console.log('');
  }
  
  if (Object.keys(stats.errors).length > 0) {
    console.log('Errors:');
    Object.entries(stats.errors).forEach(([error, count]) => {
      console.log(`  ${error}: ${count}`);
    });
    console.log('');
  }
  
  console.log('='.repeat(60));
  
  process.exit(0);
}

// Run the test
if (require.main === module) {
  runLoadTest().catch(error => {
    console.error('Load test failed:', error);
    process.exit(1);
  });
}

module.exports = { runLoadTest };

