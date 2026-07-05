import http from 'k6/http';
import { check } from 'k6';
import { Counter, Trend } from 'k6/metrics';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8081';

const lockedReqs = new Counter('locked_reqs');
const unlockedReqs = new Counter('unlocked_reqs');
const lockedDuration = new Trend('locked_duration_ms');
const unlockedDuration = new Trend('unlocked_duration_ms');
const lockWaitMs = new Trend('tp_lock_wait_ms');
const conflictCount = new Counter('tp_conflict');

export const options = {
  scenarios: {
    locked_test: {
      executor: 'constant-vus',
      vus: 20,
      duration: '10s',
      exec: 'lockedTest',
    },
    unlocked_test: {
      executor: 'constant-vus',
      vus: 20,
      duration: '10s',
      exec: 'unlockedTest',
      startTime: '15s',
    },
  },
};

function makePayload() {
  return JSON.stringify({
    transaction_date: new Date().toISOString(),
    transaction_value: '50.00',
    card_brand: 'mastercard',
    reason_code: '4837',
  });
}

export function lockedTest() {
  const start = Date.now();
  const res = http.post(`${BASE_URL}/api/v1/chargebacks`, makePayload(), {
    headers: { 'Content-Type': 'application/json' },
  });
  const elapsed = Date.now() - start;

  lockedReqs.add(1);
  lockedDuration.add(elapsed);

  if (res.status === 409) conflictCount.add(1);

  const waitMs = res.headers['X-Lock-Wait-Ms'];
  if (waitMs) lockWaitMs.add(parseFloat(waitMs));

  check(res, {
    'status is 201 or 409': (r) => r.status === 201 || r.status === 409,
    'has lock headers': (r) => r.headers['X-Lock-Acquired'] !== undefined,
  });
}

export function unlockedTest() {
  const start = Date.now();
  const res = http.post(`${BASE_URL}/api/v1/chargebacks/nolock`, makePayload(), {
    headers: { 'Content-Type': 'application/json' },
  });
  const elapsed = Date.now() - start;

  unlockedReqs.add(1);
  unlockedDuration.add(elapsed);

  check(res, {
    'status is 201': (r) => r.status === 201,
    'no lock headers': (r) => r.headers['X-Lock-Acquired'] === undefined,
  });
}

function safe(v, decimals = 2) {
  if (v === undefined || v === null || isNaN(v)) return 'N/A';
  return Number(v).toFixed(decimals);
}

export function handleSummary(data) {
  const dash = '═'.repeat(60);
  const dash2 = '─'.repeat(60);

  const lockedCount = data.metrics.locked_reqs?.values?.count || 0;
  const unlockedCount = data.metrics.unlocked_reqs?.values?.count || 0;
  const lockedRate = data.metrics.locked_reqs?.values?.rate || 0;
  const unlockedRate = data.metrics.unlocked_reqs?.values?.rate || 0;
  const lockedAvg = data.metrics.locked_duration_ms?.values?.avg || 0;
  const unlockedAvg = data.metrics.unlocked_duration_ms?.values?.avg || 0;
  const lockedP95 = data.metrics.locked_duration_ms?.values?.['p(95)'] || 0;
  const unlockedP95 = data.metrics.unlocked_duration_ms?.values?.['p(95)'] || 0;
  const conflicts = data.metrics.tp_conflict?.values?.count || 0;
  const waitAvg = data.metrics.tp_lock_wait_ms?.values?.avg || 0;
  const waitMax = data.metrics.tp_lock_wait_ms?.values?.max || 0;

  const ratio = unlockedRate > 0 && lockedRate > 0
    ? (unlockedRate / lockedRate).toFixed(2)
    : 'N/A';

  const output = `
${dash}
   COMPARAÇÃO DE THROUGHPUT: COM vs SEM LOCK 2PL
   (20 VUs constantes por 10s cada cenário)
${dash}

  ┌─────────────────────────┬──────────────┬──────────────┐
  │ Métrica                 │ COM LOCK 2PL │ SEM LOCK    │
  ├─────────────────────────┼──────────────┼──────────────┤
  │ Requests totais         │ ${String(lockedCount).padStart(10)}  │ ${String(unlockedCount).padStart(10)}  │
  │ Throughput (req/s)      │ ${safe(lockedRate).padStart(10)}  │ ${safe(unlockedRate).padStart(10)}  │
  │ Latência média (ms)     │ ${safe(lockedAvg).padStart(10)}  │ ${safe(unlockedAvg).padStart(10)}  │
  │ Latência p95 (ms)       │ ${safe(lockedP95).padStart(10)}  │ ${safe(unlockedP95).padStart(10)}  │
  │ 409 Conflict             │ ${String(conflicts).padStart(10)}  │ ${String(0).padStart(10)}  │
  └─────────────────────────┴──────────────┴──────────────┘

${dash2}

  LOCK WAIT (apenas no endpoint COM lock):
${dash2}
    Wait médio:  ${safe(waitAvg)} ms
    Wait máximo: ${safe(waitMax)} ms

${dash2}

  ANÁLISE:
${dash2}
    • SEM LOCK processou ${ratio}x mais requests por segundo
    • COM LOCK: ${conflicts} requests bloqueadas (409 Conflict)
    • COM LOCK: latência ${lockedAvg > unlockedAvg ? 'MAIOR' : 'menor'} devido à serialização do 2PL
    • SEM LOCK: 0 conflitos, mas sem garantia de consistência
${dash2}

  PROVA DO 2PL:
${dash2}
    ${conflicts > 0 ? `✅ ${conflicts} requests bloqueadas pelo lock (409)` : '⚠️  Nenhuma block - aumente VUs'}
    ${waitAvg > 0 ? `✅ Lock wait médio = ${safe(waitAvg)}ms (serialização ativa)` : '⚠️  Sem wait - pouca contenção'}
    ${lockedRate < unlockedRate ? '✅ Throughput menor com lock = serialização confirmada' : '⚠️  Throughput similar'}
${dash}
`;

  return { stdout: output };
}