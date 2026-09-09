import http from 'k6/http';
import { check, sleep } from 'k6';

// Shaping-mode benchmark: confirms the gateway rejects fast (429) without
// touching the backend, and measures gateway-only overhead while throttling.
// Flip the tenant to shaping mode via POST /admin/mode before running this.

export const options = {
  scenarios: {
    steady_load: {
      executor: 'constant-vus',
      vus: 20,
      duration: '30s',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<50'], // shaping should reject near-instantly
    // no http_req_failed threshold here -- 429 is the expected, correct outcome
  },
};

const BASE_URL = 'http://localhost:8080';

export default function () {
  const res = http.get(`${BASE_URL}/`, {
    headers: { Host: 'localhost' },
  });

  check(res, {
    'status is 429 (throttled as expected)': (r) => r.status === 429,
  });

  sleep(0.1);
}