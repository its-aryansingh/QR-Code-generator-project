import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend, Rate } from 'k6/metrics';

const redirectDuration = new Trend('redirect_duration_ms');
const successRate = new Rate('successful_redirects');

export const options = {
  stages: [
    { duration: '30s', target: 50 },  // Ramp-up to 50 VUs
    { duration: '1m', target: 200 },   // Scale to 200 VUs
    { duration: '2m', target: 500 },   // Peak stress at 500 VUs
    { duration: '30s', target: 0 },    // Ramp-down
  ],
  thresholds: {
    'http_req_duration': ['p(95)<50', 'p(99)<100'], // p95 sub-50ms, p99 sub-100ms
    'successful_redirects': ['rate>0.999'],          // 99.9% success rate
  },
};

const BASE_URL = __ENV.TARGET_URL || 'http://localhost:8080';
const TEST_CODES = ['ABC1234', 'XYZ7890', 'DEMO999', 'LAUNCH1'];

export default function () {
  const shortCode = TEST_CODES[Math.floor(Math.random() * TEST_CODES.length)];
  const url = `${BASE_URL}/${shortCode}`;

  const res = http.get(url, {
    redirects: 0, // Do not follow redirect; we want to measure the redirect engine itself
    headers: {
      'User-Agent': 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15',
      'Accept-Language': 'en-US,en;q=0.9',
    },
  });

  const isRedirect = res.status === 302 || res.status === 307;
  const hasLocation = Boolean(res.headers['Location'] || res.headers['location']);

  successRate.add(isRedirect && hasLocation);
  redirectDuration.add(res.timings.duration);

  check(res, {
    'is redirect (302/307)': (r) => r.status === 302 || r.status === 307,
    'has valid location header': () => hasLocation,
  });

  sleep(0.05); // 50ms pacing between requests per VU
}
