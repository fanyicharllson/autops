import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend } from 'k6/metrics';

// Measures end-to-end virtual-queue wait under a concurrent spike:
// enqueue → poll /queue/status until released → redeem through the proxy.
//
// RELEASE_INTERVAL_SECONDS / RELEASE_BATCH_SIZE (currently 5s / 5) directly
// determine how fast tickets drain, so observed waits should roughly match:
//   expected_wait ≈ (concurrent_queue_size / RELEASE_BATCH_SIZE) * RELEASE_INTERVAL_SECONDS
//
// Prerequisite: tenant "localhost" is already in shaping mode (POST /admin/mode).

const queueWait = new Trend('queue_wait_seconds', true);

export const options = {
  scenarios: {
    spike: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '10s', target: 50 }, // ramp up
        { duration: '30s', target: 50 }, // hold spike
        { duration: '10s', target: 0 }, // ramp down
      ],
      gracefulRampDown: '10s',
    },
  },
};

const BASE_URL = 'http://localhost:8080';
const TENANT = 'localhost';
const POLL_INTERVAL_S = 1;
const MAX_WAIT_S = 60;

export default function () {
  const enqueueRes = http.get(`${BASE_URL}/`, {
    headers: { Host: TENANT },
  });

  let queueId = null;
  try {
    const body = enqueueRes.json();
    queueId = body && body.queue_id;
  } catch (_) {
    queueId = null;
  }

  const enqueued = check(enqueueRes, {
    'enqueue returned queue_id': () => !!queueId,
  });
  if (!enqueued) {
    return;
  }

  const started = Date.now();
  let released = false;

  while ((Date.now() - started) / 1000 < MAX_WAIT_S) {
    const statusRes = http.get(
      `${BASE_URL}/queue/status?tenant=${TENANT}&queue_id=${queueId}`,
    );

    let statusBody = null;
    try {
      statusBody = statusRes.json();
    } catch (_) {
      statusBody = null;
    }

    if (statusBody && statusBody.released === true) {
      released = true;
      break;
    }

    sleep(POLL_INTERVAL_S);
  }

  const waitedOk = check(null, {
    'released within 60s': () => released,
  });
  if (!waitedOk) {
    return;
  }

  queueWait.add((Date.now() - started) / 1000);

  const redeemRes = http.get(`${BASE_URL}/?queue_id=${queueId}`, {
    headers: { Host: TENANT },
  });
  check(redeemRes, {
    'redeem reached backend (200)': (r) => r.status === 200,
  });
}
