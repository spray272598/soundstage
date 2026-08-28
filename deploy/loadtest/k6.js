// k6 load test for SoundStage: SSE AI chat + danmaku ingest.
//
// Requires k6 (https://k6.io). Run:
//   k6 run -e BASE=http://localhost:8080 deploy/loadtest/k6.js
//
// The SSE endpoint streams until the agent run completes (server sends
// "event: done" and closes), so a normal GET captures the full turn and its
// latency. Danmaku is POSTed at a constant arrival rate.

import http from 'k6/http';
import { check } from 'k6';
import { Trend, Rate } from 'k6/metrics';

const BASE = __ENV.BASE || 'http://localhost:8080';

const sseDuration = new Trend('sse_duration');
const danmakuErrors = new Rate('danmaku_errors');

export const options = {
  scenarios: {
    sse_chat: {
      executor: 'constant-vus',
      vus: 20,
      duration: '1m',
      exec: 'chat',
    },
    danmaku: {
      executor: 'constant-arrival-rate',
      rate: 200,
      timeUnit: '1s',
      duration: '1m',
      preAllocatedVUs: 50,
      exec: 'danmaku',
    },
  },
  thresholds: {
    sse_duration: ['p(95)<5000'],
    danmaku_errors: ['rate<0.01'],
  },
};

// Create one room up-front and share its id across VUs.
export function setup() {
  const res = http.post(
    `${BASE}/api/v1/rooms`,
    JSON.stringify({ anchor_id: 'seeder', title: 'k6-loadtest' }),
    { headers: { 'Content-Type': 'application/json' } }
  );
  return { roomId: res.json('id') || 'room-loadtest' };
}

export function chat(data) {
  const roomId = data.roomId;
  const url = `${BASE}/rooms/${roomId}/ai/chat?user_id=vu-${__VU}&message=${encodeURIComponent(
    '房间现在多少人？礼物榜第一是谁？'
  )}`;
  const res = http.get(url, { responseType: 'text' });
  sseDuration.add(res.timings.duration);
  check(res, {
    'sse status 200': (r) => r.status === 200,
    'sse reached done': (r) => r.body && r.body.includes('event: done'),
  });
}

export function danmaku(data) {
  const roomId = data.roomId;
  const res = http.post(
    `${BASE}/rooms/${roomId}/danmaku`,
    JSON.stringify({ user_id: `vu-${__VU}`, text: '加油主播！' }),
    { headers: { 'Content-Type': 'application/json' } }
  );
  danmakuErrors.add(res.status !== 200);
  check(res, { 'danmaku status 200': (r) => r.status === 200 });
}
