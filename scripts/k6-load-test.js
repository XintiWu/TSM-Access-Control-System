import http from 'k6/http';
import { check } from 'k6';
import { Trend } from 'k6/metrics';

// Custom metrics to track latency SLA
const fastPathDuration = new Trend('fast_path_duration');
const slowPathDuration = new Trend('slow_path_duration');

export const options = {
  scenarios: {
    // Fast path: target 50 RPS (Limit is 100 RPS)
    fast_path: {
      executor: 'constant-arrival-rate',
      rate: 50,
      timeUnit: '1s',
      duration: '10s',
      preAllocatedVUs: 2,
      maxVUs: 10,
      exec: 'testFastPath',
    },
    // Slow path: target 5 RPS (Limit is 50 RPS)
    slow_path: {
      executor: 'constant-arrival-rate',
      rate: 5,
      timeUnit: '1s',
      duration: '10s',
      preAllocatedVUs: 2,
      maxVUs: 5,
      exec: 'testSlowPath',
    },
  },
  thresholds: {
    // SLA assertions:
    // access-api (Fast Path) must have p99 latency < 50ms.
    fast_path_duration: ['p(99)<50'],
    // report-api (Slow Path) must have p95 latency < 200ms.
    slow_path_duration: ['p(95)<200'],
  },
};

const API_URL = __ENV.API_URL || 'http://localhost:8080';
const REPORT_URL = __ENV.REPORT_URL || 'http://localhost:8082';
const API_KEY = __ENV.API_KEY || 'dev-api-key-2026';
const MANAGER = 'cccccccc-cccc-cccc-cccc-cccccccccccc';
const ORG = 'a0000000-0000-0000-0000-000000000003';

export function testFastPath() {
  const url = `${API_URL}/access/swipe`;
  const payload = JSON.stringify({
    userId: '22222222-2222-2222-2222-222222222222',
    doorId: '11111111-1111-1111-1111-111111111111',
    direction: 'IN',
    cardUid: 'CARD001',
    timestamp: new Date().toISOString(),
  });
  const params = {
    headers: { 
      'Content-Type': 'application/json',
      'X-API-Key': API_KEY
    },
  };
  const res = http.post(url, payload, params);
  
  fastPathDuration.add(res.timings.duration);
  check(res, {
    'fast_path status is 200': (r) => r.status === 200,
  });
}

export function testSlowPath() {
  const today = new Date().toISOString().split('T')[0];
  const firstDayOfMonth = new Date(new Date().getFullYear(), new Date().getMonth(), 1).toISOString().split('T')[0];
  const url = `${REPORT_URL}/reports/department?orgUnitId=${ORG}&startDate=${firstDayOfMonth}&endDate=${today}&granularity=daily`;
  const params = {
    headers: { 
      'X-User-ID': MANAGER,
      'X-API-Key': API_KEY
    },
  };
  const res = http.get(url, params);
  
  slowPathDuration.add(res.timings.duration);
  check(res, {
    'slow_path status is 200': (r) => r.status === 200,
  });
}
