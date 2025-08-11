import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');

// Test configuration
export const options = {
  scenarios: {
    // Steady load test
    steady_load: {
      executor: 'constant-vus',
      vus: 10, // 10 virtual users
      duration: '2m', // for 2 minutes
      gracefulStop: '10s',
    },
    // Spike test
    spike_load: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 20 }, // Ramp up to 20 users
        { duration: '1m', target: 50 },  // Spike to 50 users
        { duration: '30s', target: 0 },  // Ramp down
      ],
      gracefulStop: '10s',
      startTime: '3m', // Start after steady load finishes
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<5000'], // 95% of requests under 5s
    http_req_failed: ['rate<0.05'],    // Less than 5% errors
    errors: ['rate<0.1'],              // Less than 10% custom errors
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Test data - simple chat completion
const chatPayload = {
  model: 'gpt-3.5-turbo',
  messages: [
    {
      role: 'user',
      content: 'Hello! How are you today?'
    }
  ],
  max_tokens: 100,
  temperature: 0, // Deterministic for caching
};

// Test data - different message for cache miss testing
const chatPayloadVariations = [
  {
    model: 'gpt-3.5-turbo',
    messages: [{ role: 'user', content: 'What is the capital of France?' }],
    max_tokens: 50,
    temperature: 0,
  },
  {
    model: 'gpt-3.5-turbo',
    messages: [{ role: 'user', content: 'Explain quantum computing in simple terms.' }],
    max_tokens: 150,
    temperature: 0,
  },
  {
    model: 'gpt-3.5-turbo',
    messages: [{ role: 'user', content: 'Tell me a joke about programming.' }],
    max_tokens: 100,
    temperature: 0.7, // Non-deterministic - should not be cached
  },
];

export default function () {
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${__ENV.OPENAI_API_KEY || 'test-key'}`,
    },
    timeout: '30s', // LLM requests can be slow
  };

  // Test scenarios with different probabilities
  const scenario = Math.random();
  
  if (scenario < 0.4) {
    // 40% - Test health endpoint (fast)
    testHealthEndpoint();
  } else if (scenario < 0.6) {
    // 20% - Test models endpoint (cacheable)
    testModelsEndpoint(params);
  } else if (scenario < 0.8) {
    // 20% - Test same chat completion (should hit cache after first request)
    testChatCompletion(chatPayload, params);
  } else {
    // 20% - Test varied chat completions (cache misses)
    const variation = chatPayloadVariations[Math.floor(Math.random() * chatPayloadVariations.length)];
    testChatCompletion(variation, params);
  }
  
  // Random sleep between 1-3 seconds to simulate real user behavior
  sleep(Math.random() * 2 + 1);
}

function testHealthEndpoint() {
  const response = http.get(`${BASE_URL}/health`);
  
  const success = check(response, {
    'health status is 200': (r) => r.status === 200,
    'health response time < 100ms': (r) => r.timings.duration < 100,
  });
  
  errorRate.add(!success);
}

function testModelsEndpoint(params) {
  const response = http.get(`${BASE_URL}/v1/models`, params);
  
  const success = check(response, {
    'models status is 200': (r) => r.status === 200,
    'models response time < 1000ms': (r) => r.timings.duration < 1000,
    'models response contains data': (r) => {
      try {
        const data = JSON.parse(r.body);
        return data.data && Array.isArray(data.data);
      } catch (e) {
        return false;
      }
    },
  });
  
  // Check for cache headers
  if (response.headers['X-Cache']) {
    console.log(`Models endpoint cache status: ${response.headers['X-Cache']}`);
  }
  
  errorRate.add(!success);
}

function testChatCompletion(payload, params) {
  const response = http.post(`${BASE_URL}/v1/chat/completions`, JSON.stringify(payload), params);
  
  const success = check(response, {
    'chat completion status is 200 or 429': (r) => r.status === 200 || r.status === 429, // 429 = rate limited
    'chat completion response time < 15s': (r) => r.timings.duration < 15000,
  });
  
  // Additional checks for successful responses
  if (response.status === 200) {
    const bodySuccess = check(response, {
      'chat response contains choices': (r) => {
        try {
          const data = JSON.parse(r.body);
          return data.choices && Array.isArray(data.choices) && data.choices.length > 0;
        } catch (e) {
          return false;
        }
      },
    });
    
    // Check for cache headers
    if (response.headers['X-Cache']) {
      console.log(`Chat completion cache status: ${response.headers['X-Cache']}`);
    }
    
    // Check for rate limit headers
    if (response.headers['X-RateLimit-Remaining']) {
      console.log(`Rate limit remaining: ${response.headers['X-RateLimit-Remaining']}`);
    }
    
    errorRate.add(!bodySuccess);
  } else if (response.status === 429) {
    // Rate limited - this is expected under high load
    console.log('Rate limit hit - this is expected behavior');
    check(response, {
      'rate limit response contains retry-after': (r) => r.headers['Retry-After'] !== undefined,
    });
  } else {
    // Unexpected error
    console.log(`Unexpected status: ${response.status}, body: ${response.body}`);
    errorRate.add(true);
  }
  
  errorRate.add(!success);
}

// Setup function - runs once before test starts
export function setup() {
  console.log('Starting load test against:', BASE_URL);
  
  // Test basic connectivity
  const healthResponse = http.get(`${BASE_URL}/health`);
  if (healthResponse.status !== 200) {
    throw new Error(`Health check failed: ${healthResponse.status}`);
  }
  
  console.log('Health check passed - starting load test');
  return {};
}

// Teardown function - runs once after test ends  
export function teardown(data) {
  console.log('Load test completed');
}

// Handle summary to format results nicely
export function handleSummary(data) {
  const summary = {
    'load-test-results.json': JSON.stringify(data, null, 2),
  };
  
  // Console summary
  console.log('\n=== LOAD TEST SUMMARY ===');
  console.log(`Total requests: ${data.metrics.http_reqs.values.count}`);
  console.log(`Failed requests: ${data.metrics.http_req_failed.values.rate * 100}%`);
  console.log(`Average response time: ${data.metrics.http_req_duration.values.avg}ms`);
  console.log(`95th percentile: ${data.metrics['http_req_duration{expected_response:true}'].values['p(95)']}ms`);
  console.log(`Max response time: ${data.metrics.http_req_duration.values.max}ms`);
  
  if (data.metrics.errors) {
    console.log(`Custom error rate: ${data.metrics.errors.values.rate * 100}%`);
  }
  
  return summary;
}